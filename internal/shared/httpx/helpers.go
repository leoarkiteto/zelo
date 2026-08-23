// Package httpx provides shared HTTP handler helpers.
package httpx

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/templates"
)

// Render renders a templ component, returning a 500 on failure.
func Render(w http.ResponseWriter, r *http.Request, comp templ.Component) {
	if err := comp.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

// RenderError renders the shared error page with the given status and message
// in the request's resolved interface language.
func RenderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.WriteHeader(status)
	Render(w, r, templates.ErrorPage(templates.ErrorPageData{
		Status:  status,
		Message: message,
		Locale:  i18n.LanguageFrom(r.Context()),
	}))
}

// SetAnonymousCSRF issues a double-submit CSRF cookie for public forms and
// returns the token to embed in the form.
func SetAnonymousCSRF(w http.ResponseWriter) string {
	tok, err := security.NewCSRFToken()
	if err != nil {
		return ""
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "zelo_csrf",
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return tok
}

// CurrentUser returns the authenticated user from the request context.
func CurrentUser(r *http.Request) *model.User {
	return middleware.UserFrom(r.Context())
}

// CurrentSession returns the active session from the request context.
func CurrentSession(r *http.Request) *model.Session {
	return middleware.SessionFrom(r.Context())
}

// AnonymousCSRFValue returns the zelo_csrf cookie value from the request.
func AnonymousCSRFValue(r *http.Request) string {
	c, err := r.Cookie("zelo_csrf")
	if err != nil {
		return ""
	}
	return c.Value
}
