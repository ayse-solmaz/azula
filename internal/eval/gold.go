package eval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Case struct {
	ID            string   `json:"id"`
	IncidentType  string   `json:"incidentType"`
	CauseKeywords []string `json:"causeKeywords"`
	FixKeywords   []string `json:"fixKeywords"`
	Aggregation   string   `json:"aggregation,omitempty"`
	NeedsReview   bool     `json:"needsReview,omitempty"`
	ModelAName    string   `json:"modelAName,omitempty"`
	ModelBName    string   `json:"modelBName,omitempty"`
	Dir           string   `json:"-"`
}

func LoadCases(root string) ([]Case, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []Case
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		raw, err := os.ReadFile(filepath.Join(dir, "expected.json"))
		if err != nil {
			continue
		}
		var c Case
		if err := json.Unmarshal(raw, &c); err != nil {
			return nil, err
		}
		if c.ID == "" {
			c.ID = e.Name()
		}
		c.Dir = dir
		out = append(out, c)
	}
	return out, nil
}

func KeywordScore(text string, keywords []string) float64 {
	if len(keywords) == 0 {
		return 0
	}
	hay := strings.ToLower(text)
	hits := 0
	for _, k := range keywords {
		if k != "" && strings.Contains(hay, strings.ToLower(k)) {
			hits++
		}
	}
	return float64(hits) / float64(len(keywords))
}

// KeywordRecall is hold-out recall of gold cause keywords in a free-text answer.
func KeywordRecall(text string, gold Case) float64 {
	return KeywordScore(text, gold.CauseKeywords)
}

func TypeMatch(got, want string) float64 {
	if strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(want)) {
		return 1
	}
	return 0
}

// FastScore is a weak single-model baseline: type match plus keyword hits in the summary.
func FastScore(incidentType, summary string, gold Case) float64 {
	return 0.6*TypeMatch(incidentType, gold.IncidentType) + 0.4*KeywordScore(summary, gold.CauseKeywords)
}

// CouncilScore uses the judged cause (and optional action) against the gold keywords.
func CouncilScore(incidentType, cause, action string, gold Case) float64 {
	text := cause + " " + action
	return 0.35*TypeMatch(incidentType, gold.IncidentType) + 0.5*KeywordScore(text, gold.CauseKeywords) + 0.15*KeywordScore(text, gold.FixKeywords)
}
