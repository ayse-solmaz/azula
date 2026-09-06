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
	var invUser, chalUser string
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
		if strings.Contains(sys, "Azula Investigator") {
			mu.Lock()
			invUser = user
			mu.Unlock()
		}
		if strings.Contains(sys, "You are the Challenger") {
			mu.Lock()
			chalUser = user
			mu.Unlock()
		}

		var payload string
		switch {
		case strings.Contains(sys, "Fast"):
			payload = `{"summary":"Schema warning and GPU OOM","incidentType":"schema_mismatch","confidence":0.61}`
		case strings.Contains(sys, "Deep"):
			payload = `{"rootCause":"Schema drift in customer_status plus batch_size OOM","confidence":0.88,"evidence":[{"file":"training.log","lines":"3-11","excerpt":"CUDA out of memory"}],"suggestedFix":"Fix encoding and reduce batch_size"}`
		case strings.Contains(sys, "Azula Investigator"):
			payload = `{"role":"investigator","hypothesis":"Schema drift in customer_status","confidence":0.89,"evidence":[{"file":"dataset.jsonl","lines":"1-8","excerpt":"mixed int/string customer_status"}]}`
		case strings.Contains(sys, "You are the Challenger"):
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
	if done.CouncilResult.Aggregation == "" {
		t.Fatal("council aggregation missing")
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
	if done.ExecutionMode != domain.ExecutionLive {
		t.Fatalf("executionMode=%s fallback=%v", done.ExecutionMode, done.FallbackStages)
	}
	if done.EscalationReason == "" || !strings.Contains(strings.ToLower(done.EscalationReason), "deep look") {
		t.Fatalf("escalationReason=%q", done.EscalationReason)
	}
	mu.Lock()
	gotInv, gotChal := invUser, chalUser
	mu.Unlock()
	if !strings.Contains(gotInv, "Prior analysis") {
		t.Fatalf("investigator should reuse Deep, got %q", clip(gotInv, 240))
	}
	if strings.Contains(gotInv, "=== BEGIN UNTRUSTED FILE dataset.jsonl") {
		t.Fatal("investigator should not re-read the full Deep file dump")
	}
	if !strings.Contains(gotChal, "pipeline.py") && !strings.Contains(gotChal, "training.log") {
		t.Fatalf("challenger should still see compact files, got %q", clip(gotChal, 240))
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
		OllamaBaseURL:        "http://localhost:11434",
		ModelAProvider:       "ollama",
		ModelAName:           "qwen2.5:1.5b",
		ModelBProvider:       "ollama",
		ModelBName:           "azula-incident",
		WorkerSlots:          5,
		RequestTimeout:       90 * time.Second,
		ForceCouncilOnSample: true,
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
	if done.CouncilResult == nil {
		t.Fatal("sample live run must reach Council")
	}
	if strings.Contains(done.DeepResult.RootCause, "@@@") {
		t.Fatalf("Model B still emitting garbage: %q", done.DeepResult.RootCause)
	}
	t.Logf("fast=%s deep=%s b=%s mode=%s agg=%s review=%v", done.FastResult.IncidentType, done.DeepResult.RootCause, done.ModelBName, done.ExecutionMode, func() string {
		if done.CouncilResult == nil {
			return "none"
		}
		return done.CouncilResult.Aggregation
	}(), done.CouncilResult != nil && done.CouncilResult.NeedsReview)
}

func TestHighConfidenceStillRunsDeepAndCouncil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{"name": "qwen2.5:1.5b"}}})
			return
		}
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		sys := ""
		for _, m := range req.Messages {
			if m.Role == "system" {
				sys = m.Content
			}
		}
		payload := `{"summary":"Clear CUDA OOM","incidentType":"memory_gpu","confidence":0.86}`
		switch {
		case strings.Contains(sys, "Deep"):
			payload = `{"rootCause":"CUDA OOM from batch_size","confidence":0.9,"evidence":[{"file":"training.log","lines":"1-8","excerpt":"CUDA out of memory"}],"suggestedFix":"Reduce batch_size"}`
		case strings.Contains(sys, "Azula Investigator"):
			payload = `{"role":"investigator","hypothesis":"GPU OOM from batch_size","confidence":0.9,"evidence":[{"file":"training.log","lines":"1-8","excerpt":"CUDA out of memory"}]}`
		case strings.Contains(sys, "You are the Challenger"):
			payload = `{"role":"challenger","hypothesis":"Memory leak in the training loop","confidence":0.6,"evidence":[{"file":"config.yaml","lines":"1-4","excerpt":"batch_size"}]}`
		case strings.Contains(sys, "Judge"):
			payload = `{"agreements":["Both mention memory pressure"],"disagreements":[{"topic":"Root cause","investigator":"OOM","challenger":"leak"}],"finalJudgment":{"mostLikelyCause":"CUDA OOM from batch_size","confidence":0.88,"recommendedAction":"Reduce batch_size"}}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": payload},
		})
	}))
	defer srv.Close()

	files := mcp.NewFilesConnector(t.TempDir())
	projectID := "skipproj"
	_, err := files.SeedFromDir(context.Background(), projectID, filepath.Join(repoRoot(t), "samples", "goldset", "gpu-oom"))
	if err != nil {
		t.Fatal(err)
	}
	projects := newMemProjects()
	_ = projects.Create(context.Background(), &domain.Project{ID: projectID, WorkspaceID: "ws1", Name: "gpu"})
	invs := newMemInvs()
	cfg := config.Config{OllamaBaseURL: srv.URL, ModelAProvider: "ollama", ModelAName: "qwen2.5:1.5b", ModelBProvider: "ollama", ModelBName: "azula-incident", WorkerSlots: 5, RequestTimeout: 10 * time.Second, ForceCouncilOnSample: true}
	svc := New(projects, invs, newMemConfigs(), files, llm.NewRouter(cfg), cfg)
	inv := &domain.Investigation{ID: "skip1", ProjectID: projectID, WorkspaceID: "ws1", Prompt: "why", Status: domain.StatusPending, Plan: DefaultPlan()}
	_ = invs.Create(context.Background(), inv)
	loaded, _ := invs.GetByID(context.Background(), inv.ID)
	if err := svc.runPipeline(context.Background(), loaded); err != nil {
		t.Fatal(err)
	}
	done, _ := invs.GetByID(context.Background(), inv.ID)
	if done.Status != domain.StatusCompleted {
		t.Fatalf("status=%s", done.Status)
	}
	if done.DeepResult == nil || done.CouncilResult == nil {
		t.Fatal("high confidence must still run deep and council")
	}
	if len(done.FilesAccessed) == 0 {
		t.Fatalf("deep stage must read files, accessed=%v", done.FilesAccessed)
	}
	if done.ExecutionMode != domain.ExecutionLive {
		t.Fatalf("mode=%s", done.ExecutionMode)
	}
}

func TestSampleForcesCouncilDespiteHighFast(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{"name": "qwen2.5:1.5b"}, {"name": "azula-incident"}}})
			return
		}
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(raw, &req)
		sys := ""
		for _, m := range req.Messages {
			if m.Role == "system" {
				sys = m.Content
			}
		}
		payload := `{"agreements":["Both see data quality issues"],"disagreements":[{"topic":"Root cause","investigator":"schema drift","challenger":"target leakage"}],"finalJudgment":{"mostLikelyCause":"Schema drift in customer_status","confidence":0.91,"recommendedAction":"Fix schema"}}`
		switch {
		case strings.Contains(sys, "Fast"):
			payload = `{"summary":"Clear schema drift","incidentType":"schema_mismatch","confidence":0.90}`
		case strings.Contains(sys, "Deep"):
			payload = `{"rootCause":"Schema drift in customer_status","confidence":0.88,"evidence":[{"file":"training.log","lines":"3-11","excerpt":"unseen categories"}],"suggestedFix":"Re-encode"}`
		case strings.Contains(sys, "Azula Investigator"):
			payload = `{"role":"investigator","hypothesis":"Schema drift in customer_status","confidence":0.89,"evidence":[{"file":"dataset.jsonl","lines":"1-8","excerpt":"mixed types"}]}`
		case strings.Contains(sys, "You are the Challenger"):
			payload = `{"role":"challenger","hypothesis":"Target leakage in pipeline.py","confidence":0.72,"evidence":[{"file":"pipeline.py","lines":"15-19","excerpt":"target_leak"}]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": payload}})
	}))
	defer srv.Close()

	files := mcp.NewFilesConnector(t.TempDir())
	projectID := "sampleforce"
	_, err := files.SeedFromDir(context.Background(), projectID, filepath.Join(repoRoot(t), "samples", "broken-pipeline"))
	if err != nil {
		t.Fatal(err)
	}
	projects := newMemProjects()
	_ = projects.Create(context.Background(), &domain.Project{
		ID: projectID, WorkspaceID: "ws1", Name: "sample-broken-pipeline", IsSample: true,
	})
	invs := newMemInvs()
	cfg := config.Config{
		OllamaBaseURL: srv.URL, ModelAProvider: "ollama", ModelAName: "qwen2.5:1.5b",
		ModelBProvider: "ollama", ModelBName: "azula-incident", WorkerSlots: 5, RequestTimeout: 10 * time.Second,
		ForceCouncilOnSample: true,
	}
	svc := New(projects, invs, newMemConfigs(), files, llm.NewRouter(cfg), cfg)
	inv := &domain.Investigation{ID: "force1", ProjectID: projectID, WorkspaceID: "ws1", Prompt: "why", Status: domain.StatusPending, Plan: DefaultPlan()}
	_ = invs.Create(context.Background(), inv)
	loaded, _ := invs.GetByID(context.Background(), inv.ID)
	if err := svc.runPipeline(context.Background(), loaded); err != nil {
		t.Fatal(err)
	}
	done, _ := invs.GetByID(context.Background(), inv.ID)
	if done.Status != domain.StatusCompleted {
		t.Fatalf("status=%s err=%s", done.Status, done.ErrorMessage)
	}
	if done.CouncilResult == nil || done.DeepResult == nil {
		t.Fatal("sample project must run Deep and Council even when Fast is ≥ 70%")
	}
	if !strings.Contains(strings.ToLower(done.EscalationReason), "deep look") {
		t.Fatalf("reason=%q", done.EscalationReason)
	}
}

func TestFallbackExecutionMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
		http.Error(w, "llm down", http.StatusBadGateway)
	}))
	defer srv.Close()

	files := mcp.NewFilesConnector(t.TempDir())
	projectID := "fbproj"
	_, err := files.SeedFromDir(context.Background(), projectID, filepath.Join(repoRoot(t), "samples", "broken-pipeline"))
	if err != nil {
		t.Fatal(err)
	}
	projects := newMemProjects()
	_ = projects.Create(context.Background(), &domain.Project{ID: projectID, WorkspaceID: "ws1", Name: "sample"})
	invs := newMemInvs()
	cfg := config.Config{OllamaBaseURL: srv.URL, ModelAProvider: "ollama", ModelAName: "qwen2.5:1.5b", ModelBProvider: "ollama", ModelBName: "azula-incident", WorkerSlots: 5, RequestTimeout: 2 * time.Second}
	svc := New(projects, invs, newMemConfigs(), files, llm.NewRouter(cfg), cfg)
	inv := &domain.Investigation{ID: "fb1", ProjectID: projectID, WorkspaceID: "ws1", Prompt: "why", Status: domain.StatusPending, Plan: DefaultPlan()}
	_ = invs.Create(context.Background(), inv)
	loaded, _ := invs.GetByID(context.Background(), inv.ID)
	if err := svc.runPipeline(context.Background(), loaded); err != nil {
		t.Fatal(err)
	}
	done, _ := invs.GetByID(context.Background(), inv.ID)
	if done.Status != domain.StatusCompleted {
		t.Fatalf("status=%s err=%s", done.Status, done.ErrorMessage)
	}
	if done.ExecutionMode != domain.ExecutionFallback {
		t.Fatalf("mode=%s stages=%v", done.ExecutionMode, done.FallbackStages)
	}
	if done.CouncilResult == nil || done.CouncilResult.FinalJudgment.Confidence == 0 {
		t.Fatal("fallback council missing")
	}
	if !strings.Contains(strings.ToLower(done.EscalationReason), "deep look") {
		t.Fatalf("fallback should continue to deep look, reason=%q", done.EscalationReason)
	}
}
