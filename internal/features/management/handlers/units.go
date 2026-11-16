package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/leoarkiteto/zelo/internal/features/management/services"
	"github.com/leoarkiteto/zelo/internal/features/management/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
)

const unitsPath = "/condominium/units"

// unitsGET lists the units with the sum of shares and the difference to 100%.
func (h *Handler) unitsGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	list, err := h.deps.UnitService.List(r.Context(), sess.CondominiumID)
	if err != nil {
		slog.Error("management: list units", "error", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "units.error.save_failed"))
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, unitsPath)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "units.error.save_failed"))
		return
	}
	flash := ""
	switch r.URL.Query().Get("flash") {
	case "created":
		flash = i18n.T(locale, "units.flash.created")
	case "updated":
		flash = i18n.T(locale, "units.flash.updated")
	}
	status := templates.ShareStatusOK
	switch {
	case list.DiffPPM > 0:
		status = templates.ShareStatusMissing
	case list.DiffPPM < 0:
		status = templates.ShareStatusExcess
	}
	rows := make([]templates.UnitRowView, 0, len(list.Units))
	for _, u := range list.Units {
		rows = append(rows, templates.UnitRowView{ID: u.ID, Code: u.Code, Share: permille.Format(u.SharePPM, locale)})
	}
	httpx.Render(w, r, templates.UnitsPage(templates.UnitsPageData{
		Shell:  shell,
		Locale: locale,
		Units:  rows,
		Sum:    permille.Format(list.SumPPM, locale),
		Diff:   permille.Format(list.DiffPPM, locale),
		Status: status,
		Flash:  flash,
	}))
}

// unitsNewGET renders the empty create form.
func (h *Handler) unitsNewGET(w http.ResponseWriter, r *http.Request) {
	h.renderUnitForm(w, r, http.StatusOK, templates.UnitFormData{Action: unitsPath})
}

// unitsPOST creates a unit.
func (h *Handler) unitsPOST(w http.ResponseWriter, r *http.Request) {
	sess := httpx.CurrentSession(r)
	u := httpx.CurrentUser(r)
	code, share := r.FormValue("code"), r.FormValue("share")
	_, err := h.deps.UnitService.Create(r.Context(), u.ID, sess.CondominiumID, code, share)
	if err != nil {
		h.unitSaveFailed(w, r, err, templates.UnitFormData{Action: unitsPath, Code: code, Share: share})
		return
	}
	http.Redirect(w, r, unitsPath+"?flash=created", http.StatusSeeOther)
}

// unitsEditGET renders the edit form of one unit.
func (h *Handler) unitsEditGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	unit, err := h.deps.UnitService.Get(r.Context(), sess.CondominiumID, r.PathValue("id"))
	if err != nil {
		h.unitSaveFailed(w, r, err, templates.UnitFormData{})
		return
	}
	h.renderUnitForm(w, r, http.StatusOK, templates.UnitFormData{
		Action: unitsPath + "/" + unit.ID + "/edit",
		Edit:   true,
		Code:   unit.Code,
		Share:  permille.Format(unit.SharePPM, locale),
	})
}

// unitsEditPOST updates a unit.
func (h *Handler) unitsEditPOST(w http.ResponseWriter, r *http.Request) {
	sess := httpx.CurrentSession(r)
	u := httpx.CurrentUser(r)
	id := r.PathValue("id")
	code, share := r.FormValue("code"), r.FormValue("share")
	if err := h.deps.UnitService.Update(r.Context(), u.ID, sess.CondominiumID, id, code, share); err != nil {
		h.unitSaveFailed(w, r, err, templates.UnitFormData{Action: unitsPath + "/" + id + "/edit", Edit: true, Code: code, Share: share})
		return
	}
	http.Redirect(w, r, unitsPath+"?flash=updated", http.StatusSeeOther)
}

// unitSaveFailed maps a service error to a re-rendered form or an error page.
// Nothing was saved on any of these paths.
func (h *Handler) unitSaveFailed(w http.ResponseWriter, r *http.Request, err error, form templates.UnitFormData) {
	locale := i18n.LanguageFrom(r.Context())
	switch {
	case errors.Is(err, services.ErrCodeRequired):
		form.CodeError = i18n.T(locale, "units.error.code_required")
	case errors.Is(err, services.ErrCodeTooLong):
		form.CodeError = i18n.T(locale, "units.error.code_too_long")
	case errors.Is(err, services.ErrDuplicateCode):
		form.CodeError = i18n.T(locale, "units.error.code_duplicate")
	case errors.Is(err, permille.ErrInvalid):
		form.ShareErr = i18n.T(locale, "units.error.invalid_share")
	case errors.Is(err, services.ErrUnitNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "units.error.not_found"))
		return
	default:
		slog.Error("management: save unit", "error", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "units.error.save_failed"))
		return
	}
	h.renderUnitForm(w, r, http.StatusUnprocessableEntity, form)
}

func (h *Handler) renderUnitForm(w http.ResponseWriter, r *http.Request, status int, data templates.UnitFormData) {
	locale := i18n.LanguageFrom(r.Context())
	shell, err := httpx.ShellData(r, h.deps.Roles, unitsPath)
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "units.error.save_failed"))
		return
	}
	data.Shell = shell
	data.Locale = locale
	data.CSRF = httpx.CurrentSession(r).CSRFToken
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	httpx.Render(w, r, templates.UnitFormPage(data))
}
