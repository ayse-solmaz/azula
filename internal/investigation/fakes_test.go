package investigation

import (
	"context"
	"sync"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
)

type memProjects struct {
	mu sync.Mutex
	m  map[string]*domain.Project
}

func newMemProjects() *memProjects {
	return &memProjects{m: map[string]*domain.Project{}}
}

func (s *memProjects) Create(_ context.Context, p *domain.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *p
	s.m[p.ID] = &cp
	return nil
}

func (s *memProjects) GetByID(_ context.Context, id string) (*domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (s *memProjects) ListByWorkspace(_ context.Context, _ string) ([]domain.Project, error) {
	return nil, nil
}

func (s *memProjects) CountByWorkspaceIDs(_ context.Context, _ []string) (int64, error) {
	return 0, nil
}

func (s *memProjects) AddFile(_ context.Context, projectID string, file domain.ProjectFile) (*domain.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.m[projectID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	p.Files = append(p.Files, file)
	cp := *p
	return &cp, nil
}

type memInvs struct {
	mu sync.Mutex
	m  map[string]*domain.Investigation
}

func newMemInvs() *memInvs {
	return &memInvs{m: map[string]*domain.Investigation{}}
}

func cloneInv(inv *domain.Investigation) *domain.Investigation {
	cp := *inv
	if inv.Plan != nil {
		cp.Plan = append([]domain.PlanStep{}, inv.Plan...)
	}
	if inv.FilesAccessed != nil {
		cp.FilesAccessed = append([]string{}, inv.FilesAccessed...)
	}
	if inv.FastResult != nil {
		fr := *inv.FastResult
		cp.FastResult = &fr
	}
	if inv.DeepResult != nil {
		dr := *inv.DeepResult
		dr.Evidence = append([]domain.Evidence{}, inv.DeepResult.Evidence...)
		cp.DeepResult = &dr
	}
	if inv.CouncilResult != nil {
		cr := *inv.CouncilResult
		cr.Models = append([]domain.CouncilModel{}, inv.CouncilResult.Models...)
		cr.Agreements = append([]string{}, inv.CouncilResult.Agreements...)
		cr.Disagreements = append([]domain.Disagreement{}, inv.CouncilResult.Disagreements...)
		cp.CouncilResult = &cr
	}
	return &cp
}

func (s *memInvs) Create(_ context.Context, inv *domain.Investigation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[inv.ID] = cloneInv(inv)
	return nil
}

func (s *memInvs) GetByID(_ context.Context, id string) (*domain.Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv, ok := s.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return cloneInv(inv), nil
}

func (s *memInvs) ListByProject(_ context.Context, projectID string) ([]domain.Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Investigation
	for _, inv := range s.m {
		if inv.ProjectID == projectID {
			out = append(out, *cloneInv(inv))
		}
	}
	return out, nil
}

func (s *memInvs) Update(_ context.Context, inv *domain.Investigation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	inv.UpdatedAt = time.Now().UTC()
	s.m[inv.ID] = cloneInv(inv)
	return nil
}

func (s *memInvs) StatsByWorkspace(_ context.Context, _ string) (int, int, int, float64, error) {
	return 0, 0, 0, 0, nil
}

type memConfigs struct {
	mu sync.Mutex
	m  map[string]*domain.ModelConfig
}

func newMemConfigs() *memConfigs {
	return &memConfigs{m: map[string]*domain.ModelConfig{}}
}

func (s *memConfigs) GetByWorkspace(_ context.Context, workspaceID string) (*domain.ModelConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.m[workspaceID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *c
	return &cp, nil
}

func (s *memConfigs) Upsert(_ context.Context, cfg *domain.ModelConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *cfg
	s.m[cfg.WorkspaceID] = &cp
	return nil
}

func (s *memProjects) DeleteByWorkspaceIDs(_ context.Context, _ []string) error { return nil }

func (s *memInvs) ListByWorkspace(_ context.Context, workspaceID string) ([]domain.Investigation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.Investigation
	for _, inv := range s.m {
		if inv.WorkspaceID == workspaceID {
			out = append(out, *cloneInv(inv))
		}
	}
	return out, nil
}

func (s *memInvs) DeleteByWorkspaceIDs(_ context.Context, _ []string) error { return nil }

func (s *memConfigs) DeleteByWorkspaceIDs(_ context.Context, _ []string) error { return nil }
