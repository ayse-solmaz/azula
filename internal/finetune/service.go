package finetune

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/llm"
	"github.com/google/uuid"
)

const adapterRel = "adapters/azula-incident"

type Service struct {
	jobs    domain.FineTuneRepository
	configs domain.ModelConfigRepository
	cfg     config.Config
}

func New(jobs domain.FineTuneRepository, configs domain.ModelConfigRepository, cfg config.Config) *Service {
	return &Service{jobs: jobs, configs: configs, cfg: cfg}
}

func (s *Service) List(ctx context.Context, workspaceID string) ([]domain.FineTuneJob, error) {
	return s.jobs.ListByWorkspace(ctx, workspaceID)
}

func (s *Service) Start(ctx context.Context, userID, workspaceID string) (*domain.FineTuneJob, error) {
	now := time.Now().UTC()
	job := &domain.FineTuneJob{
		ID: uuid.NewString(), WorkspaceID: workspaceID, UserID: userID,
		Status: "queued", CreatedAt: now, UpdatedAt: now,
	}
	if llm.AdapterOnDisk() {
		job.Status = "ready"
		job.AdapterPath = filepath.ToSlash(adapterRel)
		if err := s.jobs.Create(ctx, job); err != nil {
			return nil, err
		}
		if _, err := s.AttachModelB(ctx, workspaceID); err != nil {
			log.Printf("finetune attach model B: %v", err)
		}
		log.Printf("finetune job %s ready (adapter on disk)", job.ID)
		return job, nil
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, err
	}
	go s.run(job.ID, workspaceID)
	return job, nil
}

func (s *Service) AttachModelB(ctx context.Context, workspaceID string) (*domain.ModelConfig, error) {
	if s.configs == nil {
		return nil, nil
	}
	want := s.cfg.ModelBName
	if want == "" {
		want = "azula-incident"
	}
	provider := s.cfg.ModelBProvider
	if provider == "" {
		provider = "ollama"
	}
	cfg, err := s.configs.GetByWorkspace(ctx, workspaceID)
	if err == domain.ErrNotFound {
		c := llm.DefaultModelConfig(s.cfg, workspaceID)
		c.ID = uuid.NewString()
		now := time.Now().UTC()
		c.CreatedAt = now
		c.UpdatedAt = now
		c.ModelBProvider = provider
		c.ModelBName = want
		if err := s.configs.Upsert(ctx, &c); err != nil {
			return nil, err
		}
		return &c, nil
	}
	if err != nil {
		return nil, err
	}
	cfg.ModelBProvider = provider
	cfg.ModelBName = want
	if err := s.configs.Upsert(ctx, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *Service) run(jobID, workspaceID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()
	job, err := s.jobs.GetByID(ctx, jobID)
	if err != nil {
		return
	}
	job.Status = "training"
	_ = s.jobs.Update(ctx, job)
	if s.cfg.FinetuneDemo {
		time.Sleep(2 * time.Second)
		job.Status = "ready"
		job.AdapterPath = filepath.ToSlash(adapterRel)
		_ = os.MkdirAll(job.AdapterPath, 0o755)
		_ = s.jobs.Update(ctx, job)
		_, _ = s.AttachModelB(ctx, workspaceID)
		log.Printf("finetune demo job %s ready", jobID)
		return
	}
	script := filepath.Join("services", "trainer", "train.py")
	data := filepath.Join(s.cfg.FinetuneDir, "incident_pairs.jsonl")
	out := filepath.Join("adapters", jobID)
	cmd := exec.CommandContext(ctx, "python", script, "--dataset", data, "--output", out)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		_ = s.jobs.Update(ctx, job)
		return
	}
	job.Status = "ready"
	job.AdapterPath = filepath.ToSlash(out)
	_ = s.jobs.Update(ctx, job)
	_, _ = s.AttachModelB(ctx, workspaceID)
}
