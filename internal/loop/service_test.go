package loop

import (
	"context"
	"strings"
	"testing"

	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/mcp"
)

type memGens struct{ m map[string]*domain.Generation }

func (s *memGens) Create(_ context.Context, g *domain.Generation) error {
	if s.m == nil {
		s.m = map[string]*domain.Generation{}
	}
	cp := *g
	s.m[g.ID] = &cp
	return nil
}
func (s *memGens) Update(_ context.Context, g *domain.Generation) error { return s.Create(context.Background(), g) }
func (s *memGens) GetByID(_ context.Context, id string) (*domain.Generation, error) {
	g, ok := s.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *g
	return &cp, nil
}
func (s *memGens) ListByProject(_ context.Context, projectID string) ([]domain.Generation, error) {
	var out []domain.Generation
	for _, g := range s.m {
		if g.ProjectID == projectID {
			out = append(out, *g)
		}
	}
	return out, nil
}
func (s *memGens) DeleteByWorkspaceIDs(_ context.Context, _ []string) error { return nil }

type memEvals struct{ m map[string]*domain.Evaluation }

func (s *memEvals) Create(_ context.Context, e *domain.Evaluation) error {
	if s.m == nil {
		s.m = map[string]*domain.Evaluation{}
	}
	cp := *e
	s.m[e.ID] = &cp
	return nil
}
func (s *memEvals) Update(_ context.Context, e *domain.Evaluation) error {
	return s.Create(context.Background(), e)
}
func (s *memEvals) GetByID(_ context.Context, id string) (*domain.Evaluation, error) {
	e, ok := s.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *e
	return &cp, nil
}
func (s *memEvals) ListByProject(_ context.Context, projectID string) ([]domain.Evaluation, error) {
	return nil, nil
}
func (s *memEvals) DeleteByWorkspaceIDs(_ context.Context, _ []string) error { return nil }

type memProj struct{ p *domain.Project }

func (s *memProj) Create(context.Context, *domain.Project) error { return nil }
func (s *memProj) GetByID(_ context.Context, id string) (*domain.Project, error) {
	if s.p == nil || s.p.ID != id {
		return nil, domain.ErrNotFound
	}
	cp := *s.p
	return &cp, nil
}
func (s *memProj) ListByWorkspace(context.Context, string) ([]domain.Project, error) { return nil, nil }
func (s *memProj) CountByWorkspaceIDs(context.Context, []string) (int64, error)      { return 0, nil }
func (s *memProj) AddFile(_ context.Context, _ string, file domain.ProjectFile) (*domain.Project, error) {
	s.p.Files = append(s.p.Files, file)
	cp := *s.p
	return &cp, nil
}
func (s *memProj) SetGit(_ context.Context, _ string, url, branch, head string) (*domain.Project, error) {
	s.p.GitURL, s.p.GitBranch, s.p.GitHead = url, branch, head
	cp := *s.p
	return &cp, nil
}
func (s *memProj) DeleteByWorkspaceIDs(context.Context, []string) error { return nil }

func TestGenerateFallbackWritesJSONL(t *testing.T) {
	root := t.TempDir()
	files := mcp.NewFilesConnector(root)
	p := &domain.Project{ID: "p1", WorkspaceID: "w1", Name: "demo"}
	svc := New(&memProj{p: p}, nil, &memGens{}, &memEvals{}, files, nil, nil)
	g, err := svc.Generate(context.Background(), "u1", "p1", "", "fix schema")
	if err != nil {
		t.Fatal(err)
	}
	if g.Status != "completed" || g.RowCount < 4 {
		t.Fatalf("generation: %+v", g)
	}
	body, err := files.ReadFile(context.Background(), "p1", g.FileName)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "customer_status") {
		t.Fatalf("jsonl: %s", body)
	}
}

func TestEvaluateFallback(t *testing.T) {
	root := t.TempDir()
	files := mcp.NewFilesConnector(root)
	_, err := files.SaveUpload(context.Background(), "p1", "metrics.json", "application/json", strings.NewReader(`{"val_accuracy":0.71}`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = files.SaveUpload(context.Background(), "p1", "fixed_dataset.jsonl", "application/json", strings.NewReader("{\"ok\":true}\n"))
	if err != nil {
		t.Fatal(err)
	}
	p := &domain.Project{ID: "p1", WorkspaceID: "w1", Name: "demo"}
	svc := New(&memProj{p: p}, nil, &memGens{}, &memEvals{}, files, nil, nil)
	ev, err := svc.Evaluate(context.Background(), "u1", "p1", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Status != "completed" || ev.Recommendation == "" || len(ev.Metrics) == 0 {
		t.Fatalf("eval: %+v", ev)
	}
}
