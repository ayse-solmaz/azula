package projectsvc

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/mcp"
	"github.com/google/uuid"
)

type Access interface {
	Authorize(ctx context.Context, userID, workspaceID, minRole string) error
	AuthorizeOrg(ctx context.Context, userID, minRole string) error
	ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error)
	WorkspaceOrgID(ctx context.Context, userID string) (string, error)
}

type Caps interface {
	ForUser(ctx context.Context, userID string) (domain.Entitlements, error)
}

type Service struct {
	workspaces domain.WorkspaceRepository
	projects   domain.ProjectRepository
	files      mcp.Connector
	maxFree    int
	sampleDir  string
	access     Access
	caps       Caps
	git        *mcp.Git
}

func New(workspaces domain.WorkspaceRepository, projects domain.ProjectRepository, files mcp.Connector, maxFreeProjects int, sampleDir string) *Service {
	return &Service{
		workspaces: workspaces,
		projects:   projects,
		files:      files,
		maxFree:    maxFreeProjects,
		sampleDir:  sampleDir,
	}
}

func (s *Service) SetAccess(a Access) {
	s.access = a
}

func (s *Service) SetCaps(c Caps) {
	s.caps = c
}

func (s *Service) SetGit(g *mcp.Git) {
	s.git = g
}

func (s *Service) CreateWorkspace(ctx context.Context, userID, name string) (*domain.Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, domain.ErrInvalidInput
	}
	if s.access != nil {
		if err := s.access.AuthorizeOrg(ctx, userID, "engineer"); err != nil {
			return nil, err
		}
	}
	now := time.Now().UTC()
	ws := &domain.Workspace{ID: uuid.NewString(), Name: name, OwnerID: userID, CreatedAt: now, UpdatedAt: now}
	if s.access != nil {
		if orgID, err := s.access.WorkspaceOrgID(ctx, userID); err == nil {
			ws.OrgID = orgID
		}
	}
	if err := s.workspaces.Create(ctx, ws); err != nil {
		return nil, err
	}
	return ws, nil
}

func (s *Service) ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error) {
	if s.access != nil {
		return s.access.ListWorkspaces(ctx, userID)
	}
	return s.workspaces.ListByOwner(ctx, userID)
}

func (s *Service) CreateProject(ctx context.Context, userID, workspaceID, name string, isSample bool) (*domain.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, domain.ErrInvalidInput
	}
	ws, err := s.requireAccess(ctx, userID, workspaceID, "engineer")
	if err != nil {
		return nil, err
	}

	if s.caps != nil {
		e, err := s.caps.ForUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		if e.MaxProjects > 0 {
			owned, err := s.workspaces.ListByOwner(ctx, userID)
			if err != nil {
				return nil, err
			}
			ids := make([]string, 0, len(owned))
			for _, w := range owned {
				ids = append(ids, w.ID)
			}
			count, err := s.projects.CountByWorkspaceIDs(ctx, ids)
			if err != nil {
				return nil, err
			}
			if int(count) >= e.MaxProjects {
				return nil, fmt.Errorf("%w: free tier allows %d projects — upgrade to Pro for unlimited projects", domain.ErrTierLimit, e.MaxProjects)
			}
		}
	} else if ws.OrgID == "" {
		owned, err := s.workspaces.ListByOwner(ctx, userID)
		if err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(owned))
		for _, w := range owned {
			ids = append(ids, w.ID)
		}
		count, err := s.projects.CountByWorkspaceIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		if int(count) >= s.maxFree {
			return nil, fmt.Errorf("%w: free tier allows %d projects — upload into an existing project or raise FREE_TIER_MAX_PROJECTS", domain.ErrTierLimit, s.maxFree)
		}
	}

	now := time.Now().UTC()
	p := &domain.Project{
		ID:          uuid.NewString(),
		WorkspaceID: ws.ID,
		Name:        name,
		IsSample:    isSample,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.projects.Create(ctx, p); err != nil {
		return nil, err
	}
	if isSample {
		files, err := s.files.SeedFromDir(ctx, p.ID, s.sampleDir)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			updated, err := s.projects.AddFile(ctx, p.ID, f)
			if err != nil {
				return nil, err
			}
			p = updated
		}
	}
	return p, nil
}

func (s *Service) GetProject(ctx context.Context, userID, projectID string) (*domain.Project, error) {
	p, err := s.projects.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireAccess(ctx, userID, p.WorkspaceID, "viewer"); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Service) UploadFile(ctx context.Context, userID, projectID, filename, mimeType string, r io.Reader) (*domain.ProjectFile, error) {
	p, err := s.GetProject(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireAccess(ctx, userID, p.WorkspaceID, "engineer"); err != nil {
		return nil, err
	}
	saved, err := s.files.SaveUpload(ctx, projectID, filename, mimeType, r)
	if err != nil {
		return nil, err
	}
	updated, err := s.projects.AddFile(ctx, projectID, saved)
	if err != nil {
		return nil, err
	}
	for i := range updated.Files {
		if updated.Files[i].Name == saved.Name {
			f := updated.Files[i]
			return &f, nil
		}
	}
	return &saved, nil
}

func (s *Service) FileVersions(ctx context.Context, userID, projectID, name string) ([]domain.FileVersion, error) {
	if _, err := s.GetProject(ctx, userID, projectID); err != nil {
		return nil, err
	}
	return s.files.ListFileVersions(ctx, projectID, name)
}

func (s *Service) SwapFileVersion(ctx context.Context, userID, projectID, name string, version int) (*domain.ProjectFile, error) {
	p, err := s.GetProject(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireAccess(ctx, userID, p.WorkspaceID, "engineer"); err != nil {
		return nil, err
	}
	saved, err := s.files.RestoreFileVersion(ctx, projectID, name, version)
	if err != nil {
		return nil, err
	}
	updated, err := s.projects.AddFile(ctx, projectID, saved)
	if err != nil {
		return nil, err
	}
	for i := range updated.Files {
		if updated.Files[i].Name == saved.Name {
			f := updated.Files[i]
			return &f, nil
		}
	}
	return &saved, nil
}

func (s *Service) FileVersionContent(ctx context.Context, userID, projectID, name string, version int) (string, error) {
	if _, err := s.GetProject(ctx, userID, projectID); err != nil {
		return "", err
	}
	return s.files.ReadFileVersion(ctx, projectID, name, version)
}

func (s *Service) CanInvestigate(ctx context.Context, userID, projectID string) error {
	p, err := s.GetProject(ctx, userID, projectID)
	if err != nil {
		return err
	}
	_, err = s.requireAccess(ctx, userID, p.WorkspaceID, "engineer")
	return err
}

func (s *Service) ConnectGit(ctx context.Context, userID, projectID, repoURL, branch string) (*domain.GitRepo, error) {
	if s.git == nil {
		return nil, domain.ErrGitNotConnected
	}
	if err := s.CanInvestigate(ctx, userID, projectID); err != nil {
		return nil, err
	}
	repo, files, err := s.git.Clone(ctx, projectID, repoURL, branch)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if _, err := s.projects.AddFile(ctx, projectID, f); err != nil {
			return nil, err
		}
	}
	if _, err := s.projects.SetGit(ctx, projectID, repo.URL, repo.Branch, repo.Head); err != nil {
		return nil, err
	}
	return repo, nil
}

func (s *Service) ProjectsForWorkspace(ctx context.Context, userID, workspaceID string) ([]domain.Project, error) {
	if _, err := s.requireAccess(ctx, userID, workspaceID, "viewer"); err != nil {
		return nil, err
	}
	return s.projects.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) requireAccess(ctx context.Context, userID, workspaceID, minRole string) (*domain.Workspace, error) {
	if s.access != nil {
		if err := s.access.Authorize(ctx, userID, workspaceID, minRole); err != nil {
			return nil, err
		}
		return s.workspaces.GetByID(ctx, workspaceID)
	}
	return s.requireWorkspaceOwner(ctx, userID, workspaceID)
}

func (s *Service) requireWorkspaceOwner(ctx context.Context, userID, workspaceID string) (*domain.Workspace, error) {
	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.OwnerID != userID {
		return nil, domain.ErrUnauthorized
	}
	return ws, nil
}
