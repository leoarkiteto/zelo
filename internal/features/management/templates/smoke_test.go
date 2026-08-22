package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	sharedtemplates "github.com/leoarkiteto/zelo/internal/shared/templates"
)

func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func shellFixture() *sharedtemplates.ShellData {
	return &sharedtemplates.ShellData{
		User: &sharedtemplates.UserView{Email: "anna@example.com", Roles: []string{"syndic"}, Initial: "A"},
		Nav:  []sharedtemplates.NavItem{{Label: "Dashboard", Path: "/", Active: true}},
		CSRF: "tok",
	}
}

// TestPagesRender guards every management page against render panics and
// checks that the core structure of each page is present in the output.
func TestPagesRender(t *testing.T) {
	cases := []struct {
		name string
		page templ.Component
		want []string
	}{
		{"roles",
			RolesPage(RolesPageData{Shell: shellFixture(), CSRF: "tok",
				Users: []UserRoleView{{UserID: "u1", Email: "a@b.example", Roles: []string{"owner"}}}}),
			[]string{"Role management", "Grant", "Revoke", "a@b.example", "owner"}},
		{"roles-empty",
			RolesPage(RolesPageData{Shell: shellFixture(), CSRF: "tok"}),
			[]string{"No users yet", "Invite user"}},
		{"invitations",
			InvitationsPage(InvitationsPageData{Shell: shellFixture(), CSRF: "tok",
				Units: []UnitOption{{ID: "1", Code: "A-1"}},
				Invitations: []InvitationView{
					{ID: "i1", UnitCode: "A-1", Role: "owner", Email: "x@y.example", Status: "pending", ExpiresAt: "2026-01-01T00:00:00Z"},
				},
				Flash: "Invitation created"}),
			[]string{"Create invitation", "pending", "Revoke", "Invitation created"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := renderToString(t, c.page)
			for _, want := range c.want {
				if !strings.Contains(html, want) {
					t.Errorf("rendered %s is missing %q", c.name, want)
				}
			}
		})
	}
}
