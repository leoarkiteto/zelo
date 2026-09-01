package organisms

import (
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/templates/testutil"
)

func TestLayoutRendersDocument(t *testing.T) {
	shell := &ShellData{
		User:   &UserView{Email: "syndic@example.com", Initial: "S"},
		Nav:    []NavItem{{Label: "Painel", Path: "/", Active: true}},
		CSRF:   "token",
		Locale: i18n.LanguagePTBR,
	}
	html := testutil.RenderToString(t, Layout("Título", i18n.LanguagePTBR, shell, testutil.Component("body")))
	for _, want := range []string{"<html", `lang="pt-br"`, "<title>Título · zelo</title>", `href="/static/css/output.css"`, `src="/static/js/htmx.min.js"`, "body"} {
		if !strings.Contains(html, want) {
			t.Errorf("Layout is missing %q in %q", want, html)
		}
	}
}

func TestShellRendersAppChrome(t *testing.T) {
	shell := &ShellData{
		User:   &UserView{Email: "syndic@example.com", Initial: "S"},
		Nav:    []NavItem{{Label: "Painel", Path: "/", Active: true}},
		CSRF:   "token",
		Locale: i18n.LanguageEN,
	}
	html := testutil.RenderToString(t, Shell(shell, testutil.Component("<p>content</p>")))
	for _, want := range []string{`id="app-shell"`, `lang="en"`, "Painel", "<p>content</p>", "material-symbols-outlined", ">menu<"} {
		if !strings.Contains(html, want) {
			t.Errorf("Shell is missing %q in %q", want, html)
		}
	}
	if strings.Contains(html, "<svg") {
		t.Errorf("Shell must not render inline <svg>, got %q", html)
	}
}

func TestErrorPageRendersStatusAndLink(t *testing.T) {
	html := testutil.RenderToString(t, ErrorPage(ErrorPageData{Status: 403, Message: "Access denied.", Locale: i18n.LanguageEN}))
	for _, want := range []string{"403", "Access denied.", `href="/"`} {
		if !strings.Contains(html, want) {
			t.Errorf("ErrorPage is missing %q in %q", want, html)
		}
	}
}
