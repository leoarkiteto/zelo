package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

func localeRequest(user *model.User) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if user != nil {
		ctx := context.WithValue(req.Context(), userKey, *user)
		ctx = context.WithValue(ctx, sessionKey, model.Session{TokenHash: "s1", UserID: user.ID, CondominiumID: "c1"})
		req = req.WithContext(ctx)
	}
	return req
}

func TestWithLocaleAnonymousDefaultsToEnglish(t *testing.T) {
	var got i18n.Language
	handler := WithLocale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = i18n.LanguageFrom(r.Context())
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, localeRequest(nil))
	if got != i18n.LanguageEN {
		t.Fatalf("anonymous locale = %q, want %q", got, i18n.LanguageEN)
	}
}

func TestWithLocaleAppliesUserPreference(t *testing.T) {
	var got i18n.Language
	handler := WithLocale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = i18n.LanguageFrom(r.Context())
	}))
	rec := httptest.NewRecorder()
	user := &model.User{ID: "u1", Status: model.UserStatusActive, LanguagePreference: "pt-br"}
	handler.ServeHTTP(rec, localeRequest(user))
	if got != i18n.LanguagePTBR {
		t.Fatalf("user locale = %q, want %q", got, i18n.LanguagePTBR)
	}
}

func TestWithLocaleInvalidPreferenceFallsBackToEnglish(t *testing.T) {
	var got i18n.Language
	handler := WithLocale(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = i18n.LanguageFrom(r.Context())
	}))
	rec := httptest.NewRecorder()
	for _, pref := range []string{"", "fr"} {
		user := &model.User{ID: "u1", Status: model.UserStatusActive, LanguagePreference: pref}
		handler.ServeHTTP(rec, localeRequest(user))
		if got != i18n.LanguageEN {
			t.Fatalf("locale for preference %q = %q, want %q", pref, got, i18n.LanguageEN)
		}
	}
}
