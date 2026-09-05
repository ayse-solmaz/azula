package llm

import "testing"

func TestModelFamilyAndPickDiverse(t *testing.T) {
	if ModelFamily("azula-incident:latest") != "qwen" {
		t.Fatal("incident merge is qwen-family")
	}
	if ModelFamily("qwen2.5:1.5b") != "qwen" {
		t.Fatal("qwen family")
	}
	got := PickDiverse([]string{"qwen2.5:1.5b", "azula-incident", "mistral:latest"}, "azula-incident")
	if got != "mistral:latest" {
		t.Fatalf("expected mistral, got %s", got)
	}
	if PickDiverse([]string{"qwen2.5:1.5b", "azula-incident"}, "qwen2.5:1.5b") != "" {
		t.Fatal("no diverse model available")
	}
}

func TestHypothesisOverlapAndAggregation(t *testing.T) {
	if hypothesisOverlap("schema drift in customer_status", "schema drift customer_status encoding") < 0.4 {
		t.Fatal("expected lexical overlap")
	}
	if hypothesisOverlap("cuda oom batch size", "target leakage in pipeline") > 0.2 {
		t.Fatal("expected low overlap")
	}
}
