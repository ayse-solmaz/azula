package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ayse-solmaz/azula/internal/config"
	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/ayse-solmaz/azula/internal/mail"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

type ctxKey string

const userIDKey ctxKey = "userID"

type Auditor interface {
	Insert(ctx context.Context, log *domain.AuditLog) error
}

type LoginOutcome struct {
	User          *domain.User
	Token         string
	MFARequired   bool
	NewDevice     bool
	EphemeralCode string
}

type InviteJoiner interface {
	AttachInvites(ctx context.Context, user *domain.User)
}

type Service struct {
	users  domain.UserRepository
	cfg    config.Config
	spaces domain.WorkspaceRepository
	audit  Auditor
	mail   mail.Mailer
	join   InviteJoiner
}

func New(users domain.UserRepository, spaces domain.WorkspaceRepository, cfg config.Config) *Service {
	return NewWithAudit(users, spaces, cfg, nil, nil)
}

func NewWithAudit(users domain.UserRepository, spaces domain.WorkspaceRepository, cfg config.Config, audit Auditor, mailer mail.Mailer) *Service {
	return &Service{users: users, spaces: spaces, cfg: cfg, audit: audit, mail: mailer}
}

func (s *Service) SetJoiner(j InviteJoiner) {
	s.join = j
}

func WithUserID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, userIDKey, id)
}

func UserIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func (s *Service) ParseToken(token string) (string, error) {
	token = strings.TrimPrefix(token, "Bearer ")
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrUnauthorized
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		return "", domain.ErrUnauthorized
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return "", domain.ErrUnauthorized
	}
	sub, _ := claims["sub"].(string)
	if sub == "" {
		return "", domain.ErrUnauthorized
	}
	return sub, nil
}

func (s *Service) issueToken(user *domain.User) (string, error) {
	return IssueToken(s.cfg.JWTSecret, user.ID, s.cfg.JWTExpiry)
}

func (s *Service) logAudit(ctx context.Context, userID, action, resource string) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Insert(ctx, &domain.AuditLog{
		ID: uuid.NewString(), UserID: userID, Action: action, Resource: resource, CreatedAt: time.Now().UTC(),
	})
}

func (s *Service) Register(ctx context.Context, email, password, deviceID, deviceName string) (*domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(password) < 8 {
		return nil, "", domain.ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}
	now := time.Now().UTC()
	user := &domain.User{
		ID:                   uuid.NewString(),
		Email:                email,
		PasswordHash:         string(hash),
		Tier:                 domain.TierFree,
		PrefsVersion:         1,
		NotifyEmail:          true,
		NotifyInvestigations: true,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if deviceID != "" {
		user.TrustedDevices = []domain.TrustedDevice{{DeviceID: deviceID, Name: deviceName, CreatedAt: now, LastSeenAt: now}}
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, "", err
	}
	if s.join != nil {
		s.join.AttachInvites(ctx, user)
		if user.OrgID != "" {
			_ = s.users.Update(ctx, user)
		}
	}
	ws := &domain.Workspace{
		ID:        uuid.NewString(),
		Name:      "My ML Lab",
		OwnerID:   user.ID,
		OrgID:     user.OrgID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.spaces.Create(ctx, ws); err != nil {
		return nil, "", err
	}
	s.logAudit(ctx, user.ID, "register", "user")
	tok, err := s.issueToken(user)
	return user, tok, err
}

func (s *Service) Login(ctx context.Context, email, password, mfaCode, deviceID, deviceName, deviceOTP string) (*LoginOutcome, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if user.Disabled {
		return nil, domain.ErrAccountDisabled
	}
	if user.MFAEnabled {
		if strings.TrimSpace(mfaCode) == "" {
			return &LoginOutcome{User: user, MFARequired: true}, nil
		}
		if !totp.Validate(mfaCode, user.MFASecret) {
			return nil, domain.ErrMFAInvalid
		}
	}
	if strings.TrimSpace(deviceID) == "" {
		return nil, domain.ErrUnknownDevice
	}
	if deviceKnown(user, deviceID) {
		s.touchDevice(user, deviceID, deviceName)
		if err := s.users.Update(ctx, user); err != nil {
			return nil, err
		}
		tok, err := s.issueToken(user)
		s.logAudit(ctx, user.ID, "login", "device:"+deviceID)
		return &LoginOutcome{User: user, Token: tok}, err
	}
	if deviceOTP != "" {
		if time.Now().UTC().After(user.PendingDeviceExp) || user.PendingDeviceID != deviceID {
			return nil, domain.ErrDeviceOTP
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PendingDeviceOTP), []byte(deviceOTP)) != nil {
			return nil, domain.ErrDeviceOTP
		}
		if err := s.trustDevice(ctx, user, deviceID, deviceName); err != nil {
			return nil, err
		}
		tok, err := s.issueToken(user)
		s.logAudit(ctx, user.ID, "login", "device-otp")
		return &LoginOutcome{User: user, Token: tok}, err
	}
	code, err := sixDigit()
	if err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user.PendingDeviceID = deviceID
	user.PendingDeviceName = deviceName
	user.PendingDeviceOTP = string(hash)
	user.PendingDeviceExp = time.Now().UTC().Add(10 * time.Minute)
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	body := fmt.Sprintf("Your Azula new-device code is %s.\nIt expires in 10 minutes.\nIf you did not try to sign in, ignore this email.\n", code)
	if s.mail != nil {
		if err := s.mail.Send(ctx, user.Email, "Azula device verification", body); err != nil {
			log.Printf("device otp email failed for %s: %v", user.Email, err)
			return nil, fmt.Errorf("could not send device verification email")
		}
	} else {
		log.Printf("device verification code for %s: %s (no mailer)", user.Email, code)
	}
	s.logAudit(ctx, user.ID, "device-otp-sent", "device:"+deviceID)
	out := &LoginOutcome{User: user, NewDevice: true}
	if s.cfg.DeviceOTPEcho {
		out.EphemeralCode = code
	}
	return out, nil
}

func deviceKnown(user *domain.User, deviceID string) bool {
	for _, d := range user.TrustedDevices {
		if d.DeviceID == deviceID {
			return true
		}
	}
	return false
}

func (s *Service) trustDevice(ctx context.Context, user *domain.User, deviceID, deviceName string) error {
	now := time.Now().UTC()
	if !deviceKnown(user, deviceID) {
		user.TrustedDevices = append(user.TrustedDevices, domain.TrustedDevice{
			DeviceID: deviceID, Name: deviceName, CreatedAt: now, LastSeenAt: now,
		})
	} else {
		s.touchDevice(user, deviceID, deviceName)
	}
	user.PendingDeviceID = ""
	user.PendingDeviceName = ""
	user.PendingDeviceOTP = ""
	user.PendingDeviceExp = time.Time{}
	return s.users.Update(ctx, user)
}

func (s *Service) touchDevice(user *domain.User, deviceID, deviceName string) {
	now := time.Now().UTC()
	for i := range user.TrustedDevices {
		if user.TrustedDevices[i].DeviceID != deviceID {
			continue
		}
		user.TrustedDevices[i].LastSeenAt = now
		if deviceName != "" {
			user.TrustedDevices[i].Name = deviceName
		}
		return
	}
}

func (s *Service) RevokeTrustedDevice(ctx context.Context, userID, deviceID string) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	deviceID = strings.TrimSpace(deviceID)
	kept := user.TrustedDevices[:0]
	found := false
	for _, d := range user.TrustedDevices {
		if d.DeviceID == deviceID {
			found = true
			continue
		}
		kept = append(kept, d)
	}
	if !found {
		return nil, domain.ErrNotFound
	}
	user.TrustedDevices = kept
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	s.logAudit(ctx, user.ID, "revoke-device", "device:"+deviceID)
	return user, nil
}

func sixDigit() (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	return fmt.Sprintf("%06d", n%1000000), nil
}

func (s *Service) EnrollMFA(ctx context.Context, userID string) (secret, otpauth string, err error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", "", err
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.cfg.MFAIssuer,
		AccountName: user.Email,
		Period:      30,
		SecretSize:  20,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", err
	}
	user.MFAPendingSecret = key.Secret()
	if err := s.users.Update(ctx, user); err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

func (s *Service) EnableMFA(ctx context.Context, userID, code string) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.MFAPendingSecret == "" {
		return nil, domain.ErrMFANotPending
	}
	if !totp.Validate(code, user.MFAPendingSecret) {
		return nil, domain.ErrMFAInvalid
	}
	user.MFASecret = user.MFAPendingSecret
	user.MFAPendingSecret = ""
	user.MFAEnabled = true
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) DisableMFA(ctx context.Context, userID, code string) (*domain.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.MFAEnabled || !totp.Validate(code, user.MFASecret) {
		return nil, domain.ErrMFAInvalid
	}
	user.MFAEnabled = false
	user.MFASecret = ""
	user.MFAPendingSecret = ""
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) LoginOrRegisterSSO(ctx context.Context, email, subject, deviceID, deviceName string) (*domain.User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || subject == "" {
		return nil, "", domain.ErrInvalidInput
	}
	now := time.Now().UTC()
	user, err := s.users.GetByEmail(ctx, email)
	if err == domain.ErrNotFound {
		rnd := make([]byte, 32)
		_, _ = rand.Read(rnd)
		hash, herr := bcrypt.GenerateFromPassword(rnd, bcrypt.DefaultCost)
		if herr != nil {
			return nil, "", herr
		}
		user = &domain.User{
			ID:                   uuid.NewString(),
			Email:                email,
			PasswordHash:         string(hash),
			Tier:                 domain.TierFree,
			SSOSubject:           subject,
			PrefsVersion:         1,
			NotifyEmail:          true,
			NotifyInvestigations: true,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if deviceID != "" {
			user.TrustedDevices = []domain.TrustedDevice{{DeviceID: deviceID, Name: deviceName, CreatedAt: now, LastSeenAt: now}}
		}
		if err := s.users.Create(ctx, user); err != nil {
			return nil, "", err
		}
		if s.join != nil {
			s.join.AttachInvites(ctx, user)
			if user.OrgID != "" {
				_ = s.users.Update(ctx, user)
			}
		}
		ws := &domain.Workspace{
			ID: uuid.NewString(), Name: "My ML Lab", OwnerID: user.ID, OrgID: user.OrgID, CreatedAt: now, UpdatedAt: now,
		}
		if err := s.spaces.Create(ctx, ws); err != nil {
			return nil, "", err
		}
	} else if err != nil {
		return nil, "", err
	} else {
		if user.Disabled {
			return nil, "", domain.ErrAccountDisabled
		}
		if user.SSOSubject == "" {
			user.SSOSubject = subject
		}
		if deviceID != "" && !deviceKnown(user, deviceID) {
			user.TrustedDevices = append(user.TrustedDevices, domain.TrustedDevice{DeviceID: deviceID, Name: deviceName, CreatedAt: now, LastSeenAt: now})
		} else if deviceID != "" {
			s.touchDevice(user, deviceID, deviceName)
		}
		if err := s.users.Update(ctx, user); err != nil {
			return nil, "", err
		}
	}
	s.logAudit(ctx, user.ID, "sso_login", "oidc")
	tok, err := s.issueToken(user)
	return user, tok, err
}

func (s *Service) RequireUser(ctx context.Context) (*domain.User, error) {
	id := UserIDFrom(ctx)
	if id == "" {
		return nil, domain.ErrUnauthorized
	}
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user.Disabled {
		return nil, domain.ErrAccountDisabled
	}
	return user, nil
}

func (s *Service) UpdateProfile(ctx context.Context, displayName string) (*domain.User, error) {
	user, err := s.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(displayName)
	if len(name) > 80 {
		return nil, domain.ErrInvalidInput
	}
	user.DisplayName = name
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	s.logAudit(ctx, user.ID, "update_profile", "user")
	return user, nil
}

func (s *Service) ChangePassword(ctx context.Context, current, next string) error {
	user, err := s.RequireUser(ctx)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		return domain.ErrInvalidCredentials
	}
	if len(next) < 8 {
		return domain.ErrInvalidInput
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	if err := s.users.Update(ctx, user); err != nil {
		return err
	}
	s.logAudit(ctx, user.ID, "change_password", "user")
	return nil
}

func (s *Service) UpdatePrefs(ctx context.Context, notifyEmail, notifyInvestigations, notifyMarketing, shareUsage *bool) (*domain.User, error) {
	user, err := s.RequireUser(ctx)
	if err != nil {
		return nil, err
	}
	if user.PrefsVersion == 0 {
		user.NotifyEmail = true
		user.NotifyInvestigations = true
	}
	if notifyEmail != nil {
		user.NotifyEmail = *notifyEmail
	}
	if notifyInvestigations != nil {
		user.NotifyInvestigations = *notifyInvestigations
	}
	if notifyMarketing != nil {
		user.NotifyMarketing = *notifyMarketing
	}
	if shareUsage != nil {
		user.ShareUsage = *shareUsage
	}
	user.PrefsVersion = 1
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	s.logAudit(ctx, user.ID, "update_prefs", "user")
	return user, nil
}

func (s *Service) DeactivateAccount(ctx context.Context) error {
	user, err := s.RequireUser(ctx)
	if err != nil {
		return err
	}
	user.Disabled = true
	if err := s.users.Update(ctx, user); err != nil {
		return err
	}
	s.logAudit(ctx, user.ID, "deactivate_account", "user")
	return nil
}
