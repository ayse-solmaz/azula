package investigation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/eval"
	"github.com/ayse-solmaz/azula/internal/llm"
)

func TestBrokenPipelineFilesRankAndPack(t *testing.T) {
	root := filepath.Join(repoRoot(t), "samples", "broken-pipeline")
	names := []string{"training.log", "config.yaml", "pipeline.py", "dataset.jsonl", "metrics.json", "README.md"}
	contents := map[string]string{}
	for _, n := range names {
		raw, err := os.ReadFile(filepath.Join(root, n))
		if err != nil {
			if n == "README.md" {
				continue
			}
			t.Fatal(err)
		}
		contents[n] = string(raw)
	}
	ranked := rankNames([]string{"training.log", "config.yaml", "pipeline.py", "dataset.jsonl", "metrics.json"}, "Why did training fail?", "schema_mismatch")
	if ranked[0] != "training.log" && ranked[0] != "config.yaml" && ranked[0] != "pipeline.py" && ranked[0] != "dataset.jsonl" {
		t.Fatalf("expected a primary evidence file first, got %v", ranked)
	}
	packed := llm.PackFiles(contents)
	for _, needle := range []string{"CUDA out of memory", "customer_status", "target_leak"} {
		if !strings.Contains(packed, needle) {
			t.Fatalf("packed prompt missing %q", needle)
		}
	}
}

func TestBrokenPipelineGoldKeywords(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "samples", "broken-pipeline", "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	cases, err := eval.LoadCases(filepath.Join(repoRoot(t), "samples"))
	if err != nil {
		t.Fatal(err)
	}
	var gold eval.Case
	for _, c := range cases {
		if c.ID == "broken-pipeline" {
			gold = c
		}
	}
	if gold.ID == "" {
		t.Fatalf("expected.json not loaded (%s)", raw[:min(len(raw), 40)])
	}
	if gold.Aggregation != llm.AggregationDisagreement || !gold.NeedsReview {
		t.Fatalf("demo fixture aggregation=%s needsReview=%v", gold.Aggregation, gold.NeedsReview)
	}
	if gold.ModelAName == "" || gold.ModelBName == "" {
		t.Fatal("demo fixture must name Model A and Model B")
	}
	// Composite demo judgment — what the onboarding Council is supposed to name.
	cause := "Schema drift in customer_status, CUDA OOM from batch_size, and target leak of the label"
	action := "Re-encode customer_status, reduce batch_size, drop target_leak"
	if eval.KeywordRecall(cause, gold) < 1 {
		t.Fatalf("demo judgment should hit all cause keywords, recall=%f", eval.KeywordRecall(cause, gold))
	}
	if eval.CouncilScore(gold.IncidentType, cause, action, gold) < eval.FastScore("unknown", "job failed", gold) {
		t.Fatal("composite council text should beat a weak fast baseline")
	}
}

func TestBrokenPipelineCouncilAggregation(t *testing.T) {
	res := &domain.CouncilResult{
		Models: []domain.CouncilModel{
			{
				Role: "investigator", Hypothesis: "Schema drift in customer_status mixed types",
				Confidence: 0.89, Evidence: []domain.Evidence{{File: "training.log"}, {File: "dataset.jsonl"}},
			},
			{
				Role: "challenger", Hypothesis: "Target leak copies label into features",
				Confidence: 0.74, Evidence: []domain.Evidence{{File: "pipeline.py"}},
			},
		},
		FinalJudgment: domain.FinalJudgment{RecommendedAction: "Fix schema and drop leak"},
	}
	llm.ApplyAggregation(res, false)
	if res.Aggregation != llm.AggregationDisagreement {
		t.Fatalf("composite sample should disagree, got %s", res.Aggregation)
	}
	if !res.NeedsReview {
		t.Fatal("disagreement must flag human/API review")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
