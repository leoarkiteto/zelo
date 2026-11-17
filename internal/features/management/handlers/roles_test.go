package handlers

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

// rolesWithUsers is fakeRoles with a populated user list.
type rolesWithUsers struct {
	fakeRoles
	users []store.UserRolesRow
}

func (f *rolesWithUsers) ListUsersWithRoles(context.Context, string) ([]store.UserRolesRow, error) {
	return f.users, nil
}

func rolesRouter(users []store.UserRolesRow) http.Handler {
	deps := Deps{
		Roles: &rolesWithUsers{fakeRoles: fakeRoles{roles: []model.Role{model.RoleSyndic}}, users: users},
		Audit: &fakeAudit{},
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)
	user := model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	var root http.Handler = mux
	root = middleware.WithLocale(root)
	return middleware.WithUser(fakeSessionReader{sess: sess}, fakeUserLoader{u: user})(root)
}

// TestRolesPaginationLinksAfterPOSTError guards code review F1: the error
// re-render of POST /roles/assign must link to GET /roles, not to the POST route.
func TestRolesPaginationLinksAfterPOSTError(t *testing.T) {
	var users []store.UserRolesRow
	for i := 1; i <= 25; i++ {
		users = append(users, store.UserRolesRow{UserID: fmt.Sprintf("id%02d", i), Email: fmt.Sprintf("user%02d@example.com", i)})
	}
	r := rolesRouter(users)

	for _, path := range []string{"/roles", "/roles/assign", "/roles/revoke"} {
		method := http.MethodPost
		form := url.Values{"user_id": {"id01"}, "role": {"not-a-role"}}
		if path == "/roles" {
			method, form = http.MethodGet, nil
		}
		body := html.UnescapeString(do(r, method, path, form).Body.String())
		if !strings.Contains(body, "user20@example.com") || strings.Contains(body, "user21@example.com") {
			t.Errorf("%s %s: page 1 should list user01..user20 only", method, path)
		}
		if !strings.Contains(body, `href="/roles?page=2"`) {
			t.Errorf("%s %s: next link should be /roles?page=2", method, path)
		}
		if strings.Contains(body, "/roles/assign?") || strings.Contains(body, "/roles/revoke?") {
			t.Errorf("%s %s: pagination must not link to a POST-only route", method, path)
		}
	}
}
