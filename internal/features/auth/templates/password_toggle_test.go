package templates

import (
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// TestLoginPasswordToggleContract locks the contract of the reveal control on
// the sign-in form. It is client-local interaction state — the ratified Alpine
// exception to constitution Principle VI — so this test pins the three things
// that would otherwise break silently:
//
//  1. it is driven by Alpine bindings, never by an htmx request;
//  2. its labels come from i18n rather than hardcoded English in JavaScript;
//  3. the server-rendered fallback still masks the field before Alpine boots,
//     and if JavaScript never runs at all.
func TestLoginPasswordToggleContract(t *testing.T) {
	html := renderToString(t, LoginPage(LoginPageData{CSRF: "tok", Locale: i18n.LanguagePTBR}))

	for _, want := range []string{
		`x-data="passwordToggle"`,
		`x-on:click="toggle"`,
		`x-bind:type="inputType"`,
		`x-bind:aria-label="toggleLabel"`,
		`x-bind:aria-pressed="pressed"`,
		`x-show="eyeVisible"`,
		`x-show="eyeOffVisible"`,
		// Static fallback: the field is masked before Alpine initialises and
		// stays masked when JavaScript never runs.
		`type="password"`,
		`style="display:none;"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("login password toggle is missing %q", want)
		}
	}

	// Translated labels reach the component through data-* attributes. The
	// previous implementation hardcoded "Hide password"/"Show password", so the
	// button flipped to English after the first click in a pt-BR session.
	for _, want := range []string{`data-label-show="Mostrar senha"`, `data-label-hide="Ocultar senha"`} {
		if !strings.Contains(html, want) {
			t.Errorf("login password toggle labels must be translated: missing %q", want)
		}
	}
	for _, unwanted := range []string{`"Hide password"`, `"Show password"`} {
		if strings.Contains(html, unwanted) {
			t.Errorf("login password toggle must not hardcode English labels: found %s", unwanted)
		}
	}

	// Client-local means no server round trip: htmx has nothing to fetch here.
	if strings.Contains(html, "hx-post") || strings.Contains(html, "hx-get") {
		t.Error("login password toggle is client-local and must not issue htmx requests")
	}

	// And no inline script: behaviour belongs in web/static/js/app.js, which is
	// the only way the page stays compatible with a strict CSP.
	if strings.Contains(html, "<script>") {
		t.Error("login must not carry an inline <script>; behaviour belongs in web/static/js/app.js")
	}
}
