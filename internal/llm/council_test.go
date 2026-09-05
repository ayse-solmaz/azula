package llm

import (
	"strings"
	"testing"

	"github.com/ayse-solmaz/azula/internal/domain"
)

func TestApplyAggregationEchoChamber(t *testing.T) {
	res := &domain.CouncilResult{
		Models: []domain.CouncilModel{
			{Role: "investigator", Hypothesis: "Schema drift in customer_status", Confidence: 0.8, Evidence: []domain.Evidence{{File: "a"}}},
			{Role: "challenger", Hypothesis: "Schema drift customer_status column", Confidence: 0.78, Evidence: []domain.Evidence{{File: "a"}}},
		},
		FinalJudgment: domain.FinalJudgment{MostLikelyCause: "x", Confidence: 0.99, RecommendedAction: "fix"},
	}
	ApplyAggregation(res, true)
	if res.Aggregation != AggregationEchoChamber || !res.NeedsReview {
		t.Fatalf("echo chamber: %+v", res)
	}
}

func TestApplyAggregationDisagreement(t *testing.T) {
	res := &domain.CouncilResult{
		Models: []domain.CouncilModel{
			{Role: "investigator", Hypothesis: "Schema drift in customer_status", Confidence: 0.89, Evidence: []domain.Evidence{{File: "dataset.jsonl"}, {File: "training.log"}}},
			{Role: "challenger", Hypothesis: "Target leakage copies the label", Confidence: 0.72, Evidence: []domain.Evidence{{File: "pipeline.py"}}},
		},
		FinalJudgment: domain.FinalJudgment{MostLikelyCause: "judge text", Confidence: 0.91, RecommendedAction: "fix both"},
	}
	ApplyAggregation(res, false)
	if res.Aggregation != AggregationDisagreement || !res.NeedsReview {
		t.Fatalf("disagreement: %+v", res)
	}
	if !strings.Contains(strings.ToLower(res.FinalJudgment.MostLikelyCause), "schema") {
		t.Fatalf("weighted vote should prefer investigator: %s", res.FinalJudgment.MostLikelyCause)
	}
	if res.FinalJudgment.Confidence >= 0.91 {
		t.Fatalf("disagreement should dampen confidence, got %f", res.FinalJudgment.Confidence)
	}
}

func TestApplyAggregationConsensus(t *testing.T) {
	res := &domain.CouncilResult{
		Models: []domain.CouncilModel{
			{Role: "investigator", Hypothesis: "CUDA OOM batch_size too large for GPU", Confidence: 0.7, Evidence: []domain.Evidence{{File: "training.log"}}},
			{Role: "challenger", Hypothesis: "CUDA OOM batch_size too large on GPU", Confidence: 0.68, Evidence: []domain.Evidence{{File: "config.yaml"}}},
		},
		FinalJudgment: domain.FinalJudgment{RecommendedAction: "reduce batch"},
	}
	ApplyAggregation(res, false)
	if res.Aggregation != AggregationConsensus || res.NeedsReview {
		t.Fatalf("consensus: %+v", res)
	}
	if res.FinalJudgment.Confidence <= 0.7 {
		t.Fatalf("expected boost, got %f", res.FinalJudgment.Confidence)
	}
}

func TestCompactLogKeepsErrors(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("info line filler padding for the log\n")
	}
	b.WriteString("ERROR CUDA out of memory at epoch 3\n")
	packed := CompactFile("training.log", b.String(), 4000)
	if !strings.Contains(packed, "CUDA out of memory") {
		t.Fatalf("error line dropped: %s", packed[:min(len(packed), 200)])
	}
	if !strings.Contains(packed, "hierarchical log") {
		t.Fatal("expected hierarchical marker")
	}
}
