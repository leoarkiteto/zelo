package handlers

import (
	"errors"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/features/auth/core/services"
	"github.com/leoarkiteto/zelo/internal/features/auth/templates"
	"github.com/leoarkiteto/zelo/internal/shared/security"
)

func (h *Handler) forgotGET(w http.ResponseWriter, r *http.Request) {
	httpx.Render(w, r, templates.ForgotPasswordPage(templates.ForgotPasswordPageData{CSRF: httpx.SetAnonymousCSRF(w)}))
}

func (h *Handler) forgotPOST(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	token, err := h.deps.PasswordReset.RequestReset(r.Context(), email)
	if err != nil {
		h.deps.Logger.Error("password reset request failed", "error", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, "Something went wrong. Please try again.")
		return
	}
	// In development the token is logged; in production an email sender would
	// deliver the link. The response never reveals whether the account exists.
	if token != "" {
		h.deps.Logger.Info("password reset link", "email", email, "token", token)
	}
	httpx.Render(w, r, templates.ForgotPasswordPage(templates.ForgotPasswordPageData{
		CSRF: httpx.AnonymousCSRFValue(r),
		Sent: true,
	}))
}

func (h *Handler) resetGET(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.RenderError(w, r, http.StatusBadRequest, "Missing reset token")
		return
	}
	httpx.Render(w, r, templates.ResetPasswordPage(templates.ResetPasswordPageData{
		Token: token,
		CSRF:  httpx.SetAnonymousCSRF(w),
	}))
}

func (h *Handler) resetPOST(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	password := r.FormValue("password")
	err := h.deps.PasswordReset.Reset(r.Context(), token, password)
	switch {
	case errors.Is(err, services.ErrInvalidResetToken):
		httpx.RenderError(w, r, http.StatusGone, "This reset link is invalid or expired. Please request a new one.")
	case errors.Is(err, security.ErrPasswordTooShort):
		httpx.RenderError(w, r, http.StatusBadRequest, "Password must be at least 12 characters.")
	case errors.Is(err, security.ErrPasswordTooLong):
		httpx.RenderError(w, r, http.StatusBadRequest, "Password must be at most 256 characters.")
	case err != nil:
		h.deps.Logger.Error("password reset failed", "error", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, "Something went wrong. Please try again.")
	default:
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
