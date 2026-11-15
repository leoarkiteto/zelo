package templates

import (
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

func TestSettingsPageRenders(t *testing.T) {
	for _, lang := range []i18n.Language{i18n.LanguageEN, i18n.LanguagePTBR, i18n.LanguagePTPT} {
		out := renderToString(t, SettingsPage(SettingsPageData{
			Shell: shellFixture(), Locale: lang, CSRF: "tok", Currency: money.EUR, Flash: "ok",
		}))
		for _, want := range []string{
			`action="/condominium/settings"`,
			`name="csrf_token" value="tok"`,
			`name="currency"`,
			`<option value="EUR" selected>`,
			`<option value="BRL">`,
			`aria-hidden="true">*</span>`,
			"required",
			i18n.T(lang, "settings.currency_warning"),
		} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: settings page missing %q", lang, want)
			}
		}
		if strings.Contains(out, "<script>") {
			t.Errorf("%s: inline script", lang)
		}
	}
}

func TestSettingsPageBRLSelectedAndError(t *testing.T) {
	out := renderToString(t, SettingsPage(SettingsPageData{
		Shell: shellFixture(), Locale: i18n.LanguageEN, CSRF: "tok", Currency: money.BRL, Error: "Choose a valid currency.",
	}))
	if !strings.Contains(out, `<option value="BRL" selected>`) || strings.Contains(out, `<option value="EUR" selected>`) {
		t.Error("BRL should be the selected option")
	}
	if !strings.Contains(out, "Choose a valid currency.") {
		t.Error("error message not rendered")
	}
	if !strings.Contains(out, "Existing values are not converted.") {
		t.Error("the not-converted warning is missing")
	}
}
