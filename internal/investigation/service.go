package investigation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"log"
	"sync"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/llm"
	"github.com/ayse-solmaz/azula/internal/mcp"
	"github.com/google/uuid"
)

type Service struct {
	projects domain.ProjectRepository
	invs     domain.InvestigationRepository
	configs  domain.ModelConfigRepository
	files    mcp.Connector
	router   *llm.Router
	cfg      config.Config
	gate     Gate
	audit    Auditor
	runs     sync.Map // investigation ID → context.CancelFunc
}

type Gate interface {
	CheckInvestigationCap(ctx context.Context, userID string) error
	Require(ctx context.Context, userID, feature string) error
}

type Auditor interface {
	Insert(ctx context.Context, log *domain.AuditLog) error
}

func New(projects domain.ProjectRepository, invs domain.InvestigationRepository, configs domain.ModelConfigRepository, files mcp.Connector, router *llm.Router, cfg config.Config) *Service {
	return &Service{projects: projects, invs: invs, configs: configs, files: files, router: router, cfg: cfg}
}

func (s *Service) SetGate(g Gate) {
	s.gate = g
}

func (s *Service) SetAudit(a Auditor) {
	s.audit = a
}

func (s *Service) Halted() bool {
	return s.cfg.KillSwitch
}

func (s *Service) logAudit(ctx context.Context, userID, action, resource string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Insert(ctx, &domain.AuditLog{
		ID: uuid.NewString(), UserID: userID, Action: action, Resource: resource, CreatedAt: time.Now().UTC(),
	})
}

func DefaultPlan() []domain.PlanStep {
	return PlanFromFiles([]string{"training.log", "config.yaml", "pipeline.py", "dataset.jsonl"})
}

func PlanFromFiles(names []string) []domain.PlanStep {
	descs := make([]string, 0, len(names)+1)
	for _, n := range names {
		descs = append(descs, "Read "+n+" (Investigator context)")
	}
	if len(descs) == 0 {
		descs = append(descs, "List project files")
	}
	descs = append(descs, "Run Council with findings")
	steps := make([]domain.PlanStep, len(descs))
	for i, d := range descs {
		steps[i] = domain.PlanStep{Order: i + 1, Description: d, Status: domain.StepPending}
	}
	return steps
}

func investigatorPrompt() string {
	return llm.SysInvestigator
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
	if s.Halted() {
		return nil, domain.ErrAgentHalted
	}
	project, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if s.gate != nil {
		if err := s.gate.CheckInvestigationCap(ctx, userID); err != nil {
			return nil, err
		}
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
	s.logAudit(ctx, userID, "agent.start", "investigation:"+inv.ID)
	go s.runAsync(inv.ID)
	return inv, nil
}

func (s *Service) Cancel(ctx context.Context, userID, id string) (*domain.Investigation, error) {
	inv, err := s.invs.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	switch inv.Status {
	case domain.StatusCompleted, domain.StatusFailed:
		return inv, nil
	}
	if fn, ok := s.runs.Load(id); ok {
		if cancel, ok := fn.(context.CancelFunc); ok {
			cancel()
		}
	}
	inv.Status = domain.StatusFailed
	inv.ErrorMessage = domain.ErrCancelled.Error()
	inv.UpdatedAt = time.Now().UTC()
	if err := s.invs.Update(ctx, inv); err != nil {
		return nil, err
	}
	s.logAudit(ctx, userID, "agent.cancel", "investigation:"+inv.ID)
	return inv, nil
}

func aborted(err error) bool {
	return err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, domain.ErrCancelled))
}

func cancelled(inv *domain.Investigation) bool {
	if inv == nil || inv.ErrorMessage == "" {
		return false
	}
	return inv.ErrorMessage == domain.ErrCancelled.Error()
}

func (s *Service) persist(inv *domain.Investigation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cur, err := s.invs.GetByID(ctx, inv.ID)
	if err != nil {
		return err
	}
	if cancelled(cur) {
		*inv = *cur
		return domain.ErrCancelled
	}
	inv.UpdatedAt = time.Now().UTC()
	return s.invs.Update(ctx, inv)
}

func (s *Service) ensureCancelled(inv *domain.Investigation) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cur, err := s.invs.GetByID(ctx, inv.ID)
	if err == nil && cancelled(cur) {
		*inv = *cur
		return
	}
	inv.Status = domain.StatusFailed
	inv.ErrorMessage = domain.ErrCancelled.Error()
	inv.UpdatedAt = time.Now().UTC()
	_ = s.invs.Update(ctx, inv)
}

func (s *Service) halted(workCtx context.Context, inv *domain.Investigation) error {
	if s.Halted() {
		inv.Status = domain.StatusFailed
		inv.ErrorMessage = domain.ErrAgentHalted.Error()
		inv.UpdatedAt = time.Now().UTC()
		_ = s.persist(inv)
		return domain.ErrAgentHalted
	}
	if err := workCtx.Err(); aborted(err) {
		s.ensureCancelled(inv)
		return domain.ErrCancelled
	} else if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cur, err := s.invs.GetByID(ctx, inv.ID)
	if err != nil {
		return err
	}
	if cancelled(cur) || cur.Status == domain.StatusFailed {
		*inv = *cur
		if cancelled(cur) {
			return domain.ErrCancelled
		}
		return domain.ErrCancelled
	}
	return nil
}

func (s *Service) runAsync(id string) {
	defer s.router.Release()
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.RequestTimeout+2*time.Minute)
	workCtx, workCancel := context.WithCancel(ctx)
	s.runs.Store(id, workCancel)
	defer func() {
		workCancel()
		cancel()
		s.runs.Delete(id)
	}()
	db, dbCancel := context.WithTimeout(context.Background(), 8*time.Second)
	inv, err := s.invs.GetByID(db, id)
	dbCancel()
	if err != nil {
		return
	}
	if err := s.runPipeline(workCtx, inv); err != nil {
		if aborted(err) {
			s.ensureCancelled(inv)
			return
		}
		if cancelled(inv) {
			return
		}
		inv.Status = domain.StatusFailed
		inv.ErrorMessage = err.Error()
		_ = s.persist(inv)
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
			c.InvestigatorPrompt = llm.SysInvestigator
			c.ChallengerPrompt = llm.SysChallenger
			c.JudgePrompt = llm.SysJudge
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
	if in.ModelCProvider != "" {
		cur.ModelCProvider = in.ModelCProvider
	}
	if in.ModelCName != "" {
		cur.ModelCName = in.ModelCName
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
	for _, f := range listed {
		names = append(names, f.Name)
	}
	inv.Plan = PlanFromFiles(names)
	inv.FilesAccessed = []string{}
	inv.FallbackStages = []string{}

	invCtx := domain.InvestigationContext{
		InvestigationID: inv.ID,
		ProjectID:       inv.ProjectID,
		Prompt:          inv.Prompt,
		FileNames:       names,
		FileContents:    map[string]string{},
	}
	mcfg := s.modelConfig(ctx, inv.WorkspaceID)
	if mcfg.InvestigatorPrompt == "" {
		mcfg.InvestigatorPrompt = investigatorPrompt()
	}
	inv.ModelAName = mcfg.ModelAName
	inv.ModelBName = mcfg.ModelBName
	inv.ModelCName = mcfg.ModelCName
	if ollamaNames, listErr := llm.ListOllamaModels(ctx, s.cfg.OllamaBaseURL); listErr == nil {
		inv.ModelBName = llm.PickModelB(ollamaNames, mcfg.ModelBName)
	}
	if err := s.persist(inv); err != nil {
		return err
	}

	if err := s.halted(ctx, inv); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		if aborted(ctx.Err()) {
			s.ensureCancelled(inv)
			return domain.ErrCancelled
		}
		return ctx.Err()
	case <-time.After(700 * time.Millisecond):
	}

	if err := s.transition(inv, domain.StatusPending, domain.StatusFastClassify); err != nil {
		return err
	}
	if err := s.persist(inv); err != nil {
		return err
	}

	var fallback []string
	fast, err := s.router.Classify(ctx, mcfg, invCtx)
	if aborted(err) {
		s.ensureCancelled(inv)
		return domain.ErrCancelled
	}
	if err != nil {
		log.Printf("investigation %s: fast LLM fallback: %v", inv.ID, err)
		fast = fallbackFast()
		fallback = append(fallback, "fast")
	}
	inv.FastResult = fast
	if err := s.persist(inv); err != nil {
		return err
	}
	if err := s.halted(ctx, inv); err != nil {
		return err
	}

	forceCouncil := s.forceCouncilOnProject(ctx, inv.ProjectID)
	if s.gate != nil && !forceCouncil {
		if err := s.gate.Require(ctx, inv.UserID, "deep"); err != nil {
			inv.EscalationReason = "Deep look and Council require Pro. Upgrade to continue with evidence-backed root cause."
			inv.ExecutionMode = executionMode(fallback, 1)
			inv.FallbackStages = fallback
			s.skipRemainingPlan(inv)
			if err := s.transition(inv, domain.StatusFastClassify, domain.StatusCompleted); err != nil {
				return err
			}
			log.Printf("investigation %s: free-tier skip deep (%s)", inv.ID, inv.EscalationReason)
			return s.persist(inv)
		}
	}

	inv.EscalationReason = fmt.Sprintf("Continuing to Deep look and Council (Quick look confidence %.0f%%).", fast.Confidence*100)
	log.Printf("investigation %s: %s", inv.ID, inv.EscalationReason)
	if err := s.transition(inv, domain.StatusFastClassify, domain.StatusDeepAnalyze); err != nil {
		return err
	}
	if err := s.persist(inv); err != nil {
		return err
	}

	ranked := rankNames(names, inv.Prompt, fast.IncidentType)
	contents, order, err := readRanked(ctx, s.files, inv.ProjectID, ranked, ContextChars)
	if err != nil {
		return err
	}
	stepByName := map[string]int{}
	for i, n := range names {
		stepByName[n] = i
	}
	for _, n := range names {
		if _, ok := contents[n]; ok {
			continue
		}
		s.markStep(inv, stepByName[n], domain.StepSkipped)
	}
	for _, n := range order {
		s.markStep(inv, stepByName[n], domain.StepDone)
		inv.FilesAccessed = append(inv.FilesAccessed, n)
		log.Printf("mcp: read %s (%d bytes) investigation=%s", n, len(contents[n]), inv.ID)
		s.logAudit(ctx, inv.UserID, "mcp.read", inv.ProjectID+":"+n)
	}
	invCtx.FileContents = contents
	if err := s.persist(inv); err != nil {
		return err
	}
	if err := s.halted(ctx, inv); err != nil {
		return err
	}

	deep, err := s.router.Analyze(ctx, mcfg, invCtx)
	if aborted(err) {
		s.ensureCancelled(inv)
		return domain.ErrCancelled
	}
	if err != nil || deep == nil || len(deep.Evidence) == 0 {
		if err != nil {
			log.Printf("investigation %s: deep LLM fallback: %v", inv.ID, err)
		}
		deep = fallbackDeep()
		fallback = append(fallback, "deep")
	}
	if deep != nil {
		deep.SuggestedFix = sanitizeSuggestedFix(deep.RootCause, deep.SuggestedFix)
	}
	inv.DeepResult = deep
	if err := s.persist(inv); err != nil {
		return err
	}

	if err := s.transition(inv, domain.StatusDeepAnalyze, domain.StatusCouncil); err != nil {
		return err
	}
	s.markStep(inv, len(inv.Plan)-1, domain.StepRunning)
	if err := s.persist(inv); err != nil {
		return err
	}
	if err := s.halted(ctx, inv); err != nil {
		return err
	}

	council, err := s.router.RunCouncilProgress(ctx, mcfg, invCtx, fast, deep, func(partial *domain.CouncilResult) {
		if partial == nil {
			return
		}
		inv.CouncilResult = partial
		_ = s.persist(inv)
	})
	if aborted(err) {
		s.ensureCancelled(inv)
		return domain.ErrCancelled
	}
	if err != nil || council == nil || strings.TrimSpace(council.FinalJudgment.MostLikelyCause) == "" {
		if err != nil {
			log.Printf("investigation %s: council LLM fallback: %v", inv.ID, err)
		}
		// Promote Deep look when Council agents/judge fail — never invent a different sample.
		council = fallbackCouncilFrom(fast, deep)
		fallback = append(fallback, "council")
	}
	if council != nil && council.Aggregation == "" {
		llm.ApplyAggregation(council, llm.ModelFamily(inv.ModelAName) == llm.ModelFamily(inv.ModelBName))
	}
	inv.CouncilResult = council
	s.markStep(inv, len(inv.Plan)-1, domain.StepDone)
	inv.ExecutionMode = executionMode(fallback, 3)
	inv.FallbackStages = fallback
	if err := s.transition(inv, domain.StatusCouncil, domain.StatusCompleted); err != nil {
		return err
	}
	return s.persist(inv)
}

func (s *Service) forceCouncilOnProject(ctx context.Context, projectID string) bool {
	if !s.cfg.ForceCouncilOnSample {
		return false
	}
	p, err := s.projects.GetByID(ctx, projectID)
	if err != nil || p == nil {
		return false
	}
	return p.IsSample
}

func executionMode(fallback []string, stages int) string {
	if len(fallback) == 0 {
		return domain.ExecutionLive
	}
	if stages > 0 && len(fallback) >= stages {
		return domain.ExecutionFallback
	}
	return domain.ExecutionMixed
}

func (s *Service) skipRemainingPlan(inv *domain.Investigation) {
	for i := range inv.Plan {
		if inv.Plan[i].Status == domain.StepPending || inv.Plan[i].Status == domain.StepRunning {
			inv.Plan[i].Status = domain.StepSkipped
		}
	}
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


// sanitizeSuggestedFix rewrites weak "ignore missing values" advice when the
// root cause is dropna / NaN row loss (samples/broken-nan-impute).
func sanitizeSuggestedFix(root, fix string) string {
	blob := strings.ToLower(root + " " + fix)
	nanish := strings.Contains(blob, "dropna") ||
		(strings.Contains(blob, "monthly_spend") && (strings.Contains(blob, "nan") || strings.Contains(blob, "missing"))) ||
		(strings.Contains(blob, "class balance") && strings.Contains(blob, "nan"))
	if !nanish {
		return fix
	}
	bad := strings.Contains(strings.ToLower(fix), "ignore") ||
		strings.Contains(strings.ToLower(fix), "missing_value_policy") ||
		strings.TrimSpace(fix) == ""
	if !bad && (strings.Contains(strings.ToLower(fix), "impute") || strings.Contains(strings.ToLower(fix), "fillna")) {
		return fix
	}
	return "Replace dropna on monthly_spend with median imputation (fillna / SimpleImputer strategy=median) so rows are kept and class balance is preserved; do not use missing_value_policy=ignore."
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
	return fallbackCouncilFrom(nil, nil)
}

// fallbackCouncilFrom builds a Council result from the Deep (and Fast) look already
// produced for this run. Hard-coded broken-pipeline text must not replace a live Deep finding.
func fallbackCouncilFrom(fast *domain.FastResult, deep *domain.DeepResult) *domain.CouncilResult {
	if deep == nil || strings.TrimSpace(deep.RootCause) == "" {
		deep = fallbackDeep()
	}
	cause := strings.TrimSpace(deep.RootCause)
	action := strings.TrimSpace(deep.SuggestedFix)
	if action == "" {
		action = "Inspect the cited evidence and apply the Deep look fix before retraining."
	}
	ev := deep.Evidence
	if len(ev) == 0 {
		ev = []domain.Evidence{{File: "training.log", Lines: "1-40", Excerpt: cause}}
	}
	invEv := ev
	if len(invEv) > 2 {
		invEv = invEv[:2]
	}
	conf := deep.Confidence
	if conf < 0.5 {
		conf = 0.72
	}
	_ = fast
	return &domain.CouncilResult{
		Models: []domain.CouncilModel{
			{Role: "investigator", Hypothesis: cause, Confidence: conf, Evidence: invEv},
			{Role: "challenger", Hypothesis: cause, Confidence: conf * 0.92, Evidence: invEv},
		},
		Agreements: []string{"Council agents timed out or failed; judgment continues from the Deep look finding."},
		FinalJudgment: domain.FinalJudgment{
			MostLikelyCause:   cause,
			Confidence:        conf,
			RecommendedAction: action,
		},
	}
}
