package i18n

import "testing"

func TestLanguageConstants(t *testing.T) {
	if LanguageEN != "en" {
		t.Fatalf("LanguageEN = %q, want %q", LanguageEN, "en")
	}
	if LanguagePTBR != "pt-br" {
		t.Fatalf("LanguagePTBR = %q, want %q", LanguagePTBR, "pt-br")
	}
}

func TestLanguageValid(t *testing.T) {
	valid := []Language{LanguageEN, LanguagePTBR}
	for _, l := range valid {
		if !l.Valid() {
			t.Errorf("Valid() = false for %q, want true", l)
		}
	}
	invalid := []Language{"", "fr", "en-US", "pt", "pt-BR"}
	for _, l := range invalid {
		if l.Valid() {
			t.Errorf("Valid() = true for %q, want false", l)
		}
	}
}

func TestLanguageDefault(t *testing.T) {
	if Default() != LanguageEN {
		t.Fatalf("Default() = %q, want %q", Default(), LanguageEN)
	}
}
