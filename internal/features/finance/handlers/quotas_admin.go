package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	financetemplates "github.com/leoarkiteto/zelo/internal/features/finance/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/percent"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
	atoms "github.com/leoarkiteto/zelo/internal/shared/templates/atoms"
)

const budgetsPath = "/finance/budgets"

// errInvalidRate is a malformed reserve rate typed by the syndic.
var errInvalidRate = errors.New("invalid reserve rate")

// quotaError maps a service error to a translated message and an HTTP status.
func quotaError(locale i18n.Language, err error) (string, int) {
	var se services.SharesError
	switch {
	case errors.As(err, &se):
		diff := se.DiffPPM()
		sign := "+"
		if diff < 0 {
			sign, diff = "-", -diff
		}
		msg := i18n.T(locale, "quotas.error.shares_not_100") + " " + sign + permille.Format(diff, locale) + i18n.T(locale, "units.share_unit")
		return msg, http.StatusUnprocessableEntity
	case errors.Is(err, services.ErrNotFound):
		return i18n.T(locale, "quotas.error.not_found"), http.StatusNotFound
	case errors.Is(err, domain.ErrBudgetNotApproved):
		return i18n.T(locale, "quotas.error.not_approved"), http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrPeriodOutOfRange):
		return i18n.T(locale, "quotas.error.period_out_of_range"), http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrNoChargeableUnits):
		return i18n.T(locale, "quotas.error.no_chargeable_units"), http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrBudgetLocked):
		return i18n.T(locale, "quotas.error.budget_locked"), http.StatusConflict
	case errors.Is(err, domain.ErrBudgetHasIssuances):
		return i18n.T(locale, "quotas.error.has_issuances"), http.StatusConflict
	case errors.Is(err, domain.ErrIssuanceHasSettled):
		return i18n.T(locale, "quotas.error.has_settled"), http.StatusConflict
	case errors.Is(err, domain.ErrExtraordinaryHasSettled):
		return i18n.T(locale, "quotas.error.extra_has_settled"), http.StatusConflict
	case errors.Is(err, domain.ErrInvalidExtraordinary):
		return i18n.T(locale, "quotas.error.invalid_extraordinary"), http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrInvalidInstallments):
		return i18n.T(locale, "quotas.error.invalid_installments"), http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrBudgetEmpty):
		return i18n.T(locale, "quotas.error.budget_empty"), http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrBudgetExists):
		return i18n.T(locale, "quotas.error.budget_exists"), http.StatusConflict
	case errors.Is(err, permille.ErrInvalid):
		return i18n.T(locale, "quotas.error.invalid_permille"), http.StatusUnprocessableEntity
	case errors.Is(err, services.ErrIssuanceNotActive):
		return i18n.T(locale, "quotas.error.issuance_not_active"), http.StatusConflict
	case errors.Is(err, errInvalidRate):
		return i18n.T(locale, "quotas.error.invalid_rate"), http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrInvalidAmount):
		return i18n.T(locale, "quotas.error.invalid_amount"), http.StatusUnprocessableEntity
	case errors.Is(err, services.ErrInvalidInput):
		return i18n.T(locale, "quotas.error.invalid_input"), http.StatusUnprocessableEntity
	}
	slog.Error("finance: quotas", "error", err)
	return i18n.T(locale, "quotas.error.save_failed"), http.StatusInternalServerError
}

func quotaFlash(r *http.Request, locale i18n.Language) string {
	switch f := r.URL.Query().Get("flash"); f {
	case "budget_created", "budget_updated", "line_added", "line_updated", "line_removed",
		"approved", "reopened", "issued", "already_issued", "issuance_canceled":
		return i18n.T(locale, i18n.MessageKey("quotas.flash."+f))
	}
	return ""
}

func budgetRedirect(w http.ResponseWriter, r *http.Request, budgetID, flash string) {
	target := budgetsPath
	if budgetID != "" {
		target += "/" + url.PathEscape(budgetID) + "/view"
	}
	http.Redirect(w, r, target+"?flash="+flash, http.StatusSeeOther)
}

// parseLineAmount parses a decimal amount, tolerating a decimal comma.
func parseLineAmount(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, ".") {
		s = strings.Replace(s, ",", ".", 1)
	}
	return domain.ParseAmountCents(s)
}

func budgetStatusBadge(locale i18n.Language, s domain.BudgetStatus) (string, atoms.BadgeVariant) {
	if s == domain.BudgetApproved {
		return i18n.T(locale, "quotas.status.approved"), atoms.BadgeVariantSuccess
	}
	return i18n.T(locale, "quotas.status.draft"), atoms.BadgeVariantWarning
}

func issuanceStatusBadge(locale i18n.Language, s domain.IssuanceStatus) (string, atoms.BadgeVariant) {
	if s == domain.IssuanceCanceled {
		return i18n.T(locale, "quotas.issuance_status.canceled"), atoms.BadgeVariantDanger
	}
	return i18n.T(locale, "quotas.issuance_status.issued"), atoms.BadgeVariantSuccess
}

// issuanceLabel is the localised period of an issuance (the machine label is
// kept for URLs only).
func issuanceLabel(locale i18n.Language, i services.IssuanceSummary) string {
	return financetemplates.PeriodText(locale, i.Year, i.Periodicity, i.PeriodIndex)
}

// previewLabel is the localised period of a preview, falling back to the
// machine label when the preview has no periodicity (refused before computing).
func previewLabel(locale i18n.Language, p services.IssuancePreview) string {
	if p.Periodicity == "" {
		return p.Label
	}
	return financetemplates.PeriodText(locale, p.Year, p.Periodicity, p.PeriodIndex)
}

func periodicityLabel(locale i18n.Language, p domain.Periodicity) string {
	return i18n.T(locale, i18n.MessageKey("quotas.periodicity."+string(p)))
}

// registerQuotaAdminRoutes registers the syndic-only budget and issuance routes.
// Budget detail lives at /finance/budgets/{id}/view: a 3-segment
// /finance/budgets/{id} would collide with /finance/{id}/edit.
func registerQuotaAdminRoutes(mux *http.ServeMux, h *Handler, syndicOnly func(http.Handler) http.Handler) {
	route := func(pattern string, fn http.HandlerFunc) {
		mux.Handle(pattern, middleware.RequireAuth(syndicOnly(fn)))
	}
	route("GET /finance/budgets", h.budgetsGET)
	route("GET /finance/budgets/new", h.budgetNewGET)
	route("POST /finance/budgets", h.budgetCreatePOST)
	route("GET /finance/budgets/{id}/view", h.budgetViewGET)
	route("POST /finance/budgets/{id}/edit", h.budgetEditPOST)
	route("POST /finance/budgets/{id}/lines", h.lineAddPOST)
	route("POST /finance/budgets/{id}/lines/{lineID}/edit", h.lineEditPOST)
	route("POST /finance/budgets/{id}/lines/{lineID}/delete", h.lineDeletePOST)
	route("POST /finance/budgets/{id}/approve", h.budgetApprovePOST)
	route("POST /finance/budgets/{id}/reopen", h.budgetReopenPOST)
	route("GET /finance/budgets/{id}/issuances/{period}/preview", h.issuancePreviewGET)
	route("POST /finance/budgets/{id}/issuances/{period}", h.issuancePOST)
	route("GET /finance/issuances/{id}/cancel", h.issuanceCancelGET)
	route("POST /finance/issuances/{id}/cancel", h.issuanceCancelPOST)

	// Extraordinary quotas: every path has a literal second segment, so none
	// collides with /finance/{id}/….
	route("GET /finance/debtors", h.debtorsGET)

	route("GET /finance/extraordinaries", h.extraListGET)
	route("GET /finance/extraordinaries/new", h.extraNewGET)
	route("POST /finance/extraordinaries/preview", h.extraPreviewPOST)
	route("POST /finance/extraordinaries", h.extraCreatePOST)
	route("GET /finance/extraordinaries/{id}/view", h.extraViewGET)
	route("GET /finance/extraordinaries/{id}/cancel", h.extraCancelGET)
	route("POST /finance/extraordinaries/{id}/cancel", h.extraCancelPOST)
}

// budgetsGET lists the condominium's budgets.
func (h *Handler) budgetsGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	list, err := h.deps.Quotas.ListBudgets(r.Context(), sess.CondominiumID)
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
	rows := make([]financetemplates.BudgetRowView, 0, len(list))
	for _, b := range list {
		label, variant := budgetStatusBadge(locale, b.Status)
		rows = append(rows, financetemplates.BudgetRowView{
			ID: b.ID, Year: b.Year, Periodicity: periodicityLabel(locale, b.Periodicity),
			Status: label, StatusVariant: variant, LineCount: b.LineCount,
			Total: financetemplates.Money(r.Context(), locale, b.TotalCents),
		})
	}
	httpx.Render(w, r, financetemplates.BudgetsPage(financetemplates.BudgetsPageData{
		Shell: shell, Locale: locale, Budgets: rows, Flash: quotaFlash(r, locale),
	}))
}

// budgetNewGET renders the create budget form with the currency defaults.
func (h *Handler) budgetNewGET(w http.ResponseWriter, r *http.Request) {
	rate := services.DefaultReserveBP(money.From(r.Context()))
	h.renderBudgetForm(w, r, http.StatusOK, financetemplates.BudgetFormData{
		Year:        strconv.Itoa(h.now().Year()),
		Periodicity: string(domain.PeriodicityMonthly),
		DueDay:      "5",
		ReserveRate: percent.FormatBP(rate, i18n.LanguageFrom(r.Context())),
	}, "")
}

func (h *Handler) renderBudgetForm(w http.ResponseWriter, r *http.Request, status int, data financetemplates.BudgetFormData, errMsg string) {
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
	httpx.Render(w, r, financetemplates.BudgetFormPage(data))
}

// budgetCreatePOST creates a draft budget.
func (h *Handler) budgetCreatePOST(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	u := httpx.CurrentUser(r)
	sess := httpx.CurrentSession(r)
	form := financetemplates.BudgetFormData{
		Year: strings.TrimSpace(r.FormValue("year")), Periodicity: r.FormValue("periodicity"),
		DueDay: strings.TrimSpace(r.FormValue("due_day")), ReserveRate: strings.TrimSpace(r.FormValue("reserve_rate")),
	}
	fail := func(err error) {
		msg, status := quotaError(locale, err)
		if status == http.StatusInternalServerError || status == http.StatusNotFound {
			httpx.RenderError(w, r, status, msg)
			return
		}
		h.renderBudgetForm(w, r, status, form, msg)
	}
	year, errY := strconv.Atoi(form.Year)
	dueDay, errD := strconv.Atoi(form.DueDay)
	if errY != nil || errD != nil {
		fail(services.ErrInvalidInput)
		return
	}
	in := services.BudgetInput{Year: year, Periodicity: domain.Periodicity(form.Periodicity), DueDay: dueDay}
	if form.ReserveRate != "" {
		bp, err := percent.ParseBP(form.ReserveRate)
		if err != nil {
			fail(errInvalidRate)
			return
		}
		in.ReserveRateBP = &bp
	}
	b, err := h.deps.Quotas.CreateBudget(r.Context(), u.ID, sess.CondominiumID, in)
	if err != nil {
		fail(err)
		return
	}
	budgetRedirect(w, r, b.ID, "budget_created")
}

// budgetViewGET renders a budget with its lines, per-unit preview and issuances.
func (h *Handler) budgetViewGET(w http.ResponseWriter, r *http.Request) {
	h.renderBudget(w, r, r.PathValue("id"), http.StatusOK, "", financetemplates.LineView{})
}

func (h *Handler) renderBudget(w http.ResponseWriter, r *http.Request, id string, status int, errMsg string, newLine financetemplates.LineView) {
	locale := i18n.LanguageFrom(r.Context())
	ctx := r.Context()
	sess := httpx.CurrentSession(r)
	detail, err := h.deps.Quotas.GetBudget(ctx, sess.CondominiumID, id)
	if err != nil {
		msg, st := quotaError(locale, err)
		httpx.RenderError(w, r, st, msg)
		return
	}
	issuances, err := h.deps.Quotas.ListIssuances(ctx, sess.CondominiumID, id)
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
	money := func(c int64) string { return financetemplates.Money(ctx, locale, c) }
	label, variant := budgetStatusBadge(locale, detail.Status)
	data := financetemplates.BudgetPageData{
		Shell: shell, Locale: locale, CSRF: sess.CSRFToken, ID: detail.ID, Year: detail.Year,
		Status: label, StatusVariant: variant,
		IsDraft: detail.Status.CanEdit(), IsApproved: detail.Status == domain.BudgetApproved,
		CanApprove:  detail.Status.CanApprove() && len(detail.Lines) > 0,
		Periodicity: string(detail.Periodicity), PeriodicityLabel: periodicityLabel(locale, detail.Periodicity),
		DueDay: strconv.Itoa(detail.DueDay), ReserveRate: percent.FormatBP(detail.ReserveRateBP, locale),
		Total:   money(detail.TotalCents),
		NewLine: newLine, Error: errMsg, Flash: quotaFlash(r, locale),
		Categories: financetemplates.CategoryOptions(locale, domain.PayableCategories()),
		Reserve:    h.reserveView(r),
	}
	data.PeriodicityOptions = []atoms.SelectOption{
		{Value: string(domain.PeriodicityMonthly), Label: periodicityLabel(locale, domain.PeriodicityMonthly)},
		{Value: string(domain.PeriodicityQuarterly), Label: periodicityLabel(locale, domain.PeriodicityQuarterly)},
	}
	if data.NewLine.Category == "" {
		data.NewLine.Category = string(domain.CategoryMaintenance)
	}
	for _, l := range detail.Lines {
		data.Lines = append(data.Lines, financetemplates.LineView{
			ID: l.ID, Description: l.Description, Category: string(l.Category),
			CategoryLabel: financetemplates.CategoryLabel(locale, domain.AccountTypePayable, l.Category),
			Amount:        centsToDecimal(l.AmountCents), AmountDisplay: money(l.AmountCents),
		})
	}
	if diff := detail.SumPPM - permille.Total; diff != 0 {
		sign := "+"
		if diff < 0 {
			sign, diff = "-", -diff
		}
		data.SharesDiff = sign + permille.Format(diff, locale)
	}
	for _, p := range detail.Periods {
		data.PeriodHeaders = append(data.PeriodHeaders, financetemplates.PeriodText(locale, detail.Year, detail.Periodicity, p.Index))
	}
	for _, u := range detail.Units {
		row := financetemplates.UnitPreviewRow{Code: u.Code, Share: permille.Format(u.SharePPM, locale)}
		for _, c := range u.PerPeriodCents {
			row.Cells = append(row.Cells, money(c))
		}
		data.UnitRows = append(data.UnitRows, row)
	}
	active := map[int]services.IssuanceSummary{}
	for _, i := range issuances {
		st, v := issuanceStatusBadge(locale, i.Status)
		data.Issuances = append(data.Issuances, financetemplates.IssuanceView{
			ID: i.ID, Label: issuanceLabel(locale, i), Total: money(i.TotalCents), Status: st, StatusVariant: v,
			SettledCount: i.SettledCount, ChargeCount: i.ChargeCount, CanCancel: i.Status.CanCancel(),
		})
		if i.Status == domain.IssuanceIssued {
			active[i.PeriodIndex] = i
			data.HasActiveIssuance = true
		}
	}
	for _, p := range detail.Periods {
		pv := financetemplates.PeriodView{
			Index: p.Index, Label: financetemplates.PeriodText(locale, detail.Year, detail.Periodicity, p.Index), DueDate: i18n.FormatDate(locale, p.DueDate), Total: money(p.TotalCents),
		}
		if i, ok := active[p.Index]; ok {
			pv.Issued, pv.IssuanceID, pv.SettledCount, pv.ChargeCount = true, i.ID, i.SettledCount, i.ChargeCount
		}
		data.Periods = append(data.Periods, pv)
	}
	if status != http.StatusOK {
		w.WriteHeader(status)
	}
	httpx.Render(w, r, financetemplates.BudgetPage(data))
}

// mutateBudget runs a budget change and redirects with a flash, or re-renders
// the budget with the translated error.
func (h *Handler) mutateBudget(w http.ResponseWriter, r *http.Request, flash string, newLine financetemplates.LineView, do func(actorID, condoID, id string) error) {
	locale := i18n.LanguageFrom(r.Context())
	id := r.PathValue("id")
	err := do(httpx.CurrentUser(r).ID, httpx.CurrentSession(r).CondominiumID, id)
	if err != nil {
		msg, status := quotaError(locale, err)
		if status == http.StatusNotFound {
			httpx.RenderError(w, r, status, msg)
			return
		}
		h.renderBudget(w, r, id, status, msg, newLine)
		return
	}
	budgetRedirect(w, r, id, flash)
}

// budgetEditPOST updates the budget parameters.
func (h *Handler) budgetEditPOST(w http.ResponseWriter, r *http.Request) {
	h.mutateBudget(w, r, "budget_updated", financetemplates.LineView{}, func(actor, condo, id string) error {
		dueDay, err := strconv.Atoi(strings.TrimSpace(r.FormValue("due_day")))
		if err != nil {
			return services.ErrInvalidInput
		}
		bp, err := percent.ParseBP(r.FormValue("reserve_rate"))
		if err != nil {
			return errInvalidRate
		}
		return h.deps.Quotas.UpdateBudget(r.Context(), actor, condo, id, services.BudgetParams{
			Periodicity: domain.Periodicity(r.FormValue("periodicity")), DueDay: dueDay, ReserveRateBP: bp,
		})
	})
}

func lineFromForm(r *http.Request) (services.LineInput, financetemplates.LineView, error) {
	raw := financetemplates.LineView{
		Description: r.FormValue("description"), Category: r.FormValue("category"), Amount: r.FormValue("amount"),
	}
	cents, err := parseLineAmount(raw.Amount)
	in := services.LineInput{Description: raw.Description, Category: domain.Category(raw.Category), AmountCents: cents}
	return in, raw, err
}

// lineAddPOST adds a line to a draft budget.
func (h *Handler) lineAddPOST(w http.ResponseWriter, r *http.Request) {
	in, raw, perr := lineFromForm(r)
	h.mutateBudget(w, r, "line_added", raw, func(actor, condo, id string) error {
		if perr != nil {
			return perr
		}
		_, err := h.deps.Quotas.AddLine(r.Context(), actor, condo, id, in)
		return err
	})
}

// lineEditPOST changes a line of a draft budget.
func (h *Handler) lineEditPOST(w http.ResponseWriter, r *http.Request) {
	in, _, perr := lineFromForm(r)
	h.mutateBudget(w, r, "line_updated", financetemplates.LineView{}, func(actor, condo, id string) error {
		if perr != nil {
			return perr
		}
		return h.deps.Quotas.UpdateLine(r.Context(), actor, condo, id, r.PathValue("lineID"), in)
	})
}

// lineDeletePOST removes a line of a draft budget.
func (h *Handler) lineDeletePOST(w http.ResponseWriter, r *http.Request) {
	h.mutateBudget(w, r, "line_removed", financetemplates.LineView{}, func(actor, condo, id string) error {
		return h.deps.Quotas.RemoveLine(r.Context(), actor, condo, id, r.PathValue("lineID"))
	})
}

// budgetApprovePOST approves a draft budget.
func (h *Handler) budgetApprovePOST(w http.ResponseWriter, r *http.Request) {
	h.mutateBudget(w, r, "approved", financetemplates.LineView{}, func(actor, condo, id string) error {
		return h.deps.Quotas.ApproveBudget(r.Context(), actor, condo, id)
	})
}

// budgetReopenPOST reopens an approved budget without active issuances.
func (h *Handler) budgetReopenPOST(w http.ResponseWriter, r *http.Request) {
	h.mutateBudget(w, r, "reopened", financetemplates.LineView{}, func(actor, condo, id string) error {
		return h.deps.Quotas.ReopenBudget(r.Context(), actor, condo, id)
	})
}

func periodParam(r *http.Request) (int, bool) {
	p, err := strconv.Atoi(r.PathValue("period"))
	return p, err == nil && p > 0
}

// issuancePreviewGET shows the preview of issuing one period (FR-9).
func (h *Handler) issuancePreviewGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	period, ok := periodParam(r)
	if !ok {
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "quotas.error.not_found"))
		return
	}
	budgetID := r.PathValue("id")
	p, err := h.deps.Quotas.PreviewIssuance(r.Context(), httpx.CurrentSession(r).CondominiumID, budgetID, period)
	if err != nil {
		h.renderPreviewError(w, r, budgetID, period, err)
		return
	}
	if p.Existing != nil {
		budgetRedirect(w, r, budgetID, "already_issued")
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "quotas.error.load_failed"))
		return
	}
	money := func(c int64) string { return financetemplates.Money(r.Context(), locale, c) }
	data := financetemplates.IssuancePreviewData{
		Shell: shell, Locale: locale, CSRF: httpx.CurrentSession(r).CSRFToken,
		BudgetID: budgetID, Period: period, Label: previewLabel(locale, p), DueDate: i18n.FormatDate(locale, p.DueDate),
		Total: money(p.TotalCents), Base: money(p.BaseCents), Reserve: money(p.ReserveCents),
	}
	for _, row := range p.Rows {
		data.Rows = append(data.Rows, financetemplates.IssuancePreviewRowView{
			Code: row.Code, Share: permille.Format(row.SharePPM, locale),
			Base: money(row.BaseCents), Reserve: money(row.ReserveCents), Quota: money(row.QuotaCents),
			DueDate: i18n.FormatDate(locale, row.DueDate),
		})
	}
	httpx.Render(w, r, financetemplates.IssuancePreviewPage(data))
}

func (h *Handler) renderPreviewError(w http.ResponseWriter, r *http.Request, budgetID string, period int, err error) {
	locale := i18n.LanguageFrom(r.Context())
	msg, status := quotaError(locale, err)
	if status == http.StatusNotFound || status == http.StatusInternalServerError {
		httpx.RenderError(w, r, status, msg)
		return
	}
	shell, serr := httpx.ShellData(r, h.deps.Roles, "/finance")
	if serr != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "quotas.error.load_failed"))
		return
	}
	w.WriteHeader(status)
	httpx.Render(w, r, financetemplates.IssuancePreviewPage(financetemplates.IssuancePreviewData{
		Shell: shell, Locale: locale, BudgetID: budgetID, Period: period, Label: strconv.Itoa(period), Error: msg,
	}))
}

// issuancePOST issues a period. An already issued period redirects to the
// existing issuance with an informative flash (FR-11).
func (h *Handler) issuancePOST(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	period, ok := periodParam(r)
	if !ok {
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "quotas.error.not_found"))
		return
	}
	budgetID := r.PathValue("id")
	_, created, err := h.deps.Quotas.Issue(r.Context(), httpx.CurrentUser(r).ID, httpx.CurrentSession(r).CondominiumID, budgetID, period)
	if err != nil {
		h.renderPreviewError(w, r, budgetID, period, err)
		return
	}
	if !created {
		budgetRedirect(w, r, budgetID, "already_issued")
		return
	}
	budgetRedirect(w, r, budgetID, "issued")
}

func (h *Handler) renderCancel(w http.ResponseWriter, r *http.Request, budgetID, issuanceID, errMsg string, status int) {
	locale := i18n.LanguageFrom(r.Context())
	if budgetID == "" {
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "quotas.error.not_found"))
		return
	}
	list, err := h.deps.Quotas.ListIssuances(r.Context(), httpx.CurrentSession(r).CondominiumID, budgetID)
	if err != nil {
		msg, st := quotaError(locale, err)
		httpx.RenderError(w, r, st, msg)
		return
	}
	for _, i := range list {
		if i.ID != issuanceID {
			continue
		}
		shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
		if err != nil {
			httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "quotas.error.load_failed"))
			return
		}
		st, v := issuanceStatusBadge(locale, i.Status)
		if !i.Status.CanCancel() && errMsg == "" {
			errMsg, status = i18n.T(locale, "quotas.error.issuance_not_active"), http.StatusConflict
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
		}
		httpx.Render(w, r, financetemplates.IssuanceCancelPage(financetemplates.IssuanceCancelData{
			Shell: shell, Locale: locale, CSRF: httpx.CurrentSession(r).CSRFToken, BudgetID: budgetID, Error: errMsg,
			Issuance: financetemplates.IssuanceView{
				ID: i.ID, Label: issuanceLabel(locale, i), Total: financetemplates.Money(r.Context(), locale, i.TotalCents),
				Status: st, StatusVariant: v, SettledCount: i.SettledCount, ChargeCount: i.ChargeCount, CanCancel: i.Status.CanCancel(),
			},
		}))
		return
	}
	httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "quotas.error.not_found"))
}

// issuanceCancelGET asks for confirmation to cancel an issuance. The budget id
// comes in the query string because issuances are listed per budget.
func (h *Handler) issuanceCancelGET(w http.ResponseWriter, r *http.Request) {
	h.renderCancel(w, r, r.URL.Query().Get("budget"), r.PathValue("id"), "", http.StatusOK)
}

// issuanceCancelPOST cancels a whole issuance (FR-15).
func (h *Handler) issuanceCancelPOST(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	budgetID := r.FormValue("budget_id")
	id := r.PathValue("id")
	err := h.deps.Quotas.CancelIssuance(r.Context(), httpx.CurrentUser(r).ID, httpx.CurrentSession(r).CondominiumID, id)
	if err != nil {
		msg, status := quotaError(locale, err)
		if status == http.StatusNotFound || status == http.StatusInternalServerError {
			httpx.RenderError(w, r, status, msg)
			return
		}
		h.renderCancel(w, r, budgetID, id, msg, status)
		return
	}
	budgetRedirect(w, r, budgetID, "issuance_canceled")
}
