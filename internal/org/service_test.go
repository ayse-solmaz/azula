package org

import (
	"context"
	"testing"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
)

type memUsers struct{ m map[string]*domain.User }

func (s *memUsers) Create(_ context.Context, u *domain.User) error {
	s.m[u.ID] = u
	return nil
}
func (s *memUsers) GetByID(_ context.Context, id string) (*domain.User, error) {
	u, ok := s.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *u
	return &cp, nil
}
func (s *memUsers) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range s.m {
		if u.Email == email {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (s *memUsers) GetByStripeCustomerID(_ context.Context, customerID string) (*domain.User, error) {
	for _, u := range s.m {
		if u.StripeCustomerID == customerID && customerID != "" {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (s *memUsers) Update(_ context.Context, u *domain.User) error { s.m[u.ID] = u; return nil }
func (s *memUsers) Delete(_ context.Context, id string) error      { delete(s.m, id); return nil }

type memOrgs struct {
	m map[string]*domain.Organization
}

func (s *memOrgs) Create(_ context.Context, o *domain.Organization) error { s.m[o.ID] = o; return nil }
func (s *memOrgs) GetByID(_ context.Context, id string) (*domain.Organization, error) {
	o, ok := s.m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *o
	return &cp, nil
}
func (s *memOrgs) GetByMemberEmail(context.Context, string) (*domain.Organization, error) {
	return nil, domain.ErrNotFound
}
func (s *memOrgs) Update(_ context.Context, o *domain.Organization) error { s.m[o.ID] = o; return nil }
func (s *memOrgs) DeleteByOwner(context.Context, string) error            { return nil }

type memSpaces struct{}

func (memSpaces) Create(context.Context, *domain.Workspace) error { return nil }
func (memSpaces) GetByID(context.Context, string) (*domain.Workspace, error) {
	return nil, domain.ErrNotFound
}
func (memSpaces) ListByOwner(context.Context, string) ([]domain.Workspace, error) { return nil, nil }
func (memSpaces) ListByOrg(context.Context, string) ([]domain.Workspace, error)   { return nil, nil }
func (memSpaces) Update(context.Context, *domain.Workspace) error                 { return nil }
func (memSpaces) DeleteByOwner(context.Context, string) error                     { return nil }

func TestUpdateAndRemoveMember(t *testing.T) {
	now := time.Now().UTC()
	admin := &domain.User{ID: "a1", Email: "a@x", OrgID: "o1", OrgRole: RoleAdmin, CreatedAt: now}
	eng := &domain.User{ID: "e1", Email: "e@x", OrgID: "o1", OrgRole: RoleEngineer, CreatedAt: now}
	users := &memUsers{m: map[string]*domain.User{"a1": admin, "e1": eng}}
	orgs := &memOrgs{m: map[string]*domain.Organization{
		"o1": {ID: "o1", Name: "Acme", OwnerID: "a1", Members: []domain.OrgMember{
			{UserID: "a1", Email: "a@x", Role: RoleAdmin},
			{UserID: "e1", Email: "e@x", Role: RoleEngineer},
		}},
	}}
	svc := New(orgs, users, memSpaces{})
	updated, err := svc.UpdateMemberRole(context.Background(), "a1", "e@x", RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Members[1].Role != RoleViewer {
		t.Fatalf("role: %+v", updated.Members)
	}
	if _, err := svc.RemoveMember(context.Background(), "a1", "a@x"); err == nil {
		t.Fatal("owner must not be removable")
	}
	if _, err := svc.RemoveMember(context.Background(), "a1", "e@x"); err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizeOrgRoles(t *testing.T) {
	now := time.Now().UTC()
	admin := &domain.User{ID: "a1", Email: "a@x", OrgID: "o1", OrgRole: RoleAdmin, CreatedAt: now}
	eng := &domain.User{ID: "e1", Email: "e@x", OrgID: "o1", OrgRole: RoleEngineer, CreatedAt: now}
	viewer := &domain.User{ID: "v1", Email: "v@x", OrgID: "o1", OrgRole: RoleViewer, CreatedAt: now}
	solo := &domain.User{ID: "s1", Email: "s@x", CreatedAt: now}
	users := &memUsers{m: map[string]*domain.User{"a1": admin, "e1": eng, "v1": viewer, "s1": solo}}
	svc := New(&memOrgs{m: map[string]*domain.Organization{}}, users, memSpaces{})
	if err := svc.AuthorizeOrg(context.Background(), "v1", RoleEngineer); err == nil {
		t.Fatal("viewer cannot engineer")
	}
	if err := svc.AuthorizeOrg(context.Background(), "e1", RoleEngineer); err != nil {
		t.Fatal(err)
	}
	if err := svc.AuthorizeOrg(context.Background(), "e1", RoleAdmin); err == nil {
		t.Fatal("engineer cannot admin")
	}
	if err := svc.AuthorizeOrg(context.Background(), "s1", RoleAdmin); err != nil {
		t.Fatal("personal account has no org gate")
	}
}
