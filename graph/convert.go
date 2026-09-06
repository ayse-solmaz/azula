package graph

import (
	"context"
	"time"

	"github.com/ayse-solmaz/azula/graph/model"
	"github.com/ayse-solmaz/azula/internal/domain"
)

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	return err
}

func (r *Resolver) requireWorkspace(ctx context.Context, userID, workspaceID string) error {
	if r.Org != nil {
		return r.Org.Authorize(ctx, userID, workspaceID, "viewer")
	}
	ws, err := r.Spaces.GetByID(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ws.OwnerID != userID {
		return domain.ErrUnauthorized
	}
	return nil
}

func (r *Resolver) requireEngineer(ctx context.Context, userID, workspaceID string) error {
	if r.Org != nil {
		return r.Org.Authorize(ctx, userID, workspaceID, "engineer")
	}
	return r.requireWorkspace(ctx, userID, workspaceID)
}

func (r *Resolver) requireAdmin(ctx context.Context, userID, workspaceID string) error {
	if r.Org != nil {
		return r.Org.Authorize(ctx, userID, workspaceID, "admin")
	}
	return r.requireWorkspace(ctx, userID, workspaceID)
}

func gqlUser(u *domain.User) *model.User {
	devs := make([]*model.TrustedDevice, 0, len(u.TrustedDevices))
	for _, d := range u.TrustedDevices {
		name := d.Name
		if name == "" {
			name = "trusted device"
		}
		last := d.LastSeenAt
		if last.IsZero() {
			last = d.CreatedAt
		}
		devs = append(devs, &model.TrustedDevice{
			ID: d.DeviceID, Name: name, CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339), LastSeenAt: last.UTC().Format(time.RFC3339),
		})
	}
	notifyEmail, notifyInv := u.NotifyEmail, u.NotifyInvestigations
	if u.PrefsVersion == 0 {
		notifyEmail, notifyInv = true, true
	}
	created := u.CreatedAt.UTC().Format(time.RFC3339)
	if u.CreatedAt.IsZero() {
		created = ""
	}
	out := &model.User{
		ID:                   u.ID,
		Email:                u.Email,
		DisplayName:          u.DisplayName,
		Tier:                 gqlTier(u.Tier),
		MfaEnabled:           u.MFAEnabled,
		TrustedDevices:       devs,
		SsoLinked:            u.SSOSubject != "",
		Disabled:             u.Disabled,
		CreatedAt:            created,
		NotifyEmail:          notifyEmail,
		NotifyInvestigations: notifyInv,
		NotifyMarketing:      u.NotifyMarketing,
		ShareUsage:           u.ShareUsage,
	}
	if u.OrgID != "" {
		out.OrgID = &u.OrgID
	}
	if u.OrgName != "" {
		out.OrgName = &u.OrgName
	}
	if u.OrgRole != "" {
		out.OrgRole = &u.OrgRole
	}
	return out
}

func gqlOrg(o *domain.Organization) *model.Organization {
	members := make([]*model.OrgMember, 0, len(o.Members))
	for _, m := range o.Members {
		om := &model.OrgMember{Email: m.Email, Role: m.Role}
		if m.UserID != "" {
			uid := m.UserID
			om.UserID = &uid
		}
		members = append(members, om)
	}
	return &model.Organization{ID: o.ID, Name: o.Name, Members: members}
}

func gqlTier(t domain.Tier) model.Tier {
	switch t {
	case domain.TierPro:
		return model.TierPro
	case domain.TierEnterprise:
		return model.TierEnterprise
	default:
		return model.TierFree
	}
}

func gqlProject(p *domain.Project) *model.Project {
	files := make([]*model.ProjectFile, 0, len(p.Files))
	for _, f := range p.Files {
		files = append(files, gqlFile(f))
	}
	return &model.Project{
		ID:          p.ID,
		WorkspaceID: p.WorkspaceID,
		Name:        p.Name,
		IsSample:    p.IsSample,
		Files:       files,
	}
}

func gqlFile(f domain.ProjectFile) *model.ProjectFile {
	return &model.ProjectFile{
		Name:       f.Name,
		Path:       f.Path,
		MimeType:   f.MimeType,
		UploadedAt: f.UploadedAt.UTC().Format(time.RFC3339),
	}
}

func gqlWorkspace(ws *domain.Workspace, projects []domain.Project) *model.Workspace {
	ps := make([]*model.Project, 0, len(projects))
	for i := range projects {
		ps = append(ps, gqlProject(&projects[i]))
	}
	return &model.Workspace{
		ID:       ws.ID,
		Name:     ws.Name,
		Projects: ps,
	}
}

func gqlInvestigation(inv *domain.Investigation) *model.Investigation {
	if inv == nil {
		return nil
	}
	out := &model.Investigation{
		ID:               inv.ID,
		ProjectID:        inv.ProjectID,
		Prompt:           inv.Prompt,
		Status:           gqlInvStatus(inv.Status),
		Plan:             gqlPlan(inv.Plan),
		FilesAccessed:    inv.FilesAccessed,
		ErrorMessage:     strPtr(inv.ErrorMessage),
		ModelAName:       strPtr(inv.ModelAName),
		ModelBName:       strPtr(inv.ModelBName),
		ModelCName:       strPtr(inv.ModelCName),
		EscalationReason: strPtr(inv.EscalationReason),
		ExecutionMode:    gqlExecutionMode(inv.ExecutionMode),
		FallbackStages:   inv.FallbackStages,
		CreatedAt:        inv.CreatedAt.UTC().Format(time.RFC3339),
	}
	if inv.FilesAccessed == nil {
		out.FilesAccessed = []string{}
	}
	if out.FallbackStages == nil {
		out.FallbackStages = []string{}
	}
	if inv.FastResult != nil {
		out.FastResult = &model.FastResult{
			Summary: inv.FastResult.Summary, IncidentType: inv.FastResult.IncidentType, Confidence: inv.FastResult.Confidence,
		}
	}
	if inv.DeepResult != nil {
		ev := make([]*model.Evidence, 0, len(inv.DeepResult.Evidence))
		for _, e := range inv.DeepResult.Evidence {
			ev = append(ev, &model.Evidence{File: e.File, Lines: e.Lines, Excerpt: e.Excerpt})
		}
		out.DeepResult = &model.DeepResult{
			RootCause: inv.DeepResult.RootCause, Confidence: inv.DeepResult.Confidence, Evidence: ev, SuggestedFix: inv.DeepResult.SuggestedFix,
		}
	}
	if inv.CouncilResult != nil {
		models := make([]*model.CouncilModel, 0, len(inv.CouncilResult.Models))
		for _, m := range inv.CouncilResult.Models {
			ev := make([]*model.Evidence, 0, len(m.Evidence))
			for _, e := range m.Evidence {
				ev = append(ev, &model.Evidence{File: e.File, Lines: e.Lines, Excerpt: e.Excerpt})
			}
			models = append(models, &model.CouncilModel{Role: m.Role, Hypothesis: m.Hypothesis, Confidence: m.Confidence, Evidence: ev, Model: strPtr(m.Model)})
		}
		dis := make([]*model.Disagreement, 0, len(inv.CouncilResult.Disagreements))
		for _, d := range inv.CouncilResult.Disagreements {
			dis = append(dis, &model.Disagreement{Topic: d.Topic, Investigator: d.Investigator, Challenger: d.Challenger})
		}
		agg := inv.CouncilResult.Aggregation
		if agg == "" {
			agg = "unknown"
		}
		agreements := inv.CouncilResult.Agreements
		if agreements == nil {
			agreements = []string{}
		}
		out.CouncilResult = &model.CouncilResult{
			Models: models, Agreements: agreements, Disagreements: dis,
			Aggregation: agg, NeedsReview: inv.CouncilResult.NeedsReview, AggregationNote: inv.CouncilResult.AggregationNote,
			FinalJudgment: &model.FinalJudgment{
				MostLikelyCause:   inv.CouncilResult.FinalJudgment.MostLikelyCause,
				Confidence:        inv.CouncilResult.FinalJudgment.Confidence,
				RecommendedAction: inv.CouncilResult.FinalJudgment.RecommendedAction,
			},
		}
	}
	return out
}

func gqlPlan(steps []domain.PlanStep) []*model.PlanStep {
	out := make([]*model.PlanStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, &model.PlanStep{Order: s.Order, Description: s.Description, Status: gqlStepStatus(s.Status)})
	}
	return out
}

func gqlInvStatus(s string) model.InvestigationStatus {
	switch s {
	case domain.StatusFastClassify:
		return model.InvestigationStatusFastClassify
	case domain.StatusDeepAnalyze:
		return model.InvestigationStatusDeepAnalyze
	case domain.StatusCouncil:
		return model.InvestigationStatusCouncil
	case domain.StatusCompleted:
		return model.InvestigationStatusCompleted
	case domain.StatusFailed:
		return model.InvestigationStatusFailed
	default:
		return model.InvestigationStatusPending
	}
}

func gqlExecutionMode(s string) *model.ExecutionMode {
	var m model.ExecutionMode
	switch s {
	case domain.ExecutionLive:
		m = model.ExecutionModeLive
	case domain.ExecutionFallback:
		m = model.ExecutionModeFallback
	case domain.ExecutionMixed:
		m = model.ExecutionModeMixed
	default:
		return nil
	}
	return &m
}

func gqlStepStatus(s string) model.StepStatus {
	switch s {
	case domain.StepRunning:
		return model.StepStatusRunning
	case domain.StepDone:
		return model.StepStatusDone
	case domain.StepSkipped, domain.StepFailed:
		return model.StepStatusSkipped
	default:
		return model.StepStatusPending
	}
}

func gqlModelConfig(c *domain.ModelConfig) *model.ModelConfig {
	return &model.ModelConfig{
		WorkspaceID:        c.WorkspaceID,
		ModelAProvider:     c.ModelAProvider,
		ModelAName:         c.ModelAName,
		ModelBProvider:     c.ModelBProvider,
		ModelBName:         c.ModelBName,
		ModelCProvider:     c.ModelCProvider,
		ModelCName:         c.ModelCName,
		Temperature:        c.Temperature,
		MaxTokens:          c.MaxTokens,
		InvestigatorPrompt: c.InvestigatorPrompt,
		ChallengerPrompt:   c.ChallengerPrompt,
		JudgePrompt:        c.JudgePrompt,
		ActiveSlot:         c.ActiveSlot,
	}
}

func gqlMetrics(m *domain.LLMOpsMetrics) *model.LLMOpsMetrics {
	models := m.OllamaModels
	if models == nil {
		models = []string{}
	}
	causes := m.TopCauses
	if causes == nil {
		causes = []string{}
	}
	return &model.LLMOpsMetrics{
		TotalInvestigations: m.TotalInvestigations,
		Completed:           m.Completed,
		Failed:              m.Failed,
		AvgConfidence:       m.AvgConfidence,
		WorkerSlots:         m.WorkerSlots,
		BusySlots:           m.BusySlots,
		ModelAName:          m.ModelAName,
		ModelBName:          m.ModelBName,
		AvgDurationSec:      m.AvgDurationSec,
		OllamaReachable:     m.OllamaReachable,
		OllamaModels:        models,
		IncidentModelReady:  m.IncidentModelReady,
		AdapterOnDisk:       m.AdapterOnDisk,
		TopCauses:           causes,
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func gqlConsent(c *domain.ConsentRecord) *model.ConsentRecord {
	if c == nil {
		return nil
	}
	return &model.ConsentRecord{
		Purpose: c.Purpose, Accepted: c.Accepted, CreatedAt: c.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func gqlJob(j *domain.FineTuneJob) *model.FineTuneJob {
	return &model.FineTuneJob{
		ID: j.ID, WorkspaceID: j.WorkspaceID, Status: j.Status, AdapterPath: j.AdapterPath,
		Error: strPtr(j.Error), CreatedAt: j.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func gqlGitRepo(g *domain.GitRepo) *model.GitRepo {
	if g == nil {
		return &model.GitRepo{Connected: false}
	}
	return &model.GitRepo{URL: g.URL, Branch: g.Branch, Head: g.Head, Connected: g.Connected}
}

func gqlGeneration(g *domain.Generation) *model.Generation {
	return &model.Generation{
		ID: g.ID, ProjectID: g.ProjectID, InvestigationID: strPtr(g.InvestigationID),
		Prompt: g.Prompt, FileName: g.FileName, RowCount: g.RowCount,
		SchemaNote: g.SchemaNote, QualityNotes: g.QualityNotes, Confidence: g.Confidence,
		Status: g.Status, Error: strPtr(g.Error), CreatedAt: g.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func gqlEvaluation(e *domain.Evaluation) *model.Evaluation {
	metrics := make([]*model.MetricDelta, 0, len(e.Metrics))
	for _, m := range e.Metrics {
		metrics = append(metrics, &model.MetricDelta{Name: m.Name, Before: m.Before, After: m.After, Delta: m.Delta})
	}
	return &model.Evaluation{
		ID: e.ID, ProjectID: e.ProjectID, InvestigationID: strPtr(e.InvestigationID),
		GenerationID: strPtr(e.GenerationID), Summary: e.Summary, Recommendation: e.Recommendation,
		Confidence: e.Confidence, Metrics: metrics, Status: e.Status, Error: strPtr(e.Error),
		CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func gqlEntitlements(e domain.Entitlements) *model.Entitlements {
	return &model.Entitlements{
		Tier: gqlTier(e.Tier), MaxProjects: e.MaxProjects, MaxInvestigationsPerMonth: e.MaxInvestigationsPerMonth,
		InvestigationsUsed: e.InvestigationsUsed, DeepAnalysis: e.DeepAnalysis, Council: e.Council,
		Generate: e.Generate, Evaluate: e.Evaluate, GitMcp: e.GitMCP, ModelSelection: e.ModelSelection,
		TeamWorkspace: e.TeamWorkspace, BillingConfigured: e.BillingConfigured, SsoEnabled: e.SSOEnabled,
		DemoUpgrade: e.DemoUpgrade,
	}
}
