package llm

import "testing"

func TestExtractJSONFromFence(t *testing.T) {
	raw := "here\n```json\n{\"summary\":\"ok\",\"confidence\":0.9}\n```\n"
	js, err := ExtractJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	var dto struct {
		Summary    string  `json:"summary"`
		Confidence float64 `json:"confidence"`
	}
	if err := DecodeModelJSON(js, &dto); err != nil {
		t.Fatal(err)
	}
	if dto.Summary != "ok" {
		t.Fatalf("got %q", dto.Summary)
	}
}
