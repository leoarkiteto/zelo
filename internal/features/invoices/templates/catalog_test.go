package templates

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

var allLanguages = []i18n.Language{i18n.LanguageEN, i18n.LanguagePTBR, i18n.LanguagePTPT}

// missingCatcher records every "missing translation" warning of i18n.T.
type missingCatcher struct{ missing []string }

func (c *missingCatcher) Enabled(context.Context, slog.Level) bool { return true }
func (c *missingCatcher) Handle(_ context.Context, r slog.Record) error {
	if r.Message != "missing translation, falling back to English" {
		return nil
	}
	var lang, key string
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "lang":
			lang = a.Value.String()
		case "key":
			key = a.Value.String()
		}
		return true
	})
	c.missing = append(c.missing, lang+" "+key)
	return nil
}
func (c *missingCatcher) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *missingCatcher) WithGroup(string) slog.Handler      { return c }

// assertKeysExist fails for every key missing in any of the three languages.
// i18n.T falls back to English with a warning, so the warning is the signal.
func assertKeysExist(t *testing.T, keys []string) {
	t.Helper()
	prev := slog.Default()
	c := &missingCatcher{}
	slog.SetDefault(slog.New(c))
	defer slog.SetDefault(prev)

	for _, lang := range allLanguages {
		for _, key := range keys {
			if got := i18n.T(lang, i18n.MessageKey(key)); got == "" || got == key {
				t.Errorf("%s: key %q has no text", lang, key)
			}
		}
	}
	sort.Strings(c.missing)
	for _, m := range c.missing {
		t.Errorf("missing translation: %s", m)
	}
}

// TestDomainMessageKeysInEveryLanguage asserts every key returned by
// domain.MessageKeys() exists in en, pt-br and pt-pt. It lives here so that
// i18n never imports a feature.
func TestDomainMessageKeysInEveryLanguage(t *testing.T) {
	keys := domain.MessageKeys()
	if len(keys) < 100 {
		t.Fatalf("domain.MessageKeys() returned only %d keys", len(keys))
	}
	assertKeysExist(t, keys)
}

// TestHandlerMessageKeysInEveryLanguage covers the flash, error and nav keys
// the handlers (T9) rely on.
func TestHandlerMessageKeysInEveryLanguage(t *testing.T) {
	assertKeysExist(t, []string{
		"nav.invoices", "nav.suppliers", "nav.condominium_settings",
		"invoices.flash.saved", "invoices.flash.registered", "invoices.flash.draft_deleted",
		"invoices.flash.annulled", "invoices.flash.payment_added",
		"invoices.flash.attachment_added", "invoices.flash.attachment_removed",
		"invoices.flash.duplicate", "invoices.flash.duplicate_link",
		"invoices.error.not_found", "invoices.error.request_too_large",
		"invoices.error.load_failed", "invoices.error.save_failed",
		"suppliers.flash.created", "suppliers.flash.updated",
		"suppliers.error.not_found", "suppliers.error.save_failed",
		"suppliers.error.duplicate_nif", "invoices.error.invalid_file", "invoices.error.no_files",
		"settings.flash.currency_changed", "settings.error.invalid_currency",
		"settings.error.save_failed",
	})
}

var keyLiteral = regexp.MustCompile(`"((?:invoices|suppliers|settings|nav|common)\.[a-z0-9_.]+)"`)

// TestTemplateKeysInEveryLanguage scans the .templ and .go sources of this
// package for message-key literals and asserts each exists in all languages.
func TestTemplateKeysInEveryLanguage(t *testing.T) {
	seen := map[string]bool{}
	for _, pattern := range []string{"*.templ", "helpers.go"} {
		files, err := filepath.Glob(pattern)
		if err != nil || len(files) == 0 {
			t.Fatalf("glob %s: %v (%d files)", pattern, err, len(files))
		}
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range keyLiteral.FindAllStringSubmatch(string(src), -1) {
				seen[m[1]] = true
			}
		}
	}
	var keys []string
	for k := range seen {
		keys = append(keys, k)
	}
	if len(keys) < 100 {
		t.Fatalf("found only %d key literals; the scan is broken", len(keys))
	}
	assertKeysExist(t, keys)
}
