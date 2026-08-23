package middleware

import (
	"net/http"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// WithLocale resolves the interface language for the request and stores it in
// the request context. It runs after WithUser: authenticated requests use the
// user's saved language preference (falling back to English when unset or
// invalid); anonymous requests use the default language, English (FR-006/FR-007).
func WithLocale(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := i18n.Default()
		if u := UserFrom(r.Context()); u != nil {
			lang = i18n.Resolve(u.LanguagePreference)
		}
		next.ServeHTTP(w, r.WithContext(i18n.WithLanguage(r.Context(), lang)))
	})
}
