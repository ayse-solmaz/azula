package llm

import (
	"strings"
	"unicode"

	"github.com/ayse-solmaz/azula/internal/domain"
)

const (
	AggregationConsensus    = "consensus"
	AggregationDisagreement = "disagreement"
	AggregationEchoChamber  = "echo_chamber"
)

func voteScore(m domain.CouncilModel) float64 {
	n := len(m.Evidence)
	if n > 4 {
		n = 4
	}
	return clamp01(m.Confidence) * (1 + 0.12*float64(n))
}

func tokenSet(s string) map[string]struct{} {
	out := map[string]struct{}{}
	var b strings.Builder
	flush := func() {
		w := strings.ToLower(b.String())
		b.Reset()
		if len(w) < 4 {
			return
		}
		out[w] = struct{}{}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func hypothesisOverlap(a, b string) float64 {
	sa, sb := tokenSet(a), tokenSet(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	inter := 0
	for w := range sa {
		if _, ok := sb[w]; ok {
			inter++
		}
	}
	union := len(sa)
	for w := range sb {
		if _, ok := sa[w]; !ok {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// ApplyAggregation overlays deterministic Council voting on the Judge output.
// Agreement from two same-family models is treated as an echo chamber, not consensus.
func ApplyAggregation(res *domain.CouncilResult, sameFamily bool) {
	if res == nil || len(res.Models) < 2 {
		return
	}
	inv, chal := res.Models[0], res.Models[1]
	for _, m := range res.Models {
		if m.Role == "investigator" {
			inv = m
		}
		if m.Role == "challenger" {
			chal = m
		}
	}
	overlap := hypothesisOverlap(inv.Hypothesis, chal.Hypothesis)
	invScore, chalScore := voteScore(inv), voteScore(chal)
	winner := inv
	if chalScore > invScore {
		winner = chal
	}

	switch {
	case overlap >= 0.55 && sameFamily:
		res.Aggregation = AggregationEchoChamber
		res.NeedsReview = true
		res.AggregationNote = "Investigator and Challenger share a model family and similar hypotheses — treat agreement as possible echo chamber, not independent consensus."
		res.FinalJudgment.MostLikelyCause = winner.Hypothesis
		res.FinalJudgment.Confidence = clamp01(winner.Confidence)
	case overlap >= 0.55:
		res.Aggregation = AggregationConsensus
		res.NeedsReview = false
		res.AggregationNote = "Independent model families agreed; confidence is boosted."
		res.FinalJudgment.MostLikelyCause = winner.Hypothesis
		res.FinalJudgment.Confidence = clamp01(winner.Confidence + 0.08)
	default:
		res.Aggregation = AggregationDisagreement
		res.NeedsReview = true
		res.AggregationNote = "Hypotheses diverge. Weighted vote (confidence × evidence) picked the winner; flag for API or human review."
		res.FinalJudgment.MostLikelyCause = winner.Hypothesis
		c := winner.Confidence - 0.12
		if c < 0.35 {
			c = 0.35
		}
		res.FinalJudgment.Confidence = clamp01(c)
		if len(res.Disagreements) == 0 {
			res.Disagreements = []domain.Disagreement{{
				Topic:        "Root cause",
				Investigator: inv.Hypothesis,
				Challenger:   chal.Hypothesis,
			}}
		}
	}
	if res.FinalJudgment.RecommendedAction == "" {
		res.FinalJudgment.RecommendedAction = "Inspect cited evidence files and apply the judged fix."
	}
	if res.FinalJudgment.Confidence < 0.7 {
		res.NeedsReview = true
		if res.AggregationNote != "" && !strings.Contains(strings.ToLower(res.AggregationNote), "0.7") {
			res.AggregationNote += " Final confidence is below 0.7 — ask a human before treating this as closed."
		}
	}
}
