package llm

import (
	"testing"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
)

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

func TestRouteCouncilFastKeepsSmallChallenger(t *testing.T) {
	available := []string{"qwen2.5:1.5b", "azula-incident:latest", "mistral:latest"}
	cfg := domain.ModelConfig{ModelAName: "qwen2.5:1.5b", ModelBName: "azula-incident"}
	fast := NewRouter(config.Config{CouncilFast: true}).routeCouncil(cfg, available)
	if fast.ChallengerName != "qwen2.5:1.5b" || fast.ChallengerSlot != "A" {
		t.Fatalf("fast council should keep small challenger: %+v", fast)
	}
	quality := NewRouter(config.Config{CouncilFast: false}).routeCouncil(cfg, available)
	if quality.ChallengerName != "mistral:latest" || quality.ChallengerSlot != "B" {
		t.Fatalf("quality council should pick diverse family: %+v", quality)
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
