package percent

import (
	"errors"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func TestParseBP(t *testing.T) {
	for in, want := range map[string]int{
		"2": 200, "2,5": 250, "2.50": 250, "0": 0, "100": 10000, ",5": 50, ".5": 50,
		"0.05": 5, "10": 1000, "7,5": 750, "12.25": 1225, " 3 ": 300,
	} {
		got, err := ParseBP(in)
		if err != nil || got != want {
			t.Errorf("ParseBP(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "100,01", "101", "1.234", "-1", "+5", "1000", ".", "1,2,3"} {
		if _, err := ParseBP(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseBP(%q) err = %v; want ErrInvalid", in, err)
		}
	}
}

func TestFormatBP(t *testing.T) {
	cases := []struct {
		bp   int
		lang i18n.Language
		want string
	}{
		{250, i18n.LanguageEN, "2.5"},
		{250, i18n.LanguagePTBR, "2,5"},
		{250, i18n.LanguagePTPT, "2,5"},
		{200, i18n.LanguageEN, "2"},
		{1225, i18n.LanguagePTBR, "12,25"},
		{5, i18n.LanguageEN, "0.05"},
		{0, i18n.LanguageEN, "0"},
		{10000, i18n.LanguagePTPT, "100"},
	}
	for _, c := range cases {
		if got := FormatBP(c.bp, c.lang); got != c.want {
			t.Errorf("FormatBP(%d, %q) = %q; want %q", c.bp, c.lang, got, c.want)
		}
	}
}
