package i18n

import (
	"testing"
	"time"
)

func TestFormatDate(t *testing.T) {
	ts := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		lang Language
		want string
	}{
		{LanguageEN, "August 23, 2026"},
		{LanguagePTBR, "23/08/2026"},
	}
	for _, c := range cases {
		if got := FormatDate(c.lang, ts); got != c.want {
			t.Errorf("FormatDate(%s) = %q, want %q", c.lang, got, c.want)
		}
	}
}

func TestFormatLongDate(t *testing.T) {
	ts := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		lang Language
		want string
	}{
		{LanguageEN, "Sunday, August 23, 2026"},
		{LanguagePTBR, "domingo, 23 de agosto de 2026"},
	}
	for _, c := range cases {
		if got := FormatLongDate(c.lang, ts); got != c.want {
			t.Errorf("FormatLongDate(%s) = %q, want %q", c.lang, got, c.want)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	if got := FormatNumber(LanguagePTBR, 1234); got != "1234" {
		t.Errorf("FormatNumber(pt-br) = %q, want %q", got, "1234")
	}
	if got := FormatNumber(LanguageEN, 1234); got != "1234" {
		t.Errorf("FormatNumber(en) = %q, want %q", got, "1234")
	}
}
