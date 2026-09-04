package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
)

const jsonOnly = "Reply with a single JSON object only. No markdown, no commentary."

func DefaultModelConfig(cfg config.Config, workspaceID string) domain.ModelConfig {
	return domain.ModelConfig{
		WorkspaceID:      workspaceID,
		ModelAProvider:   cfg.ModelAProvider,
		ModelAName:       cfg.ModelAName,
		ModelBProvider:   cfg.ModelBProvider,
		ModelBName:       cfg.ModelBName,
		Temperature:      0.3,
		MaxTokens:        1200,
		ActiveSlot:       "A",
	}
}

func (r *Router) Classify(ctx context.Context, cfg domain.ModelConfig, invCtx domain.InvestigationContext) (*domain.FastResult, error) {
	user := fmt.Sprintf("Prompt: %s\nFiles: %s\nClassify the ML incident.\nSchema: {\"summary\":\"...\",\"incidentType\":\"schema_mismatch|memory_gpu|data_leakage|config_error|unknown\",\"confidence\":0.0}",
		invCtx.Prompt, strings.Join(invCtx.FileNames, ", "))
	var dto struct {
		Summary      string  `json:"summary"`
		IncidentType string  `json:"incidentType"`
		Confidence   float64 `json:"confidence"`
	}
	if err := r.completeParsed(ctx, cfg, "A", "You are Azula Fast: classify ML pipeline incidents quickly. "+jsonOnly, user, &dto); err != nil {
		return nil, err
	}
	if dto.IncidentType == "" {
		dto.IncidentType = "unknown"
	}
	return &domain.FastResult{Summary: dto.Summary, IncidentType: dto.IncidentType, Confidence: clamp01(dto.Confidence)}, nil
}

func (r *Router) Analyze(ctx context.Context, cfg domain.ModelConfig, invCtx domain.InvestigationContext) (*domain.DeepResult, error) {
	user := fmt.Sprintf("Prompt: %s\n\nProject files:\n%s\nFind the root cause with evidence.\nSchema: {\"rootCause\":\"...\",\"confidence\":0.0,\"evidence\":[{\"file\":\"name\",\"lines\":\"1-5\",\"excerpt\":\"...\"}],\"suggestedFix\":\"...\"}",
		invCtx.Prompt, formatFiles(invCtx.FileContents))
	var dto struct {
		RootCause    string            `json:"rootCause"`
		Confidence   float64           `json:"confidence"`
		Evidence     []domain.Evidence `json:"evidence"`
		SuggestedFix string            `json:"suggestedFix"`
	}
	if err := r.completeParsed(ctx, cfg, "B", "You are Azula Deep: investigate ML pipeline failures using file evidence. Every claim needs evidence. "+jsonOnly, user, &dto); err != nil {
		return nil, err
	}
	return &domain.DeepResult{RootCause: dto.RootCause, Confidence: clamp01(dto.Confidence), Evidence: dto.Evidence, SuggestedFix: dto.SuggestedFix}, nil
}

func (r *Router) RunCouncil(ctx context.Context, cfg domain.ModelConfig, invCtx domain.InvestigationContext, fast *domain.FastResult, deep *domain.DeepResult) (*domain.CouncilResult, error) {
	brief := fmt.Sprintf("Prompt: %s\nFast: %+v\nDeep: %+v\n\nFiles:\n%s", invCtx.Prompt, fast, deep, formatFiles(invCtx.FileContents))

	type hyp struct {
		out domain.CouncilModel
		err error
	}
	invCh := make(chan hyp, 1)
	chalCh := make(chan hyp, 1)

	invSys := cfg.InvestigatorPrompt
	if invSys == "" {
		invSys = "You are the Investigator. Build and defend the strongest root-cause hypothesis from the files. " + jsonOnly
	}
	chalSys := cfg.ChallengerPrompt
	if chalSys == "" {
		chalSys = "You are the Challenger. You MUST disagree or find a weakness in the obvious hypothesis. Propose an alternative root cause with evidence. " + jsonOnly
	}
	judgeSys := cfg.JudgePrompt
	if judgeSys == "" {
		judgeSys = "You are the Judge. Synthesize both hypotheses. Prefer evidence-backed causes. Include both agreements and disagreements. " + jsonOnly
	}

	go func() {
		var dto domain.CouncilModel
		err := r.completeParsed(ctx, cfg, "B", invSys, brief+"\nSchema: {\"role\":\"investigator\",\"hypothesis\":\"...\",\"confidence\":0.0,\"evidence\":[{\"file\":\"...\",\"lines\":\"...\",\"excerpt\":\"...\"}]}", &dto)
		dto.Role = "investigator"
		invCh <- hyp{out: dto, err: err}
	}()
	go func() {
		var dto domain.CouncilModel
		err := r.completeParsed(ctx, cfg, "B", chalSys, brief+"\nSchema: {\"role\":\"challenger\",\"hypothesis\":\"...\",\"confidence\":0.0,\"evidence\":[{\"file\":\"...\",\"lines\":\"...\",\"excerpt\":\"...\"}]}", &dto)
		dto.Role = "challenger"
		chalCh <- hyp{out: dto, err: err}
	}()

	inv := <-invCh
	chal := <-chalCh
	if inv.err != nil {
		return nil, fmt.Errorf("investigator: %w", inv.err)
	}
	if chal.err != nil {
		return nil, fmt.Errorf("challenger: %w", chal.err)
	}

	var dto struct {
		Agreements    []string              `json:"agreements"`
		Disagreements []domain.Disagreement `json:"disagreements"`
		FinalJudgment domain.FinalJudgment  `json:"finalJudgment"`
	}
	judgeUser := fmt.Sprintf("Investigator: %+v\nChallenger: %+v\nSchema: {\"agreements\":[\"...\"],\"disagreements\":[{\"topic\":\"...\",\"investigator\":\"...\",\"challenger\":\"...\"}],\"finalJudgment\":{\"mostLikelyCause\":\"...\",\"confidence\":0.0,\"recommendedAction\":\"...\"}}", inv.out, chal.out)
	if err := r.completeParsed(ctx, cfg, "A", judgeSys, judgeUser, &dto); err != nil {
		return nil, fmt.Errorf("judge: %w", err)
	}
	if dto.Agreements == nil {
		dto.Agreements = []string{}
	}
	if dto.Disagreements == nil {
		dto.Disagreements = []domain.Disagreement{}
	}
	dto.FinalJudgment.Confidence = clamp01(dto.FinalJudgment.Confidence)
	inv.out.Confidence = clamp01(inv.out.Confidence)
	chal.out.Confidence = clamp01(chal.out.Confidence)
	return &domain.CouncilResult{
		Models:        []domain.CouncilModel{inv.out, chal.out},
		Agreements:    dto.Agreements,
		Disagreements: dto.Disagreements,
		FinalJudgment: dto.FinalJudgment,
	}, nil
}

func (r *Router) completeParsed(ctx context.Context, cfg domain.ModelConfig, slot, system, user string, dest any) error {
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		sys := system
		if attempt == 1 {
			sys += " STRICT: output must be valid minified JSON starting with {."
		}
		text, err := r.CompleteJSON(ctx, cfg, slot, sys, user)
		if err != nil {
			last = err
			continue
		}
		if err := DecodeModelJSON(text, dest); err != nil {
			last = err
			continue
		}
		return nil
	}
	return last
}

func formatFiles(files map[string]string) string {
	var b strings.Builder
	for name, content := range files {
		b.WriteString("=== ")
		b.WriteString(name)
		b.WriteString(" ===\n")
		if len(content) > 8000 {
			content = content[:8000]
		}
		b.WriteString(content)
		b.WriteString("\n\n")
	}
	return b.String()
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
