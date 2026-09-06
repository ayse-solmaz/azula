package investigation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/llm"
	"github.com/ayse-solmaz/azula/internal/mcp"
)

func TestCancelAbortsInFlightLLMAndDoesNotFallback(t *testing.T) {
	var started atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": "qwen2.5:1.5b"}, {"name": "azula-incident"}},
			})
			return
		}
		started.Add(1)
		_, _ = io.ReadAll(r.Body)
		select {
		case <-r.Context().Done():
			http.Error(w, "aborted", http.StatusRequestTimeout)
			return
		case <-time.After(8 * time.Second):
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]string{"content": `{"summary":"should not finish","incidentType":"unknown","confidence":0.1}`},
		})
	}))
	defer srv.Close()

	files := mcp.NewFilesConnector(t.TempDir())
	projectID := "cancelproj"
	_, err := files.SeedFromDir(context.Background(), projectID, filepath.Join(repoRoot(t), "samples", "broken-pipeline"))
	if err != nil {
		t.Fatal(err)
	}
	projects := newMemProjects()
	_ = projects.Create(context.Background(), &domain.Project{
		ID: projectID, WorkspaceID: "ws1", Name: "sample", IsSample: true,
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

	startedInv, err := svc.Start(context.Background(), "u1", projectID, "Why fail?")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for started.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("LLM call never started")
		}
		time.Sleep(20 * time.Millisecond)
	}

	got, err := svc.Cancel(context.Background(), "u1", startedInv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled(got) {
		t.Fatalf("cancel should mark cancelled, got status=%s err=%q", got.Status, got.ErrorMessage)
	}

	wait := time.Now().Add(3 * time.Second)
	var done *domain.Investigation
	for {
		done, err = invs.GetByID(context.Background(), startedInv.ID)
		if err != nil {
			t.Fatal(err)
		}
		if done.Status == domain.StatusFailed && cancelled(done) {
			break
		}
		if done.Status == domain.StatusCompleted {
			t.Fatal("cancel must not complete with fallback Council")
		}
		if time.Now().After(wait) {
			t.Fatalf("pipeline did not settle cancelled: status=%s err=%q mode=%s", done.Status, done.ErrorMessage, done.ExecutionMode)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if done.ExecutionMode == domain.ExecutionFallback {
		t.Fatal("cancel must not apply canned fallback")
	}
	if done.CouncilResult != nil && len(done.CouncilResult.Agreements) > 0 && done.CouncilResult.FinalJudgment.MostLikelyCause != "" {
		if strings.Contains(done.CouncilResult.FinalJudgment.MostLikelyCause, "customer_status") && done.ExecutionMode != domain.ExecutionLive {
			t.Fatal("cancelled run stored canned council judgment")
		}
	}
}
