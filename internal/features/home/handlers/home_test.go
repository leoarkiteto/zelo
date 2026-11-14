package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type fakeRoles struct {
	roles []model.Role
}

func (f fakeRoles) ActiveRolesForUser(_ context.Context, _, _ string) ([]model.Role, error) {
	return f.roles, nil
}

type fakeAudit struct{}

func (fakeAudit) RecordEvent(_ context.Context, _ *string, _ model.AuditEventType, _ map[string]any) error {
	return nil
}

type fakeSessionReader struct{ sess model.Session }

func (f fakeSessionReader) Read(*http.Request) (model.Session, error) { return f.sess, nil }

type fakeUserLoader struct{ u model.User }

func (f fakeUserLoader) GetUserByID(_ context.Context, _ string) (model.User, error) { return f.u, nil }

func homeRouter(user *model.User, sess *model.Session, roles []model.Role) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Roles: fakeRoles{roles: roles}, Audit: fakeAudit{}})
	var root http.Handler = mux
	if user != nil && sess != nil {
		root = middleware.WithUser(fakeSessionReader{sess: *sess}, fakeUserLoader{u: *user})(root)
	}
	return root
}

func TestHomeRoutesUnauthenticatedRedirect(t *testing.T) {
	router := homeRouter(nil, nil, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("GET / = %d %q, want 303 /login", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHomeDashboardForSyndic(t *testing.T) {
	user := &model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	router := homeRouter(user, sess, []model.Role{model.RoleSyndic})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
}

func homeBody(t *testing.T, roles []model.Role) string {
	t.Helper()
	user := &model.User{ID: "u3", Email: "member@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th3", UserID: "u3", CondominiumID: "c1", CSRFToken: "x"}
	rec := httptest.NewRecorder()
	homeRouter(user, sess, roles).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

func TestHomeShowsFinancePlaceholderForMembers(t *testing.T) {
	for _, role := range []model.Role{model.RoleOwner, model.RoleTenant, model.RoleSyndic} {
		body := homeBody(t, []model.Role{role})
		for _, want := range []string{`id="finance-overview"`, `hx-get="/finance/overview"`, `href="/finance/overview"`} {
			if !strings.Contains(body, want) {
				t.Errorf("role %s: body missing %q", role, want)
			}
		}
	}
}

func TestHomeHidesFinancePlaceholderWithoutRoles(t *testing.T) {
	body := homeBody(t, nil)
	if strings.Contains(body, `id="finance-overview"`) || strings.Contains(body, "/finance/overview") {
		t.Error("user without roles must not see the finance placeholder")
	}
}

type spyRoles struct {
	calls int
	err   error
}

func (s *spyRoles) ActiveRolesForUser(_ context.Context, _, _ string) ([]model.Role, error) {
	s.calls++
	return nil, s.err
}

func spyHome(spy *spyRoles, condominiumID string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Roles: spy, Audit: fakeAudit{}})
	user := model.User{ID: "u4", Email: "new@example.com", Status: model.UserStatusActive}
	sess := model.Session{TokenHash: "th4", UserID: "u4", CondominiumID: condominiumID, CSRFToken: "x"}
	root := middleware.WithUser(fakeSessionReader{sess: sess}, fakeUserLoader{u: user})(mux)
	rec := httptest.NewRecorder()
	root.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec
}

func TestHomeWithoutCondominiumRendersWithoutRoleLookup(t *testing.T) {
	spy := &spyRoles{err: context.DeadlineExceeded}
	rec := spyHome(spy, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if spy.calls != 0 {
		t.Errorf("ActiveRolesForUser called %d times, want 0", spy.calls)
	}
	if body := rec.Body.String(); strings.Contains(body, `id="finance-overview"`) || strings.Contains(body, "/finance/overview") {
		t.Error("user without condominium must not see the finance placeholder")
	}
}

func TestHomeRoleLookupErrorReturns500(t *testing.T) {
	spy := &spyRoles{err: context.DeadlineExceeded}
	rec := spyHome(spy, "c1")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET / = %d, want 500", rec.Code)
	}
	if spy.calls != 1 {
		t.Errorf("ActiveRolesForUser called %d times, want 1", spy.calls)
	}
}

func TestHomeCondominiumForTenantForbidden(t *testing.T) {
	user := &model.User{ID: "u2", Email: "tenant@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th2", UserID: "u2", CondominiumID: "c1", CSRFToken: "x"}
	router := homeRouter(user, sess, []model.Role{model.RoleTenant})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/condominium", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("GET /condominium = %d, want 403", rec.Code)
	}
}
