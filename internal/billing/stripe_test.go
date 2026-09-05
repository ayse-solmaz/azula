package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
)

func TestVerifyStripeSignature(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"type":"checkout.session.completed"}`)
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(ts, 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))
	header := "t=" + strconv.FormatInt(ts, 10) + ",v1=" + sig
	if err := verifyStripeSignature(payload, header, secret); err != nil {
		t.Fatal(err)
	}
	if err := verifyStripeSignature(payload, header, "wrong"); err == nil {
		t.Fatal("expected mismatch")
	}
}

type hookUsers struct{ u *domain.User }

func (s *hookUsers) Create(context.Context, *domain.User) error { return nil }
func (s *hookUsers) GetByID(_ context.Context, id string) (*domain.User, error) {
	if s.u == nil || s.u.ID != id {
		return nil, domain.ErrNotFound
	}
	cp := *s.u
	return &cp, nil
}
func (s *hookUsers) GetByEmail(context.Context, string) (*domain.User, error) {
	return nil, domain.ErrNotFound
}
func (s *hookUsers) GetByStripeCustomerID(_ context.Context, id string) (*domain.User, error) {
	if s.u == nil || s.u.StripeCustomerID != id {
		return nil, domain.ErrNotFound
	}
	cp := *s.u
	return &cp, nil
}
func (s *hookUsers) Update(_ context.Context, u *domain.User) error { s.u = u; return nil }
func (s *hookUsers) Delete(context.Context, string) error          { return nil }

func signed(t *testing.T, secret string, payload []byte) string {
	t.Helper()
	ts := time.Now().Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(ts, 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return "t=" + strconv.FormatInt(ts, 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookSubscriptionDeletedDowngrades(t *testing.T) {
	users := &hookUsers{u: &domain.User{ID: "u1", Email: "a@x", Tier: domain.TierPro, StripeCustomerID: "cus_1", StripeSubscription: "sub_1"}}
	svc := New(config.Config{StripeWebhookSecret: "whsec_test"}, users, nil)
	payload := []byte(`{"type":"customer.subscription.deleted","data":{"object":{"customer":"cus_1","metadata":{"userId":"u1"}}}}`)
	if err := svc.HandleWebhook(context.Background(), payload, signed(t, "whsec_test", payload)); err != nil {
		t.Fatal(err)
	}
	if users.u.Tier != domain.TierFree || users.u.StripeSubscription != "" {
		t.Fatalf("downgrade: %+v", users.u)
	}
}
