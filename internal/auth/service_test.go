package auth

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/mail"
	"github.com/pquerna/otp/totp"
)

type memUsers struct {
	mu sync.Mutex
	m  map[string]*domain.User
}

func (s *memUsers) Create(_ context.Context, u *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]*domain.User{}
	}
	cp := *u
	s.m[u.ID] = &cp
	return nil
}
func (s *memUsers) GetByID(_ context.Context, id string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *u
	if u.TrustedDevices != nil {
		cp.TrustedDevices = append([]domain.TrustedDevice(nil), u.TrustedDevices...)
	}
	return &cp, nil
}
func (s *memUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.m {
		if u.Email == email {
			cp := *u
			if u.TrustedDevices != nil {
				cp.TrustedDevices = append([]domain.TrustedDevice(nil), u.TrustedDevices...)
			}
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (s *memUsers) GetByStripeCustomerID(_ context.Context, customerID string) (*domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.m {
		if u.StripeCustomerID != "" && u.StripeCustomerID == customerID {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (s *memUsers) Update(_ context.Context, u *domain.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *u
	s.m[u.ID] = &cp
	return nil
}
func (s *memUsers) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, id)
	return nil
}

type memSpaces struct{}

func (memSpaces) Create(context.Context, *domain.Workspace) error { return nil }
func (memSpaces) GetByID(context.Context, string) (*domain.Workspace, error) {
	return nil, domain.ErrNotFound
}
func (memSpaces) ListByOwner(context.Context, string) ([]domain.Workspace, error) {
	return nil, nil
}
func (memSpaces) ListByOrg(context.Context, string) ([]domain.Workspace, error) { return nil, nil }
func (memSpaces) Update(context.Context, *domain.Workspace) error               { return nil }
func (memSpaces) DeleteByOwner(context.Context, string) error                   { return nil }

func TestUnknownDeviceBlockedUntilEmailOTP(t *testing.T) {
	users := &memUsers{m: map[string]*domain.User{}}
	box := &mail.Memory{}
	svc := NewWithAudit(users, memSpaces{}, config.Config{JWTSecret: "test", JWTExpiry: time.Hour}, nil, box)
	u, tok, err := svc.Register(context.Background(), "a@x.dev", "password1", "dev-a", "laptop")
	if err != nil || tok == "" {
		t.Fatalf("register: %v", err)
	}
	_ = u

	out, err := svc.Login(context.Background(), "a@x.dev", "password1", "", "dev-b", "phone", "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Token != "" || !out.NewDevice {
		t.Fatalf("unknown device must not issue token: %+v", out)
	}
	if len(box.Messages) != 1 {
		t.Fatalf("expected email OTP, got %+v", box.Messages)
	}
	code := ""
	for _, w := range strings.FieldsFunc(box.Messages[0].Body, func(r rune) bool {
		return r < '0' || r > '9'
	}) {
		if len(w) == 6 {
			code = w
			break
		}
	}
	if code == "" {
		t.Fatalf("no code in mail: %s", box.Messages[0].Body)
	}
	out2, err := svc.Login(context.Background(), "a@x.dev", "password1", "", "dev-b", "phone", code)
	if err != nil || out2.Token == "" {
		t.Fatalf("otp login: %v %+v", err, out2)
	}
}

func TestMultipleTrustedDevicesAndRevoke(t *testing.T) {
	users := &memUsers{m: map[string]*domain.User{}}
	box := &mail.Memory{}
	svc := NewWithAudit(users, memSpaces{}, config.Config{JWTSecret: "test", JWTExpiry: time.Hour, DeviceOTPEcho: true}, nil, box)
	if _, _, err := svc.Register(context.Background(), "multi@x.dev", "password1", "dev-laptop", "laptop"); err != nil {
		t.Fatal(err)
	}
	out, err := svc.Login(context.Background(), "multi@x.dev", "password1", "", "dev-phone", "phone", "")
	if err != nil || !out.NewDevice || out.EphemeralCode == "" {
		t.Fatalf("phone challenge: %v %+v", err, out)
	}
	out2, err := svc.Login(context.Background(), "multi@x.dev", "password1", "", "dev-phone", "phone", out.EphemeralCode)
	if err != nil || out2.Token == "" {
		t.Fatalf("phone otp: %v %+v", err, out2)
	}
	out3, err := svc.Login(context.Background(), "multi@x.dev", "password1", "", "dev-tablet", "tablet", "")
	if err != nil || !out3.NewDevice {
		t.Fatalf("tablet challenge: %v %+v", err, out3)
	}
	if _, err := svc.Login(context.Background(), "multi@x.dev", "password1", "", "dev-tablet", "tablet", out3.EphemeralCode); err != nil {
		t.Fatal(err)
	}
	user, err := svc.users.GetByEmail(context.Background(), "multi@x.dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(user.TrustedDevices) != 3 {
		t.Fatalf("want 3 trusted devices, got %d", len(user.TrustedDevices))
	}
	known, err := svc.Login(context.Background(), "multi@x.dev", "password1", "", "dev-laptop", "laptop", "")
	if err != nil || known.Token == "" || known.NewDevice {
		t.Fatalf("laptop should stay trusted: %v %+v", err, known)
	}
	if _, err := svc.RevokeTrustedDevice(context.Background(), user.ID, "dev-phone"); err != nil {
		t.Fatal(err)
	}
	again, err := svc.Login(context.Background(), "multi@x.dev", "password1", "", "dev-phone", "phone", "")
	if err != nil || !again.NewDevice || again.Token != "" {
		t.Fatalf("revoked phone must challenge again: %v %+v", err, again)
	}
	still, err := svc.Login(context.Background(), "multi@x.dev", "password1", "", "dev-tablet", "tablet", "")
	if err != nil || still.Token == "" {
		t.Fatalf("tablet should remain: %v %+v", err, still)
	}
}

func TestMFAThenNewDeviceOTP(t *testing.T) {
	users := &memUsers{m: map[string]*domain.User{}}
	box := &mail.Memory{}
	svc := NewWithAudit(users, memSpaces{}, config.Config{JWTSecret: "test", JWTExpiry: time.Hour, MFAIssuer: "Azula", DeviceOTPEcho: true}, nil, box)
	u, _, err := svc.Register(context.Background(), "mfa@x.dev", "password1", "dev-a", "laptop")
	if err != nil {
		t.Fatal(err)
	}
	secret, _, err := svc.EnrollMFA(context.Background(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.EnableMFA(context.Background(), u.ID, code); err != nil {
		t.Fatal(err)
	}
	blocked, err := svc.Login(context.Background(), "mfa@x.dev", "password1", "", "dev-b", "phone", "")
	if err != nil || !blocked.MFARequired || blocked.Token != "" {
		t.Fatalf("mfa first: %v %+v", err, blocked)
	}
	mfaCode, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	chal, err := svc.Login(context.Background(), "mfa@x.dev", "password1", mfaCode, "dev-b", "phone", "")
	if err != nil || !chal.NewDevice || chal.Token != "" {
		t.Fatalf("device after mfa: %v %+v", err, chal)
	}
	mfaCode, err = totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	ok, err := svc.Login(context.Background(), "mfa@x.dev", "password1", mfaCode, "dev-b", "phone", chal.EphemeralCode)
	if err != nil || ok.Token == "" {
		t.Fatalf("mfa+otp: %v %+v", err, ok)
	}
	user, _ := svc.users.GetByEmail(context.Background(), "mfa@x.dev")
	if len(user.TrustedDevices) != 2 {
		t.Fatalf("want laptop+phone, got %d", len(user.TrustedDevices))
	}
}

func TestChangePasswordAndDisable(t *testing.T) {
	users := &memUsers{m: map[string]*domain.User{}}
	svc := NewWithAudit(users, memSpaces{}, config.Config{JWTSecret: "test", JWTExpiry: time.Hour}, nil, &mail.Memory{})
	u, _, err := svc.Register(context.Background(), "c@x.dev", "password1", "dev-a", "laptop")
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithUserID(context.Background(), u.ID)
	if _, err := svc.UpdateProfile(ctx, "Ada"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.RequireUser(ctx)
	if err != nil || got.DisplayName != "Ada" {
		t.Fatalf("profile %v %v", got, err)
	}
	if err := svc.ChangePassword(ctx, "password1", "password2"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(context.Background(), "c@x.dev", "password1", "", "dev-a", "laptop", ""); err != domain.ErrInvalidCredentials {
		t.Fatalf("old password still worked: %v", err)
	}
	if _, err := svc.Login(context.Background(), "c@x.dev", "password2", "", "dev-a", "laptop", ""); err != nil {
		t.Fatal(err)
	}
	email, investigations, marketing, share := false, true, false, true
	if _, err := svc.UpdatePrefs(ctx, &email, &investigations, &marketing, &share); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeactivateAccount(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Login(context.Background(), "c@x.dev", "password2", "", "dev-a", "laptop", ""); err != domain.ErrAccountDisabled {
		t.Fatalf("got %v", err)
	}
}

func TestMissingDeviceIDRejected(t *testing.T) {
	users := &memUsers{m: map[string]*domain.User{}}
	svc := NewWithAudit(users, memSpaces{}, config.Config{JWTSecret: "test", JWTExpiry: time.Hour}, nil, &mail.Memory{})
	if _, _, err := svc.Register(context.Background(), "b@x.dev", "password1", "dev-a", "laptop"); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Login(context.Background(), "b@x.dev", "password1", "", "", "", "")
	if err != domain.ErrUnknownDevice {
		t.Fatalf("got %v", err)
	}
}
