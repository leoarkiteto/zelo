package handler

import (
	"errors"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/auth"
	"github.com/leoarkiteto/zelo/internal/service"
	"github.com/leoarkiteto/zelo/web/templates"
)

func (h *Handler) forgotGET(w http.ResponseWriter, r *http.Request) {
	render(w, r, templates.ForgotPasswordPage(templates.ForgotPasswordPageData{CSRF: setAnonymousCSRF(w)}))
}

func (h *Handler) forgotPOST(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	token, err := h.deps.PasswordReset.RequestReset(r.Context(), email)
	if err != nil {
		h.deps.Logger.Error("password reset request failed", "error", err)
		renderError(w, r, http.StatusInternalServerError, "Something went wrong. Please try again.")
		return
	}
	// In development the token is logged; in production an email sender would
	// deliver the link. The response never reveals whether the account exists.
	if token != "" {
		h.deps.Logger.Info("password reset link", "email", email, "token", token)
	}
	render(w, r, templates.ForgotPasswordPage(templates.ForgotPasswordPageData{
		CSRF: anonymousCSRFValue(r),
		Sent: true,
	}))
}

func (h *Handler) resetGET(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		renderError(w, r, http.StatusBadRequest, "Missing reset token")
		return
	}
	render(w, r, templates.ResetPasswordPage(templates.ResetPasswordPageData{
		Token: token,
		CSRF:  setAnonymousCSRF(w),
	}))
}

func (h *Handler) resetPOST(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	password := r.FormValue("password")
	err := h.deps.PasswordReset.Reset(r.Context(), token, password)
	switch {
	case errors.Is(err, service.ErrInvalidResetToken):
		renderError(w, r, http.StatusGone, "This reset link is invalid or expired. Please request a new one.")
	case errors.Is(err, auth.ErrPasswordTooShort):
		renderError(w, r, http.StatusBadRequest, "Password must be at least 12 characters.")
	case errors.Is(err, auth.ErrPasswordTooLong):
		renderError(w, r, http.StatusBadRequest, "Password must be at most 256 characters.")
	case err != nil:
		h.deps.Logger.Error("password reset failed", "error", err)
		renderError(w, r, http.StatusInternalServerError, "Something went wrong. Please try again.")
	default:
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
