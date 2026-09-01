package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

// TestPagesRender guards every auth page against render panics and checks
// that the core structure of each page is present in the output.
func TestPagesRender(t *testing.T) {
	cases := []struct {
		name string
		page templ.Component
		want []string
	}{
		{"login",
			LoginPage(LoginPageData{CSRF: "tok"}),
			[]string{"Sign in", "Forgot your password", "material-symbols-outlined", `data-icon="eye"`, `data-icon="eye-off"`, "visibility", "visibility_off", `style="display:none;"`}},
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
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := renderToString(t, c.page)
			for _, want := range c.want {
				if !strings.Contains(html, want) {
					t.Errorf("rendered %s is missing %q", c.name, want)
				}
			}
			if c.name == "login" && strings.Contains(html, "<svg") {
				t.Errorf("rendered %s must not contain inline <svg>", c.name)
			}
		})
	}
}
