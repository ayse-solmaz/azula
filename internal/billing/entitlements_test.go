package billing

import (
	"testing"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
)

func TestForTierFreeBlocksProFeatures(t *testing.T) {
	cfg := config.Config{FreeTierMaxProjects: 3, FreeTierMaxInvs: 10}
	e := ForTier(domain.TierFree, cfg)
	if e.DeepAnalysis || e.Council || e.Generate || e.Evaluate || e.GitMCP {
		t.Fatalf("free tier must not unlock Pro features: %+v", e)
	}
	if e.MaxProjects != 3 || e.MaxInvestigationsPerMonth != 10 {
		t.Fatalf("free caps: %+v", e)
	}
}

func TestForTierFreeDemoUnlocksPipeline(t *testing.T) {
	e := ForTier(domain.TierFree, config.Config{BillingDemo: true, FreeTierMaxProjects: 3})
	if !e.DeepAnalysis || !e.Council || !e.Generate || !e.Evaluate || !e.GitMCP {
		t.Fatalf("local billing demo should unlock the same pipeline as Pro: %+v", e)
	}
}

func TestForTierProUnlocksLoopAndGit(t *testing.T) {
	e := ForTier(domain.TierPro, config.Config{})
	if !e.DeepAnalysis || !e.Council || !e.Generate || !e.Evaluate || !e.GitMCP || !e.ModelSelection {
		t.Fatalf("pro missing features: %+v", e)
	}
	if e.MaxInvestigationsPerMonth != 100 {
		t.Fatalf("pro inv cap: %d", e.MaxInvestigationsPerMonth)
	}
}

func TestForTierEnterpriseUnlimited(t *testing.T) {
	e := ForTier(domain.TierEnterprise, config.Config{})
	if e.MaxProjects != 0 || e.MaxInvestigationsPerMonth != 0 || !e.TeamWorkspace {
		t.Fatalf("enterprise: %+v", e)
	}
}
