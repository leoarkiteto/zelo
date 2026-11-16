package permille

import (
	"errors"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func TestParse(t *testing.T) {
	ok := map[string]int{
		"0": 0, "45,5": 45500, "45.5": 45500, "333.333": 333333, "1000": 1000000,
		"1000.000": 1000000, "0,001": 1, "0.001": 1, " 12 ": 12000, ".5": 500, "7.": 7000, "5.": 5000,
	}
	for in, want := range ok {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	bad := []string{
		"", " ", ".", "-1", "-0,5", "1000,001", "1000.001", "1001", "0,0001", "1.2345",
		"abc", "1,2,3", "1e3", "1e2", "+5", "10000",
	}
	for _, in := range bad {
		if _, err := Parse(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("Parse(%q) err = %v, want ErrInvalid", in, err)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := []struct {
		ppm  int
		lang i18n.Language
		want string
	}{
		{45500, i18n.LanguagePTBR, "45,5"},
		{45500, i18n.LanguagePTPT, "45,5"},
		{45500, i18n.LanguageEN, "45.5"},
		{0, i18n.LanguageEN, "0"},
		{1000000, i18n.LanguagePTBR, "1000"},
		{333333, i18n.LanguageEN, "333.333"},
		{1, i18n.LanguageEN, "0.001"},
		{-250000, i18n.LanguageEN, "-250"},
		{-12500, i18n.LanguagePTBR, "-12,5"},
	}
	for _, c := range cases {
		if got := Format(c.ppm, c.lang); got != c.want {
			t.Errorf("Format(%d,%s) = %q, want %q", c.ppm, c.lang, got, c.want)
		}
	}
}
