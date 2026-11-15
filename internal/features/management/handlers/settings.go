package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/features/management/services"
	"github.com/leoarkiteto/zelo/internal/features/management/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

const settingsPath = "/condominium/settings"

// settingsGET renders the currency settings page (FR-2).
func (h *Handler) settingsGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	flash := ""
	if r.URL.Query().Get("flash") == "currency_changed" {
		flash = i18n.T(locale, "settings.flash.currency_changed")
	}
	h.renderSettings(w, r, http.StatusOK, flash, "", "")
}

// settingsPOST changes the condominium currency and redirects with a flash.
func (h *Handler) settingsPOST(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	u := httpx.CurrentUser(r)
	submitted := money.Currency(r.FormValue("currency"))

	err := h.deps.SettingsService.ChangeCurrency(r.Context(), u.ID, sess.CondominiumID, submitted)
	switch {
	case errors.Is(err, services.ErrInvalidCurrency):
		h.renderSettings(w, r, http.StatusUnprocessableEntity, "", i18n.T(locale, "settings.error.invalid_currency"), submitted)
	case errors.Is(err, store.ErrNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "settings.error.save_failed"))
	case err != nil:
		slog.Error("management: change currency", "error", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "settings.error.save_failed"))
	default:
		http.Redirect(w, r, settingsPath+"?flash=currency_changed", http.StatusSeeOther)
	}
}

// renderSettings renders the page with the stored currency, unless override is
// a valid currency the user selected before a failed save.
func (h *Handler) renderSettings(w http.ResponseWriter, r *http.Request, status int, flash, errMsg string, override money.Currency) {
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	cur, err := h.deps.Condominiums.GetCurrency(r.Context(), sess.CondominiumID)
	if err != nil {
		slog.Error("management: load currency", "error", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "settings.error.save_failed"))
		return
	}
	if override.Valid() {
		cur = override
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, settingsPath)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "settings.error.save_failed"))
		return
	}
	data := templates.SettingsPageData{
		Shell:    shell,
		Locale:   locale,
		CSRF:     sess.CSRFToken,
		Currency: cur,
		Flash:    flash,
		Error:    errMsg,
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	httpx.Render(w, r, templates.SettingsPage(data))
}
