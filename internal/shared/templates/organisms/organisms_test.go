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
	for _, want := range []string{"<html", `lang="pt-br"`, "<title>Título · zelo</title>", `href="/static/css/output.css"`, `src="/static/js/htmx.min.js"`, `src="/static/js/app.js"`, `src="/static/js/alpine-csp.min.js"`, "body"} {
		if !strings.Contains(html, want) {
			t.Errorf("Layout is missing %q in %q", want, html)
		}
	}
	// Load order is load-bearing: app.js registers the Alpine components on
	// `alpine:init`, and the Alpine CSP build dispatches that event from a
	// microtask, which runs before the next deferred script is executed.
	appAt := strings.Index(html, `src="/static/js/app.js"`)
	alpineAt := strings.Index(html, `src="/static/js/alpine-csp.min.js"`)
	if appAt > alpineAt {
		t.Errorf("Layout must load app.js before alpine-csp.min.js (app.js at %d, alpine at %d)", appAt, alpineAt)
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

func TestShellRendersNavGroupsAsCollapsibleSubmenus(t *testing.T) {
	shell := &ShellData{
		User: &UserView{Email: "syndic@example.com", Initial: "S"},
		Nav: []NavItem{
			{Label: "Dashboard", Path: "/"},
			{Label: "Finance", Active: true, Children: []NavItem{
				{Label: "Overview", Path: "/finance"},
				{Label: "Invoices", Path: "/invoices", Active: true},
			}},
			{Label: "Tickets", Children: []NavItem{
				{Label: "Ticket inbox", Path: "/tickets/inbox"},
			}},
		},
		CSRF:   "token",
		Locale: i18n.LanguageEN,
	}
	html := testutil.RenderToString(t, Shell(shell, testutil.Component("")))

	if got := strings.Count(html, `<details class="nav-details group"`); got != 2 {
		t.Fatalf("want 2 nav groups, got %d in %q", got, html)
	}
	if got := strings.Count(html, `<details class="nav-details group" open`); got != 1 {
		t.Fatalf("want exactly the active group open, got %d open in %q", got, html)
	}
	// The open group must be the one holding the active page.
	finance := html[strings.Index(html, ">Finance<"):strings.Index(html, ">Tickets<")]
	if !strings.Contains(finance, `nav-link nav-link-active" href="/invoices" aria-current="page"`) {
		t.Errorf("active child /invoices is not highlighted inside the Finance group: %q", finance)
	}
	if strings.Contains(html, `href="/finance" aria-current`) {
		t.Errorf("inactive child must not carry aria-current: %q", html)
	}
	for _, want := range []string{"<summary", `class="nav-sublist"`, `href="/tickets/inbox"`, `href="/"`} {
		if !strings.Contains(html, want) {
			t.Errorf("Shell is missing %q in %q", want, html)
		}
	}
	if strings.Contains(html, "<script") {
		t.Errorf("nav groups must not need inline script: %q", html)
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
