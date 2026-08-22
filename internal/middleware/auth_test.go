package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leoarkiteto/zelo/internal/model"
)

type fakeRoleChecker struct {
	roles []model.Role
	err   error
}

func (f *fakeRoleChecker) ActiveRolesForUser(_ context.Context, _, _ string) ([]model.Role, error) {
	return f.roles, f.err
}

type fakeAuditRecorder struct {
	denied int
}

func (f *fakeAuditRecorder) RecordEvent(_ context.Context, _ *string, eventType model.AuditEventType, _ map[string]any) error {
	if eventType == model.AuditAccessDenied {
		f.denied++
	}
	return nil
}

func authenticatedRequest(roleSession bool) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := context.WithValue(req.Context(), userKey, model.User{ID: "u1", Status: model.UserStatusActive})
	ctx = context.WithValue(ctx, sessionKey, model.Session{TokenHash: "s1", UserID: "u1", CondominiumID: "c1"})
	return req.WithContext(ctx)
}

func TestRequireRoleAllowsMatchingRole(t *testing.T) {
	handler := RequireRole(
		&fakeRoleChecker{roles: []model.Role{model.RoleSyndic}},
		&fakeAuditRecorder{},
		model.RoleSyndic,
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authenticatedRequest(true))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRequireRoleDeniesMissingRole(t *testing.T) {
	audit := &fakeAuditRecorder{}
	handler := RequireRole(
		&fakeRoleChecker{roles: []model.Role{model.RoleOwner}},
		audit,
		model.RoleSyndic,
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authenticatedRequest(true))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if audit.denied != 1 {
		t.Fatalf("denied audit events = %d, want 1", audit.denied)
	}
}

func TestRequireRoleRedirectsAnonymous(t *testing.T) {
	handler := RequireRole(
		&fakeRoleChecker{roles: nil},
		&fakeAuditRecorder{},
		model.RoleSyndic,
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Fatalf("location = %q, want /login", loc)
	}
}
