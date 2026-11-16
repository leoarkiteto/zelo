package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/permille"
)

type fakeQuotas struct {
	budgets      []services.BudgetSummary
	detail       services.BudgetDetail
	issuances    []services.IssuanceSummary
	preview      services.IssuancePreview
	previewErr   error
	issueErr     error
	issueCreated bool
	approveErr   error
	cancelErr    error
	createErr    error

	lastCreate services.BudgetInput
	lastLine   services.LineInput
	approved   bool
	canceled   string
	issuedCall bool
}

func (f *fakeQuotas) CreateBudget(_ context.Context, _, _ string, in services.BudgetInput) (services.Budget, error) {
	f.lastCreate = in
	if f.createErr != nil {
		return services.Budget{}, f.createErr
	}
	return services.Budget{ID: "b1", Year: in.Year}, nil
}
func (f *fakeQuotas) UpdateBudget(context.Context, string, string, string, services.BudgetParams) error {
	return nil
}
func (f *fakeQuotas) AddLine(_ context.Context, _, _, _ string, in services.LineInput) (string, error) {
	f.lastLine = in
	return "l1", nil
}
func (f *fakeQuotas) UpdateLine(context.Context, string, string, string, string, services.LineInput) error {
	return nil
}
func (f *fakeQuotas) RemoveLine(context.Context, string, string, string, string) error { return nil }
func (f *fakeQuotas) ApproveBudget(context.Context, string, string, string) error {
	f.approved = f.approveErr == nil
	return f.approveErr
}
func (f *fakeQuotas) ReopenBudget(context.Context, string, string, string) error { return nil }
func (f *fakeQuotas) ListBudgets(context.Context, string) ([]services.BudgetSummary, error) {
	return f.budgets, nil
}
func (f *fakeQuotas) GetBudget(context.Context, string, string) (services.BudgetDetail, error) {
	return f.detail, nil
}
func (f *fakeQuotas) PreviewIssuance(context.Context, string, string, int) (services.IssuancePreview, error) {
	return f.preview, f.previewErr
}
func (f *fakeQuotas) Issue(context.Context, string, string, string, int) (string, bool, error) {
	f.issuedCall = true
	return "i1", f.issueCreated, f.issueErr
}
func (f *fakeQuotas) CancelIssuance(_ context.Context, _, _, id string) error {
	f.canceled = id
	return f.cancelErr
}
func (f *fakeQuotas) ListIssuances(context.Context, string, string) ([]services.IssuanceSummary, error) {
	return f.issuances, nil
}

func (f *fakeQuotas) Statement(context.Context, services.Viewer, string, string) (services.Statement, error) {
	return services.Statement{}, nil
}
func (f *fakeQuotas) Receipt(context.Context, services.Viewer, string, string) (services.ReceiptData, error) {
	return services.ReceiptData{}, nil
}
func (f *fakeQuotas) Card(context.Context, services.Viewer, string, time.Time) (services.CardData, error) {
	return services.CardData{}, nil
}

func quotaDeps(q *fakeQuotas, role model.Role) Deps {
	d := baseDeps(&fakeFinance{})
	d.Roles = &fakeRoles{roles: []model.Role{role}}
	d.Quotas = q
	return d
}

func sampleDetail() services.BudgetDetail {
	return services.BudgetDetail{
		Budget: services.Budget{
			ID: "b1", Year: 2026, Status: domain.BudgetApproved, Periodicity: domain.PeriodicityQuarterly,
			DueDay: 5, ReserveRateBP: 1000,
		},
		Lines:      []services.BudgetLine{{ID: "l1", Description: "Elevator", Category: domain.CategoryMaintenance, AmountCents: 120000}},
		TotalCents: 120000,
		SumPPM:     permille.Total,
		Periods:    []services.PeriodQuota{{Index: 1, Label: "2026-T1", DueDate: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), TotalCents: 33000}},
		Units:      []services.UnitQuotas{{UnitID: "u1", Code: "A1", SharePPM: 1_000_000, PerPeriodCents: []int64{33000}}},
	}
}

func do(router http.Handler, method, path string, form url.Values) *httptest.ResponseRecorder {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestQuotaAdminSyndicOnly(t *testing.T) {
	user, sess := syndicSession()
	for _, role := range []model.Role{model.RoleOwner, model.RoleTenant} {
		router := financeRouter(user, sess, quotaDeps(&fakeQuotas{}, role))
		for _, c := range []struct{ method, path string }{
			{"GET", "/finance/budgets"},
			{"GET", "/finance/budgets/b1/view"},
			{"POST", "/finance/budgets/b1/approve"},
			{"GET", "/finance/budgets/b1/issuances/1/preview"},
			{"POST", "/finance/issuances/i1/cancel"},
		} {
			if rec := do(router, c.method, c.path, url.Values{}); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as %s = %d, want 403", c.method, c.path, role, rec.Code)
			}
		}
	}
}

func TestBudgetsListAndNew(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{budgets: []services.BudgetSummary{{
		Budget: services.Budget{ID: "b1", Year: 2026, Status: domain.BudgetDraft, Periodicity: domain.PeriodicityMonthly}, LineCount: 2, TotalCents: 50000,
	}}}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))

	rec := do(router, "GET", "/finance/budgets", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "2026") || !strings.Contains(rec.Body.String(), "/finance/budgets/b1/view") {
		t.Fatalf("list = %d, body missing budget", rec.Code)
	}
	rec = do(router, "GET", "/finance/budgets/new", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `action="/finance/budgets"`) {
		t.Fatalf("new = %d", rec.Code)
	}
}

func TestBudgetCreateHappyAndDuplicate(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))
	form := url.Values{"year": {"2026"}, "periodicity": {"monthly"}, "due_day": {"5"}, "reserve_rate": {"7,5"}, "csrf_token": {"x"}}

	rec := do(router, "POST", "/finance/budgets", form)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/finance/budgets/b1/view?flash=budget_created" {
		t.Fatalf("create = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if q.lastCreate.ReserveRateBP == nil || *q.lastCreate.ReserveRateBP != 750 || q.lastCreate.Year != 2026 {
		t.Fatalf("input = %+v", q.lastCreate)
	}

	q.createErr = domain.ErrBudgetExists
	rec = do(router, "POST", "/finance/budgets", form)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "A budget already exists for this year.") {
		t.Fatalf("duplicate = %d", rec.Code)
	}
}

func TestBudgetViewShowsLinesPreviewAndPeriods(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{detail: sampleDetail(), issuances: []services.IssuanceSummary{{
		ID: "i1", BudgetID: "b1", Year: 2026, Periodicity: domain.PeriodicityQuarterly, PeriodIndex: 1,
		Status: domain.IssuanceIssued, TotalCents: 33000, ChargeCount: 4, SettledCount: 1,
	}}}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))
	rec := do(router, "GET", "/finance/budgets/b1/view", nil)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("view = %d", rec.Code)
	}
	for _, want := range []string{"Elevator", "A1", "Q1 2026", "1 / 4", "/finance/issuances/i1/cancel?budget=b1", "Ownership share"} {
		if !strings.Contains(body, want) {
			t.Errorf("view missing %q", want)
		}
	}
	if strings.Contains(body, "Reopen as draft") {
		t.Error("reopen must be hidden while an issuance is active")
	}
}

func TestLineAddAmountError(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{detail: sampleDetail()}
	q.detail.Status = domain.BudgetDraft
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))

	rec := do(router, "POST", "/finance/budgets/b1/lines", url.Values{"description": {"Paint"}, "category": {"cleaning"}, "amount": {"abc"}})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Enter an amount greater than zero") {
		t.Fatalf("bad amount = %d", rec.Code)
	}
	rec = do(router, "POST", "/finance/budgets/b1/lines", url.Values{"description": {"Paint"}, "category": {"cleaning"}, "amount": {"12,50"}})
	if rec.Code != http.StatusSeeOther || q.lastLine.AmountCents != 1250 {
		t.Fatalf("add = %d cents=%d", rec.Code, q.lastLine.AmountCents)
	}
}

func TestApproveErrorsAreTranslated(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{detail: sampleDetail(), approveErr: domain.ErrBudgetEmpty}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))
	rec := do(router, "POST", "/finance/budgets/b1/approve", url.Values{})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "Add at least one expense line first.") {
		t.Fatalf("approve error = %d", rec.Code)
	}
	q.approveErr = nil
	rec = do(router, "POST", "/finance/budgets/b1/approve", url.Values{})
	if rec.Code != http.StatusSeeOther || !q.approved {
		t.Fatalf("approve = %d", rec.Code)
	}
}

func TestIssuancePreviewAndErrors(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{preview: services.IssuancePreview{
		BudgetID: "b1", Year: 2026, Periodicity: domain.PeriodicityQuarterly, PeriodIndex: 1, Label: "2026-T1", DueDate: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC),
		TotalCents: 33000, BaseCents: 30000, ReserveCents: 3000,
		Rows: []services.PreviewRow{{UnitID: "u1", Code: "A1", SharePPM: 1_000_000, BaseCents: 30000, ReserveCents: 3000, QuotaCents: 33000}},
	}}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))

	rec := do(router, "GET", "/finance/budgets/b1/issuances/1/preview", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Q1 2026") || strings.Contains(rec.Body.String(), "2026-T1") || !strings.Contains(rec.Body.String(), `action="/finance/budgets/b1/issuances/1"`) {
		t.Fatalf("preview = %d", rec.Code)
	}

	q.previewErr = services.SharesError{SumPPM: 987_500}
	rec = do(router, "GET", "/finance/budgets/b1/issuances/1/preview", nil)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "do not add up to 100%") || !strings.Contains(rec.Body.String(), "-12.5") {
		t.Fatalf("shares error = %d body=%q", rec.Code, rec.Body.String())
	}

	q.previewErr = domain.ErrBudgetNotApproved
	rec = do(router, "GET", "/finance/budgets/b1/issuances/1/preview", nil)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "not approved") {
		t.Fatalf("not approved = %d", rec.Code)
	}
}

func TestIssueCreatedAndAlreadyIssued(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{issueCreated: true}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))

	rec := do(router, "POST", "/finance/budgets/b1/issuances/1", url.Values{})
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/finance/budgets/b1/view?flash=issued" {
		t.Fatalf("issue = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	q.issueCreated = false
	rec = do(router, "POST", "/finance/budgets/b1/issuances/1", url.Values{})
	if rec.Header().Get("Location") != "/finance/budgets/b1/view?flash=already_issued" {
		t.Fatalf("re-issue location = %q", rec.Header().Get("Location"))
	}
	q.issueErr = domain.ErrNoChargeableUnits
	rec = do(router, "POST", "/finance/budgets/b1/issuances/1", url.Values{})
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "ownership share greater than zero") {
		t.Fatalf("no chargeable = %d", rec.Code)
	}
}

func TestIssuanceCancelFlow(t *testing.T) {
	user, sess := syndicSession()
	q := &fakeQuotas{issuances: []services.IssuanceSummary{{
		ID: "i1", BudgetID: "b1", Year: 2026, Periodicity: domain.PeriodicityMonthly, PeriodIndex: 3,
		Status: domain.IssuanceIssued, TotalCents: 1000, ChargeCount: 2,
	}}}
	router := financeRouter(user, sess, quotaDeps(q, model.RoleSyndic))

	rec := do(router, "GET", "/finance/issuances/i1/cancel?budget=b1", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "March 2026") {
		t.Fatalf("cancel page = %d", rec.Code)
	}
	rec = do(router, "POST", "/finance/issuances/i1/cancel", url.Values{"budget_id": {"b1"}})
	if rec.Code != http.StatusSeeOther || q.canceled != "i1" || rec.Header().Get("Location") != "/finance/budgets/b1/view?flash=issuance_canceled" {
		t.Fatalf("cancel = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	q.cancelErr = domain.ErrIssuanceHasSettled
	rec = do(router, "POST", "/finance/issuances/i1/cancel", url.Values{"budget_id": {"b1"}})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "already settled") {
		t.Fatalf("settled = %d", rec.Code)
	}
}

func TestParseRateBP(t *testing.T) {
	for in, want := range map[string]int{"10": 1000, "7,5": 750, "0": 0, "12.25": 1225, "100": 10000} {
		if got, err := parseRateBP(in); err != nil || got != want {
			t.Errorf("parseRateBP(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "101", "1.234", "-1", "+5", ".5"} {
		if _, err := parseRateBP(in); err == nil {
			t.Errorf("parseRateBP(%q) should fail", in)
		}
	}
}
