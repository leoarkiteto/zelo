package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	financetemplates "github.com/leoarkiteto/zelo/internal/features/finance/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// viewer builds the statement/receipt/card viewer from the request: the syndic
// role (set by the role guard) sees everything.
func viewer(r *http.Request) services.Viewer {
	v := services.Viewer{}
	if u := httpx.CurrentUser(r); u != nil {
		v.UserID = u.ID
	}
	for _, role := range middleware.RolesFrom(r.Context()) {
		if role == model.RoleSyndic {
			v.IsSyndic = true
		}
	}
	return v
}

// renderQuotaError maps the statement/receipt service errors to responses.
// ErrForbidden was already audited by the service.
func renderQuotaError(w http.ResponseWriter, r *http.Request, err error) {
	locale := i18n.LanguageFrom(r.Context())
	switch {
	case errors.Is(err, services.ErrForbidden):
		httpx.RenderError(w, r, http.StatusForbidden, i18n.T(locale, "finance.overview.no_access_text"))
	case errors.Is(err, services.ErrNotFound):
		httpx.RenderError(w, r, http.StatusNotFound, i18n.T(locale, "quotas.error.not_found"))
	default:
		log.Printf("finance quotas: %v", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
	}
}

func (h *Handler) statementRows(ctx context.Context, locale i18n.Language, rows []services.StatementRow, settled bool) []financetemplates.StatementRowView {
	now := time.Now()
	out := make([]financetemplates.StatementRowView, 0, len(rows))
	for _, row := range rows {
		a := row.Account
		eff := a.EffectiveStatus(now)
		v := financetemplates.StatementRowView{
			ID:          a.ID,
			Title:       a.Title,
			Amount:      financetemplates.Money(ctx, locale, a.AmountCents),
			DueDate:     i18n.FormatDate(locale, a.DueDate),
			StatusLabel: financetemplates.StatusLabel(locale, eff),
			Overdue:     eff == domain.StatusOverdue,
			CanReceipt:  settled && a.SettlementDate != nil,
		}
		if row.IsQuota {
			v.Title = financetemplates.QuotaTitle(locale, row.Period)
			v.PeriodLabel = financetemplates.QuotaPeriodText(locale, row.Period)
		}
		if a.SettlementDate != nil {
			v.SettlementDate = i18n.FormatDate(locale, *a.SettlementDate)
		}
		out = append(out, v)
	}
	return out
}

// myChargesGET renders the per-unit Conta corrente (FR-18..20). ?unit= picks
// the unit; without it the first unit by code is shown.
func (h *Handler) myChargesGET(w http.ResponseWriter, r *http.Request) {
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	if h.deps.Quotas == nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
		return
	}
	shell, err := httpx.ShellDataWithRoles(r, "/finance/my-charges", middleware.RolesFrom(r.Context()))
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load_session"))
		return
	}
	st, err := h.deps.Quotas.Statement(r.Context(), viewer(r), sess.CondominiumID, r.URL.Query().Get("unit"))
	data := financetemplates.StatementPageData{Shell: shell, Locale: locale}
	switch {
	case errors.Is(err, services.ErrNoUnit):
		data.NoUnit = true
	case err != nil:
		renderQuotaError(w, r, err)
		return
	default:
		ctx := r.Context()
		data.UnitID, data.UnitCode = st.Unit.ID, st.Unit.Code
		data.Units = financetemplates.UnitOptions(st.Units)
		data.Pending = h.statementRows(ctx, locale, st.Pending, false)
		data.Settled = h.statementRows(ctx, locale, st.Settled, true)
		data.Canceled = h.statementRows(ctx, locale, st.Canceled, false)
		data.PendingTotal = financetemplates.Money(ctx, locale, st.PendingTotalCents)
		data.OverdueTotal = financetemplates.Money(ctx, locale, st.OverdueTotalCents)
		data.OverdueCount = st.OverdueCount
	}
	httpx.Render(w, r, financetemplates.StatementPage(data))
}

// reciboGET renders the printable Recibo of a settled receivable (FR-21, FR-26).
func (h *Handler) reciboGET(w http.ResponseWriter, r *http.Request) {
	sess := httpx.CurrentSession(r)
	locale := i18n.LanguageFrom(r.Context())
	if h.deps.Quotas == nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
		return
	}
	rec, err := h.deps.Quotas.Receipt(r.Context(), viewer(r), sess.CondominiumID, r.PathValue("id"))
	if err != nil {
		renderQuotaError(w, r, err)
		return
	}
	shell, err := httpx.ShellDataWithRoles(r, "/finance/my-charges", middleware.RolesFrom(r.Context()))
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load_session"))
		return
	}
	cur := rec.Currency
	if !cur.Valid() {
		cur = money.From(r.Context())
	}
	fmtMoney := func(cents int64) string { return money.Format(locale, cur, cents) }
	data := financetemplates.ReceiptPageData{
		Shell:       shell,
		Locale:      locale,
		Number:      rec.Number,
		Condominium: rec.CondominiumName,
		UnitCode:    rec.UnitCode,
		Title:       rec.Title,
		Amount:      fmtMoney(rec.AmountCents),
		BackHref:    "/finance/my-charges",
	}
	if rec.UnitID != "" {
		data.BackHref = "/finance/my-charges?unit=" + rec.UnitID
	}
	if rec.SettlementDate != nil {
		data.SettledOn = i18n.FormatDate(locale, *rec.SettlementDate)
	}
	if rec.IsQuota {
		data.PeriodLabel = financetemplates.QuotaPeriodText(locale, rec.Period)
		data.Title = financetemplates.QuotaTitle(locale, rec.Period)
		data.Base = fmtMoney(rec.BaseCents)
		data.Reserve = fmtMoney(rec.ReserveCents)
	}
	httpx.Render(w, r, financetemplates.ReceiptPage(data))
}

// quotasCardNoAccess is the htmx-friendly denied response for the card.
func quotasCardNoAccess(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	httpx.Render(w, r, financetemplates.QuotasCardNoAccess(i18n.LanguageFrom(r.Context())))
}

// quotasCardGET renders the dashboard quotas card fragment (FR-22). Non-HX
// requests have no page of their own and go to the dashboard.
func (h *Handler) quotasCardGET(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Vary", "HX-Request")
	locale := i18n.LanguageFrom(r.Context())
	if !middleware.IsHX(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	fail := func(err error) {
		log.Printf("finance quotas card: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		httpx.Render(w, r, financetemplates.QuotasCardError(locale))
	}
	if h.deps.Quotas == nil {
		fail(errors.New("quotas not configured"))
		return
	}
	sess := httpx.CurrentSession(r)
	card, err := h.deps.Quotas.Card(r.Context(), viewer(r), sess.CondominiumID, time.Now())
	if err != nil {
		fail(err)
		return
	}
	data := financetemplates.QuotasCardData{Locale: locale}
	if rc := card.Resident; rc != nil {
		v := &financetemplates.ResidentCardView{
			OverdueCount: rc.OverdueCount,
			OverdueTotal: financetemplates.Money(r.Context(), locale, rc.OverdueTotalCents),
		}
		if rc.NextPending != nil {
			v.HasNext = true
			v.NextAmount = financetemplates.Money(r.Context(), locale, rc.NextPending.AmountCents)
			v.NextDue = i18n.FormatDate(locale, rc.NextPending.DueDate)
		}
		data.Resident = v
	}
	if sc := card.Syndic; sc != nil {
		v := &financetemplates.SyndicCardView{HasBudget: sc.HasBudget, Issued: sc.Issued}
		if sc.HasBudget {
			v.PeriodLabel = financetemplates.QuotaPeriodText(locale, sc.Period)
		}
		if sc.HasBudget {
			v.PreviewHref = "/finance/budgets/" + sc.BudgetID + "/issuances/" + strconv.Itoa(sc.PeriodIndex) + "/preview"
		}
		if sc.Issued {
			v.Progress = financetemplates.SettledProgress(locale, sc.SettledCount, sc.ChargeCount)
		}
		data.Syndic = v
	}
	httpx.Render(w, r, financetemplates.QuotasCard(data))
}
