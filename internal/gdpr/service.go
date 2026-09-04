package gdpr

import (
	"context"
	"encoding/json"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/mcp"
	"github.com/google/uuid"
)

type Service struct {
	users    domain.UserRepository
	spaces   domain.WorkspaceRepository
	projects domain.ProjectRepository
	invs     domain.InvestigationRepository
	configs  domain.ModelConfigRepository
	audits   domain.AuditRepository
	consents domain.ConsentRepository
	versions domain.FileVersionRepository
	jobs     domain.FineTuneRepository
	orgs     domain.OrganizationRepository
	files    mcp.Connector
}

func New(
	users domain.UserRepository,
	spaces domain.WorkspaceRepository,
	projects domain.ProjectRepository,
	invs domain.InvestigationRepository,
	configs domain.ModelConfigRepository,
	audits domain.AuditRepository,
	consents domain.ConsentRepository,
	versions domain.FileVersionRepository,
	jobs domain.FineTuneRepository,
	orgs domain.OrganizationRepository,
	files mcp.Connector,
) *Service {
	return &Service{
		users: users, spaces: spaces, projects: projects, invs: invs, configs: configs,
		audits: audits, consents: consents, versions: versions, jobs: jobs, orgs: orgs, files: files,
	}
}

func (s *Service) RecordConsent(ctx context.Context, userID, purpose string, accepted bool) (*domain.ConsentRecord, error) {
	rec := &domain.ConsentRecord{
		ID: uuid.NewString(), UserID: userID, Purpose: purpose, Accepted: accepted, CreatedAt: time.Now().UTC(),
	}
	if err := s.consents.Upsert(ctx, rec); err != nil {
		return nil, err
	}
	if s.audits != nil {
		_ = s.audits.Insert(ctx, &domain.AuditLog{
			ID: uuid.NewString(), UserID: userID, Action: "consent", Resource: purpose, CreatedAt: rec.CreatedAt,
		})
	}
	return rec, nil
}

func (s *Service) LatestConsent(ctx context.Context, userID, purpose string) (*domain.ConsentRecord, error) {
	return s.consents.GetLatest(ctx, userID, purpose)
}

func (s *Service) AuditLogs(ctx context.Context, userID string) ([]domain.AuditLog, error) {
	return s.audits.ListByUser(ctx, userID, 50)
}

func (s *Service) CreateOrganization(ctx context.Context, userID, name string) (*domain.Organization, error) {
	return nil, domain.ErrInvalidInput
}

func (s *Service) Export(ctx context.Context, userID string) (string, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	spaces, err := s.spaces.ListByOwner(ctx, userID)
	if err != nil {
		return "", err
	}
	type dump struct {
		User           *domain.User           `json:"user"`
		Workspaces     []domain.Workspace     `json:"workspaces"`
		Projects       []domain.Project       `json:"projects"`
		Investigations []domain.Investigation `json:"investigations"`
		Consents       []domain.ConsentRecord `json:"consents"`
		Audit          []domain.AuditLog      `json:"auditLogs"`
	}
	out := dump{User: &domain.User{ID: user.ID, Email: user.Email, Tier: user.Tier, MFAEnabled: user.MFAEnabled, OrgID: user.OrgID, OrgName: user.OrgName, CreatedAt: user.CreatedAt}}
	out.Workspaces = spaces
	ids := make([]string, 0, len(spaces))
	for _, ws := range spaces {
		ids = append(ids, ws.ID)
		ps, err := s.projects.ListByWorkspace(ctx, ws.ID)
		if err != nil {
			return "", err
		}
		out.Projects = append(out.Projects, ps...)
		invs, err := s.invs.ListByWorkspace(ctx, ws.ID)
		if err != nil {
			return "", err
		}
		out.Investigations = append(out.Investigations, invs...)
	}
	_ = ids
	if c, err := s.consents.ListByUser(ctx, userID); err == nil {
		out.Consents = c
	}
	if a, err := s.audits.ListByUser(ctx, userID, 200); err == nil {
		out.Audit = a
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	_ = s.audits.Insert(ctx, &domain.AuditLog{ID: uuid.NewString(), UserID: userID, Action: "export", Resource: "user", CreatedAt: time.Now().UTC()})
	return string(b), nil
}

func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	spaces, err := s.spaces.ListByOwner(ctx, userID)
	if err != nil {
		return err
	}
	wsIDs := make([]string, 0, len(spaces))
	var projectIDs []string
	for _, ws := range spaces {
		wsIDs = append(wsIDs, ws.ID)
		ps, err := s.projects.ListByWorkspace(ctx, ws.ID)
		if err != nil {
			return err
		}
		for _, p := range ps {
			projectIDs = append(projectIDs, p.ID)
			_ = s.files.RemoveProject(ctx, p.ID)
		}
	}
	_ = s.invs.DeleteByWorkspaceIDs(ctx, wsIDs)
	_ = s.configs.DeleteByWorkspaceIDs(ctx, wsIDs)
	_ = s.jobs.DeleteByWorkspaceIDs(ctx, wsIDs)
	_ = s.versions.DeleteByProjectIDs(ctx, projectIDs)
	_ = s.projects.DeleteByWorkspaceIDs(ctx, wsIDs)
	_ = s.spaces.DeleteByOwner(ctx, userID)
	_ = s.orgs.DeleteByOwner(ctx, userID)
	_ = s.consents.DeleteByUser(ctx, userID)
	_ = s.audits.DeleteByUser(ctx, userID)
	return s.users.Delete(ctx, userID)
}
