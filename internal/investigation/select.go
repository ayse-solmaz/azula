package investigation

import (
	"context"
	"sort"
	"strings"

	"github.com/ayse-solmaz/azula/internal/llm"
	"github.com/ayse-solmaz/azula/internal/mcp"
)

// ContextChars is the Deep/Council prompt budget (~6k tokens at 4 chars/token).
const ContextChars = 24000

const perFileChars = 8000

func rankNames(names []string, prompt, incidentType string) []string {
	type scored struct {
		name  string
		score int
		idx   int
	}
	hint := strings.ToLower(strings.TrimSpace(prompt + " " + incidentType))
	items := make([]scored, 0, len(names))
	for i, name := range names {
		items = append(items, scored{name: name, score: fileScore(name, hint, incidentType), idx: i})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].idx < items[j].idx
		}
		return items[i].score > items[j].score
	})
	out := make([]string, len(items))
	for i, s := range items {
		out[i] = s.name
	}
	return out
}

func fileScore(name, hint, incidentType string) int {
	n := strings.ToLower(name)
	score := 1
	switch {
	case strings.Contains(n, "training") && strings.HasSuffix(n, ".log"):
		score += 40
	case strings.HasSuffix(n, ".log"):
		score += 28
	case n == "config.yaml" || n == "config.yml":
		score += 36
	case strings.HasSuffix(n, ".yaml") || strings.HasSuffix(n, ".yml"):
		score += 22
	case strings.HasSuffix(n, ".py"):
		score += 34
	case strings.Contains(n, "dataset") || strings.HasSuffix(n, ".jsonl"):
		score += 30
	case strings.Contains(n, "metrics"):
		score += 18
	}
	switch strings.ToLower(incidentType) {
	case "memory_gpu":
		if strings.Contains(n, "log") || strings.Contains(n, "config") {
			score += 24
		}
	case "schema_mismatch":
		if strings.Contains(n, "dataset") || strings.HasSuffix(n, ".jsonl") || strings.Contains(n, "log") {
			score += 24
		}
	case "data_leakage":
		if strings.HasSuffix(n, ".py") || strings.Contains(n, "metrics") {
			score += 24
		}
	case "config_error":
		if strings.Contains(n, "config") {
			score += 24
		}
	case "data_quality":
		if strings.Contains(n, "dataset") || strings.HasSuffix(n, ".jsonl") || strings.HasSuffix(n, ".py") || strings.Contains(n, "log") {
			score += 24
		}
	}
	base := n
	if i := strings.LastIndex(base, "."); i >= 0 {
		base = base[:i]
	}
	for _, tok := range strings.Fields(hint) {
		if len(tok) < 4 {
			continue
		}
		if strings.Contains(n, tok) || strings.Contains(base, tok) {
			score += 8
		}
	}
	return score
}

func clip(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}

// pickWithinBudget keeps ranked files until the character budget is spent.
func pickWithinBudget(ranked []string, contents map[string]string, budget int) map[string]string {
	if budget <= 0 {
		budget = ContextChars
	}
	out := map[string]string{}
	used := 0
	for _, name := range ranked {
		body, ok := contents[name]
		if !ok {
			continue
		}
		body = llm.CompactFile(name, body, perFileChars)
		if used >= budget {
			break
		}
		remain := budget - used
		if remain < 200 {
			break
		}
		if len(body) > remain {
			body = clip(body, remain)
		}
		out[name] = body
		used += len(body)
	}
	return out
}

func readRanked(ctx context.Context, files mcp.Connector, projectID string, ranked []string, budget int) (map[string]string, []string, error) {
	if budget <= 0 {
		budget = ContextChars
	}
	out := map[string]string{}
	order := make([]string, 0, len(ranked))
	used := 0
	for _, name := range ranked {
		if used >= budget {
			break
		}
		remain := budget - used
		if remain < 200 {
			break
		}
		body, err := files.ReadFile(ctx, projectID, name)
		if err != nil {
			continue
		}
		body = llm.CompactFile(name, body, perFileChars)
		if len(body) > remain {
			body = clip(body, remain)
		}
		out[name] = body
		order = append(order, name)
		used += len(body)
	}
	return out, order, nil
}
