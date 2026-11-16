package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	financetemplates "github.com/leoarkiteto/zelo/internal/features/finance/templates"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// settleDate is the settlement date of the request: ?date= (or the form's
// settlement_date) when valid, today otherwise.
func (h *Handler) settleDate(r *http.Request) time.Time {
	raw := r.FormValue("date")
	if raw == "" {
		raw = r.FormValue("settlement_date")
	}
	if on, err := domain.ParseSettlementDate(raw); err == nil {
		return on
	}
	return h.today()
}

// lateFeeFragment builds the late fee part of the settle form for a quota
// charge settled on the given date.
func (h *Handler) lateFeeFragment(r *http.Request, accountID string, on time.Time) financetemplates.SettleLateFeeData {
	locale := i18n.LanguageFrom(r.Context())
	d := financetemplates.SettleLateFeeData{Locale: locale, AccountID: accountID}
	q, err := h.deps.Quotas.LateFeePreview(r.Context(), httpx.CurrentSession(r).CondominiumID, accountID, on)
	if err != nil {
		log.Printf("finance settle: late fee preview of %s: %v", accountID, err)
		d.Error = i18n.T(locale, "quotas.late_fee.error")
		return d
	}
	if q.TotalCents > 0 {
		d.HasLateFee = true
		d.Fine = financetemplates.Money(r.Context(), locale, q.FineCents)
		d.Interest = financetemplates.Money(r.Context(), locale, q.InterestCents)
		d.Total = financetemplates.Money(r.Context(), locale, q.TotalCents)
		d.DaysLate = q.DaysLate
	}
	return d
}

// settleLateFeeGET returns the late fee fragment of the settle form for the
// chosen date (HX). Problems are reported inside the fragment with their status,
// because htmx v4 swaps 4xx bodies.
func (h *Handler) settleLateFeeGET(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Vary", "HX-Request")
	locale := i18n.LanguageFrom(r.Context())
	id := r.PathValue("id")
	fail := func(status int, key i18n.MessageKey) {
		w.WriteHeader(status)
		httpx.Render(w, r, financetemplates.SettleLateFee(financetemplates.SettleLateFeeData{
			Locale: locale, AccountID: id, Error: i18n.T(locale, key),
		}))
	}
	account, err := h.accountForCondominium(r, id)
	if err != nil {
		if errors.Is(err, services.ErrNotFound) {
			fail(http.StatusNotFound, "finance.error.account_gone")
			return
		}
		log.Printf("finance settle: load %s: %v", id, err)
		fail(http.StatusInternalServerError, "finance.error.load")
		return
	}
	if !account.CanSettle() {
		fail(http.StatusBadRequest, "finance.error.not_editable")
		return
	}
	httpx.Render(w, r, financetemplates.SettleLateFee(h.lateFeeFragment(r, id, h.settleDate(r))))
}

// settleLateFeeNoAccess is the htmx-friendly denied response of the fragment.
func settleLateFeeNoAccess(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	w.WriteHeader(http.StatusForbidden)
	httpx.Render(w, r, financetemplates.SettleLateFee(financetemplates.SettleLateFeeData{
		Locale: locale, Error: i18n.T(locale, "finance.overview.no_access_text"),
	}))
}

// debtorsGET lists the units with overdue receivables (FR-1, syndic only).
func (h *Handler) debtorsGET(w http.ResponseWriter, r *http.Request) {
	locale := i18n.LanguageFrom(r.Context())
	sess := httpx.CurrentSession(r)
	debtors, err := h.deps.Quotas.Debtors(r.Context(), sess.CondominiumID, h.today())
	if err != nil {
		log.Printf("finance debtors: %v", err)
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
		return
	}
	shell, err := httpx.ShellData(r, h.deps.Roles, "/finance")
	if err != nil {
		httpx.RenderError(w, r, http.StatusInternalServerError, i18n.T(locale, "finance.error.load"))
		return
	}
	data := financetemplates.DebtorsPageData{Shell: shell, Locale: locale}
	for _, d := range debtors {
		row := financetemplates.DebtorRowView{
			UnitCode:      d.UnitCode,
			StatementHref: "/finance/my-charges?unit=" + d.UnitID,
			OverdueCount:  d.OverdueCount,
			Total:         financetemplates.Money(r.Context(), locale, d.OverdueTotalCents),
			OldestDue:     i18n.FormatDate(locale, d.OldestDue),
			DaysLate:      d.DaysLate,
			Estimated:     financetemplates.Money(r.Context(), locale, d.EstimatedLateFeeCents),
		}
		for _, o := range d.Occupants {
			row.Occupants = append(row.Occupants, fmt.Sprintf("%s (%s)", o.Email, occupancyLabel(locale, o.Type)))
		}
		data.Rows = append(data.Rows, row)
	}
	httpx.Render(w, r, financetemplates.DebtorsPage(data))
}

func occupancyLabel(locale i18n.Language, t model.OccupancyType) string {
	switch t {
	case model.OccupancyOwner:
		return i18n.T(locale, "quotas.debtors.role.owner")
	case model.OccupancyTenant:
		return i18n.T(locale, "quotas.debtors.role.tenant")
	}
	return string(t)
}

// reserveView loads the reserve fund balance for the syndic screens (FR-17).
// A failure is logged and hides the card instead of failing the page.
func (h *Handler) reserveView(r *http.Request) *financetemplates.ReserveView {
	if h.deps.Quotas == nil {
		return nil
	}
	locale := i18n.LanguageFrom(r.Context())
	rb, err := h.deps.Quotas.ReserveBalance(r.Context(), httpx.CurrentSession(r).CondominiumID, h.today())
	if err != nil {
		log.Printf("finance reserve balance: %v", err)
		return nil
	}
	return &financetemplates.ReserveView{
		Realized:     financetemplates.Money(r.Context(), locale, rb.RealizedCents),
		Forecast:     financetemplates.Money(r.Context(), locale, rb.ForecastCents),
		RealizedYear: financetemplates.Money(r.Context(), locale, rb.RealizedYearCents),
	}
}
