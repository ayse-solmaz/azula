package llm

import (
	"strings"

	"github.com/ayse-solmaz/azula/internal/domain"
)

func ModelFamily(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(n, "azula-incident"), strings.Contains(n, "qwen"):
		return "qwen"
	case strings.Contains(n, "mixtral"), strings.Contains(n, "mistral"):
		return "mistral"
	case strings.Contains(n, "llama"):
		return "llama"
	case strings.Contains(n, "deepseek"):
		return "deepseek"
	case strings.Contains(n, "phi"):
		return "phi"
	case strings.Contains(n, "gemma"):
		return "gemma"
	case strings.Contains(n, "claude"):
		return "claude"
	case strings.Contains(n, "gpt"), strings.Contains(n, "o1"), strings.Contains(n, "o3"), strings.Contains(n, "o4"):
		return "openai"
	default:
		if n == "" {
			return "unknown"
		}
		return n
	}
}

var diversePrefer = []string{"mistral", "mixtral", "llama", "deepseek", "phi", "gemma"}

// PickDiverse returns an installed Ollama model whose family differs from avoid.
func PickDiverse(available []string, avoid string) string {
	avoidFam := ModelFamily(avoid)
	for _, pref := range diversePrefer {
		for _, n := range available {
			if ModelFamily(n) == avoidFam {
				continue
			}
			if strings.Contains(strings.ToLower(n), pref) {
				return n
			}
		}
	}
	for _, n := range available {
		if ModelFamily(n) != avoidFam {
			return n
		}
	}
	return ""
}

type CouncilRouting struct {
	InvestigatorSlot string
	ChallengerSlot   string
	JudgeSlot        string
	ChallengerName   string
	InvestigatorName string
	JudgeName        string
	SameFamily       bool
}

func (r *Router) routeCouncil(cfg domain.ModelConfig, available []string) CouncilRouting {
	invName := cfg.ModelBName
	if hit := PickModelB(available, invName); hit != "" {
		invName = hit
	}
	// Fast council (default): Challenger stays on the small env Fast model
	// (r.cfg.ModelAName), even if the workspace Models page set Model A to
	// azula-incident — otherwise both agents share one GPU family and time out.
	// Set AZULA_COUNCIL_FAST=false to restore family-diversity routing.
	chalName := r.cfg.ModelAName
	if chalName == "" {
		chalName = "qwen2.5:1.5b"
	}
	chalSlot := "A"
	if !r.cfg.CouncilFast {
		chalName = cfg.ModelAName
		if diverse := PickDiverse(available, invName); diverse != "" {
			chalName = diverse
			chalSlot = "B"
		} else if ModelFamily(cfg.ModelAName) != ModelFamily(invName) {
			chalName = cfg.ModelAName
			chalSlot = "A"
		}
	} else if hit := findOllamaModel(available, chalName); hit != "" {
		chalName = hit
	}
	judgeSlot := "A"
	judgeName := cfg.ModelAName
	if r.hasAPIJudge(cfg) {
		judgeSlot = "C"
		judgeName = cfg.ModelCName
		if judgeName == "" {
			judgeName = "gpt-4o-mini"
		}
	}
	return CouncilRouting{
		InvestigatorSlot: "B",
		ChallengerSlot:   chalSlot,
		JudgeSlot:        judgeSlot,
		InvestigatorName: invName,
		ChallengerName:   chalName,
		JudgeName:        judgeName,
		SameFamily:       ModelFamily(invName) == ModelFamily(chalName),
	}
}

func (r *Router) hasAPIJudge(cfg domain.ModelConfig) bool {
	prov := strings.ToLower(cfg.ModelCProvider)
	if prov == "" {
		prov = "openai"
	}
	return prov == "openai" && r.cfg.OpenAIKey != ""
}
