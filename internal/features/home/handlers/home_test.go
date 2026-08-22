package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
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
