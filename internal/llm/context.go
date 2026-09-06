package llm

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// PromptBudgetChars is the Deep user-payload budget (~6k tokens at 4 chars/token).
// Council Challenger uses CouncilBudgetChars (see prompts.go).
const PromptBudgetChars = 24000

const perFileChars = 8000

var errorHints = []string{
	"error", "exception", "traceback", "oom", "out of memory", "cuda",
	"warning", "failed", "fatal", "schema", "mismatch", "leak",
}

// PackFiles formats selected files for a prompt. Large logs keep error lines
// plus head/tail instead of a blind prefix truncate.
func PackFiles(files map[string]string) string {
	return PackFilesBudget(files, PromptBudgetChars)
}

func PackFilesBudget(files map[string]string, budget int) string {
	if budget <= 0 {
		budget = PromptBudgetChars
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	used := 0
	for _, name := range names {
		remain := budget - used
		if remain < 200 {
			b.WriteString("=== [budget exhausted; remaining files omitted] ===\n")
			break
		}
		content := CompactFile(name, RedactSecrets(files[name]), min(perFileChars, remain))
		b.WriteString("=== BEGIN UNTRUSTED FILE ")
		b.WriteString(name)
		b.WriteString(" ===\n")
		b.WriteString(content)
		b.WriteString("\n=== END UNTRUSTED FILE ")
		b.WriteString(name)
		b.WriteString(" ===\n\n")
		used += len(content)
	}
	return wrapUntrustedFiles(b.String())
}

func CompactFile(name, content string, max int) string {
	if max <= 0 {
		return ""
	}
	if isLogName(name) && len(content) > max {
		content = compactLog(content, max)
	}
	if len(content) <= max {
		return content
	}
	return clipRunes(content, max)
}

func isLogName(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".log") || strings.HasSuffix(n, ".txt") || strings.Contains(n, "training")
}

func compactLog(content string, max int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= 80 && len(content) <= max {
		return content
	}
	keep := map[int]struct{}{}
	head, tail := 40, 40
	if len(lines) < head+tail {
		return clipRunes(content, max)
	}
	for i := 0; i < head; i++ {
		keep[i] = struct{}{}
	}
	for i := len(lines) - tail; i < len(lines); i++ {
		keep[i] = struct{}{}
	}
	for i, line := range lines {
		low := strings.ToLower(line)
		for _, h := range errorHints {
			if strings.Contains(low, h) {
				keep[i] = struct{}{}
				if i > 0 {
					keep[i-1] = struct{}{}
				}
				if i+1 < len(lines) {
					keep[i+1] = struct{}{}
				}
				break
			}
		}
	}
	var out []string
	out = append(out, "[hierarchical log: errors + head/tail; omitted lines marked]")
	prev := -2
	for i := range lines {
		if _, ok := keep[i]; !ok {
			continue
		}
		if i > prev+1 {
			out = append(out, "...")
		}
		out = append(out, lines[i])
		prev = i
	}
	joined := strings.Join(out, "\n")
	if len(joined) > max {
		return clipRunes(joined, max)
	}
	return joined
}

func clipRunes(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if utf8.RuneCountInString(s) <= n {
		return s[:n]
	}
	return s[:n] + "\n...[truncated]"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
