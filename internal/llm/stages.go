package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
)

func DefaultModelConfig(cfg config.Config, workspaceID string) domain.ModelConfig {
	return domain.ModelConfig{
		WorkspaceID:        workspaceID,
		ModelAProvider:     cfg.ModelAProvider,
		ModelAName:         cfg.ModelAName,
		ModelBProvider:     cfg.ModelBProvider,
		ModelBName:         cfg.ModelBName,
		ModelCProvider:     cfg.ModelCProvider,
		ModelCName:         cfg.ModelCName,
		Temperature:        0.3,
		MaxTokens:          1200,
		ActiveSlot:         "A",
		InvestigatorPrompt: SysInvestigator,
		ChallengerPrompt:   SysChallenger,
		JudgePrompt:        SysJudge,
	}
}

func (r *Router) Classify(ctx context.Context, cfg domain.ModelConfig, invCtx domain.InvestigationContext) (*domain.FastResult, error) {
	user := classifyUser(invCtx.Prompt, strings.Join(invCtx.FileNames, ", "))
	var dto struct {
		Summary      string  `json:"summary"`
		IncidentType string  `json:"incidentType"`
		Confidence   float64 `json:"confidence"`
	}
	if err := r.completeParsed(ctx, cfg, "A", "", SysFast, user, &dto); err != nil {
		return nil, err
	}
	if dto.IncidentType == "" {
		dto.IncidentType = "unknown"
	}
	return &domain.FastResult{Summary: dto.Summary, IncidentType: dto.IncidentType, Confidence: clamp01(dto.Confidence)}, nil
}

func (r *Router) Analyze(ctx context.Context, cfg domain.ModelConfig, invCtx domain.InvestigationContext) (*domain.DeepResult, error) {
	user := analyzeUser(invCtx.Prompt, PackFiles(invCtx.FileContents))
	var dto struct {
		RootCause    string            `json:"rootCause"`
		Confidence   float64           `json:"confidence"`
		Evidence     []domain.Evidence `json:"evidence"`
		SuggestedFix string            `json:"suggestedFix"`
	}
	if err := r.completeParsed(ctx, cfg, "B", "", SysDeep, user, &dto); err != nil {
		return nil, err
	}
	return &domain.DeepResult{RootCause: dto.RootCause, Confidence: clamp01(dto.Confidence), Evidence: dto.Evidence, SuggestedFix: dto.SuggestedFix}, nil
}

// CouncilProgress receives a partial Council result as Investigator / Challenger finish.
type CouncilProgress func(*domain.CouncilResult)

func (r *Router) RunCouncil(ctx context.Context, cfg domain.ModelConfig, invCtx domain.InvestigationContext, fast *domain.FastResult, deep *domain.DeepResult) (*domain.CouncilResult, error) {
	return r.RunCouncilProgress(ctx, cfg, invCtx, fast, deep, nil)
}

func (r *Router) RunCouncilProgress(ctx context.Context, cfg domain.ModelConfig, invCtx domain.InvestigationContext, fast *domain.FastResult, deep *domain.DeepResult, onPartial CouncilProgress) (*domain.CouncilResult, error) {
	chalBudget := r.cfg.CouncilContextChars
	if chalBudget <= 0 {
		chalBudget = CouncilBudgetChars
	}
	chalFiles := PackFilesBudget(invCtx.FileContents, chalBudget)
	available, _ := ListOllamaModels(ctx, r.cfg.OllamaBaseURL)
	route := r.routeCouncil(cfg, available)

	invSys := cfg.InvestigatorPrompt
	if invSys == "" {
		invSys = SysInvestigator
	}
	chalSys := cfg.ChallengerPrompt
	if chalSys == "" {
		chalSys = SysChallenger
	}
	judgeSys := cfg.JudgePrompt
	if judgeSys == "" {
		judgeSys = SysJudge
	}

	hypCfg := cfg
	if n := r.cfg.CouncilMaxTokens; n > 0 {
		hypCfg.MaxTokens = n
	} else if hypCfg.MaxTokens <= 0 || hypCfg.MaxTokens > 512 {
		hypCfg.MaxTokens = 512
	}

	agentTO := r.cfg.CouncilAgentTimeout
	if agentTO <= 0 {
		agentTO = 25 * time.Second
	}

	type hyp struct {
		out domain.CouncilModel
		err error
	}
	invCh := make(chan hyp, 1)
	chalCh := make(chan hyp, 1)

	var progMu sync.Mutex
	var invModel, chalModel *domain.CouncilModel
	emitPartial := func() {
		if onPartial == nil {
			return
		}
		res := emptyPartialCouncil()
		if invModel != nil {
			res.Models = append(res.Models, *invModel)
		}
		if chalModel != nil {
			res.Models = append(res.Models, *chalModel)
		}
		onPartial(res)
	}

	// Investigator first so a single Ollama GPU keeps the Deep model loaded,
	// then Challenger immediately — both run concurrently.
	go func() {
		var dto domain.CouncilModel
		agentCtx, cancel := context.WithTimeout(ctx, agentTO)
		defer cancel()
		err := r.completeParsed(agentCtx, hypCfg, route.InvestigatorSlot, "", invSys, investigatorUser(invCtx.Prompt, fast, deep, invCtx.FileContents), &dto)
		dto.Role = "investigator"
		dto.Model = route.InvestigatorName
		if err == nil {
			dto.Confidence = clamp01(dto.Confidence)
			progMu.Lock()
			invModel = &dto
			emitPartial()
			progMu.Unlock()
		}
		invCh <- hyp{out: dto, err: err}
	}()
	go func() {
		var dto domain.CouncilModel
		chalCfg := hypCfg
		override := ""
		if route.ChallengerSlot == "B" && route.ChallengerName != "" {
			override = route.ChallengerName
			chalCfg.ModelBName = route.ChallengerName
		}
		agentCtx, cancel := context.WithTimeout(ctx, agentTO)
		defer cancel()
		err := r.completeParsed(agentCtx, chalCfg, route.ChallengerSlot, override, chalSys, challengerUser(invCtx.Prompt, chalFiles, fast, deep), &dto)
		dto.Role = "challenger"
		dto.Model = route.ChallengerName
		if err == nil {
			dto.Confidence = clamp01(dto.Confidence)
			progMu.Lock()
			chalModel = &dto
			emitPartial()
			progMu.Unlock()
		}
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
	judgeUser := fmt.Sprintf("Investigator (%s): role=%s hypothesis=%s confidence=%.2f evidence=%d\nChallenger (%s): role=%s hypothesis=%s confidence=%.2f evidence=%d\nSameFamily=%v\n%s",
		inv.out.Model, inv.out.Role, inv.out.Hypothesis, inv.out.Confidence, len(inv.out.Evidence),
		chal.out.Model, chal.out.Role, chal.out.Hypothesis, chal.out.Confidence, len(chal.out.Evidence),
		route.SameFamily, judgeSchema())
	judgeCfg := cfg
	if n := r.cfg.CouncilMaxTokens; n > 0 && (judgeCfg.MaxTokens <= 0 || judgeCfg.MaxTokens > n+400) {
		judgeCfg.MaxTokens = n + 400
	}
	judgeCtx, cancel := context.WithTimeout(ctx, agentTO)
	defer cancel()
	if err := r.completeParsed(judgeCtx, judgeCfg, route.JudgeSlot, "", judgeSys, judgeUser, &dto); err != nil {
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
	res := &domain.CouncilResult{
		Models:        []domain.CouncilModel{inv.out, chal.out},
		Agreements:    dto.Agreements,
		Disagreements: dto.Disagreements,
		FinalJudgment: dto.FinalJudgment,
	}
	ApplyAggregation(res, route.SameFamily)
	return res, nil
}

func emptyPartialCouncil() *domain.CouncilResult {
	return &domain.CouncilResult{
		Models:        []domain.CouncilModel{},
		Agreements:    []string{},
		Disagreements: []domain.Disagreement{},
	}
}

type GeneratedDataset struct {
	FileName     string           `json:"fileName"`
	SchemaNote   string           `json:"schemaNote"`
	QualityNotes string           `json:"qualityNotes"`
	Confidence   float64          `json:"confidence"`
	Rows         []map[string]any `json:"rows"`
}

type EvalOutcome struct {
	Summary        string               `json:"summary"`
	Recommendation string               `json:"recommendation"`
	Confidence     float64              `json:"confidence"`
	Metrics        []domain.MetricDelta `json:"metrics"`
}

func (r *Router) Generate(ctx context.Context, cfg domain.ModelConfig, prompt, contextBlob string) (*GeneratedDataset, error) {
	user := fmt.Sprintf("Prompt: %s\n\nInvestigation context:\n%s\nSchema: {\"fileName\":\"fixed_dataset.jsonl\",\"schemaNote\":\"...\",\"qualityNotes\":\"...\",\"confidence\":0.0,\"rows\":[{\"field\":\"value\"}]}", prompt, contextBlob)
	var dto GeneratedDataset
	if err := r.completeParsed(ctx, cfg, "B", "", "You are Azula Generator. Produce a small synthetic JSONL dataset that reflects the recommended fix. 8-20 rows. "+jsonOnly, user, &dto); err != nil {
		return nil, err
	}
	if dto.FileName == "" {
		dto.FileName = "fixed_dataset.jsonl"
	}
	dto.Confidence = clamp01(dto.Confidence)
	return &dto, nil
}

func (r *Router) Evaluate(ctx context.Context, cfg domain.ModelConfig, prompt, original, candidate string) (*EvalOutcome, error) {
	user := fmt.Sprintf("Prompt: %s\n\nOriginal metrics/data (truncated):\n%s\n\nCandidate (truncated):\n%s\nSchema: {\"summary\":\"...\",\"recommendation\":\"adopt|reject|iterate\",\"confidence\":0.0,\"metrics\":[{\"name\":\"accuracy\",\"before\":0.0,\"after\":0.0,\"delta\":0.0}]}", prompt, original, candidate)
	var dto EvalOutcome
	if err := r.completeParsed(ctx, cfg, "A", "", "You are Azula Evaluator. Compare original vs fixed artifacts. Prefer evidence in the files. "+jsonOnly, user, &dto); err != nil {
		return nil, err
	}
	dto.Confidence = clamp01(dto.Confidence)
	if dto.Recommendation == "" {
		dto.Recommendation = "iterate"
	}
	return &dto, nil
}

func (r *Router) completeParsed(ctx context.Context, cfg domain.ModelConfig, slot, modelOverride, system, user string, dest any) error {
	var last error
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			if last != nil {
				return last
			}
			return err
		}
		sys := system
		if attempt == 1 {
			sys += " STRICT: output must be valid minified JSON starting with {."
		}
		text, err := r.completeJSON(ctx, cfg, slot, modelOverride, sys, user)
		if err != nil {
			last = err
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || ctx.Err() != nil {
				return err
			}
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

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
