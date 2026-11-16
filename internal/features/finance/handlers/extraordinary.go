package handlers

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	financetemplates "github.com/leoarkiteto/zelo/internal/features/finance/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
	atoms "github.com/leoarkiteto/zelo/internal/shared/templates/atoms"
)

const extraordinariesPath = "/finance/extraordinaries"

func extraFlash(r *http.Request, locale i18n.Language) string {
	switch f := r.URL.Query().Get("flash"); f {
	case "extra_created", "extra_canceled":
		return i18n.T(locale, i18n.MessageKey("quotas.flash."+f))
	}
	return ""
}

func extraStatusBadge(locale i18n.Language, s domain.IssuanceStatus) (string, atoms.BadgeVariant) {
	if s == domain.IssuanceCanceled {
		return i18n.T(locale, "quotas.extra.status.canceled"), atoms.BadgeVariantDanger
	}
	return i18n.T(locale, "quotas.extra.status.issued"), atoms.BadgeVariantSuccess
}

func divisionLabel(locale i18n.Language, d services.Division) string {
	return i18n.T(locale, i18n.MessageKey("quotas.extra.division."+string(d)))
}

// extraInput parses the extraordinary quota form. A malformed field yields the
// service error that the translated message belongs to.
func extraInput(r *http.Request) (services.ExtraordinaryInput, financetemplates.ExtraFormData, error) {
	raw := financetemplates.ExtraFormData{
		Description:  strings.TrimSpace(r.FormValue("description")),
		Total:        strings.TrimSpace(r.FormValue("total")),
		Division:     r.FormValue("division"),
		Installments: strings.TrimSpace(r.FormValue("installments")),
		FirstDue:     strings.TrimSpace(r.FormValue("first_due")),
	}
	in := services.ExtraordinaryInput{Description: raw.Description, Division: services.Division(raw.Division)}
	cents, err := parseLineAmount(raw.Total)
	if err != nil {
		return in, raw, err
	}
	in.TotalCents = cents
	n, err := strconv.Atoi(raw.Installments)
	if err != nil {
		return in, raw, domain.ErrInvalidInstallments
	}
	in.Installments = n
	due, err := time.Parse("2006-01-02", raw.FirstDue)
	if err != nil {
		return in, raw, domain.ErrInvalidExtraordinary
	}
	in.FirstDueDate = due
	return in, raw, nil
}

func extraRowView(r *http.Request, locale i18n.Language, s services.ExtraordinarySummary) financetemplates.ExtraRowView {
	st, v := extraStatusBadge(locale, s.Status)
	return financetemplates.ExtraRowView{
		ID: s.ID, Description: s.Description, Total: financetemplates.Money(r.Context(), locale, s.TotalCents),
		Installments: s.Installments, Status: st, StatusVariant: v,
		InstallmentProgress: strconv.Itoa(s.SettledInstallments) + " / " + strconv.Itoa(s.Installments),
		CanCancel:           s.Status.CanCancel(),
	}
}

// extraListGET lists the extraordinary quotas (FR-12).
func (h *Handler) extraListGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	list, err := h.deps.Quotas.ListExtraordinaries(r.Context(), httpx.CurrentSession(r).CondominiumID)
	if err != nil {
		msg, status := quotaError(locale, err)
		httpx.RenderError(w, r, status, msg)
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "quotas.error.load_failed"))
		return
	}
	rows := make([]financetemplates.ExtraRowView, 0, len(list))
	for _, s := range list {
		rows = append(rows, extraRowView(r, locale, s))
	}
	httpx.Render(w, r, financetemplates.ExtraListPage(financetemplates.ExtraListData{
		Shell: shell, Locale: locale, Rows: rows, Flash: extraFlash(r, locale),
	}))
}

func (h *Handler) renderExtraForm(w http.ResponseWriter, r *http.Request, status int, data financetemplates.ExtraFormData, errMsg string) {
	locale := i18n.LanguageFrom(r.Context())
	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "quotas.error.load_failed"))
		return
	}
	data.Shell, data.Locale, data.Error = shell, locale, errMsg
	data.CSRF = httpx.CurrentSession(r).CSRFToken
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	httpx.Render(w, r, financetemplates.ExtraFormPage(data))
}

// extraNewGET renders the empty create form.
func (h *Handler) extraNewGET(w http.ResponseWriter, r *http.Request) {
	h.renderExtraForm(w, r, http.StatusOK, financetemplates.ExtraFormData{
		Division: string(services.DivisionShare), Installments: "1",
	}, "")
}

// extraPreviewPOST renders the preview fragment swapped in by HTMX (FR-9).
func (h *Handler) extraPreviewPOST(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	fail := func(err error) {
		msg, status := quotaError(locale, err)
		w.WriteHeader(status)
		httpx.Render(w, r, financetemplates.ExtraPreview(financetemplates.ExtraPreviewData{Locale: locale, Error: msg}))
	}
	in, _, err := extraInput(r)
	if err != nil {
		fail(err)
		return
	}
	p, err := h.deps.Quotas.PreviewExtraordinary(r.Context(), httpx.CurrentSession(r).CondominiumID, in)
	if err != nil {
		fail(err)
		return
	}
	money := func(c int64) string { return financetemplates.Money(r.Context(), locale, c) }
	data := financetemplates.ExtraPreviewData{Locale: locale, Total: money(p.TotalCents)}
	for _, i := range p.Installments {
		data.Installments = append(data.Installments, financetemplates.ExtraPreviewInstallmentView{
			Number: i.Number, DueDate: i18n.FormatDate(locale, i.DueDate), Total: money(i.TotalCents),
		})
	}
	for _, u := range p.Units {
		row := financetemplates.ExtraPreviewUnitView{Code: u.Code, Share: permille.Format(u.SharePPM, locale), Total: money(u.TotalCents)}
		for _, c := range u.PerInstallmentCents {
			row.Cells = append(row.Cells, money(c))
		}
		data.Units = append(data.Units, row)
	}
	httpx.Render(w, r, financetemplates.ExtraPreview(data))
}

// extraCreatePOST creates the extraordinary quota and its installments.
func (h *Handler) extraCreatePOST(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	in, raw, err := extraInput(r)
	if err == nil {
		var id string
		id, err = h.deps.Quotas.CreateExtraordinary(r.Context(), httpx.CurrentUser(r).ID, httpx.CurrentSession(r).CondominiumID, in)
		if err == nil {
			http.Redirect(w, r, extraordinariesPath+"/"+url.PathEscape(id)+"/view?flash=extra_created", http.StatusSeeOther)
			return
		}
	}
	msg, status := quotaError(locale, err)
	if status == http.StatusInternalServerError || status == http.StatusNotFound {
		httpx.RenderError(w, r, status, msg)
		return
	}
	h.renderExtraForm(w, r, status, raw, msg)
}

// extraViewGET shows an extraordinary quota with its installments.
func (h *Handler) extraViewGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	d, err := h.deps.Quotas.GetExtraordinary(r.Context(), httpx.CurrentSession(r).CondominiumID, r.PathValue("id"))
	if err != nil {
		msg, status := quotaError(locale, err)
		httpx.RenderError(w, r, status, msg)
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "quotas.error.load_failed"))
		return
	}
	st, v := extraStatusBadge(locale, d.Status)
	data := financetemplates.ExtraViewData{
		Shell: shell, Locale: locale, ID: d.ID, Description: d.Description,
		Total:    financetemplates.Money(r.Context(), locale, d.TotalCents),
		Division: divisionLabel(locale, d.Division), FirstDue: i18n.FormatDate(locale, d.FirstDueDate),
		Status: st, StatusVariant: v,
		Progress:    strconv.Itoa(d.SettledInstallments) + " / " + strconv.Itoa(d.Installments),
		Receivables: strconv.Itoa(d.SettledCount) + " / " + strconv.Itoa(d.ChargeCount),
		CanCancel:   d.Status.CanCancel(), Flash: extraFlash(r, locale),
	}
	for _, i := range d.Issuances {
		ist, iv := issuanceStatusBadge(locale, i.Status)
		data.Installments = append(data.Installments, financetemplates.ExtraInstallmentView{
			Number: i.PeriodIndex, DueDate: i18n.FormatDate(locale, d.DueDateOf(i.PeriodIndex)),
			Total: financetemplates.Money(r.Context(), locale, i.TotalCents), Status: ist, StatusVariant: iv,
			Progress: progressText(i.SettledCount, i.ChargeCount),
		})
	}
	httpx.Render(w, r, financetemplates.ExtraViewPage(data))
}

func progressText(settled, count int) string {
	return strconv.Itoa(settled) + " / " + strconv.Itoa(count)
}

func (h *Handler) renderExtraCancel(w http.ResponseWriter, r *http.Request, id, errMsg string, status int) {
	locale := i18n.LanguageFrom(r.Context())
	d, err := h.deps.Quotas.GetExtraordinary(r.Context(), httpx.CurrentSession(r).CondominiumID, id)
	if err != nil {
		msg, st := quotaError(locale, err)
		httpx.RenderError(w, r, st, msg)
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "quotas.error.load_failed"))
		return
	}
	if !d.Status.CanCancel() && errMsg == "" {
		errMsg, status = i18n.T(locale, "quotas.error.issuance_not_active"), http.StatusConflict
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	httpx.Render(w, r, financetemplates.ExtraCancelPage(financetemplates.ExtraCancelData{
		Shell: shell, Locale: locale, CSRF: httpx.CurrentSession(r).CSRFToken,
		Row: extraRowView(r, locale, d.ExtraordinarySummary), Receiv: progressText(d.SettledCount, d.ChargeCount), Error: errMsg,
	}))
}

// extraCancelGET asks for confirmation to cancel an extraordinary quota.
func (h *Handler) extraCancelGET(w http.ResponseWriter, r *http.Request) {
	h.renderExtraCancel(w, r, r.PathValue("id"), "", http.StatusOK)
}

// extraCancelPOST cancels a whole extraordinary quota (FR-12).
func (h *Handler) extraCancelPOST(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	id := r.PathValue("id")
	err := h.deps.Quotas.CancelExtraordinary(r.Context(), httpx.CurrentUser(r).ID, httpx.CurrentSession(r).CondominiumID, id)
	if err != nil {
		msg, status := quotaError(locale, err)
		if status == http.StatusNotFound || status == http.StatusInternalServerError {
			httpx.RenderError(w, r, status, msg)
			return
		}
		h.renderExtraCancel(w, r, id, msg, status)
		return
	}
	http.Redirect(w, r, extraordinariesPath+"?flash=extra_canceled", http.StatusSeeOther)
}
