package money

import (
	"context"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		lang  i18n.Language
		cur   Currency
		cents int64
		want  string
	}{
		{i18n.LanguageEN, BRL, 123456, "R$ 1,234.56"},
		{i18n.LanguagePTBR, BRL, 123456, "R$ 1.234,56"},
		{i18n.LanguagePTPT, BRL, 123456, "R$ 1.234,56"},
		{i18n.LanguageEN, EUR, 123456, "€1,234.56"},
		{i18n.LanguagePTBR, EUR, 123456, "€ 1.234,56"},
		{i18n.LanguagePTPT, EUR, 123456, "1 234,56 €"},
		{i18n.LanguageEN, BRL, -123456, "-R$ 1,234.56"},
		{i18n.LanguagePTBR, BRL, -5, "-R$ 0,05"},
		{i18n.LanguageEN, EUR, -123456, "-€1,234.56"},
		{i18n.LanguagePTBR, EUR, -123456, "-€ 1.234,56"},
		{i18n.LanguagePTPT, EUR, -123456, "-1 234,56 €"},
		{i18n.LanguageEN, BRL, 0, "R$ 0.00"},
		{i18n.LanguagePTPT, EUR, 100000000, "1 000 000,00 €"},
	}
	for _, tc := range tests {
		if got := Format(tc.lang, tc.cur, tc.cents); got != tc.want {
			t.Errorf("Format(%q,%q,%d) = %q, want %q", tc.lang, tc.cur, tc.cents, got, tc.want)
		}
	}
}

func TestParseCents(t *testing.T) {
	ok := []struct {
		lang i18n.Language
		in   string
		want int64
	}{
		{i18n.LanguagePTBR, "1234,56", 123456},
		{i18n.LanguagePTBR, "1.234,56", 123456},
		{i18n.LanguagePTBR, "1234", 123400},
		{i18n.LanguagePTBR, "0,5", 50},
		{i18n.LanguagePTPT, "1234,56", 123456},
		{i18n.LanguagePTPT, "1.234,56", 123456},
		{i18n.LanguagePTPT, "1234", 123400},
		{i18n.LanguageEN, "1234.56", 123456},
		{i18n.LanguageEN, "1,234.56", 123456},
		{i18n.LanguageEN, "1234", 123400},
		{i18n.LanguageEN, " 12.30 ", 1230},
	}
	for _, tc := range ok {
		got, err := ParseCents(tc.lang, tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseCents(%q,%q) = %d, %v; want %d", tc.lang, tc.in, got, err, tc.want)
		}
	}
	bad := []struct {
		lang i18n.Language
		in   string
	}{
		{i18n.LanguageEN, ""},
		{i18n.LanguageEN, "abc"},
		{i18n.LanguageEN, "-1.00"},
		{i18n.LanguageEN, "1.234"},
		{i18n.LanguageEN, "1.2.3"},
		{i18n.LanguageEN, "1."},
		{i18n.LanguageEN, ".5"},
		{i18n.LanguageEN, "1,23.45"},
		{i18n.LanguageEN, "1e3"},
		{i18n.LanguagePTBR, "1234.56"},
		{i18n.LanguagePTBR, "1,234"},
		{i18n.LanguagePTBR, "12,345"},
		{i18n.LanguagePTBR, "-5"},
		{i18n.LanguagePTBR, "99999999999999999999"},
		{i18n.LanguagePTPT, "1.23,45"},
	}
	for _, tc := range bad {
		if got, err := ParseCents(tc.lang, tc.in); err == nil {
			t.Errorf("ParseCents(%q,%q) = %d, want error", tc.lang, tc.in, got)
		}
	}
}

func TestCurrencyContext(t *testing.T) {
	if !EUR.Valid() || !BRL.Valid() || Currency("USD").Valid() || Currency("").Valid() {
		t.Fatal("Valid() wrong")
	}
	if Default() != BRL {
		t.Fatal("Default must be BRL")
	}
	ctx := context.Background()
	if From(ctx) != BRL {
		t.Fatal("unset context must give BRL")
	}
	if From(WithCurrency(ctx, EUR)) != EUR {
		t.Fatal("WithCurrency(EUR) not returned")
	}
}
