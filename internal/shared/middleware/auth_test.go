package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
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

func TestRequireRoleWithCustomDeniedAuditsFirst(t *testing.T) {
	audit := &fakeAuditRecorder{}
	var auditAtDenied int
	handler := RequireRoleWith(
		&fakeRoleChecker{roles: []model.Role{model.RoleOwner}},
		audit,
		RoleGuardResponders{Denied: func(w http.ResponseWriter, _ *http.Request) {
			auditAtDenied = audit.denied
			w.WriteHeader(http.StatusTeapot)
		}},
		model.RoleSyndic,
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next must not run")
	}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, authenticatedRequest(true))
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", rec.Code)
	}
	if auditAtDenied != 1 {
		t.Fatalf("audit events before Denied = %d, want 1", auditAtDenied)
	}
}

func TestRequireRoleWithCustomUnauthenticated(t *testing.T) {
	handler := RequireRoleWith(
		&fakeRoleChecker{},
		&fakeAuditRecorder{},
		RoleGuardResponders{Unauthenticated: func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}},
		model.RoleSyndic,
	)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRolesFromOnAllow(t *testing.T) {
	var got []model.Role
	handler := RequireRoleWith(
		&fakeRoleChecker{roles: []model.Role{model.RoleOwner, model.RoleSyndic}},
		&fakeAuditRecorder{},
		RoleGuardResponders{},
		model.RoleSyndic,
	)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = RolesFrom(r.Context())
	}))

	handler.ServeHTTP(httptest.NewRecorder(), authenticatedRequest(true))
	if len(got) != 2 || got[0] != model.RoleOwner || got[1] != model.RoleSyndic {
		t.Fatalf("roles = %v, want [owner syndic]", got)
	}
	if RolesFrom(context.Background()) != nil {
		t.Fatal("RolesFrom on bare context must be nil")
	}
}

func TestIsHX(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if IsHX(r) {
		t.Fatal("IsHX without header = true")
	}
	r.Header.Set("HX-Request", "true")
	if !IsHX(r) {
		t.Fatal("IsHX with header = false")
	}
}

func TestHXRedirect(t *testing.T) {
	rec := httptest.NewRecorder()
	HXRedirect(rec, "/login")
	if got := rec.Header().Get("HX-Redirect"); got != "/login" {
		t.Fatalf("HX-Redirect = %q, want /login", got)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
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
