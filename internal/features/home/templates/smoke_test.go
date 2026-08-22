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

// TestPagesRender guards every home page against render panics and checks
// that the core structure of each page is present in the output.
func TestPagesRender(t *testing.T) {
	cases := []struct {
		name string
		page templ.Component
		want []string
	}{
		{"dashboard",
			DashboardPage(DashboardData{
				Shell:    shellFixture(),
				Greeting: "Good morning", Name: "Anna", Date: "Monday, January 13, 2025",
				Roles: []string{"syndic", "owner"},
				QuickActions: []QuickAction{
					{Icon: "users", Label: "Role management", Description: "Assign roles", Path: "/roles"},
				},
			}),
			[]string{"topbar", "Quick actions", "Role management", "Your roles", "syndic"}},
		{"area",
			AreaPage(AreaPageData{Shell: shellFixture(), Title: "My unit", Message: "Manage your unit.",
				Links: []DashboardLink{{Label: "Dashboard", Path: "/"}}}),
			[]string{"My unit", "Breadcrumb", "Dashboard"}},
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

// TestShellRendersMobileDrawer checks the CSS-only navigation toggle is
// present on shell pages so mobile navigation keeps working.
func TestShellRendersMobileDrawer(t *testing.T) {
	html := renderToString(t, DashboardPage(DashboardData{
		Shell: shellFixture(), Greeting: "Good morning", Name: "Anna", Date: "x",
		Roles: []string{"syndic"},
	}))
	for _, want := range []string{"nav-toggle", "peer-checked:translate-x-0", `action="/logout"`, "csrf_token"} {
		if !strings.Contains(html, want) {
			t.Errorf("shell is missing %q", want)
		}
	}
}
