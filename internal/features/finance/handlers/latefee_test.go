package handlers

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

func pendingReceivable() *fakeAccountReader {
	return &fakeAccountReader{account: domain.FinancialAccount{
		ID: "a1", CondominiumID: "c1", Status: domain.StatusPending, Type: domain.AccountTypeReceivable,
		Title: "Stored title", AmountCents: 35000, DueDate: time.Now().AddDate(0, 0, -20),
	}}
}

func sampleLateFee() services.LateFeeQuote {
	return services.LateFeeQuote{
		LateFee: domain.LateFee{DaysLate: 20, FineCents: 700, InterestCents: 233, TotalCents: 933},
		FeeBP:   200, InterestBP: 100, GraceDays: 0,
	}
}

func settleRouter(role model.Role, fin *fakeFinance, q *fakeQuotas) http.Handler {
	user, sess := syndicSession()
	deps := baseDeps(fin)
	deps.Roles = &fakeRoles{roles: []model.Role{role}}
	deps.Quotas = q
	deps.Accounts = pendingReceivable()
	return financeRouter(user, sess, deps)
}

func TestSettlePageShowsLateFeeBreakdownAndDecisionButtons(t *testing.T) {
	q := &fakeQuotas{lateFee: sampleLateFee()}
	router := settleRouter(model.RoleSyndic, &fakeFinance{quotaIDs: map[string]bool{"a1": true}}, q)

	rec := get(router, "/finance/a1/settle", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET settle = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"R$ 7.00", "R$ 2.33", "R$ 9.33", "20",
		`name="late_fee_decision" value="apply"`, `name="late_fee_decision" value="waive"`,
		"Settle and apply late fee", "Settle and waive late fee",
		`hx-get="/finance/a1/settle/late-fee"`, `hx-trigger="change"`, `id="settle-actions"`,
		`value="` + time.Now().UTC().Format("2006-01-02") + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("settle page is missing %q", want)
		}
	}
}

func TestSettlePageWithoutLateFeeKeepsSingleButton(t *testing.T) {
	router := settleRouter(model.RoleSyndic, &fakeFinance{quotaIDs: map[string]bool{"a1": true}}, &fakeQuotas{})
	body := get(router, "/finance/a1/settle", false).Body.String()
	if strings.Contains(body, "late_fee_decision") {
		t.Error("no late fee: decision buttons must not render")
	}
	if !strings.Contains(body, "Settle this account") {
		t.Error("the usual settle button is missing")
	}
}

func TestSettlePageOfPlainReceivableHasNoLateFeeFragment(t *testing.T) {
	router := settleRouter(model.RoleSyndic, &fakeFinance{}, &fakeQuotas{lateFee: sampleLateFee()})
	body := get(router, "/finance/a1/settle", false).Body.String()
	if strings.Contains(body, "late_fee_decision") || strings.Contains(body, "settle/late-fee") {
		t.Error("a plain receivable settles generically, without late fee UI")
	}
}

func TestSettleLateFeeFragment(t *testing.T) {
	q := &fakeQuotas{lateFee: sampleLateFee()}
	router := settleRouter(model.RoleSyndic, &fakeFinance{quotaIDs: map[string]bool{"a1": true}}, q)

	rec := get(router, "/finance/a1/settle/late-fee?settlement_date=2026-03-15", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("fragment = %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<html") {
		t.Error("the fragment must not be a full page")
	}
	for _, want := range []string{`id="settle-actions"`, "R$ 9.33", `value="apply"`, `value="waive"`} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment is missing %q", want)
		}
	}
	if !q.previewOn.Equal(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("preview date = %v, want the chosen date", q.previewOn)
	}

	// ?date= is accepted too, and a date without late fee gives the usual button.
	q.lateFee = services.LateFeeQuote{}
	body = get(router, "/finance/a1/settle/late-fee?date=2026-01-01", true).Body.String()
	if strings.Contains(body, "late_fee_decision") || !strings.Contains(body, "Settle this account") {
		t.Errorf("no late fee: want the usual settle button, got %s", body)
	}
}

func TestSettleLateFeeFragmentSyndicOnlyAndErrors(t *testing.T) {
	for _, role := range []model.Role{model.RoleOwner, model.RoleTenant} {
		router := settleRouter(role, &fakeFinance{}, &fakeQuotas{})
		rec := get(router, "/finance/a1/settle/late-fee", true)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s = %d, want 403", role, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "late_fee_decision") {
			t.Errorf("%s must not see the buttons", role)
		}
	}

	q := &fakeQuotas{lateFeeErr: services.ErrNotFound}
	router := settleRouter(model.RoleSyndic, &fakeFinance{quotaIDs: map[string]bool{"a1": true}}, q)
	rec := get(router, "/finance/a1/settle/late-fee", true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "The late fee could not be calculated.") {
		t.Errorf("preview failure = %d, want the message inside the fragment", rec.Code)
	}
}

func TestSettleDecisionRequiredRendersBreakdown(t *testing.T) {
	fin := &fakeFinance{quotaIDs: map[string]bool{"a1": true}}
	q := &settleQuotas{err: services.ErrLateFeeDecisionRequired}
	q.lateFee = sampleLateFee()
	rec := settleQuotaRequest(t, fin, q, url.Values{"settlement_date": {"2026-03-15"}})
	body := rec.Body.String()
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("POST settle = %d, want 422", rec.Code)
	}
	for _, want := range []string{"Choose whether to apply or waive the late fee", "R$ 9.33", `value="apply"`, `value="waive"`, `value="2026-03-15"`} {
		if !strings.Contains(body, want) {
			t.Errorf("re-rendered settle page is missing %q", want)
		}
	}
	if !q.previewOn.Equal(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("breakdown date = %v, want the posted date", q.previewOn)
	}
}

func lateFeeFinance() *fakeFinance {
	return &fakeFinance{lateFees: map[string]domain.QuotaPeriod{
		"a1": {Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: 3, UnitCode: "A"},
	}}
}

// AC-8: a late fee receivable cannot be edited but can be canceled (later waiver).
func TestLateFeeAccountEditRefusedCancelAllowed(t *testing.T) {
	user, sess := syndicSession()
	fin := lateFeeFinance()
	deps := baseDeps(fin)
	deps.Accounts = pendingReceivable()
	router := financeRouter(user, sess, deps)

	if rec := do(router, "GET", "/finance/a1/edit", nil); rec.Code != http.StatusConflict {
		t.Errorf("GET edit = %d, want 409", rec.Code)
	}
	if rec := do(router, "POST", "/finance/a1/edit", url.Values{"csrf_token": {"x"}, "title": {"x"}}); rec.Code != http.StatusConflict || fin.lastEdit != "" {
		t.Errorf("POST edit = %d (edit called: %q), want 409 without calling the service", rec.Code, fin.lastEdit)
	}

	rec := do(router, "GET", "/finance/a1/cancel", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET cancel = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "later waiver") || !strings.Contains(body, "Late fee and interest — Condo fee March 2026 — Unit A") {
		t.Errorf("cancel page should name the later waiver and the localised title: %s", body)
	}

	rec = do(router, "POST", "/finance/a1/cancel", url.Values{"csrf_token": {"x"}})
	if rec.Code != http.StatusSeeOther || fin.lastCancel != "a1" {
		t.Errorf("POST cancel = %d (canceled %q), want 303 and the service called", rec.Code, fin.lastCancel)
	}
}

func TestQuotaStillRefusedOnEditAndCancel(t *testing.T) {
	user, sess := syndicSession()
	deps := baseDeps(&fakeFinance{quotaIDs: map[string]bool{"a1": true}})
	deps.Accounts = pendingReceivable()
	router := financeRouter(user, sess, deps)
	for _, p := range []string{"/finance/a1/edit", "/finance/a1/cancel"} {
		if rec := do(router, "GET", p, nil); rec.Code != http.StatusConflict {
			t.Errorf("GET %s = %d, want 409", p, rec.Code)
		}
	}
}

func TestFinanceListShowsLateFeeWithCancelButNoEdit(t *testing.T) {
	user, sess := syndicSession()
	fin := lateFeeFinance()
	fin.list = []domain.FinancialAccount{pendingReceivable().account}
	deps := baseDeps(fin)
	router := financeRouter(user, sess, deps)

	body := do(router, "GET", "/finance", nil).Body.String()
	if !strings.Contains(body, "Late fee and interest — Condo fee March 2026 — Unit A") {
		t.Error("list should show the late fee title")
	}
	if !strings.Contains(body, "/finance/a1/cancel") {
		t.Error("list should keep the cancel action for a late fee")
	}
	if strings.Contains(body, "/finance/a1/edit") {
		t.Error("list must hide edit for a late fee")
	}
}

func TestDebtorsPage(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{debtors: []services.Debtor{
		{
			DebtorRow: services.DebtorRow{
				UnitID: "un1", UnitCode: "101", OverdueCount: 2, OverdueTotalCents: 70000,
				OldestDue: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
				Occupants: []services.DebtorOccupant{{Email: "ana@example.com", Type: model.OccupancyOwner}, {Email: "bob@example.com", Type: model.OccupancyTenant}},
			},
			DaysLate: 40, EstimatedLateFeeCents: 2100,
		},
		{DebtorRow: services.DebtorRow{UnitID: "un2", UnitCode: "202", OverdueCount: 1, OverdueTotalCents: 35000, OldestDue: time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)}, DaysLate: 10},
	}}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))

	rec := get(router, "/finance/debtors", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET debtors = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Debtors", "101", "202", "ana@example.com (owner)", "bob@example.com (tenant)", "No occupants",
		"R$ 700.00", "R$ 21.00", "40", "/finance/my-charges?unit=un1", "/finance/my-charges?unit=un2",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("debtors page is missing %q", want)
		}
	}
	if strings.Index(body, "101") > strings.Index(body, "202") {
		t.Error("rows keep the service order (largest total first)")
	}

	empty := financeRouter(user, sess, quotaDeps(&fakeQuotas{}, model.RoleSyndic))
	if body := get(empty, "/finance/debtors", false).Body.String(); !strings.Contains(body, "No unit has overdue receivables.") {
		t.Error("empty state missing")
	}
}

func TestDebtorsAndReserveSyndicOnly(t *testing.T) {
	user, sess := syndicSession()
	for _, role := range []model.Role{model.RoleOwner, model.RoleTenant} {
		router := financeRouter(user, sess, quotaDeps(&fakeQuotas{}, role))
		if rec := get(router, "/finance/debtors", false); rec.Code != http.StatusForbidden {
			t.Errorf("debtors as %s = %d, want 403", role, rec.Code)
		}
	}
}

func TestSyndicCardDebtorsLine(t *testing.T) {
	q := &fakeResidentQuotas{card: services.CardData{Syndic: &services.SyndicCard{DebtorUnits: 3, DebtorTotalCents: 105000}}}
	router, _ := residentRouter(q, model.RoleSyndic)
	body := get(router, "/finance/quotas/card", true).Body.String()
	if !strings.Contains(body, "3 units overdue — R$ 1,050.00") || !strings.Contains(body, `href="/finance/debtors"`) {
		t.Errorf("card is missing the debtors line: %s", body)
	}

	q.card.Syndic.DebtorUnits = 1
	if body := get(router, "/finance/quotas/card", true).Body.String(); !strings.Contains(body, "1 unit overdue") {
		t.Errorf("singular line missing: %s", body)
	}

	q.card.Syndic.DebtorUnits, q.card.Syndic.DebtorTotalCents = 0, 0
	if body := get(router, "/finance/quotas/card", true).Body.String(); strings.Contains(body, "overdue —") || strings.Contains(body, "/finance/debtors") {
		t.Errorf("the line must be hidden with no debtors: %s", body)
	}
}

func TestOverviewReserveCardSyndicOnly(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{reserve: services.ReserveBalance{RealizedCents: 100000, ForecastCents: 250000, RealizedYearCents: 40000}}

	syn := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))
	body := get(syn, "/finance/overview", true).Body.String()
	for _, want := range []string{"Reserve fund", "R$ 1,000.00", "R$ 2,500.00", "R$ 400.00", `href="/finance/debtors"`} {
		if !strings.Contains(body, want) {
			t.Errorf("syndic overview is missing %q", want)
		}
	}

	owner := financeRouter(user, sess, quotaDeps(q, model.RoleOwner))
	body = get(owner, "/finance/overview", true).Body.String()
	if strings.Contains(body, "Reserve fund") || strings.Contains(body, "/finance/debtors") {
		t.Error("a non-syndic must not see the reserve card or the debtors link")
	}
}

func TestHandlersUseInjectedClock(t *testing.T) {
	fixed := time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)
	wantDay := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	user, sess := syndicSession()

	q := &fakeQuotas{}
	deps := quotaDeps(q, model.RoleSyndic)
	deps.Now = func() time.Time { return fixed }
	router := financeRouter(user, sess, deps)

	if rec := get(router, "/finance/debtors", false); rec.Code != http.StatusOK {
		t.Fatalf("GET debtors = %d", rec.Code)
	}
	if !q.debtorsToday.Equal(wantDay) {
		t.Errorf("Debtors today = %v, want %v", q.debtorsToday, wantDay)
	}

	if rec := get(router, "/finance/overview", true); rec.Code != http.StatusOK {
		t.Fatalf("GET overview = %d", rec.Code)
	}
	if !q.reserveToday.Equal(wantDay) {
		t.Errorf("ReserveBalance today = %v, want %v", q.reserveToday, wantDay)
	}

	sq := &fakeQuotas{lateFee: sampleLateFee()}
	srouter := settleRouterAt(model.RoleSyndic, &fakeFinance{quotaIDs: map[string]bool{"a1": true}}, sq, fixed)
	body := get(srouter, "/finance/a1/settle", false).Body.String()
	if !strings.Contains(body, `value="2026-03-15"`) {
		t.Error("settle form default date is not the injected date")
	}
}

func settleRouterAt(role model.Role, fin *fakeFinance, q *fakeQuotas, now time.Time) http.Handler {
	user, sess := syndicSession()
	deps := baseDeps(fin)
	deps.Roles = &fakeRoles{roles: []model.Role{role}}
	deps.Quotas = q
	deps.Accounts = pendingReceivable()
	deps.Now = func() time.Time { return now }
	return financeRouter(user, sess, deps)
}

func TestBudgetPageShowsReserveFigures(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{detail: sampleDetail(), reserve: services.ReserveBalance{RealizedCents: 100000, ForecastCents: 250000, RealizedYearCents: 40000}}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))
	body := get(router, "/finance/budgets/b1/view", false).Body.String()
	for _, want := range []string{"Reserve fund", "R$ 1,000.00", "R$ 2,500.00", "R$ 400.00"} {
		if !strings.Contains(body, want) {
			t.Errorf("budget page is missing %q", want)
		}
	}
}

func TestReciboShowsLateFeeBreakdown(t *testing.T) {
	settled := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	q := &fakeResidentQuotas{receipt: services.ReceiptData{Number: "ABCD1234", ReceiptRow: services.ReceiptRow{
		AccountID: "lf1", CondominiumName: "Edificio Sol", UnitID: "un1", UnitCode: "101", AmountCents: 933,
		SettlementDate: &settled, IsLateFee: true,
		LateFeeOrigin: domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: 3, UnitCode: "101"},
		LateFee:       &domain.LateFeeDetail{FineCents: 700, InterestCents: 233, DaysLate: 20, FeeBP: 200, InterestBP: 100, GraceDays: 5},
	}}}
	router, _ := residentRouter(q, model.RoleOwner)
	rec := get(router, "/finance/lf1/recibo", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET recibo = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Late fee and interest — Condo fee March 2026 — Unit 101", "Origin quota", "R$ 7.00 (2%)", "R$ 2.33 (1%)", "Days late", "Grace days", "R$ 9.33",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("recibo is missing %q", want)
		}
	}
}

func TestSettleInvalidDecisionIs400(t *testing.T) {
	fin := &fakeFinance{quotaIDs: map[string]bool{"a1": true}}
	q := &settleQuotas{err: services.ErrInvalidInput}
	rec := settleQuotaRequest(t, fin, q, url.Values{"settlement_date": {"2026-03-15"}, "late_fee_decision": {"maybe"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST settle with invalid decision = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Invalid late fee choice") {
		t.Error("the translated error is missing")
	}
	if q.decision != "maybe" {
		t.Errorf("decision passed to the service = %q", q.decision)
	}
}

// Implicit submission (Enter in the date input) must never apply or waive:
// the first submit button of the form carries no decision.
func TestSettleFirstSubmitButtonHasNoDecision(t *testing.T) {
	q := &fakeQuotas{lateFee: sampleLateFee()}
	router := settleRouter(model.RoleSyndic, &fakeFinance{quotaIDs: map[string]bool{"a1": true}}, q)
	body := get(router, "/finance/a1/settle", false).Body.String()
	form := body[strings.Index(body, "<form method=\"POST\""):]
	first := form[strings.Index(form, "<button"):]
	first = first[:strings.Index(first, ">")]
	if strings.Contains(first, "late_fee_decision") || !strings.Contains(first, `type="submit"`) {
		t.Errorf("first button = %q, want a plain submit without decision", first)
	}
}
