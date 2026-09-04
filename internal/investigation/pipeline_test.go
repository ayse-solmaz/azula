package investigation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/llm"
	"github.com/ayse-solmaz/azula/internal/mcp"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestPipelineMCPSampleAndCouncil(t *testing.T) {
	var mu sync.Mutex
	var models []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{
					{"name": "qwen2.5:1.5b"},
					{"name": "azula-incident"},
				},
			})
			return
		}
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		mu.Lock()
		models = append(models, req.Model)
		mu.Unlock()

		sys, user := "", ""
		for _, m := range req.Messages {
			if m.Role == "system" {
				sys = m.Content
			}
			if m.Role == "user" {
				user = m.Content
			}
		}
		if !strings.Contains(user, "training.log") && !strings.Contains(user, "CUDA") && !strings.Contains(sys, "Judge") && !strings.Contains(sys, "Investigator") && !strings.Contains(sys, "Challenger") && !strings.Contains(sys, "Fast") {
			// still ok — classify may only list filenames
		}

		var payload string
		switch {
		case strings.Contains(sys, "Fast"):
			payload = `{"summary":"Schema warning and GPU OOM","incidentType":"schema_mismatch","confidence":0.61}`
		case strings.Contains(sys, "Deep"):
			payload = `{"rootCause":"Schema drift in customer_status plus batch_size OOM","confidence":0.88,"evidence":[{"file":"training.log","lines":"3-11","excerpt":"CUDA out of memory"}],"suggestedFix":"Fix encoding and reduce batch_size"}`
		case strings.Contains(sys, "Investigator"):
			payload = `{"role":"investigator","hypothesis":"Schema drift in customer_status","confidence":0.89,"evidence":[{"file":"dataset.jsonl","lines":"1-8","excerpt":"mixed int/string customer_status"}]}`
		case strings.Contains(sys, "Challenger"):
			payload = `{"role":"challenger","hypothesis":"Target leakage in pipeline.py","confidence":0.72,"evidence":[{"file":"pipeline.py","lines":"15-19","excerpt":"target_leak = df[label]"}]}`
		default:
			payload = `{"agreements":["Both see data quality issues"],"disagreements":[{"topic":"Root cause","investigator":"schema drift","challenger":"target leakage"}],"finalJudgment":{"mostLikelyCause":"Schema drift in customer_status","confidence":0.91,"recommendedAction":"Fix schema, drop leak, reduce batch_size"}}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": payload},
		})
	}))
	defer srv.Close()

	root := repoRoot(t)
	sample := filepath.Join(root, "samples", "broken-pipeline")
	files := mcp.NewFilesConnector(t.TempDir())
	projectID := "sampleproj"
	seeded, err := files.SeedFromDir(context.Background(), projectID, sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(seeded) < 5 {
		t.Fatalf("expected sample files, got %d", len(seeded))
	}

	projects := newMemProjects()
	_ = projects.Create(context.Background(), &domain.Project{
		ID: projectID, WorkspaceID: "ws1", Name: "sample-broken-pipeline", IsSample: true, Files: seeded,
	})
	invs := newMemInvs()
	cfg := config.Config{
		OllamaBaseURL:  srv.URL,
		ModelAProvider: "ollama",
		ModelAName:     "qwen2.5:1.5b",
		ModelBProvider: "ollama",
		ModelBName:     "azula-incident",
		WorkerSlots:    5,
		RequestTimeout: 10 * time.Second,
	}
	svc := New(projects, invs, newMemConfigs(), files, llm.NewRouter(cfg), cfg)

	inv := &domain.Investigation{
		ID:          "inv1",
		ProjectID:   projectID,
		WorkspaceID: "ws1",
		Prompt:      "Why did training fail?",
		Status:      domain.StatusPending,
		Plan:        DefaultPlan(),
	}
	if err := invs.Create(context.Background(), inv); err != nil {
		t.Fatal(err)
	}
	loaded, _ := invs.GetByID(context.Background(), inv.ID)
	if err := svc.runPipeline(context.Background(), loaded); err != nil {
		t.Fatal(err)
	}
	done, err := invs.GetByID(context.Background(), inv.ID)
	if err != nil {
		t.Fatal(err)
	}

	if done.Status != domain.StatusCompleted {
		t.Fatalf("status=%s err=%s", done.Status, done.ErrorMessage)
	}
	joined := strings.Join(done.FilesAccessed, ",")
	for _, name := range []string{"training.log", "config.yaml", "pipeline.py", "dataset.jsonl"} {
		if !strings.Contains(joined, name) {
			t.Fatalf("MCP did not read %s (accessed=%v)", name, done.FilesAccessed)
		}
	}
	if done.FastResult == nil || done.FastResult.IncidentType == "" {
		t.Fatal("missing fast result")
	}
	if done.DeepResult == nil || len(done.DeepResult.Evidence) == 0 {
		t.Fatal("missing deep evidence")
	}
	if done.CouncilResult == nil {
		t.Fatal("missing council")
	}
	if len(done.CouncilResult.Models) != 2 {
		t.Fatalf("council models=%d", len(done.CouncilResult.Models))
	}
	if len(done.CouncilResult.Agreements) == 0 || len(done.CouncilResult.Disagreements) == 0 {
		t.Fatalf("council agreements/disagreements missing: %+v", done.CouncilResult)
	}
	if done.CouncilResult.FinalJudgment.MostLikelyCause == "" || done.CouncilResult.FinalJudgment.Confidence == 0 {
		t.Fatalf("final judgment incomplete: %+v", done.CouncilResult.FinalJudgment)
	}

	mu.Lock()
	used := append([]string{}, models...)
	mu.Unlock()
	var sawA, sawB bool
	for _, m := range used {
		if m == "qwen2.5:1.5b" {
			sawA = true
		}
		if m == "azula-incident" {
			sawB = true
		}
	}
	if !sawA || !sawB {
		t.Fatalf("expected Model A qwen2.5:1.5b and Model B azula-incident, got %v", used)
	}
	if done.ModelBName == "" || (done.ModelBName != "azula-incident" && done.ModelBName != "azula-incident:latest") {
		t.Fatalf("investigation ModelBName=%s", done.ModelBName)
	}
	if done.FastResult.IncidentType != "schema_mismatch" {
		t.Fatalf("incidentType=%s", done.FastResult.IncidentType)
	}
}

func TestEnsureConfigAttachesIncidentModel(t *testing.T) {
	store := newMemConfigs()
	_ = store.Upsert(context.Background(), &domain.ModelConfig{
		WorkspaceID:    "ws1",
		ModelAProvider: "ollama",
		ModelAName:     "qwen2.5:1.5b",
		ModelBProvider: "ollama",
		ModelBName:     "qwen2.5:1.5b",
	})
	svc := New(newMemProjects(), newMemInvs(), store, nil, llm.NewRouter(config.Config{}), config.Config{
		ModelBProvider: "ollama",
		ModelBName:     "azula-incident",
	})
	got, err := svc.EnsureConfig(context.Background(), "ws1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelBName != "azula-incident" {
		t.Fatalf("model B = %s", got.ModelBName)
	}
}

func TestAttachIncidentModelForcesB(t *testing.T) {
	store := newMemConfigs()
	_ = store.Upsert(context.Background(), &domain.ModelConfig{
		WorkspaceID: "ws1", ModelBName: "custom-old",
	})
	svc := New(newMemProjects(), newMemInvs(), store, nil, llm.NewRouter(config.Config{}), config.Config{
		ModelBProvider: "ollama",
		ModelBName:     "azula-incident",
	})
	got, err := svc.AttachIncidentModel(context.Background(), "ws1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ModelBName != "azula-incident" {
		t.Fatalf("model B = %s", got.ModelBName)
	}
}

func TestLiveOllamaAzulaIncident(t *testing.T) {
	if os.Getenv("AZULA_LIVE_OLLAMA") != "1" {
		t.Skip("set AZULA_LIVE_OLLAMA=1 to run against local Ollama")
	}
	root := repoRoot(t)
	files := mcp.NewFilesConnector(t.TempDir())
	projectID := "liveproj"
	seeded, err := files.SeedFromDir(context.Background(), projectID, filepath.Join(root, "samples", "broken-pipeline"))
	if err != nil {
		t.Fatal(err)
	}
	projects := newMemProjects()
	_ = projects.Create(context.Background(), &domain.Project{
		ID: projectID, WorkspaceID: "ws1", Name: "sample-broken-pipeline", IsSample: true, Files: seeded,
	})
	invs := newMemInvs()
	cfg := config.Config{
		OllamaBaseURL:  "http://localhost:11434",
		ModelAProvider: "ollama",
		ModelAName:     "qwen2.5:1.5b",
		ModelBProvider: "ollama",
		ModelBName:     "azula-incident",
		WorkerSlots:    5,
		RequestTimeout: 90 * time.Second,
	}
	svc := New(projects, invs, newMemConfigs(), files, llm.NewRouter(cfg), cfg)
	inv := &domain.Investigation{
		ID: "live1", ProjectID: projectID, WorkspaceID: "ws1",
		Prompt: "Why did this training pipeline fail?", Status: domain.StatusPending, Plan: DefaultPlan(),
	}
	if err := invs.Create(context.Background(), inv); err != nil {
		t.Fatal(err)
	}
	loaded, _ := invs.GetByID(context.Background(), inv.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if err := svc.runPipeline(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	done, err := invs.GetByID(context.Background(), inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != domain.StatusCompleted {
		t.Fatalf("status=%s err=%s", done.Status, done.ErrorMessage)
	}
	if done.ModelBName != "azula-incident" && done.ModelBName != "azula-incident:latest" {
		t.Fatalf("Model B = %s", done.ModelBName)
	}
	if done.DeepResult == nil || done.DeepResult.RootCause == "" {
		t.Fatal("missing deep result")
	}
	if strings.Contains(done.DeepResult.RootCause, "@@@") {
		t.Fatalf("Model B still emitting garbage: %q", done.DeepResult.RootCause)
	}
	t.Logf("fast=%s deep=%s b=%s", done.FastResult.IncidentType, done.DeepResult.RootCause, done.ModelBName)
}
