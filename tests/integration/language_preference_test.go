package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

// languagePreferenceFor reads the stored preference for a user email.
func languagePreferenceFor(t *testing.T, email string) string {
	t.Helper()
	url := testutil.TestDatabaseURL("integration")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := store.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	var pref string
	if err := db.QueryRowContext(context.Background(),
		`SELECT COALESCE(language_preference, '') FROM users WHERE lower(email) = lower($1)`,
		email).Scan(&pref); err != nil {
		t.Fatalf("read preference for %s: %v", email, err)
	}
	return pref
}

// TestIntegrationLanguagePreferenceSavedAndAppliedAtLogin proves US2: the
// language choice is saved to the account and applied automatically at login,
// with English as the default for users without a preference.
func TestIntegrationLanguagePreferenceSavedAndAppliedAtLogin(t *testing.T) {
	router := newApp(t)
	ts := httptest.NewServer(router)
	defer ts.Close()
	base := ts.URL
	client := loginAsSyndic(t, base)

	// No saved preference → default English (FR-007).
	code, body := getPage(t, client, base, "/")
	if code != http.StatusOK || !strings.Contains(body, `lang="en"`) {
		t.Fatalf("dashboard before toggle = %d, want 200 with lang=en", code)
	}

	// Toggle to pt-br from the profile page (plain form → 302 to /profile).
	csrf := csrfFrom(t, client, base, "/profile")
	code, _ = postForm(t, client, base, "/profile/language", map[string]string{
		"language": "pt-br", "csrf_token": csrf,
	})
	if code != http.StatusSeeOther {
		t.Fatalf("toggle language = %d, want 302", code)
	}
	if got := languagePreferenceFor(t, "syndic@example.com"); got != "pt-br" {
		t.Fatalf("stored preference = %q, want pt-br", got)
	}

	// Sign out, then sign in again: the interface appears in pt-br (FR-006).
	code, _ = postForm(t, client, base, "/logout", map[string]string{
		"csrf_token": csrfFrom(t, client, base, "/"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("logout = %d, want 302", code)
	}
	code, _ = postForm(t, client, base, "/login", map[string]string{
		"email": "syndic@example.com", "password": "syndic-pass-123",
		"csrf_token": csrfFrom(t, client, base, "/login"),
	})
	if code != http.StatusSeeOther {
		t.Fatalf("re-login = %d, want 302", code)
	}
	code, body = getPage(t, client, base, "/")
	if code != http.StatusOK || !strings.Contains(body, `lang="pt-br"`) {
		t.Fatalf("dashboard after re-login = %d, want 200 with lang=pt-br", code)
	}
}
