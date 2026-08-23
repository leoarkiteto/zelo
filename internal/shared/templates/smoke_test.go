package templates

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
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

// TestPagesRender guards every shared page against render panics and checks
// that the core structure of each page is present in the output.
func TestPagesRender(t *testing.T) {
	cases := []struct {
		name string
		page templ.Component
		want []string
	}{
		{"error",
			ErrorPage(ErrorPageData{Status: 403, Message: "Access denied.", Locale: i18n.LanguageEN}),
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

// TestLayoutRendersLocale guards the i18n shell contract: the rendered page
// carries the locale on <html lang> and on the swappable #app-shell element.
func TestLayoutRendersLocale(t *testing.T) {
	shell := &ShellData{
		User:   &UserView{Email: "syndic@example.com", Initial: "S"},
		Nav:    []NavItem{{Label: "Painel", Path: "/", Active: true}},
		CSRF:   "token",
		Locale: i18n.LanguagePTBR,
	}
	body := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, "<p>body</p>")
		return err
	})
	html := renderToString(t, Layout("Perfil", i18n.LanguagePTBR, shell, body))
	for _, want := range []string{`<html lang="pt-br">`, `<div id="app-shell" lang="pt-br"`} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered layout is missing %q", want)
		}
	}
}
