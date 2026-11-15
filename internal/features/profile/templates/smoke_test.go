package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func renderToString(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func shell(locale i18n.Language) *organisms.ShellData {
	return &organisms.ShellData{
		User:   &organisms.UserView{Email: "syndic@example.com", Initial: "S"},
		Nav:    []organisms.NavItem{{Label: "Painel", Path: "/", Active: true}},
		CSRF:   "token",
		Locale: locale,
	}
}

func TestProfilePageRendersBothLanguageOptions(t *testing.T) {
	html := renderToString(t, ProfilePage(ProfilePageData{
		Shell:           shell(i18n.LanguageEN),
		CurrentLanguage: i18n.LanguageEN,
	}))
	for _, want := range []string{"🇺🇸", "🇧🇷", "🇵🇹", "English", "Português (BR)", "Português (PT)", `name="language"`, `value="en"`, `value="pt-br"`, `value="pt-pt"`} {
		if !strings.Contains(html, want) {
			t.Errorf("profile page is missing %q", want)
		}
	}
}

func TestProfilePageMarksPTPTSelected(t *testing.T) {
	html := renderToString(t, ProfilePage(ProfilePageData{
		Shell:           shell(i18n.LanguagePTPT),
		CurrentLanguage: i18n.LanguagePTPT,
	}))
	if !strings.Contains(html, `value="pt-pt" checked`) {
		t.Errorf("pt-pt option is not marked selected")
	}
	if strings.Contains(html, `value="en" checked`) || strings.Contains(html, `value="pt-br" checked`) {
		t.Errorf("other options are checked while pt-pt is current")
	}
}

func TestProfilePageMarksCurrentLanguageSelected(t *testing.T) {
	html := renderToString(t, ProfilePage(ProfilePageData{
		Shell:           shell(i18n.LanguagePTBR),
		CurrentLanguage: i18n.LanguagePTBR,
	}))
	if !strings.Contains(html, `value="pt-br" checked`) {
		t.Errorf("current language option is not marked selected")
	}
}
