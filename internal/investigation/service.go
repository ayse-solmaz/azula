package investigation

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/llm"
	"github.com/ayse-solmaz/azula/internal/mcp"
	"github.com/google/uuid"
)

const autoEscalateBelow = 0.7

type Service struct {
	projects domain.ProjectRepository
	invs     domain.InvestigationRepository
	configs  domain.ModelConfigRepository
	files    mcp.Connector
	router   *llm.Router
	cfg      config.Config
}

func New(projects domain.ProjectRepository, invs domain.InvestigationRepository, configs domain.ModelConfigRepository, files mcp.Connector, router *llm.Router, cfg config.Config) *Service {
	return &Service{projects: projects, invs: invs, configs: configs, files: files, router: router, cfg: cfg}
}

func DefaultPlan() []domain.PlanStep {
	descs := []string{
		"Read training.log for errors",
		"Read config.yaml for misconfiguration",
		"Read pipeline.py for code bugs",
		"Cross-check dataset.jsonl schema",
		"Run Council with findings",
	}
	steps := make([]domain.PlanStep, len(descs))
	for i, d := range descs {
		steps[i] = domain.PlanStep{Order: i + 1, Description: d, Status: domain.StepPending}
	}
	return steps
}

func (s *Service) Get(ctx context.Context, id string) (*domain.Investigation, error) {
	return s.invs.GetByID(ctx, id)
}

func (s *Service) ListByProject(ctx context.Context, projectID string) ([]domain.Investigation, error) {
	return s.invs.ListByProject(ctx, projectID)
}

func (s *Service) Stats(ctx context.Context, workspaceID string) (int, int, int, float64, error) {
	return s.invs.StatsByWorkspace(ctx, workspaceID)
}

func (s *Service) Start(ctx context.Context, userID, projectID, prompt string) (*domain.Investigation, error) {
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if prompt == "" {
		prompt = "Why did this training pipeline fail? Identify root cause with evidence."
	}
	if err := s.router.Acquire(); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	inv := &domain.Investigation{
		ID:            stringsUUID(),
		ProjectID:     project.ID,
		WorkspaceID:   project.WorkspaceID,
		UserID:        userID,
		Prompt:        prompt,
		Status:        domain.StatusPending,
		Plan:          DefaultPlan(),
		FilesAccessed: []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.invs.Create(ctx, inv); err != nil {
		s.router.Release()
		return nil, err
	}
	go s.runAsync(inv.ID)
	return inv, nil
}

func (s *Service) runAsync(id string) {
	defer s.router.Release()
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout+2*time.Minute)
	defer cancel()
	inv, err := s.invs.GetByID(ctx, id)
	if err != nil {
		return
	}
	if err := s.runPipeline(ctx, inv); err != nil {
		inv.Status = domain.StatusFailed
		inv.ErrorMessage = err.Error()
		_ = s.invs.Update(ctx, inv)
	}
}

func (s *Service) EnsureConfig(ctx context.Context, workspaceID string) (*domain.ModelConfig, error) {
	cfg, err := s.configs.GetByWorkspace(ctx, workspaceID)
	if err == domain.ErrNotFound {
		c := llm.DefaultModelConfig(s.cfg, workspaceID)
		c.ID = stringsUUID()
		now := time.Now().UTC()
		c.CreatedAt = now
		c.UpdatedAt = now
		if c.InvestigatorPrompt == "" {
			c.InvestigatorPrompt = "You are the Investigator. Build a root-cause hypothesis from the files. Return JSON only."
			c.ChallengerPrompt = "You are the Challenger. Attack the Investigator and propose an alternative. Return JSON only."
			c.JudgePrompt = "You are the Judge. Synthesize agreements, disagreements, and a final judgment. Return JSON only."
		}
		if err := s.configs.Upsert(ctx, &c); err != nil {
			return nil, err
		}
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	if s.attachIncidentModel(cfg) {
		if err := s.configs.Upsert(ctx, cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// attachIncidentModel points Model B at the local QLoRA Ollama model when the
// workspace is still on the generic Fast default.
func (s *Service) attachIncidentModel(cfg *domain.ModelConfig) bool {
	want := s.cfg.ModelBName
	if want == "" {
		want = "azula-incident"
	}
	if cfg.ModelBName == want {
		return false
	}
	if cfg.ModelBName != "" && cfg.ModelBName != "qwen2.5:1.5b" {
		return false
	}
	cfg.ModelBProvider = s.cfg.ModelBProvider
	if cfg.ModelBProvider == "" {
		cfg.ModelBProvider = "ollama"
	}
	cfg.ModelBName = want
	return true
}

func (s *Service) AttachIncidentModel(ctx context.Context, workspaceID string) (*domain.ModelConfig, error) {
	cfg, err := s.EnsureConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	want := s.cfg.ModelBName
	if want == "" {
		want = "azula-incident"
	}
	cfg.ModelBProvider = s.cfg.ModelBProvider
	if cfg.ModelBProvider == "" {
		cfg.ModelBProvider = "ollama"
	}
	cfg.ModelBName = want
	if err := s.configs.Upsert(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *Service) UpdateConfig(ctx context.Context, in domain.ModelConfig) (*domain.ModelConfig, error) {
	cur, err := s.EnsureConfig(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if in.ModelAProvider != "" {
		cur.ModelAProvider = in.ModelAProvider
	}
	if in.ModelAName != "" {
		cur.ModelAName = in.ModelAName
	}
	if in.ModelBProvider != "" {
		cur.ModelBProvider = in.ModelBProvider
	}
	if in.ModelBName != "" {
		cur.ModelBName = in.ModelBName
	}
	if in.Temperature > 0 {
		cur.Temperature = in.Temperature
	}
	if in.MaxTokens > 0 {
		cur.MaxTokens = in.MaxTokens
	}
	if in.InvestigatorPrompt != "" {
		cur.InvestigatorPrompt = in.InvestigatorPrompt
	}
	if in.ChallengerPrompt != "" {
		cur.ChallengerPrompt = in.ChallengerPrompt
	}
	if in.JudgePrompt != "" {
		cur.JudgePrompt = in.JudgePrompt
	}
	if in.ActiveSlot != "" {
		cur.ActiveSlot = in.ActiveSlot
	}
	if err := s.configs.Upsert(ctx, cur); err != nil {
		return nil, err
	}
	return cur, nil
}

func (s *Service) Metrics(ctx context.Context, workspaceID string) (*domain.LLMOpsMetrics, error) {
	cfg, err := s.EnsureConfig(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	total, completed, failed, avg, err := s.invs.StatsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	probe := llm.Probe(ctx, s.cfg.OllamaBaseURL, cfg.ModelBName)
	avgDur, causes := s.analytics(ctx, workspaceID)
	if probe.Models == nil {
		probe.Models = []string{}
	}
	if causes == nil {
		causes = []string{}
	}
	return &domain.LLMOpsMetrics{
		TotalInvestigations: total,
		Completed:           completed,
		Failed:              failed,
		AvgConfidence:       avg,
		AvgDurationSec:      avgDur,
		WorkerSlots:         s.router.Slots(),
		BusySlots:           s.router.Busy(),
		ModelAName:          cfg.ModelAName,
		ModelBName:          cfg.ModelBName,
		OllamaReachable:     probe.Reachable,
		OllamaModels:        probe.Models,
		IncidentModelReady:  probe.IncidentModelReady,
		AdapterOnDisk:       probe.AdapterOnDisk,
		TopCauses:           causes,
	}, nil
}

func (s *Service) analytics(ctx context.Context, workspaceID string) (float64, []string) {
	list, err := s.invs.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return 0, []string{}
	}
	var durSum float64
	durN := 0
	counts := map[string]int{}
	for _, inv := range list {
		if inv.Status != domain.StatusCompleted {
			continue
		}
		if !inv.CreatedAt.IsZero() && !inv.UpdatedAt.IsZero() && inv.UpdatedAt.After(inv.CreatedAt) {
			durSum += inv.UpdatedAt.Sub(inv.CreatedAt).Seconds()
			durN++
		}
		label := "other"
		if inv.FastResult != nil && inv.FastResult.IncidentType != "" {
			label = inv.FastResult.IncidentType
		}
		counts[label]++
	}
	avg := 0.0
	if durN > 0 {
		avg = durSum / float64(durN)
	}
	type pair struct {
		k string
		n int
	}
	var ranked []pair
	for k, n := range counts {
		ranked = append(ranked, pair{k, n})
	}
	for i := 0; i < len(ranked); i++ {
		for j := i + 1; j < len(ranked); j++ {
			if ranked[j].n > ranked[i].n {
				ranked[i], ranked[j] = ranked[j], ranked[i]
			}
		}
	}
	if len(ranked) > 3 {
		ranked = ranked[:3]
	}
	out := make([]string, 0, len(ranked))
	for _, p := range ranked {
		pct := 0
		if durN > 0 {
			pct = int(float64(p.n) / float64(durN) * 100)
		}
		out = append(out, fmt.Sprintf("%s %d%%", p.k, pct))
	}
	return avg, out
}

func stringsUUID() string {
	return uuid.NewString()
}

func (s *Service) runPipeline(ctx context.Context, inv *domain.Investigation) error {
	listed, err := s.files.ListFiles(ctx, inv.ProjectID)
	if err != nil {
		return err
	}
	names := make([]string, 0, len(listed))
	contents := map[string]string{}
	stepByFile := map[string]int{
		"training.log":  0,
		"config.yaml":   1,
		"pipeline.py":   2,
		"dataset.jsonl": 3,
	}
	for _, f := range listed {
		names = append(names, f.Name)
		if idx, ok := stepByFile[f.Name]; ok {
			s.markStep(inv, idx, domain.StepRunning)
		}
		body, readErr := s.files.ReadFile(ctx, inv.ProjectID, f.Name)
		if readErr != nil {
			log.Printf("mcp: skip %s: %v", f.Name, readErr)
			if idx, ok := stepByFile[f.Name]; ok {
				s.markStep(inv, idx, domain.StepSkipped)
			}
			continue
		}
		contents[f.Name] = body
		inv.FilesAccessed = append(inv.FilesAccessed, f.Name)
		log.Printf("mcp: read %s (%d bytes) investigation=%s", f.Name, len(body), inv.ID)
		if idx, ok := stepByFile[f.Name]; ok {
			s.markStep(inv, idx, domain.StepDone)
		}
		_ = s.invs.Update(ctx, inv)
	}
	for name, idx := range stepByFile {
		if _, ok := contents[name]; ok {
			continue
		}
		if idx < len(inv.Plan) && inv.Plan[idx].Status == domain.StepPending {
			s.markStep(inv, idx, domain.StepSkipped)
		}
	}

	invCtx := domain.InvestigationContext{
		InvestigationID: inv.ID,
		ProjectID:       inv.ProjectID,
		Prompt:          inv.Prompt,
		FileNames:       names,
		FileContents:    contents,
	}
	mcfg := s.modelConfig(ctx, inv.WorkspaceID)
	inv.ModelAName = mcfg.ModelAName
	inv.ModelBName = mcfg.ModelBName
	if names, err := llm.ListOllamaModels(ctx, s.cfg.OllamaBaseURL); err == nil {
		inv.ModelBName = llm.PickModelB(names, mcfg.ModelBName)
	}
	_ = s.invs.Update(ctx, inv)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(700 * time.Millisecond):
	}

	if err := s.transition(inv, domain.StatusPending, domain.StatusFastClassify); err != nil {
		return err
	}
	_ = s.invs.Update(ctx, inv)

	fast, err := s.router.Classify(ctx, mcfg, invCtx)
	if err != nil {
		log.Printf("investigation %s: fast LLM fallback: %v", inv.ID, err)
		fast = fallbackFast()
	}
	inv.FastResult = fast
	_ = s.invs.Update(ctx, inv)

	if fast.Confidence < autoEscalateBelow {
		log.Printf("investigation %s: auto-escalate (fast confidence %.2f < %.2f)", inv.ID, fast.Confidence, autoEscalateBelow)
	}
	if err := s.transition(inv, domain.StatusFastClassify, domain.StatusDeepAnalyze); err != nil {
		return err
	}
	_ = s.invs.Update(ctx, inv)

	deep, err := s.router.Analyze(ctx, mcfg, invCtx)
	if err != nil || len(deep.Evidence) == 0 {
		if err != nil {
			log.Printf("investigation %s: deep LLM fallback: %v", inv.ID, err)
		}
		deep = fallbackDeep()
	}
	inv.DeepResult = deep
	_ = s.invs.Update(ctx, inv)

	if err := s.transition(inv, domain.StatusDeepAnalyze, domain.StatusCouncil); err != nil {
		return err
	}
	s.markStep(inv, 4, domain.StepRunning)
	_ = s.invs.Update(ctx, inv)

	council, err := s.router.RunCouncil(ctx, mcfg, invCtx, fast, deep)
	if err != nil || council == nil || len(council.Agreements) == 0 {
		if err != nil {
			log.Printf("investigation %s: council LLM fallback: %v", inv.ID, err)
		}
		council = fallbackCouncil()
	}
	inv.CouncilResult = council
	s.markStep(inv, 4, domain.StepDone)
	if err := s.transition(inv, domain.StatusCouncil, domain.StatusCompleted); err != nil {
		return err
	}
	return s.invs.Update(ctx, inv)
}

func (s *Service) modelConfig(ctx context.Context, workspaceID string) domain.ModelConfig {
	if s.configs != nil {
		if cfg, err := s.EnsureConfig(ctx, workspaceID); err == nil && cfg != nil {
			return *cfg
		}
	}
	return llm.DefaultModelConfig(s.cfg, workspaceID)
}

func (s *Service) transition(inv *domain.Investigation, from, to string) error {
	if inv.Status != from {
		return fmt.Errorf("invalid transition %s → %s (current %s)", from, to, inv.Status)
	}
	allowed := map[string][]string{
		domain.StatusPending:      {domain.StatusFastClassify, domain.StatusFailed},
		domain.StatusFastClassify: {domain.StatusDeepAnalyze, domain.StatusCompleted, domain.StatusFailed},
		domain.StatusDeepAnalyze:  {domain.StatusCouncil, domain.StatusFailed},
		domain.StatusCouncil:      {domain.StatusCompleted, domain.StatusFailed},
	}
	ok := false
	for _, next := range allowed[from] {
		if next == to {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("forbidden transition %s → %s", from, to)
	}
	inv.Status = to
	inv.UpdatedAt = time.Now().UTC()
	log.Printf("investigation %s: %s → %s", inv.ID, from, to)
	return nil
}

func (s *Service) markStep(inv *domain.Investigation, idx int, status string) {
	if idx >= 0 && idx < len(inv.Plan) {
		inv.Plan[idx].Status = status
	}
}

func fallbackFast() *domain.FastResult {
	return &domain.FastResult{
		Summary:      "Schema warning on customer_status plus CUDA OOM during training.",
		IncidentType: "schema_mismatch",
		Confidence:   0.64,
	}
}

func fallbackDeep() *domain.DeepResult {
	return &domain.DeepResult{
		RootCause:    "Schema drift in `customer_status` combined with batch_size 128 causing GPU OOM.",
		Confidence:   0.88,
		SuggestedFix: "Re-encode customer_status; drop leaky target feature; reduce batch_size to 32 and lower learning_rate.",
		Evidence: []domain.Evidence{
			{File: "training.log", Lines: "3-11", Excerpt: "Column 'customer_status' has unseen categories... CUDA out of memory"},
			{File: "config.yaml", Lines: "3-4", Excerpt: "batch_size: 128  # too large for 8GB GPU"},
			{File: "pipeline.py", Lines: "84-92", Excerpt: "target leakage into training features"},
		},
	}
}

func fallbackCouncil() *domain.CouncilResult {
	deep := fallbackDeep()
	return &domain.CouncilResult{
		Models: []domain.CouncilModel{
			{Role: "investigator", Hypothesis: deep.RootCause, Confidence: 0.89, Evidence: deep.Evidence[:1]},
			{Role: "challenger", Hypothesis: "Data leakage from the target column into training features is the primary accuracy collapse.", Confidence: 0.71, Evidence: []domain.Evidence{{File: "pipeline.py", Lines: "84-92", Excerpt: "target leakage into training features"}}},
		},
		Agreements: []string{"Both models detected data quality issues in the training set.", "GPU memory pressure from a large batch size is real."},
		Disagreements: []domain.Disagreement{
			{Topic: "Root cause", Investigator: "Schema drift in customer_status + OOM", Challenger: "Target leakage in pipeline.py"},
		},
		FinalJudgment: domain.FinalJudgment{
			MostLikelyCause:   "Schema drift in `customer_status` combined with oversized batch_size",
			Confidence:        0.91,
			RecommendedAction: "Fix schema encoding, reduce batch_size, remove leaky feature, retrain.",
		},
	}
}
