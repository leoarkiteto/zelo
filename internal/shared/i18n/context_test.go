package i18n

import (
	"context"
	"testing"
)

func TestLanguageFromDefaultsToEnglish(t *testing.T) {
	if got := LanguageFrom(context.Background()); got != Default() {
		t.Fatalf("LanguageFrom(background) = %q, want %q", got, Default())
	}
}

func TestWithLanguageAndLanguageFrom(t *testing.T) {
	ctx := WithLanguage(context.Background(), LanguagePTBR)
	if got := LanguageFrom(ctx); got != LanguagePTBR {
		t.Fatalf("LanguageFrom = %q, want %q", got, LanguagePTBR)
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		in   string
		want Language
	}{
		{"en", LanguageEN},
		{"pt-br", LanguagePTBR},
		{"", LanguageEN},
		{"fr", LanguageEN},
		{"EN", LanguageEN},
		{"pt-BR", LanguageEN},
	}
	for _, c := range cases {
		if got := Resolve(c.in); got != c.want {
			t.Errorf("Resolve(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
