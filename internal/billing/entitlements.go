package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
)

type Service struct {
	cfg   config.Config
	users domain.UserRepository
	invs  domain.InvestigationRepository
}

func New(cfg config.Config, users domain.UserRepository, invs domain.InvestigationRepository) *Service {
	return &Service{cfg: cfg, users: users, invs: invs}
}

func ForTier(tier domain.Tier, cfg config.Config) domain.Entitlements {
	e := domain.Entitlements{
		Tier:                      tier,
		BillingConfigured:         cfg.StripeSecretKey != "",
		SSOEnabled:                cfg.OIDCIssuer != "" && cfg.OIDCClientID != "",
		DemoUpgrade:               cfg.BillingDemo,
		MaxProjects:               cfg.FreeTierMaxProjects,
		MaxInvestigationsPerMonth: cfg.FreeTierMaxInvs,
	}
	switch tier {
	case domain.TierPro:
		e.MaxProjects = 0
		e.MaxInvestigationsPerMonth = 100
		e.DeepAnalysis = true
		e.Council = true
		e.Generate = true
		e.Evaluate = true
		e.GitMCP = true
		e.ModelSelection = true
	case domain.TierEnterprise:
		e.MaxProjects = 0
		e.MaxInvestigationsPerMonth = 0
		e.DeepAnalysis = true
		e.Council = true
		e.Generate = true
		e.Evaluate = true
		e.GitMCP = true
		e.ModelSelection = true
		e.TeamWorkspace = true
	}
	if cfg.BillingDemo && tier == domain.TierFree {
		e.DeepAnalysis = true
		e.Council = true
		e.Generate = true
		e.Evaluate = true
		e.GitMCP = true
		e.ModelSelection = true
	}
	return e
}

func (s *Service) ForUser(ctx context.Context, userID string) (domain.Entitlements, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return domain.Entitlements{}, err
	}
	e := ForTier(user.Tier, s.cfg)
	if s.invs != nil {
		start := time.Now().UTC()
		since := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
		n, err := s.invs.CountByUserSince(ctx, userID, since)
		if err != nil {
			return domain.Entitlements{}, err
		}
		e.InvestigationsUsed = int(n)
	}
	return e, nil
}

func (s *Service) Require(ctx context.Context, userID, feature string) error {
	e, err := s.ForUser(ctx, userID)
	if err != nil {
		return err
	}
	ok := false
	switch feature {
	case "deep":
		ok = e.DeepAnalysis
	case "council":
		ok = e.Council
	case "generate":
		ok = e.Generate
	case "evaluate":
		ok = e.Evaluate
	case "git":
		ok = e.GitMCP
	case "models":
		ok = e.ModelSelection
	default:
		return fmt.Errorf("%w: unknown feature %s", domain.ErrInvalidInput, feature)
	}
	if !ok {
		return fmt.Errorf("%w: %s is available on Pro", domain.ErrProRequired, featureName(feature))
	}
	return nil
}

func featureName(feature string) string {
	switch feature {
	case "deep":
		return "Deep analysis"
	case "council":
		return "AI Council"
	case "generate":
		return "Generate"
	case "evaluate":
		return "Evaluate"
	case "git":
		return "Git MCP"
	case "models":
		return "Model selection"
	default:
		return feature
	}
}

func (s *Service) CheckInvestigationCap(ctx context.Context, userID string) error {
	e, err := s.ForUser(ctx, userID)
	if err != nil {
		return err
	}
	if e.MaxInvestigationsPerMonth <= 0 {
		return nil
	}
	if e.InvestigationsUsed >= e.MaxInvestigationsPerMonth {
		return fmt.Errorf("%w: %d/%d investigations used this month — upgrade to Pro for 100/month", domain.ErrInvestigationLimit, e.InvestigationsUsed, e.MaxInvestigationsPerMonth)
	}
	return nil
}

func (s *Service) ActivatePro(ctx context.Context, userID, customerID, subID string) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.Tier == domain.TierEnterprise {
		return user, nil
	}
	user.Tier = domain.TierPro
	if customerID != "" {
		user.StripeCustomerID = customerID
	}
	if subID != "" {
		user.StripeSubscription = subID
	}
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) ActivateProDemo(ctx context.Context, userID string) (*domain.User, error) {
	if !s.cfg.BillingDemo && s.cfg.StripeSecretKey != "" {
		return nil, errors.New("demo upgrade is disabled when Stripe is configured")
	}
	return s.ActivatePro(ctx, userID, "", "demo")
}

func Unlimited(n int) bool {
	return n <= 0
}
