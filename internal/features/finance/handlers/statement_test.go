package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// fakeResidentQuotas adds the statement/receipt/card use cases to fakeQuotas.
type fakeResidentQuotas struct {
	fakeQuotas
	statement    services.Statement
	statementErr error
	receipt      services.ReceiptData
	receiptErr   error
	card         services.CardData
	cardErr      error

	lastViewer services.Viewer
	lastUnit   string
}

func (f *fakeResidentQuotas) Statement(_ context.Context, v services.Viewer, _, unitID string) (services.Statement, error) {
	f.lastViewer, f.lastUnit = v, unitID
	return f.statement, f.statementErr
}

func (f *fakeResidentQuotas) Receipt(_ context.Context, v services.Viewer, _, _ string) (services.ReceiptData, error) {
	f.lastViewer = v
	return f.receipt, f.receiptErr
}

func (f *fakeResidentQuotas) Card(_ context.Context, v services.Viewer, _ string, _ time.Time) (services.CardData, error) {
	f.lastViewer = v
	return f.card, f.cardErr
}

func residentRouter(q *fakeResidentQuotas, role model.Role) (http.Handler, *fakeAudit) {
	user, sess := syndicSession()
	audit := &fakeAudit{}
	deps := baseDeps(&fakeFinance{})
	deps.Roles = &fakeRoles{roles: []model.Role{role}}
	deps.Audit = audit
	deps.Quotas = q
	return financeRouter(user, sess, deps), audit
}

func get(router http.Handler, target string, hx bool) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, overviewReq(target, hx))
	return rec
}

func sampleStatement() services.Statement {
	day := 24 * time.Hour
	settled := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	return services.Statement{
		Unit:  model.Unit{ID: "un1", Code: "101"},
		Units: []model.Unit{{ID: "un1", Code: "101"}, {ID: "un2", Code: "102"}},
		Pending: []services.StatementRow{
			{Account: domain.FinancialAccount{ID: "p1", Title: "Overdue fee", AmountCents: 35000, DueDate: time.Now().Add(-30 * day), Status: domain.StatusPending}, IsQuota: true,
				Period: domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: 8, UnitCode: "101"}},
			{Account: domain.FinancialAccount{ID: "p2", Title: "Future fee", AmountCents: 35000, DueDate: time.Now().Add(30 * day), Status: domain.StatusPending}},
		},
		Settled: []services.StatementRow{
			{Account: domain.FinancialAccount{ID: "s1", Title: "Paid fee", AmountCents: 35000, DueDate: settled, SettlementDate: &settled, Status: domain.StatusSettled}},
		},
		PendingTotalCents: 70000, OverdueCount: 1, OverdueTotalCents: 35000,
	}
}

func TestStatementRendersPendingSettledAndSelector(t *testing.T) {
	q := &fakeResidentQuotas{statement: sampleStatement()}
	router, _ := residentRouter(q, model.RoleTenant)

	rec := get(router, "/finance/my-charges?unit=un2", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET my-charges = %d, want 200", rec.Code)
	}
	if q.lastUnit != "un2" || q.lastViewer.IsSyndic || q.lastViewer.UserID != "u1" {
		t.Fatalf("service got unit=%q viewer=%+v", q.lastUnit, q.lastViewer)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Account statement", "Condo fee August 2026 — Unit 101", "Future fee", "Paid fee",
		`name="unit"`, `value="un2"`, "/finance/s1/recibo", "bg-red-50", "R$ 700.00",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("statement is missing %q", want)
		}
	}
	if strings.Contains(body, "/finance/p1/recibo") {
		t.Errorf("pending rows must not link to a Recibo")
	}
	// The stored fallback title and the machine period label are never shown.
	for _, absent := range []string{"Overdue fee", "2026-08"} {
		if strings.Contains(body, absent) {
			t.Errorf("statement shows %q", absent)
		}
	}
	if strings.Count(body, "bg-red-50") != 1 {
		t.Errorf("only the overdue row is highlighted")
	}
}

func TestStatementSingleUnitHasNoSelector(t *testing.T) {
	st := sampleStatement()
	st.Units = st.Units[:1]
	router, _ := residentRouter(&fakeResidentQuotas{statement: st}, model.RoleOwner)
	rec := get(router, "/finance/my-charges", false)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `name="unit"`) {
		t.Fatalf("single unit: code=%d selector shown=%v", rec.Code, strings.Contains(rec.Body.String(), `name="unit"`))
	}
}

func TestStatementSyndicViewer(t *testing.T) {
	q := &fakeResidentQuotas{statement: sampleStatement()}
	router, _ := residentRouter(q, model.RoleSyndic)
	get(router, "/finance/my-charges", false)
	if !q.lastViewer.IsSyndic {
		t.Fatalf("syndic role must reach the service as IsSyndic")
	}
}

func TestStatementErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  error
		code int
		text string
	}{
		{"no unit", services.ErrNoUnit, http.StatusOK, "You need an active unit"},
		{"forbidden", services.ErrForbidden, http.StatusForbidden, ""},
		{"not found", services.ErrNotFound, http.StatusNotFound, ""},
	} {
		router, _ := residentRouter(&fakeResidentQuotas{statementErr: tt.err}, model.RoleTenant)
		rec := get(router, "/finance/my-charges?unit=x", false)
		if rec.Code != tt.code {
			t.Errorf("%s: code = %d, want %d", tt.name, rec.Code, tt.code)
		}
		if tt.text != "" && !strings.Contains(rec.Body.String(), tt.text) {
			t.Errorf("%s: missing %q", tt.name, tt.text)
		}
	}
}

func TestReciboContentForQuota(t *testing.T) {
	settled := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	q := &fakeResidentQuotas{receipt: services.ReceiptData{
		ReceiptRow: services.ReceiptRow{
			AccountID: "s1", CondominiumName: "Sunrise Court", Currency: money.EUR, UnitID: "un1", UnitCode: "101",
			Title: "Quota 2026-08 101", AmountCents: 35000, Status: domain.StatusSettled, SettlementDate: &settled,
			IsQuota: true, BaseCents: 31000, ReserveCents: 4000,
			Period: domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: 8, UnitCode: "101"},
		},
		Number: "AB12CD34",
	}}
	router, _ := residentRouter(q, model.RoleTenant)

	rec := get(router, "/finance/s1/recibo", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET recibo = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Sunrise Court", "101", "Condo fee August 2026 — Unit 101", "AB12CD34", "November 3, 2026",
		"receipt-sheet", "no-print", "Ctrl+P", "/finance/my-charges?unit=un1",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("recibo is missing %q", want)
		}
	}
	if strings.Contains(body, "2026-08") || strings.Contains(body, "Quota 2026") {
		t.Errorf("recibo shows the machine label or the stored fallback title")
	}
	// Money is shown in the condominium currency carried by the receipt (EUR), not the default.
	if !strings.Contains(body, "€") || strings.Contains(body, "R$") {
		t.Errorf("recibo must use the condominium currency: %s", body)
	}
}

func TestReciboNonQuotaHidesPeriod(t *testing.T) {
	settled := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	q := &fakeResidentQuotas{receipt: services.ReceiptData{
		ReceiptRow: services.ReceiptRow{AccountID: "s2", CondominiumName: "Sunrise Court", Currency: money.BRL, UnitCode: "101",
			Title: "Party room", AmountCents: 5000, Status: domain.StatusSettled, SettlementDate: &settled},
		Number: "FF00FF00",
	}}
	router, _ := residentRouter(q, model.RoleSyndic)
	rec := get(router, "/finance/s2/recibo", false)
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "Reserve") {
		t.Fatalf("non-quota recibo: code=%d, must not show base/reserve", rec.Code)
	}
}

func TestReciboErrors(t *testing.T) {
	for _, tt := range []struct {
		err  error
		code int
	}{
		{services.ErrNotFound, http.StatusNotFound},
		{services.ErrForbidden, http.StatusForbidden},
	} {
		router, _ := residentRouter(&fakeResidentQuotas{receiptErr: tt.err}, model.RoleOwner)
		if rec := get(router, "/finance/x/recibo", false); rec.Code != tt.code {
			t.Errorf("recibo %v = %d, want %d", tt.err, rec.Code, tt.code)
		}
	}
}

func TestReciboRequiresMembership(t *testing.T) {
	user, sess := syndicSession()
	deps := baseDeps(&fakeFinance{})
	deps.Roles = &fakeRoles{}
	deps.Audit = &fakeAudit{}
	deps.Quotas = &fakeResidentQuotas{}
	rec := get(financeRouter(user, sess, deps), "/finance/x/recibo", false)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-member recibo = %d, want 403", rec.Code)
	}
}

func TestQuotasCardResidentVariants(t *testing.T) {
	due := time.Date(2026, 12, 10, 0, 0, 0, 0, time.UTC)
	next := &services.ResidentCard{NextPending: &services.CardCharge{AmountCents: 35000, DueDate: due}}
	overdue := &services.ResidentCard{NextPending: &services.CardCharge{AmountCents: 35000, DueDate: due}, OverdueCount: 2, OverdueTotalCents: 70000}
	for _, tt := range []struct {
		name string
		card *services.ResidentCard
		want []string
		not  []string
	}{
		{"next", next, []string{"Next condo fee", "R$ 350.00", "December 10, 2026"}, []string{"Overdue condo fees"}},
		{"overdue", overdue, []string{"Overdue condo fees", "R$ 700.00", ">2<"}, []string{"Next condo fee"}},
		{"none", &services.ResidentCard{}, []string{"No pending condo fees."}, nil},
	} {
		router, _ := residentRouter(&fakeResidentQuotas{card: services.CardData{Resident: tt.card}}, model.RoleTenant)
		rec := get(router, "/finance/quotas/card", true)
		if rec.Code != http.StatusOK || rec.Header().Get("Vary") != "HX-Request" {
			t.Fatalf("%s: code=%d vary=%q", tt.name, rec.Code, rec.Header().Get("Vary"))
		}
		body := rec.Body.String()
		if strings.Contains(body, "<html") || !strings.Contains(body, `id="quotas-card"`) {
			t.Errorf("%s: want bare fragment", tt.name)
		}
		for _, w := range tt.want {
			if !strings.Contains(body, w) {
				t.Errorf("%s: missing %q in %s", tt.name, w, body)
			}
		}
		for _, w := range tt.not {
			if strings.Contains(body, w) {
				t.Errorf("%s: unexpected %q", tt.name, w)
			}
		}
	}
}

func TestQuotasCardSyndicVariants(t *testing.T) {
	nov2026 := domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: 11}
	for _, tt := range []struct {
		name string
		card *services.SyndicCard
		want []string
	}{
		{"not issued", &services.SyndicCard{HasBudget: true, BudgetID: "b1", PeriodIndex: 11, Period: nov2026}, []string{"November 2026 — Not issued", "/finance/budgets/b1/issuances/11/preview"}},
		{"issued", &services.SyndicCard{HasBudget: true, BudgetID: "b1", PeriodIndex: 11, Period: nov2026, Issued: true, ChargeCount: 10, SettledCount: 4}, []string{"November 2026 — Issued: 4 of 10 settled"}},
		{"no budget", &services.SyndicCard{}, []string{"No approved budget", "/finance/budgets"}},
	} {
		router, _ := residentRouter(&fakeResidentQuotas{card: services.CardData{Syndic: tt.card}}, model.RoleSyndic)
		rec := get(router, "/finance/quotas/card", true)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: code = %d", tt.name, rec.Code)
		}
		for _, w := range tt.want {
			if !strings.Contains(rec.Body.String(), w) {
				t.Errorf("%s: missing %q", tt.name, w)
			}
		}
	}
}

func TestQuotasCardGuardsAndErrors(t *testing.T) {
	// No role: 403 partial plus audit.
	user, sess := syndicSession()
	audit := &fakeAudit{}
	deps := baseDeps(&fakeFinance{})
	deps.Roles = &fakeRoles{}
	deps.Audit = audit
	deps.Quotas = &fakeResidentQuotas{}
	rec := get(financeRouter(user, sess, deps), "/finance/quotas/card", true)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), `id="quotas-card"`) {
		t.Fatalf("denied card = %d %q", rec.Code, rec.Body.String())
	}
	if len(audit.events) != 1 || audit.events[0] != model.AuditAccessDenied {
		t.Errorf("audit = %v, want one access_denied", audit.events)
	}

	// Unauthenticated HX: HX-Redirect.
	anon := get(financeRouter(nil, nil, deps), "/finance/quotas/card", true)
	if anon.Header().Get("HX-Redirect") != "/login" {
		t.Errorf("unauth HX-Redirect = %q", anon.Header().Get("HX-Redirect"))
	}

	// Service failure: 500 partial with retry.
	router, _ := residentRouter(&fakeResidentQuotas{cardErr: context.DeadlineExceeded}, model.RoleTenant)
	rec = get(router, "/finance/quotas/card", true)
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "Try again") {
		t.Errorf("card error = %d", rec.Code)
	}

	// Non-HX goes to the dashboard.
	router, _ = residentRouter(&fakeResidentQuotas{}, model.RoleTenant)
	rec = get(router, "/finance/quotas/card", false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Errorf("non-HX card = %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestFinanceListHidesQuotaActionsAndShowsRecibo(t *testing.T) {
	user, sess := syndicSession()
	settled := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	due := time.Now().Add(7 * 24 * time.Hour)
	fin := &fakeFinance{
		list: []domain.FinancialAccount{
			{ID: "q1", Type: domain.AccountTypeReceivable, Title: "Quota pend", Category: domain.CategoryCondoFee, AmountCents: 100, DueDate: due, Status: domain.StatusPending},
			{ID: "r1", Type: domain.AccountTypeReceivable, Title: "Plain pend", Category: domain.CategoryCondoFee, AmountCents: 100, DueDate: due, Status: domain.StatusPending},
			{ID: "q2", Type: domain.AccountTypeReceivable, Title: "Quota paid", Category: domain.CategoryCondoFee, AmountCents: 100, DueDate: settled, SettlementDate: &settled, Status: domain.StatusSettled},
			{ID: "p1", Type: domain.AccountTypePayable, Title: "Payable paid", Category: domain.CategoryMaintenance, AmountCents: 100, DueDate: settled, SettlementDate: &settled, Status: domain.StatusSettled, ReceiptPath: "x.pdf"},
		},
		quotaIDs: map[string]bool{"q1": true, "q2": true},
	}
	rec := get(financeRouter(user, sess, baseDeps(fin)), "/finance", false)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /finance = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, absent := range []string{"/finance/q1/edit", "/finance/q1/cancel", "/finance/p1/recibo"} {
		if strings.Contains(body, absent) {
			t.Errorf("list must not contain %q", absent)
		}
	}
	for _, present := range []string{"/finance/q1/settle", "/finance/r1/edit", "/finance/r1/cancel", "/finance/q2/recibo", "/finance/p1/receipt", `href="/finance/budgets"`} {
		if !strings.Contains(body, present) {
			t.Errorf("list is missing %q", present)
		}
	}
	if strings.Contains(body, "/finance/r1/recibo") {
		t.Errorf("pending receivable must not link to a Recibo")
	}
	if !strings.Contains(body, "Condo fee Q2 2026 — Unit A") || strings.Contains(body, "Quota pend") {
		t.Errorf("quota rows must show the localised title, not the stored one")
	}
	if !strings.Contains(body, "Plain pend") {
		t.Errorf("non-quota rows keep their own title")
	}
}

// TestQuotaTitleAndPeriodPerLocale covers AC-22: the title and period of a
// quota are rendered in the viewer's language on the list, statement, Recibo and card.
func TestQuotaTitleAndPeriodPerLocale(t *testing.T) {
	settled := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	q2 := domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityQuarterly, Index: 2, UnitCode: "A"}
	dec := domain.QuotaPeriod{Year: 2026, Periodicity: domain.PeriodicityMonthly, Index: 12, UnitCode: "A"}
	for _, tt := range []struct {
		lang                string
		title, period, card string
		notShown            []string
	}{
		{"pt-br", "Taxa condominial 2º trimestre de 2026 — Unidade A", "2º trimestre de 2026", "dezembro de 2026 — Não emitida", []string{"Quota 2026", "2026-T2", "2026-12"}},
		{"pt-pt", "Quota 2.º trimestre de 2026 — Unidade A", "2.º trimestre de 2026", "dezembro de 2026 — Não emitida", []string{"Quota 2026", "2026-T2", "2026-12"}},
		{"en", "Condo fee Q2 2026 — Unit A", "Q2 2026", "December 2026 — Not issued", []string{"Quota 2026", "2026-T2", "2026-12"}},
	} {
		user, sess := syndicSession()
		user.LanguagePreference = tt.lang
		fq := &fakeResidentQuotas{
			statement: services.Statement{
				Unit: model.Unit{ID: "un1", Code: "A"}, Units: []model.Unit{{ID: "un1", Code: "A"}},
				Settled: []services.StatementRow{{Account: domain.FinancialAccount{ID: "s1", Title: "Quota 2026-T2 A", AmountCents: 100, DueDate: settled, SettlementDate: &settled, Status: domain.StatusSettled}, IsQuota: true, Period: q2}},
			},
			receipt: services.ReceiptData{ReceiptRow: services.ReceiptRow{
				AccountID: "s1", Currency: money.EUR, UnitCode: "A", Title: "Quota 2026-T2 A", AmountCents: 100, Status: domain.StatusSettled,
				SettlementDate: &settled, IsQuota: true, Period: q2,
			}},
			card: services.CardData{Syndic: &services.SyndicCard{HasBudget: true, BudgetID: "b1", PeriodIndex: 12, Period: dec}},
		}
		deps := baseDeps(&fakeFinance{
			list:     []domain.FinancialAccount{{ID: "q1", Type: domain.AccountTypeReceivable, Title: "Quota 2026-T2 A", Category: domain.CategoryCondoFee, AmountCents: 100, DueDate: settled, Status: domain.StatusPending}},
			quotaIDs: map[string]bool{"q1": true},
		})
		deps.Quotas = fq
		router := financeRouter(user, sess, deps)
		for name, page := range map[string]struct {
			target, want string
		}{
			"list":          {"/finance", tt.title},
			"statement":     {"/finance/my-charges", tt.title},
			"recibo":        {"/finance/s1/recibo", tt.title},
			"recibo period": {"/finance/s1/recibo", tt.period},
			"card":          {"/finance/quotas/card", tt.card},
		} {
			rec := get(router, page.target, name == "card")
			body := rec.Body.String()
			if rec.Code != http.StatusOK || !strings.Contains(body, page.want) {
				t.Errorf("%s/%s: code=%d missing %q", tt.lang, name, rec.Code, page.want)
			}
			for _, absent := range tt.notShown {
				if strings.Contains(body, absent) {
					t.Errorf("%s/%s: shows %q", tt.lang, name, absent)
				}
			}
		}
	}
}

// TestSettlePageShowsLocalisedQuotaTitle covers I9: the generic settle page
// shows the localised quota title; other receivables keep the stored title.
func TestSettlePageShowsLocalisedQuotaTitle(t *testing.T) {
	due := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct{ lang, want string }{
		{"pt-br", "Taxa condominial 2º trimestre de 2026 — Unidade A"},
		{"en", "Condo fee Q2 2026 — Unit A"},
	} {
		user, sess := syndicSession()
		user.LanguagePreference = tt.lang
		acc := func(id, title string) domain.FinancialAccount {
			return domain.FinancialAccount{ID: id, CondominiumID: "c1", Type: domain.AccountTypeReceivable, Title: title, AmountCents: 100, DueDate: due, Status: domain.StatusPending}
		}
		// No list: the settle page must resolve the quota period for the one
		// account directly, not by listing the month's receivables.
		fin := &fakeFinance{quotaIDs: map[string]bool{"q1": true}}
		for id, c := range map[string]struct {
			title string
			want  bool
		}{
			"q1": {"Quota 2026-T2 A", true},
			"o1": {"Rent hall", false},
		} {
			deps := baseDeps(fin)
			deps.Quotas = &fakeResidentQuotas{}
			deps.Accounts = &fakeAccountReader{account: acc(id, c.title)}
			rec := get(financeRouter(user, sess, deps), "/finance/"+id+"/settle", false)
			body := rec.Body.String()
			if rec.Code != http.StatusOK {
				t.Fatalf("%s/%s: code=%d", tt.lang, id, rec.Code)
			}
			if c.want {
				if !strings.Contains(body, tt.want) || strings.Contains(body, "Quota 2026-T2 A") {
					t.Errorf("%s/%s: want localised title %q only", tt.lang, id, tt.want)
				}
			} else if !strings.Contains(body, "Rent hall") {
				t.Errorf("%s/%s: stored title missing", tt.lang, id)
			}
		}
	}
}

// TestQuotaRoutesAbsentWithoutQuotas covers I6: with nil Quotas none of the
// quota-dependent routes is registered.
func TestQuotaRoutesAbsentWithoutQuotas(t *testing.T) {
	user, sess := syndicSession()
	router := financeRouter(user, sess, baseDeps(&fakeFinance{}))
	for _, target := range []string{
		"/finance/budgets", "/finance/my-charges", "/finance/a1/recibo", "/finance/quotas/card",
	} {
		if rec := get(router, target, true); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s with nil Quotas = %d, want 404", target, rec.Code)
		}
	}
}

func TestQuotaAccountEditAndCancelRefused(t *testing.T) {
	user, sess := syndicSession()
	deps := baseDeps(&fakeFinance{quotaIDs: map[string]bool{"q1": true}})
	deps.Accounts = &fakeAccountReader{account: domain.FinancialAccount{
		ID: "q1", CondominiumID: "c1", Type: domain.AccountTypeReceivable, Status: domain.StatusPending,
	}}
	router := financeRouter(user, sess, deps)
	for _, path := range []string{"/finance/q1/edit", "/finance/q1/cancel"} {
		if rec := get(router, path, false); rec.Code != http.StatusConflict {
			t.Errorf("GET %s = %d, want 409", path, rec.Code)
		}
	}
	for _, path := range []string{"/finance/q1/edit", "/finance/q1/cancel"} {
		rec := do(router, http.MethodPost, path, map[string][]string{"csrf_token": {"x"}, "type": {"receivable"}})
		if rec.Code != http.StatusConflict {
			t.Errorf("POST %s = %d, want 409", path, rec.Code)
		}
	}
}

func TestQuotaManagedErrorFromServiceIsRefused(t *testing.T) {
	user, sess := syndicSession()
	deps := baseDeps(&fakeFinance{cancelErr: services.ErrQuotaManaged})
	rec := do(financeRouter(user, sess, deps), http.MethodPost, "/finance/q1/cancel", map[string][]string{"csrf_token": {"x"}})
	if rec.Code != http.StatusConflict {
		t.Fatalf("cancel quota via service error = %d, want 409", rec.Code)
	}
}
