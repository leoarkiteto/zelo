package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/profile/core/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// TestProfileToggleFlagsAndAccessibility proves US3: the toggle renders
// exactly two options with the country flags (aria-hidden) plus accessible
// text labels, and the current language is clearly marked selected (FR-002,
// FR-012).
func TestProfileToggleFlagsAndAccessibility(t *testing.T) {
	user := &model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	svc := &fakeProfileService{profile: domain.Profile{Language: i18n.LanguageEN}}
	router := profileRouter(user, sess, svc)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/profile", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /profile = %d, want 200", rec.Code)
	}
	body := rec.Body.String()

	// Exactly two options with flags and labels.
	if strings.Count(body, `name="language"`) != 2 {
		t.Errorf("expected exactly 2 language options, got %d", strings.Count(body, `name="language"`))
	}
	for _, want := range []string{"🇺🇸", "🇧🇷", "English", "Português (BR)"} {
		if !strings.Contains(body, want) {
			t.Errorf("toggle is missing %q", want)
		}
	}
	// Flags are decorative: aria-hidden, never the only identifier (FR-012).
	if !strings.Contains(body, `aria-hidden="true"`) {
		t.Errorf("flags are not marked aria-hidden")
	}
	// The active language is selected (English by default here).
	if !strings.Contains(body, `value="en" checked`) {
		t.Errorf("current language (en) is not marked selected")
	}
}
