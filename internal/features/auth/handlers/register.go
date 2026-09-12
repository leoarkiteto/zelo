package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/auth/services"
	"github.com/leoarkiteto/zelo/internal/features/auth/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

func (h *Handler) registerGET(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.RenderError(w, r, http.StatusBadRequest, "Missing invitation token")
		return
	}
	inv, err := h.deps.Invitations.GetInvitationByTokenHash(r.Context(), h.deps.Tokens.HashToken(token))
	if err != nil || inv.Status != model.InvitationPending || !inv.ExpiresAt.After(time.Now()) {
		httpx.RenderError(w, r, http.StatusGone, "This invitation is invalid, expired, or already used. Please contact the syndic.")
		return
	}
	csrf := httpx.SetAnonymousCSRF(w)
	httpx.Render(w, r, templates.RegisterPage(templates.RegisterPageData{
		Token:        token,
		InvitedEmail: inv.InvitedEmail,
		InvitedRole:  string(inv.InvitedRole),
		CSRF:         csrf,
		Locale:       i18n.Default(),
	}))
}

func (h *Handler) registerPOST(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	email := r.FormValue("email")
	password := r.FormValue("password")
	confirm := r.FormValue("password_confirm")

	page := templates.RegisterPageData{
		Token:  token,
		CSRF:   httpx.AnonymousCSRFValue(r),
		Locale: i18n.Default(),
	}
	if password != confirm {
		page.Error = "Passwords do not match."
		w.WriteHeader(http.StatusBadRequest)
		httpx.Render(w, r, templates.RegisterPage(page))
		return
	}

	err := h.deps.Registration.Register(r.Context(), token, email, password)
	switch {
	case errors.Is(err, services.ErrInvalidInvitation):
		httpx.RenderError(w, r, http.StatusGone, "This invitation is invalid, expired, or already used.")
	case errors.Is(err, services.ErrEmailTaken):
		page.Error = "An account with this email already exists."
		w.WriteHeader(http.StatusConflict)
		httpx.Render(w, r, templates.RegisterPage(page))
	case errors.Is(err, services.ErrEmailMismatch):
		page.Error = "This email does not match the invitation."
		w.WriteHeader(http.StatusBadRequest)
		httpx.Render(w, r, templates.RegisterPage(page))
	case errors.Is(err, services.ErrInvalidEmail):
		page.Error = "Please enter a valid email address."
		w.WriteHeader(http.StatusBadRequest)
		httpx.Render(w, r, templates.RegisterPage(page))
	case err != nil:
		page.Error = "Something went wrong. Please try again."
		w.WriteHeader(http.StatusInternalServerError)
		httpx.Render(w, r, templates.RegisterPage(page))
	default:
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
