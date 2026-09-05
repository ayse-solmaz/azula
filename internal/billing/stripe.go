package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
)

func (s *Service) CreateCheckoutURL(user *domain.User) (string, error) {
	if s.cfg.StripeSecretKey == "" || s.cfg.StripePriceID == "" {
		return "", domain.ErrBillingNotConfigured
	}
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("success_url", strings.TrimRight(s.cfg.WebURL, "/")+"/security?billing=success")
	form.Set("cancel_url", strings.TrimRight(s.cfg.WebURL, "/")+"/security?billing=cancel")
	form.Set("line_items[0][price]", s.cfg.StripePriceID)
	form.Set("line_items[0][quantity]", "1")
	form.Set("client_reference_id", user.ID)
	form.Set("customer_email", user.Email)
	form.Set("metadata[userId]", user.ID)
	form.Set("subscription_data[metadata][userId]", user.ID)
	if user.StripeCustomerID != "" {
		form.Set("customer", user.StripeCustomerID)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.stripe.com/v1/checkout/sessions", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(s.cfg.StripeSecretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("stripe checkout: %s", strings.TrimSpace(string(body)))
	}
	var parsed struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if parsed.URL == "" {
		return "", fmt.Errorf("stripe checkout: empty url")
	}
	return parsed.URL, nil
}

func (s *Service) HandleWebhook(ctx context.Context, payload []byte, sigHeader string) error {
	if s.cfg.StripeWebhookSecret == "" {
		return domain.ErrBillingNotConfigured
	}
	if err := verifyStripeSignature(payload, sigHeader, s.cfg.StripeWebhookSecret); err != nil {
		return err
	}
	var evt struct {
		Type string `json:"type"`
		Data struct {
			Object json.RawMessage `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(payload, &evt); err != nil {
		return err
	}
	switch evt.Type {
	case "checkout.session.completed":
		var sess struct {
			Customer          string            `json:"customer"`
			Subscription      string            `json:"subscription"`
			ClientReferenceID string            `json:"client_reference_id"`
			Metadata          map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(evt.Data.Object, &sess); err != nil {
			return err
		}
		userID := sess.ClientReferenceID
		if userID == "" && sess.Metadata != nil {
			userID = sess.Metadata["userId"]
		}
		if userID == "" {
			return nil
		}
		_, err := s.ActivatePro(ctx, userID, sess.Customer, sess.Subscription)
		return err
	case "customer.subscription.deleted":
		var sub struct {
			Customer string            `json:"customer"`
			Metadata map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(evt.Data.Object, &sub); err != nil {
			return err
		}
		userID := ""
		if sub.Metadata != nil {
			userID = sub.Metadata["userId"]
		}
		return s.deactivateFromStripe(ctx, userID, sub.Customer)
	case "customer.subscription.updated":
		var sub struct {
			Customer     string            `json:"customer"`
			ID           string            `json:"id"`
			Status       string            `json:"status"`
			Metadata     map[string]string `json:"metadata"`
		}
		if err := json.Unmarshal(evt.Data.Object, &sub); err != nil {
			return err
		}
		userID := ""
		if sub.Metadata != nil {
			userID = sub.Metadata["userId"]
		}
		switch sub.Status {
		case "active", "trialing":
			u, err := s.lookupStripeUser(ctx, userID, sub.Customer)
			if err != nil {
				return nil
			}
			_, err = s.ActivatePro(ctx, u.ID, sub.Customer, sub.ID)
			return err
		case "canceled", "unpaid", "incomplete_expired":
			return s.deactivateFromStripe(ctx, userID, sub.Customer)
		}
		return nil
	default:
		return nil
	}
}

func (s *Service) lookupStripeUser(ctx context.Context, userID, customerID string) (*domain.User, error) {
	if userID != "" {
		return s.users.GetByID(ctx, userID)
	}
	if customerID != "" {
		return s.users.GetByStripeCustomerID(ctx, customerID)
	}
	return nil, domain.ErrNotFound
}

func (s *Service) deactivateFromStripe(ctx context.Context, userID, customerID string) error {
	u, err := s.lookupStripeUser(ctx, userID, customerID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return err
	}
	if u.Tier == domain.TierEnterprise {
		return nil
	}
	u.Tier = domain.TierFree
	u.StripeSubscription = ""
	return s.users.Update(ctx, u)
}

func verifyStripeSignature(payload []byte, header, secret string) error {
	var ts int64
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			n, err := strconv.ParseInt(kv[1], 10, 64)
			if err != nil {
				return domain.ErrUnauthorized
			}
			ts = n
		case "v1":
			sigs = append(sigs, kv[1])
		}
	}
	if ts == 0 || len(sigs) == 0 {
		return domain.ErrUnauthorized
	}
	if abs64(time.Now().Unix()-ts) > 300 {
		return domain.ErrUnauthorized
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strconv.FormatInt(ts, 10)))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	expect := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range sigs {
		if hmac.Equal([]byte(expect), []byte(sig)) {
			return nil
		}
	}
	return domain.ErrUnauthorized
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
