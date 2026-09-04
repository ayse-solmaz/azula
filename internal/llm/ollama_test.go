package llm

import "testing"

func TestPickModelB(t *testing.T) {
	available := []string{"qwen2.5:1.5b", "azula-incident:latest"}
	if got := PickModelB(available, "azula-incident"); got != "azula-incident:latest" {
		t.Fatalf("got %s", got)
	}
	if got := PickModelB([]string{"qwen2.5:1.5b"}, "azula-incident"); got != "qwen2.5:1.5b" {
		t.Fatalf("missing incident should fall back, got %s", got)
	}
	if got := PickModelB(available, "custom-deep"); got != "azula-incident:latest" {
		t.Fatalf("unavailable custom should use incident, got %s", got)
	}
	if got := PickModelB([]string{"qwen2.5:1.5b", "custom-deep"}, "custom-deep"); got != "custom-deep" {
		t.Fatalf("available custom should win, got %s", got)
	}
}
