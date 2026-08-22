package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// renderToString renders a component and fails the test on any error.
func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func shellFixture() *ShellData {
	return &ShellData{
		User: &UserView{Email: "anna@example.com", Roles: []string{"syndic"}, Initial: "A"},
		Nav:  []NavItem{{Label: "Dashboard", Path: "/", Active: true}},
		CSRF: "tok",
	}
}

// TestPagesRender guards every page against render panics and checks that the
// core structure of each page is present in the output.
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
		{"area",
			AreaPage(AreaPageData{Shell: shellFixture(), Title: "My unit", Message: "Manage your unit.",
				Links: []DashboardLink{{Label: "Dashboard", Path: "/"}}}),
			[]string{"My unit", "Breadcrumb", "Dashboard"}},
		{"login",
			LoginPage(LoginPageData{CSRF: "tok"}),
			[]string{"Sign in", "Forgot your password"}},
		{"login-error",
			LoginPage(LoginPageData{CSRF: "tok", Error: "Invalid email or password."}),
			[]string{"alert-error", "Invalid email or password."}},
		{"register",
			RegisterPage(RegisterPageData{Token: "t", InvitedEmail: "a@b.example", InvitedRole: "owner", CSRF: "tok"}),
			[]string{"Create your account", "Register", "owner"}},
		{"forgot",
			ForgotPasswordPage(ForgotPasswordPageData{CSRF: "tok", Sent: true}),
			[]string{"reset link has been sent"}},
		{"reset",
			ResetPasswordPage(ResetPasswordPageData{Token: "t", CSRF: "tok"}),
			[]string{"Choose a new password", "Set new password"}},
		{"error",
			ErrorPage(ErrorPageData{Status: 403, Message: "Access denied."}),
			[]string{"403", "Access denied.", "Go to dashboard"}},
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
