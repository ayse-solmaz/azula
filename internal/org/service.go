package org

import (
	"context"
	"strings"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
	"github.com/google/uuid"
)

const (
	RoleAdmin    = "admin"
	RoleEngineer = "engineer"
	RoleViewer   = "viewer"
)

func rank(role string) int {
	switch role {
	case RoleAdmin:
		return 3
	case RoleEngineer:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

type Service struct {
	orgs   domain.OrganizationRepository
	users  domain.UserRepository
	spaces domain.WorkspaceRepository
}

func New(orgs domain.OrganizationRepository, users domain.UserRepository, spaces domain.WorkspaceRepository) *Service {
	return &Service{orgs: orgs, users: users, spaces: spaces}
}

func (s *Service) Create(ctx context.Context, userID, name string) (*domain.Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, domain.ErrInvalidInput
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.OrgID != "" {
		return nil, domain.ErrOrgConflict
	}
	now := time.Now().UTC()
	org := &domain.Organization{
		ID: uuid.NewString(), Name: name, OwnerID: userID, CreatedAt: now,
		Members: []domain.OrgMember{{UserID: userID, Email: user.Email, Role: RoleAdmin}},
	}
	if err := s.orgs.Create(ctx, org); err != nil {
		return nil, err
	}
	user.OrgID = org.ID
	user.OrgName = org.Name
	user.OrgRole = RoleAdmin
	user.Tier = domain.TierEnterprise
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	owned, err := s.spaces.ListByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range owned {
		if owned[i].OrgID == "" {
			owned[i].OrgID = org.ID
			_ = s.spaces.Update(ctx, &owned[i])
		}
	}
	return org, nil
}

func (s *Service) GetForUser(ctx context.Context, userID string) (*domain.Organization, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.OrgID == "" {
		return nil, domain.ErrNotFound
	}
	return s.orgs.GetByID(ctx, user.OrgID)
}

func (s *Service) Invite(ctx context.Context, adminID, email, role string) (*domain.Organization, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	role = strings.ToLower(strings.TrimSpace(role))
	if email == "" {
		return nil, domain.ErrInvalidInput
	}
	if rank(role) == 0 {
		role = RoleViewer
	}
	admin, err := s.users.GetByID(ctx, adminID)
	if err != nil {
		return nil, err
	}
	if admin.OrgID == "" || admin.OrgRole != RoleAdmin {
		return nil, domain.ErrForbidden
	}
	org, err := s.orgs.GetByID(ctx, admin.OrgID)
	if err != nil {
		return nil, err
	}
	for _, m := range org.Members {
		if strings.EqualFold(m.Email, email) {
			return org, nil
		}
	}
	member := domain.OrgMember{Email: email, Role: role}
	if existing, err := s.users.GetByEmail(ctx, email); err == nil {
		if existing.OrgID != "" && existing.OrgID != org.ID {
			return nil, domain.ErrOrgConflict
		}
		member.UserID = existing.ID
		existing.OrgID = org.ID
		existing.OrgName = org.Name
		existing.OrgRole = role
		existing.Tier = domain.TierEnterprise
		if err := s.users.Update(ctx, existing); err != nil {
			return nil, err
		}
	}
	org.Members = append(org.Members, member)
	if err := s.orgs.Update(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *Service) AttachInvites(ctx context.Context, user *domain.User) {
	org, err := s.orgs.GetByMemberEmail(ctx, user.Email)
	if err != nil || org == nil {
		return
	}
	for i, m := range org.Members {
		if !strings.EqualFold(m.Email, user.Email) {
			continue
		}
		if m.UserID == "" {
			org.Members[i].UserID = user.ID
			_ = s.orgs.Update(ctx, org)
		}
		user.OrgID = org.ID
		user.OrgName = org.Name
		user.OrgRole = m.Role
		if user.OrgRole == "" {
			user.OrgRole = RoleViewer
		}
		user.Tier = domain.TierEnterprise
		return
	}
}

func (s *Service) AuthorizeOrg(ctx context.Context, userID, minRole string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return err
	}
	if user.OrgID == "" {
		return nil
	}
	role := user.OrgRole
	if role == "" {
		role = RoleViewer
	}
	if rank(role) < rank(minRole) {
		return domain.ErrForbidden
	}
	return nil
}

func (s *Service) RoleFor(ctx context.Context, userID, workspaceID string) (string, error) {
	ws, err := s.spaces.GetByID(ctx, workspaceID)
	if err != nil {
		return "", err
	}
	if ws.OwnerID == userID {
		return RoleAdmin, nil
	}
	if ws.OrgID == "" {
		return "", domain.ErrUnauthorized
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if user.OrgID != ws.OrgID {
		return "", domain.ErrUnauthorized
	}
	if user.OrgRole == "" {
		return RoleViewer, nil
	}
	return user.OrgRole, nil
}

func (s *Service) Authorize(ctx context.Context, userID, workspaceID, minRole string) error {
	role, err := s.RoleFor(ctx, userID, workspaceID)
	if err != nil {
		return err
	}
	if rank(role) < rank(minRole) {
		return domain.ErrForbidden
	}
	return nil
}

func (s *Service) WorkspaceOrgID(ctx context.Context, userID string) (string, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return "", err
	}
	return user.OrgID, nil
}

func (s *Service) ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error) {
	owned, err := s.spaces.ListByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.OrgID == "" {
		return owned, nil
	}
	shared, err := s.spaces.ListByOrg(ctx, user.OrgID)
	if err != nil {
		return owned, nil
	}
	seen := map[string]struct{}{}
	var out []domain.Workspace
	for _, w := range append(owned, shared...) {
		if _, ok := seen[w.ID]; ok {
			continue
		}
		seen[w.ID] = struct{}{}
		out = append(out, w)
	}
	return out, nil
}

func (s *Service) UpdateMemberRole(ctx context.Context, adminID, email, role string) (*domain.Organization, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	role = strings.ToLower(strings.TrimSpace(role))
	if rank(role) == 0 {
		return nil, domain.ErrInvalidInput
	}
	admin, err := s.users.GetByID(ctx, adminID)
	if err != nil {
		return nil, err
	}
	if admin.OrgID == "" || admin.OrgRole != RoleAdmin {
		return nil, domain.ErrForbidden
	}
	org, err := s.orgs.GetByID(ctx, admin.OrgID)
	if err != nil {
		return nil, err
	}
	found := false
	for i, m := range org.Members {
		if !strings.EqualFold(m.Email, email) {
			continue
		}
		if m.UserID == org.OwnerID && role != RoleAdmin {
			return nil, domain.ErrForbidden
		}
		org.Members[i].Role = role
		if m.UserID != "" {
			if u, err := s.users.GetByID(ctx, m.UserID); err == nil {
				u.OrgRole = role
				_ = s.users.Update(ctx, u)
			}
		}
		found = true
		break
	}
	if !found {
		return nil, domain.ErrNotFound
	}
	if err := s.orgs.Update(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}

func (s *Service) RemoveMember(ctx context.Context, adminID, email string) (*domain.Organization, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	admin, err := s.users.GetByID(ctx, adminID)
	if err != nil {
		return nil, err
	}
	if admin.OrgID == "" || admin.OrgRole != RoleAdmin {
		return nil, domain.ErrForbidden
	}
	org, err := s.orgs.GetByID(ctx, admin.OrgID)
	if err != nil {
		return nil, err
	}
	var next []domain.OrgMember
	removed := ""
	for _, m := range org.Members {
		if strings.EqualFold(m.Email, email) {
			if m.UserID == org.OwnerID {
				return nil, domain.ErrForbidden
			}
			removed = m.UserID
			continue
		}
		next = append(next, m)
	}
	if len(next) == len(org.Members) {
		return nil, domain.ErrNotFound
	}
	org.Members = next
	if removed != "" {
		if u, err := s.users.GetByID(ctx, removed); err == nil {
			u.OrgID = ""
			u.OrgName = ""
			u.OrgRole = ""
			_ = s.users.Update(ctx, u)
		}
	}
	if err := s.orgs.Update(ctx, org); err != nil {
		return nil, err
	}
	return org, nil
}
