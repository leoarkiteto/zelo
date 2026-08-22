package handler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/leoarkiteto/zelo/internal/auth"
	"github.com/leoarkiteto/zelo/internal/middleware"
	"github.com/leoarkiteto/zelo/internal/model"
	"github.com/leoarkiteto/zelo/web/templates"
)

func render(w http.ResponseWriter, r *http.Request, comp templ.Component) {
	if err := comp.Render(r.Context(), w); err != nil {
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.WriteHeader(status)
	render(w, r, templates.ErrorPage(templates.ErrorPageData{Status: status, Message: message}))
}

// setAnonymousCSRF issues a double-submit CSRF cookie for public forms and
// returns the token to embed in the form.
func setAnonymousCSRF(w http.ResponseWriter) string {
	tok, err := auth.NewCSRFToken()
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

func currentUser(r *http.Request) *model.User {
	return middleware.UserFrom(r.Context())
}

func currentSession(r *http.Request) *model.Session {
	return middleware.SessionFrom(r.Context())
}

func anonymousCSRFValue(r *http.Request) string {
	c, err := r.Cookie("zelo_csrf")
	if err != nil {
		return ""
	}
	return c.Value
}
