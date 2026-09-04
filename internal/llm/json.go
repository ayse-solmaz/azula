package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

func ExtractJSON(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("empty model output")
	}
	if i := strings.Index(s, "```"); i >= 0 {
		rest := s[i+3:]
		rest = strings.TrimPrefix(rest, "json")
		rest = strings.TrimPrefix(rest, "JSON")
		if j := strings.Index(rest, "```"); j >= 0 {
			s = strings.TrimSpace(rest[:j])
		}
	}
	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")
	start := startObj
	if startObj < 0 || (startArr >= 0 && startArr < startObj) {
		start = startArr
	}
	if start < 0 {
		return "", fmt.Errorf("no JSON object in model output")
	}
	s = s[start:]
	endObj := strings.LastIndex(s, "}")
	endArr := strings.LastIndex(s, "]")
	end := endObj
	if endArr > end {
		end = endArr
	}
	if end < 0 {
		return "", fmt.Errorf("unterminated JSON")
	}
	return s[:end+1], nil
}

func DecodeModelJSON(raw string, dest any) error {
	js, err := ExtractJSON(raw)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(js), dest); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}
	return nil
}
