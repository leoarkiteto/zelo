package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/leoarkiteto/zelo/internal/features/management/services"
	"github.com/leoarkiteto/zelo/internal/features/management/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/percent"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

const settingsPath = "/condominium/settings"

// settingsGET renders the condominium settings page (FR-2, FR-3).
func (h *Handler) settingsGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	flash := ""
	switch r.URL.Query().Get("flash") {
	case "currency_changed":
		flash = i18n.T(locale, "settings.flash.currency_changed")
	case "late_fee_saved":
		flash = i18n.T(locale, "settings.flash.late_fee_saved")
	}
	h.renderSettings(w, r, http.StatusOK, flash, "", "", nil)
}

// settingsPOST changes the condominium currency or the late-fee settings
// (form=late_fee) and redirects with a flash.
func (h *Handler) settingsPOST(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("form") == "late_fee" {
		h.lateFeePOST(w, r)
		return
	}
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	u := httpx.CurrentUser(r)
	submitted := money.Currency(r.FormValue("currency"))

	err := h.deps.SettingsService.ChangeCurrency(r.Context(), u.ID, sess.CondominiumID, submitted)
	switch {
	case errors.Is(err, services.ErrInvalidCurrency):
		h.renderSettings(w, r, http.StatusUnprocessableEntity, "", i18n.T(locale, "settings.error.invalid_currency"), submitted, nil)
	case errors.Is(err, store.ErrNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "settings.error.save_failed"))
	case err != nil:
		slog.Error("management: change currency", "error", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "settings.error.save_failed"))
	default:
		http.Redirect(w, r, settingsPath+"?flash=currency_changed", http.StatusSeeOther)
	}
}

// lateFeeForm is the rejected late-fee input echoed back to the user.
type lateFeeForm struct{ fee, interest, grace string }

func (h *Handler) lateFeePOST(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	u := httpx.CurrentUser(r)
	in := services.LateFeeInput{
		Fee:       r.FormValue("late_fee"),
		Interest:  r.FormValue("late_interest"),
		GraceDays: r.FormValue("late_grace_days"),
	}

	err := h.deps.SettingsService.SaveLateFee(r.Context(), u.ID, sess.CondominiumID, in)
	var key i18n.MessageKey
	switch {
	case err == nil:
		http.Redirect(w, r, settingsPath+"?flash=late_fee_saved", http.StatusSeeOther)
		return
	case errors.Is(err, services.ErrInvalidFee):
		key = "settings.error.invalid_fee"
	case errors.Is(err, services.ErrInvalidInterest):
		key = "settings.error.invalid_interest"
	case errors.Is(err, services.ErrInvalidGrace):
		key = "settings.error.invalid_grace"
	case errors.Is(err, services.ErrFeeAboveBRLCap):
		key = "settings.error.fee_above_cap"
	case errors.Is(err, store.ErrNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "settings.error.save_failed"))
		return
	default:
		slog.Error("management: save late fee", "error", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "settings.error.save_failed"))
		return
	}
	h.renderSettings(w, r, http.StatusUnprocessableEntity, "", i18n.T(locale, key), "",
		&lateFeeForm{fee: in.Fee, interest: in.Interest, grace: in.GraceDays})
}

// renderSettings renders the page with the stored settings, unless override is
// a valid currency or form holds input the user submitted before a failed save.
func (h *Handler) renderSettings(w http.ResponseWriter, r *http.Request, status int, flash, errMsg string, override money.Currency, form *lateFeeForm) {
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	cur, err := h.deps.Condominiums.GetCurrency(r.Context(), sess.CondominiumID)
	if err != nil {
		slog.Error("management: load currency", "error", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "settings.error.save_failed"))
		return
	}
	lf, err := h.deps.Condominiums.GetLateFeeSettings(r.Context(), sess.CondominiumID)
	if err != nil {
		slog.Error("management: load late fee settings", "error", err)
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
		Shell:        shell,
		Locale:       locale,
		CSRF:         sess.CSRFToken,
		Currency:     cur,
		Flash:        flash,
		Error:        errMsg,
		FeeText:      percent.FormatBP(lf.FeeBP, locale),
		InterestText: percent.FormatBP(lf.InterestBP, locale),
		GraceText:    strconv.Itoa(lf.GraceDays),
	}
	if form != nil {
		data.FeeText, data.InterestText, data.GraceText = form.fee, form.interest, form.grace
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	httpx.Render(w, r, templates.SettingsPage(data))
}
