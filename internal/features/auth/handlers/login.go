package handlers

import (
	"errors"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/features/auth/core/services"
	"github.com/leoarkiteto/zelo/internal/features/auth/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func (h *Handler) loginGET(w http.ResponseWriter, r *http.Request) {
	httpx.Render(w, r, templates.LoginPage(templates.LoginPageData{
		CSRF:   httpx.SetAnonymousCSRF(w),
		Locale: i18n.Default(),
	}))
}

func (h *Handler) loginPOST(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	u, condominiumID, err := h.deps.AuthService.Authenticate(r.Context(), email, password)
	switch {
	case errors.Is(err, services.ErrAccountLocked):
		w.WriteHeader(http.StatusLocked)
		httpx.Render(w, r, templates.LoginPage(templates.LoginPageData{
			CSRF:   httpx.AnonymousCSRFValue(r),
			Error:  "This account is temporarily locked. Please try again in 15 minutes.",
			Locale: i18n.Default(),
		}))
		return
	case err != nil:
		w.WriteHeader(http.StatusUnauthorized)
		httpx.Render(w, r, templates.LoginPage(templates.LoginPageData{
			CSRF:   httpx.AnonymousCSRFValue(r),
			Error:  "Invalid email or password.",
			Locale: i18n.Default(),
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
