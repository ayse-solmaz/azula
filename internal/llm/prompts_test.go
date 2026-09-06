package llm

import (
	"strings"
	"testing"
)

func TestPromptsRankPrimaryAndDiversifyFewShots(t *testing.T) {
	if !strings.Contains(fewShotIncidents, "dropna") || !strings.Contains(fewShotIncidents, "monthly_spend") {
		t.Fatal("fast few-shots must include the missing-value / dropna example")
	}
	if !strings.Contains(fewShotIncidents, "data_quality") {
		t.Fatal("fast few-shots must map dropna/NaN to data_quality")
	}
	if !strings.Contains(SysFast, "dominant") {
		t.Fatal("fast prompt should prefer a dominant type")
	}
	for _, p := range []string{SysDeep, SysInvestigator, SysJudge} {
		if !strings.Contains(p, "primary") {
			t.Fatalf("prompt must rank primary vs secondary: %s", p[:min(len(p), 60)])
		}
	}
	if !strings.Contains(SysDeep, "file:line") && !strings.Contains(fileLineEvidence, "file:line") {
		t.Fatal("deep analysis must require file:line evidence")
	}
	if strings.Contains(SysChallenger, "You MUST disagree") {
		t.Fatal("challenger must not force a disagreement on every run")
	}
	if !strings.Contains(SysChallenger, "dropna") {
		t.Fatal("challenger should recognize a dominating dropna / data-quality case")
	}
	if !strings.Contains(classifyUser("why", "training.log"), "data_quality") {
		t.Fatal("fast schema must include data_quality")
	}
}
