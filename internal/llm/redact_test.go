package llm

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	in := "OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz\nbearer " +
		"eyJ" + "hbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.aaaaabbbbbcccccdddddeeeee.fffffggggghhhhhiiiiijjjjj\n"
	out := RedactSecrets(in)
	if strings.Contains(out, "sk-abcdefghijklmnopqrstuvwxyz") {
		t.Fatalf("expected api key redacted: %s", out)
	}
	if strings.Contains(out, "eyJhbGci") {
		t.Fatalf("expected jwt redacted: %s", out)
	}
}

func TestPackFilesWrapsUntrusted(t *testing.T) {
	got := PackFiles(map[string]string{"a.py": "print(1)"})
	if !strings.Contains(got, "UNTRUSTED") {
		t.Fatal(got)
	}
	if !strings.Contains(got, "print(1)") {
		t.Fatal(got)
	}
}
