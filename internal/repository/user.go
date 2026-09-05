package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserRepo struct {
	col *mongo.Collection
}

func NewUserRepo(db *mongo.Database) *UserRepo {
	return &UserRepo{col: db.Collection("Users")}
}

type userDoc struct {
	ID                   string             `bson:"_id"`
	Email                string             `bson:"email"`
	PasswordHash         string             `bson:"passwordHash"`
	Tier                 string             `bson:"tier"`
	MFAEnabled           bool               `bson:"mfaEnabled"`
	MFASecret            string             `bson:"mfaSecret,omitempty"`
	MFAPendingSecret     string             `bson:"mfaPendingSecret,omitempty"`
	TrustedDevices       []trustedDeviceDoc `bson:"trustedDevices,omitempty"`
	PendingDeviceID      string             `bson:"pendingDeviceId,omitempty"`
	PendingDeviceName    string             `bson:"pendingDeviceName,omitempty"`
	PendingDeviceOTP     string             `bson:"pendingDeviceOtp,omitempty"`
	PendingDeviceExp     time.Time          `bson:"pendingDeviceExp,omitempty"`
	OrgID                string             `bson:"orgId,omitempty"`
	OrgName              string             `bson:"orgName,omitempty"`
	OrgRole              string             `bson:"orgRole,omitempty"`
	StripeCustomerID     string             `bson:"stripeCustomerId,omitempty"`
	StripeSubscription   string             `bson:"stripeSubscription,omitempty"`
	SSOSubject           string             `bson:"ssoSubject,omitempty"`
	DisplayName          string             `bson:"displayName,omitempty"`
	Disabled             bool               `bson:"disabled,omitempty"`
	PrefsVersion         int                `bson:"prefsVersion,omitempty"`
	NotifyEmail          bool               `bson:"notifyEmail,omitempty"`
	NotifyInvestigations bool               `bson:"notifyInvestigations,omitempty"`
	NotifyMarketing      bool               `bson:"notifyMarketing,omitempty"`
	ShareUsage           bool               `bson:"shareUsage,omitempty"`
	CreatedAt            time.Time          `bson:"createdAt"`
	UpdatedAt            time.Time          `bson:"updatedAt"`
}

type trustedDeviceDoc struct {
	DeviceID   string    `bson:"deviceId"`
	Name       string    `bson:"name"`
	CreatedAt  time.Time `bson:"createdAt"`
	LastSeenAt time.Time `bson:"lastSeenAt,omitempty"`
}

func toUser(d userDoc) *domain.User {
	devs := make([]domain.TrustedDevice, 0, len(d.TrustedDevices))
	for _, t := range d.TrustedDevices {
		devs = append(devs, domain.TrustedDevice{DeviceID: t.DeviceID, Name: t.Name, CreatedAt: t.CreatedAt, LastSeenAt: t.LastSeenAt})
	}
	return &domain.User{
		ID:                   d.ID,
		Email:                d.Email,
		PasswordHash:         d.PasswordHash,
		Tier:                 domain.Tier(d.Tier),
		MFAEnabled:           d.MFAEnabled,
		MFASecret:            d.MFASecret,
		MFAPendingSecret:     d.MFAPendingSecret,
		TrustedDevices:       devs,
		PendingDeviceID:      d.PendingDeviceID,
		PendingDeviceName:    d.PendingDeviceName,
		PendingDeviceOTP:     d.PendingDeviceOTP,
		PendingDeviceExp:     d.PendingDeviceExp,
		OrgID:                d.OrgID,
		OrgName:              d.OrgName,
		OrgRole:              d.OrgRole,
		StripeCustomerID:     d.StripeCustomerID,
		StripeSubscription:   d.StripeSubscription,
		SSOSubject:           d.SSOSubject,
		DisplayName:          d.DisplayName,
		Disabled:             d.Disabled,
		PrefsVersion:         d.PrefsVersion,
		NotifyEmail:          d.NotifyEmail,
		NotifyInvestigations: d.NotifyInvestigations,
		NotifyMarketing:      d.NotifyMarketing,
		ShareUsage:           d.ShareUsage,
		CreatedAt:            d.CreatedAt,
		UpdatedAt:            d.UpdatedAt,
	}
}

func fromUser(u *domain.User) userDoc {
	devs := make([]trustedDeviceDoc, 0, len(u.TrustedDevices))
	for _, t := range u.TrustedDevices {
		devs = append(devs, trustedDeviceDoc{DeviceID: t.DeviceID, Name: t.Name, CreatedAt: t.CreatedAt, LastSeenAt: t.LastSeenAt})
	}
	return userDoc{
		ID:                   u.ID,
		Email:                u.Email,
		PasswordHash:         u.PasswordHash,
		Tier:                 string(u.Tier),
		MFAEnabled:           u.MFAEnabled,
		MFASecret:            u.MFASecret,
		MFAPendingSecret:     u.MFAPendingSecret,
		TrustedDevices:       devs,
		PendingDeviceID:      u.PendingDeviceID,
		PendingDeviceName:    u.PendingDeviceName,
		PendingDeviceOTP:     u.PendingDeviceOTP,
		PendingDeviceExp:     u.PendingDeviceExp,
		OrgID:                u.OrgID,
		OrgName:              u.OrgName,
		OrgRole:              u.OrgRole,
		StripeCustomerID:     u.StripeCustomerID,
		StripeSubscription:   u.StripeSubscription,
		SSOSubject:           u.SSOSubject,
		DisplayName:          u.DisplayName,
		Disabled:             u.Disabled,
		PrefsVersion:         u.PrefsVersion,
		NotifyEmail:          u.NotifyEmail,
		NotifyInvestigations: u.NotifyInvestigations,
		NotifyMarketing:      u.NotifyMarketing,
		ShareUsage:           u.ShareUsage,
		CreatedAt:            u.CreatedAt,
		UpdatedAt:            u.UpdatedAt,
	}
}

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	_, err := r.col.InsertOne(ctx, fromUser(user))
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrEmailTaken
	}
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	var d userDoc
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toUser(d), nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var d userDoc
	err := r.col.FindOne(ctx, bson.M{"email": email}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toUser(d), nil
}

func (r *UserRepo) GetByStripeCustomerID(ctx context.Context, customerID string) (*domain.User, error) {
	if strings.TrimSpace(customerID) == "" {
		return nil, domain.ErrNotFound
	}
	var d userDoc
	err := r.col.FindOne(ctx, bson.M{"stripeCustomerId": customerID}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return toUser(d), nil
}

func (r *UserRepo) Update(ctx context.Context, user *domain.User) error {
	user.UpdatedAt = time.Now().UTC()
	_, err := r.col.ReplaceOne(ctx, bson.M{"_id": user.ID}, fromUser(user))
	return err
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	_, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	return err
}
