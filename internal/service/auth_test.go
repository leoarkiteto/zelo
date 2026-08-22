package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/model"
)

func authServiceFixture() (*AuthService, *fakeUserStore, *fakeRoleStore, *fakeAudit) {
	users := newFakeUserStore()
	roles := &fakeRoleStore{}
	audit := &fakeAudit{}
	svc := &AuthService{
		Users: users, Roles: roles, Passwords: testHasher{}, Audit: audit,
		LockThreshold: 3, LockDuration: 15 * time.Minute, Now: time.Now,
	}
	return svc, users, roles, audit
}

func seedUser(t *testing.T, users *fakeUserStore, email, password string, roles *fakeRoleStore, condoID string) string {
	t.Helper()
	hash, _ := testHasher{}.Hash(password)
	id, err := users.CreateUser(context.Background(), model.User{Email: email, PasswordHash: hash, Status: model.UserStatusActive})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if roles != nil && condoID != "" {
		_ = roles.GrantRole(context.Background(), model.RoleAssignment{
			UserID: id, CondominiumID: condoID, Role: model.RoleOwner,
		})
	}
	return id
}

func TestAuthServiceAuthenticateSuccess(t *testing.T) {
	svc, users, roles, _ := authServiceFixture()
	seedUser(t, users, "owner@example.com", "correct-password", roles, "condo-1")

	u, condoID, err := svc.Authenticate(context.Background(), "owner@example.com", "correct-password")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if u.Email != "owner@example.com" {
		t.Fatalf("user email = %q", u.Email)
	}
	if condoID != "condo-1" {
		t.Fatalf("condominium id = %q, want condo-1", condoID)
	}
}

func TestAuthServiceAuthenticateInvalidCredentials(t *testing.T) {
	svc, users, _, _ := authServiceFixture()
	seedUser(t, users, "owner@example.com", "correct-password", nil, "")

	_, _, err := svc.Authenticate(context.Background(), "owner@example.com", "wrong-password")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Authenticate() error = %v, want ErrInvalidCredentials", err)
	}
	// Unknown email is indistinguishable from a wrong password.
	_, _, err = svc.Authenticate(context.Background(), "nobody@example.com", "whatever")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Authenticate() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthServiceAuthenticateLockout(t *testing.T) {
	svc, users, _, audit := authServiceFixture()
	seedUser(t, users, "owner@example.com", "correct-password", nil, "")

	for i := 0; i < 2; i++ {
		_, _, err := svc.Authenticate(context.Background(), "owner@example.com", "wrong-password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("attempt %d: error = %v", i+1, err)
		}
	}
	_, _, err := svc.Authenticate(context.Background(), "owner@example.com", "wrong-password")
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("final attempt: error = %v, want ErrAccountLocked", err)
	}
	// Even the correct password is rejected while locked.
	_, _, err = svc.Authenticate(context.Background(), "owner@example.com", "correct-password")
	if !errors.Is(err, ErrAccountLocked) {
		t.Fatalf("locked attempt: error = %v, want ErrAccountLocked", err)
	}
	found := false
	for _, e := range audit.events {
		if e.EventType == model.AuditAccountLocked {
			found = true
		}
	}
	if !found {
		t.Fatal("account lockout event not recorded")
	}
}
