package eval

import (
	"path/filepath"
	"runtime"
	"testing"
)

func goldRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "samples", "goldset"))
}

func TestGoldSetCouncilBeatsFast(t *testing.T) {
	cases, err := LoadCases(goldRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 4 {
		t.Fatalf("need at least 4 gold incidents, got %d", len(cases))
	}

	// These maps are fixtures: this test does not call a model.
	// It only checks that a Council-shaped answer with gold keywords beats a vague Fast summary.
	weakFast := map[string]struct{ typ, summary string }{
		"schema-drift": {"unknown", "Training looks unstable."},
		"gpu-oom":      {"unknown", "Job stopped early."},
		"target-leak":  {"config_error", "Check hyperparameters."},
		"nan-impute":   {"unknown", "Validation metric dropped."},
	}
	// Council judgment that actually names the gold cause.
	council := map[string]struct{ cause, action string }{
		"schema-drift": {
			"Schema drift: customer_status has unseen mixed types",
			"Re-encode customer_status and freeze the schema",
		},
		"gpu-oom": {
			"CUDA OOM from oversized batch_size on 8GB GPU",
			"Reduce batch_size and retry",
		},
		"target-leak": {
			"Target leakage: label copied into features as target_leak",
			"Remove the leaky column and retrain",
		},
		"nan-impute": {
			"dropna on monthly_spend NaNs flips class balance and collapses val AUC",
			"Median-impute monthly_spend instead of dropping rows",
		},
	}

	fastWins, councilWins := 0, 0
	gotAgg := map[string]string{}
	for _, c := range cases {
		gotAgg[c.ID] = c.Aggregation
		f := weakFast[c.ID]
		j := council[c.ID]
		if f.typ == "" || j.cause == "" {
			t.Fatalf("missing fixture mapping for %s", c.ID)
		}
		fs := FastScore(f.typ, f.summary, c)
		cs := CouncilScore(c.IncidentType, j.cause, j.action, c)
		if cs > fs {
			councilWins++
		} else {
			fastWins++
		}
		t.Logf("%s fast=%.2f council=%.2f", c.ID, fs, cs)
	}
	if councilWins <= fastWins {
		t.Fatalf("council should beat fast on gold set (council %d vs fast %d)", councilWins, fastWins)
	}
	if gotAgg["schema-drift"] != "consensus" || gotAgg["gpu-oom"] != "echo_chamber" || gotAgg["target-leak"] != "disagreement" || gotAgg["nan-impute"] != "consensus" {
		t.Fatalf("gold aggregation fixtures: %+v", gotAgg)
	}
}

func TestKeywordScore(t *testing.T) {
	if KeywordScore("CUDA OOM; batch_size 256", []string{"oom", "batch_size", "cuda"}) < 1 {
		t.Fatal("expected all keywords")
	}
	if TypeMatch("schema_mismatch", "SCHEMA_MISMATCH") != 1 {
		t.Fatal("type match is case-insensitive")
	}
}
