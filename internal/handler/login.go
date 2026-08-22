package handler

import (
	"errors"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/service"
	"github.com/leoarkiteto/zelo/web/templates"
)

func (h *Handler) loginGET(w http.ResponseWriter, r *http.Request) {
	render(w, r, templates.LoginPage(templates.LoginPageData{CSRF: setAnonymousCSRF(w)}))
}

func (h *Handler) loginPOST(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	u, condominiumID, err := h.deps.AuthService.Authenticate(r.Context(), email, password)
	switch {
	case errors.Is(err, service.ErrAccountLocked):
		w.WriteHeader(http.StatusLocked)
		render(w, r, templates.LoginPage(templates.LoginPageData{
			CSRF:  anonymousCSRFValue(r),
			Error: "This account is temporarily locked. Please try again in 15 minutes.",
		}))
		return
	case err != nil:
		w.WriteHeader(http.StatusUnauthorized)
		render(w, r, templates.LoginPage(templates.LoginPageData{
			CSRF:  anonymousCSRFValue(r),
			Error: "Invalid email or password.",
		}))
		return
	}

	rawID, _, err := h.deps.Sessions.Create(r.Context(), u.ID, condominiumID)
	if err != nil {
		http.Error(w, "Failed to start session", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, h.deps.Sessions.Cookie(rawID))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
