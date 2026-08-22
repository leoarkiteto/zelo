package service

import (
	"context"
	"errors"
	"testing"

	"github.com/leoarkiteto/zelo/internal/model"
)

func rolesServiceFixture() (*RoleService, *fakeRoleStore, *fakeAudit) {
	roles := &fakeRoleStore{}
	audit := &fakeAudit{}
	return &RoleService{Roles: roles, Audit: audit}, roles, audit
}

func TestRoleServiceGrantRequiresSyndic(t *testing.T) {
	svc, roles, _ := rolesServiceFixture()
	_ = roles.GrantRole(context.Background(), model.RoleAssignment{UserID: "owner-1", CondominiumID: "condo-1", Role: model.RoleOwner})

	err := svc.GrantRole(context.Background(), "owner-1", "owner-1", "condo-1", model.RoleSyndic)
	if !errors.Is(err, ErrNotSyndic) {
		t.Fatalf("GrantRole() error = %v, want ErrNotSyndic", err)
	}
}

func TestRoleServiceGrantSyndicToOwner(t *testing.T) {
	svc, roles, audit := rolesServiceFixture()
	_ = roles.GrantRole(context.Background(), model.RoleAssignment{UserID: "syndic-1", CondominiumID: "condo-1", Role: model.RoleSyndic})
	_ = roles.GrantRole(context.Background(), model.RoleAssignment{UserID: "owner-1", CondominiumID: "condo-1", Role: model.RoleOwner})

	if err := svc.GrantRole(context.Background(), "syndic-1", "owner-1", "condo-1", model.RoleSyndic); err != nil {
		t.Fatalf("GrantRole() error = %v", err)
	}
	found := false
	for _, e := range audit.events {
		if e.EventType == model.AuditRoleGranted {
			found = true
		}
	}
	if !found {
		t.Fatal("role grant event not recorded")
	}
}

func TestRoleServiceGrantSyndicToTenantRejected(t *testing.T) {
	svc, roles, _ := rolesServiceFixture()
	_ = roles.GrantRole(context.Background(), model.RoleAssignment{UserID: "syndic-1", CondominiumID: "condo-1", Role: model.RoleSyndic})
	_ = roles.GrantRole(context.Background(), model.RoleAssignment{UserID: "tenant-1", CondominiumID: "condo-1", Role: model.RoleTenant})

	err := svc.GrantRole(context.Background(), "syndic-1", "tenant-1", "condo-1", model.RoleSyndic)
	if !errors.Is(err, ErrTenantNotEligible) {
		t.Fatalf("GrantRole() error = %v, want ErrTenantNotEligible", err)
	}
}

func TestRoleServiceGrantSyndicToNonOwnerRejected(t *testing.T) {
	svc, roles, _ := rolesServiceFixture()
	_ = roles.GrantRole(context.Background(), model.RoleAssignment{UserID: "syndic-1", CondominiumID: "condo-1", Role: model.RoleSyndic})

	err := svc.GrantRole(context.Background(), "syndic-1", "stranger-1", "condo-1", model.RoleSyndic)
	if !errors.Is(err, ErrTargetNotOwner) {
		t.Fatalf("GrantRole() error = %v, want ErrTargetNotOwner", err)
	}
}

func TestRoleServiceRevokeOwnerAutoRevokesSyndic(t *testing.T) {
	svc, roles, _ := rolesServiceFixture()
	_ = roles.GrantRole(context.Background(), model.RoleAssignment{UserID: "syndic-1", CondominiumID: "condo-1", Role: model.RoleSyndic})
	_ = roles.GrantRole(context.Background(), model.RoleAssignment{UserID: "owner-1", CondominiumID: "condo-1", Role: model.RoleOwner})
	_ = roles.GrantRole(context.Background(), model.RoleAssignment{UserID: "owner-1", CondominiumID: "condo-1", Role: model.RoleSyndic})

	if err := svc.RevokeRole(context.Background(), "syndic-1", "owner-1", "condo-1", model.RoleOwner); err != nil {
		t.Fatalf("RevokeRole() error = %v", err)
	}
	hasOwner, _ := roles.HasActiveRole(context.Background(), "owner-1", "condo-1", model.RoleOwner)
	if hasOwner {
		t.Fatal("owner role still active after revoke")
	}
	hasSyndic, _ := roles.HasActiveRole(context.Background(), "owner-1", "condo-1", model.RoleSyndic)
	if hasSyndic {
		t.Fatal("syndic role was not auto-revoked")
	}
}
