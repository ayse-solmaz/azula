package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
)

func TestPriorAnalysisBriefAndCitedSnippets(t *testing.T) {
	fast := &domain.FastResult{IncidentType: "schema_mismatch", Confidence: 0.64, Summary: "schema warning"}
	deep := &domain.DeepResult{
		RootCause:    "Schema drift in customer_status",
		Confidence:   0.88,
		SuggestedFix: "re-encode",
		Evidence:     []domain.Evidence{{File: "training.log", Lines: "3-11", Excerpt: "unseen categories"}},
	}
	brief := priorAnalysisBrief(fast, deep)
	if !strings.Contains(brief, "schema_mismatch") || !strings.Contains(brief, "customer_status") {
		t.Fatalf("brief missing prior analysis: %s", brief)
	}
	files := map[string]string{
		"training.log":  "ERROR CUDA out of memory",
		"dataset.jsonl": strings.Repeat("UNIQUE_DATASET_ROW ", 200),
		"pipeline.py":   `df["target_leak"] = df["label"]`,
	}
	invUser := investigatorUser("Why fail?", fast, deep, files)
	if !strings.Contains(invUser, "Prior analysis") {
		t.Fatal("investigator should reuse deep, not start from scratch")
	}
	if strings.Contains(invUser, "UNIQUE_DATASET_ROW") {
		t.Fatal("investigator must not re-read uncited files")
	}
	if !strings.Contains(invUser, "training.log") {
		t.Fatal("investigator should see cited file snippet")
	}
	chalUser := challengerUser("Why fail?", PackFilesBudget(files, 8000), fast, deep)
	if !strings.Contains(chalUser, "UNIQUE_DATASET_ROW") {
		t.Fatal("challenger still needs compact files to find an alternative")
	}
}

func TestRunCouncilParallelCompactPrompts(t *testing.T) {
	var mu sync.Mutex
	type call struct {
		sys   string
		user  string
		start time.Time
		model string
	}
	var calls []call
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]string{{"name": "qwen2.5:1.5b"}, {"name": "azula-incident"}},
			})
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
		sys, user := "", ""
		for _, m := range req.Messages {
			if m.Role == "system" {
				sys = m.Content
			}
			if m.Role == "user" {
				user = m.Content
			}
		}
		mu.Lock()
		calls = append(calls, call{sys: sys, user: user, start: time.Now(), model: req.Model})
		mu.Unlock()
		if strings.Contains(sys, "Investigator") || strings.Contains(sys, "Challenger") {
			time.Sleep(180 * time.Millisecond)
		}
		payload := `{"agreements":["data quality"],"disagreements":[{"topic":"Root cause","investigator":"schema","challenger":"leak"}],"finalJudgment":{"mostLikelyCause":"schema drift","confidence":0.8,"recommendedAction":"fix"}}`
		switch {
		case strings.Contains(sys, "Azula Investigator"):
			payload = `{"role":"investigator","hypothesis":"Schema drift","confidence":0.89,"evidence":[{"file":"training.log","lines":"1-2","excerpt":"schema"}]}`
		case strings.Contains(sys, "You are the Challenger"):
			payload = `{"role":"challenger","hypothesis":"Target leakage","confidence":0.71,"evidence":[{"file":"pipeline.py","lines":"1-2","excerpt":"leak"}]}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"message": map[string]string{"content": payload}})
	}))
	defer srv.Close()

	cfg := config.Config{
		OllamaBaseURL:       srv.URL,
		CouncilFast:         true,
		CouncilContextChars: 8000,
		CouncilAgentTimeout: 5 * time.Second,
		CouncilMaxTokens:    512,
		RequestTimeout:      8 * time.Second,
	}
	r := NewRouter(cfg)
	mcfg := DefaultModelConfig(cfg, "ws")
	files := map[string]string{
		"training.log":  "WARNING unseen categories customer_status\nERROR CUDA out of memory",
		"dataset.jsonl": strings.Repeat("UNIQUE_DATASET_ROW ", 400),
		"pipeline.py":   `df["target_leak"] = df["label"]`,
	}
	fast := &domain.FastResult{IncidentType: "schema_mismatch", Confidence: 0.61, Summary: "schema + oom"}
	deep := &domain.DeepResult{
		RootCause: "Schema drift in customer_status", Confidence: 0.88, SuggestedFix: "re-encode",
		Evidence: []domain.Evidence{{File: "training.log", Lines: "1-2", Excerpt: "unseen categories"}},
	}
	var partials int
	res, err := r.RunCouncilProgress(context.Background(), mcfg, domain.InvestigationContext{
		Prompt: "Why fail?", FileContents: files,
	}, fast, deep, func(*domain.CouncilResult) { partials++ })
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || len(res.Models) != 2 || len(res.Agreements) == 0 {
		t.Fatalf("council result: %+v", res)
	}
	if partials < 1 {
		t.Fatal("expected at least one partial UI update")
	}

	mu.Lock()
	got := append([]call{}, calls...)
	mu.Unlock()
	var invStart, chalStart time.Time
	var invUser, chalUser string
	for _, c := range got {
		if strings.Contains(c.sys, "Azula Investigator") {
			invStart, invUser = c.start, c.user
		}
		if strings.Contains(c.sys, "You are the Challenger") {
			chalStart, chalUser = c.start, c.user
		}
	}
	if invStart.IsZero() || chalStart.IsZero() {
		t.Fatalf("missing agent calls: %+v", got)
	}
	gap := invStart.Sub(chalStart)
	if gap < 0 {
		gap = -gap
	}
	if gap > 150*time.Millisecond {
		t.Fatalf("investigator and challenger should start together, gap=%s", gap)
	}
	if strings.Contains(invUser, "UNIQUE_DATASET_ROW") {
		t.Fatal("investigator prompt re-read uncited dataset")
	}
	if !strings.Contains(invUser, "Prior analysis") || !strings.Contains(invUser, "customer_status") {
		t.Fatalf("investigator should reuse deep: %s", invUser[:min(len(invUser), 400)])
	}
	if !strings.Contains(chalUser, "pipeline.py") {
		t.Fatal("challenger should still see compact files")
	}
}
