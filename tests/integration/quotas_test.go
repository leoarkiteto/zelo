package integration

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// AC-3: the suggested reserve rate is 10% for EUR and 0% for BRL, and a second
// budget of the same year is refused.
func TestBudgetCreateDefaultsAndDuplicate(t *testing.T) {
	e := newQuotaEnv(t)

	if _, err := e.db.Exec(`UPDATE condominiums SET currency='EUR' WHERE id=$1`, e.condoA); err != nil {
		t.Fatalf("set EUR: %v", err)
	}
	r := e.get(t, e.syndic, "/finance/budgets/new")
	if r.code != http.StatusOK {
		t.Fatalf("new budget form (EUR) = %d", r.code)
	}
	if v, ok := inputValue(r.body, "reserve_rate"); !ok || v != "10" {
		t.Fatalf("EUR suggested reserve rate = %q (field present %v), want 10", v, ok)
	}
	// Leaving the rate empty applies the currency default.
	id := e.createBudget(t, 2027, "monthly", 5, "")
	var bp int
	if err := e.db.QueryRow(`SELECT reserve_rate_bp FROM quota_budgets WHERE id=$1`, id).Scan(&bp); err != nil || bp != 1000 {
		t.Fatalf("EUR stored reserve_rate_bp = %d (%v), want 1000", bp, err)
	}

	dup := e.post(t, e.syndic, "/finance/budgets", url.Values{
		"year": {"2027"}, "periodicity": {"quarterly"}, "due_day": {"9"}, "reserve_rate": {"5"},
	})
	if dup.code != http.StatusConflict {
		t.Fatalf("second budget of 2027 = %d, want 409", dup.code)
	}
	mustContain(t, "duplicate budget", dup.body, "A budget already exists for this year.")
	if n := e.count(t, `SELECT count(*) FROM quota_budgets WHERE condominium_id=$1 AND year=2027`, e.condoA); n != 1 {
		t.Fatalf("budgets of 2027 = %d, want 1", n)
	}

	// Condominium B keeps the BRL default of 0%.
	r = e.get(t, e.syndicB, "/finance/budgets/new")
	if r.code != http.StatusOK {
		t.Fatalf("new budget form (BRL) = %d", r.code)
	}
	if v, ok := inputValue(r.body, "reserve_rate"); !ok || v != "0" {
		t.Fatalf("BRL suggested reserve rate = %q (field present %v), want 0", v, ok)
	}
	rb := e.post(t, e.syndicB, "/finance/budgets", url.Values{
		"year": {"2027"}, "periodicity": {"monthly"}, "due_day": {"5"}, "reserve_rate": {""},
	})
	if rb.code != http.StatusSeeOther {
		t.Fatalf("BRL create = %d\n%s", rb.code, rb.body)
	}
	if err := e.db.QueryRow(`SELECT reserve_rate_bp FROM quota_budgets WHERE condominium_id=$1`, e.condoB).Scan(&bp); err != nil || bp != 0 {
		t.Fatalf("BRL stored reserve_rate_bp = %d (%v), want 0", bp, err)
	}
}

// AC-4: lines are edited while the budget is a draft, approval needs a line,
// and an approved budget refuses every change.
func TestBudgetApproveAndLock(t *testing.T) {
	e := newQuotaEnv(t)
	id := e.createBudget(t, 2027, "monthly", 8, "10")

	if r := e.approve(t, id); r.code != http.StatusUnprocessableEntity {
		t.Fatalf("approve without lines = %d, want 422", r.code)
	} else {
		mustContain(t, "approve without lines", r.body, "Add at least one expense line first.")
	}
	assertStatus := func(want string) {
		t.Helper()
		var got string
		if err := e.db.QueryRow(`SELECT status FROM quota_budgets WHERE id=$1`, id).Scan(&got); err != nil || got != want {
			t.Fatalf("budget status = %q (%v), want %q", got, err, want)
		}
	}
	assertStatus("draft")

	if r := e.addLine(t, id, "Maintenance", "maintenance", "8000.00"); r.code != http.StatusSeeOther {
		t.Fatalf("add line = %d\n%s", r.code, r.body)
	}
	if r := e.addLine(t, id, "Cleaning", "cleaning", "4000.00"); r.code != http.StatusSeeOther {
		t.Fatalf("add line = %d\n%s", r.code, r.body)
	}
	var lineID string
	if err := e.db.QueryRow(`SELECT id FROM quota_budget_lines WHERE budget_id=$1 ORDER BY position, id LIMIT 1`, id).Scan(&lineID); err != nil {
		t.Fatalf("line id: %v", err)
	}
	// Draft edits work: line and parameters.
	if r := e.post(t, e.syndic, "/finance/budgets/"+id+"/lines/"+lineID+"/edit", url.Values{
		"description": {"Maintenance"}, "category": {"maintenance"}, "amount": {"8500.00"}}); r.code != http.StatusSeeOther {
		t.Fatalf("edit line (draft) = %d\n%s", r.code, r.body)
	}
	if r := e.post(t, e.syndic, "/finance/budgets/"+id+"/edit", url.Values{
		"periodicity": {"monthly"}, "due_day": {"9"}, "reserve_rate": {"10"}}); r.code != http.StatusSeeOther {
		t.Fatalf("edit params (draft) = %d\n%s", r.code, r.body)
	}
	if r := e.get(t, e.syndic, "/finance/budgets/"+id+"/view"); r.code != http.StatusOK {
		t.Fatalf("budget view = %d", r.code)
	} else {
		mustContain(t, "budget view", r.body, "R$ 12,500.00")
	}

	if r := e.approve(t, id); r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=approved") {
		t.Fatalf("approve = %d Location=%q", r.code, r.location())
	}
	assertStatus("approved")

	// Every change is refused and nothing is stored.
	linesBefore := e.count(t, `SELECT count(*) FROM quota_budget_lines WHERE budget_id=$1`, id)
	refused := map[string]resp{
		"add line":    e.addLine(t, id, "Extra", "other", "1.00"),
		"edit line":   e.post(t, e.syndic, "/finance/budgets/"+id+"/lines/"+lineID+"/edit", url.Values{"description": {"X"}, "category": {"maintenance"}, "amount": {"1.00"}}),
		"delete line": e.post(t, e.syndic, "/finance/budgets/"+id+"/lines/"+lineID+"/delete", nil),
		"edit params": e.post(t, e.syndic, "/finance/budgets/"+id+"/edit", url.Values{"periodicity": {"quarterly"}, "due_day": {"20"}, "reserve_rate": {"50"}}),
	}
	for what, r := range refused {
		if r.code != http.StatusConflict {
			t.Errorf("%s on an approved budget = %d, want 409", what, r.code)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM quota_budget_lines WHERE budget_id=$1`, id); n != linesBefore {
		t.Errorf("lines after refused changes = %d, want %d", n, linesBefore)
	}
	var amount int64
	var due, rate int
	if err := e.db.QueryRow(`SELECT amount_cents FROM quota_budget_lines WHERE id=$1`, lineID).Scan(&amount); err != nil || amount != 850000 {
		t.Errorf("line amount = %d (%v), want 850000", amount, err)
	}
	if err := e.db.QueryRow(`SELECT due_day, reserve_rate_bp FROM quota_budgets WHERE id=$1`, id).Scan(&due, &rate); err != nil || due != 9 || rate != 1000 {
		t.Errorf("params = due %d rate %d (%v), want 9 and 1000", due, rate, err)
	}
	if n := e.auditCount(t, "budget_updated", e.syndicAID, ""); n < 1 {
		t.Errorf("budget_updated events = %d, want >= 1", n)
	}
}

// AC-8, AC-9, AC-10 (sequential), AC-21 (issue): preview writes nothing; the
// issuance creates one pending receivable per unit with a positive share, due on
// the 8th of December, and the total matches the period.
func TestIssuancePreviewAndIssue(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")

	pv := e.get(t, e.syndic, "/finance/budgets/"+id+"/issuances/12/preview")
	if pv.code != http.StatusOK {
		t.Fatalf("preview = %d\n%s", pv.code, pv.body)
	}
	due := i18n.FormatDate(i18n.LanguageEN, time.Date(2027, 12, 8, 0, 0, 0, 0, time.UTC))
	mustContain(t, "preview", pv.body,
		"December 2027", "R$ 1,100.00", "R$ 1,000.00", "R$ 100.00", // total, base, reserve
		"R$ 440.00", "R$ 385.00", "R$ 275.00", due)
	if strings.Contains(pv.body, ">B-1<") {
		t.Errorf("preview lists the zero-share unit B-1")
	}
	if n := e.count(t, `SELECT count(*) FROM quota_issuances`); n != 0 {
		t.Fatalf("preview stored %d issuances", n)
	}
	if n := e.count(t, `SELECT count(*) FROM finance_accounts WHERE category='condo_fee' AND account_type='receivable'`); n != 0 {
		t.Fatalf("preview stored %d receivables", n)
	}

	r := e.issue(t, id, 12)
	if r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=issued") {
		t.Fatalf("issue = %d Location=%q\n%s", r.code, r.location(), r.body)
	}
	iss := e.activeIssuanceID(t, id, 12)
	if iss == "" {
		t.Fatal("no active issuance for period 12")
	}
	rows := e.quotaRows(t, iss)
	want := map[string]struct {
		cents, base, reserve int64
		ppm                  int
	}{
		"A": {44000, 40000, 4000, 400000},
		"B": {38500, 35000, 3500, 350000},
		"C": {27500, 25000, 2500, 250000},
	}
	if len(rows) != len(want) {
		t.Fatalf("quota rows = %d, want %d (zero-share unit B-1 must not be charged)", len(rows), len(want))
	}
	var sum int64
	for _, q := range rows {
		w, ok := want[q.Unit]
		if !ok {
			t.Errorf("unexpected quota for unit %s", q.Unit)
			continue
		}
		sum += q.Cents
		if q.Cents != w.cents || q.Base != w.base || q.Reserve != w.reserve || q.SharePPM != w.ppm {
			t.Errorf("unit %s quota = %+v, want %+v", q.Unit, q, w)
		}
		if q.Status != "pending" || q.Due != "2027-12-08" || q.Category != "condo_fee" || q.Type != "receivable" {
			t.Errorf("unit %s receivable = %s/%s/%s/%s, want pending/2027-12-08/condo_fee/receivable", q.Unit, q.Status, q.Due, q.Category, q.Type)
		}
	}
	var total int64
	if err := e.db.QueryRow(`SELECT total_cents FROM quota_issuances WHERE id=$1`, iss).Scan(&total); err != nil {
		t.Fatalf("issuance total: %v", err)
	}
	if sum != total || sum != 110000 {
		t.Errorf("sum of quotas = %d, issuance total = %d, want both 110000", sum, total)
	}

	// The budget page lists the issuance.
	view := e.get(t, e.syndic, "/finance/budgets/"+id+"/view?flash=issued")
	mustContain(t, "budget view", view.body, "Condo fees issued.", "Issued")

	// Sequential re-issue: nothing new, existing issuance shown (FR-11).
	again := e.issue(t, id, 12)
	if again.code != http.StatusSeeOther || !strings.Contains(again.location(), "flash=already_issued") {
		t.Fatalf("re-issue = %d Location=%q, want 303 with flash=already_issued", again.code, again.location())
	}
	if n := e.count(t, `SELECT count(*) FROM quota_issuances WHERE budget_id=$1`, id); n != 1 {
		t.Errorf("issuances after re-issue = %d, want 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM quota_charges`); n != 3 {
		t.Errorf("charges after re-issue = %d, want 3", n)
	}
	pv = e.get(t, e.syndic, "/finance/budgets/"+id+"/issuances/12/preview")
	if pv.code != http.StatusSeeOther || !strings.Contains(pv.location(), "flash=already_issued") {
		t.Errorf("preview of an issued period = %d Location=%q, want 303 already_issued", pv.code, pv.location())
	}
	ex := e.get(t, e.syndic, "/finance/budgets/"+id+"/view?flash=already_issued")
	mustContain(t, "existing issuance", ex.body, "This period was already issued. Showing the existing issuance.")
	if n := e.auditCount(t, "quota_issued", e.syndicAID, ""); n != 1 {
		t.Errorf("quota_issued events = %d, want exactly 1", n)
	}
}

// AC-10 (concurrent): simultaneous issue requests of the same period create at
// most one active issuance and one receivable per unit.
func TestIssuanceConcurrentRequests(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")
	token := e.csrf(t, e.syndic)

	const n = 12
	var wg sync.WaitGroup
	start := make(chan struct{})
	type result struct {
		code int
		out  string
		err  error
	}
	results := make([]result, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			code, out, err := e.postRaw(e.syndic, token, "/finance/budgets/"+id+"/issuances/11", nil)
			results[i] = result{code, out, err}
		}(i)
	}
	close(start)
	wg.Wait()

	issued := 0
	for i, r := range results {
		if r.err != nil {
			t.Errorf("request %d: %v", i, r.err)
			continue
		}
		if r.code != http.StatusSeeOther {
			t.Errorf("request %d = %d, want 303\n%s", i, r.code, r.out)
			continue
		}
		if strings.Contains(r.out, "flash=issued") {
			issued++
		} else if !strings.Contains(r.out, "flash=already_issued") {
			t.Errorf("request %d redirect %q has neither issued nor already_issued", i, strings.SplitN(r.out, "\n", 2)[0])
		}
	}
	if issued != 1 {
		t.Errorf("requests reporting a new issuance = %d, want exactly 1", issued)
	}
	if got := e.count(t, `SELECT count(*) FROM quota_issuances WHERE budget_id=$1 AND period_index=11 AND status='issued'`, id); got != 1 {
		t.Errorf("active issuances = %d, want 1", got)
	}
	if got := e.count(t, `SELECT count(*) FROM quota_issuances WHERE budget_id=$1`, id); got != 1 {
		t.Errorf("issuances = %d, want 1", got)
	}
	if got := e.count(t, `SELECT count(*) FROM quota_charges`); got != 3 {
		t.Errorf("charges = %d, want 3", got)
	}
	if got := e.count(t, `SELECT count(*) FROM finance_accounts WHERE category='condo_fee' AND account_type='receivable'`); got != 3 {
		t.Errorf("receivables = %d, want 3", got)
	}
	if got := e.auditCount(t, "quota_issued", "", ""); got != 1 {
		t.Errorf("quota_issued events = %d, want 1", got)
	}
}

// AC-11: refused issuances. Shares at 98.75%, a draft budget, a period of
// another year and no chargeable unit all refuse with a message and store nothing.
func TestIssuanceRefusals(t *testing.T) {
	e := newQuotaEnv(t)
	aID, _, cID := e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")

	nothingStored := func(what string) {
		t.Helper()
		if n := e.count(t, `SELECT count(*) FROM quota_issuances`); n != 0 {
			t.Errorf("%s stored %d issuances", what, n)
		}
		if n := e.count(t, `SELECT count(*) FROM finance_accounts WHERE category='condo_fee'`); n != 0 {
			t.Errorf("%s stored %d receivables", what, n)
		}
	}

	// 98.75% = 987.5 per mille: C holds 237.5 instead of 250.
	if _, err := e.db.Exec(`UPDATE units SET share_ppm=237500 WHERE id=$1`, cID); err != nil {
		t.Fatalf("set share: %v", err)
	}
	r := e.issue(t, id, 12)
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("issue with shares at 98.75%% = %d, want 422", r.code)
	}
	mustContain(t, "shares refusal", r.body, "The ownership shares do not add up to 100%", "-12.5‰")
	if p := e.get(t, e.syndic, "/finance/budgets/"+id+"/issuances/12/preview"); p.code != http.StatusUnprocessableEntity {
		t.Errorf("preview with shares at 98.75%% = %d, want 422", p.code)
	}
	nothingStored("issue at 98.75%")

	// Shares above 100% are refused the same way (+).
	if _, err := e.db.Exec(`UPDATE units SET share_ppm=262500 WHERE id=$1`, cID); err != nil {
		t.Fatalf("set share: %v", err)
	}
	r = e.issue(t, id, 12)
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("issue with shares at 101.25%% = %d, want 422", r.code)
	}
	mustContain(t, "excess refusal", r.body, "+12.5‰")
	nothingStored("issue at 101.25%")

	// Draft budget (reopened) is refused.
	if _, err := e.db.Exec(`UPDATE units SET share_ppm=250000 WHERE id=$1`, cID); err != nil {
		t.Fatalf("restore share: %v", err)
	}
	if rr := e.post(t, e.syndic, "/finance/budgets/"+id+"/reopen", nil); rr.code != http.StatusSeeOther {
		t.Fatalf("reopen = %d\n%s", rr.code, rr.body)
	}
	r = e.issue(t, id, 12)
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("issue of a draft budget = %d, want 422", r.code)
	}
	mustContain(t, "draft refusal", r.body, "The budget of this year is not approved.")
	nothingStored("issue of a draft budget")
	if rr := e.approve(t, id); rr.code != http.StatusSeeOther {
		t.Fatalf("approve = %d", rr.code)
	}

	// A period outside the budget year (monthly has 12 periods).
	r = e.issue(t, id, 13)
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("issue of period 13 = %d, want 422", r.code)
	}
	mustContain(t, "period refusal", r.body, "This period does not belong to the year of the budget.")
	nothingStored("issue of period 13")

	// No unit with a share above zero.
	if _, err := e.db.Exec(`UPDATE units SET share_ppm=0 WHERE condominium_id=$1`, e.condoA); err != nil {
		t.Fatalf("zero shares: %v", err)
	}
	r = e.issue(t, id, 12)
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("issue without chargeable units = %d, want 422", r.code)
	}
	nothingStored("issue without chargeable units")

	// Back to normal: the same request now succeeds.
	if _, err := e.db.Exec(`UPDATE units SET share_ppm=400000 WHERE id=$1`, aID); err != nil {
		t.Fatalf("restore A: %v", err)
	}
	if _, err := e.db.Exec(`UPDATE units SET share_ppm=350000 WHERE condominium_id=$1 AND code='B'`, e.condoA); err != nil {
		t.Fatalf("restore B: %v", err)
	}
	if _, err := e.db.Exec(`UPDATE units SET share_ppm=250000 WHERE id=$1`, cID); err != nil {
		t.Fatalf("restore C: %v", err)
	}
	if r := e.issue(t, id, 12); r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=issued") {
		t.Fatalf("issue after fixing the shares = %d Location=%q", r.code, r.location())
	}
}

// AC-12: a quarterly budget with due day 15 issues the second quarter due on
// April 15.
func TestIssuanceQuarterlyDueDate(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2027, "quarterly", 15, "10")
	r := e.issue(t, id, 2)
	if r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=issued") {
		t.Fatalf("issue Q2 = %d Location=%q\n%s", r.code, r.location(), r.body)
	}
	iss := e.activeIssuanceID(t, id, 2)
	rows := e.quotaRows(t, iss)
	if len(rows) != 3 {
		t.Fatalf("Q2 quotas = %d, want 3", len(rows))
	}
	var sum int64
	for _, q := range rows {
		sum += q.Cents
		if q.Due != "2027-04-15" {
			t.Errorf("unit %s due %s, want 2027-04-15", q.Unit, q.Due)
		}
	}
	// 12000.00 / 4 = 3000.00 base + 10% reserve = 3300.00 per quarter.
	if sum != 330000 {
		t.Errorf("Q2 total = %d, want 330000", sum)
	}
	if v := e.get(t, e.syndic, "/finance/budgets/"+id+"/view"); !strings.Contains(v.body, "Q2 2027") {
		t.Errorf("budget view does not show period Q2 2027")
	}
}

// AC-13 and AC-21 (cancel): an issuance without settled quotas is canceled
// whole and the period can be issued again; with a settled quota it is refused.
func TestIssuanceCancel(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")
	if r := e.issue(t, id, 12); r.code != http.StatusSeeOther {
		t.Fatalf("issue = %d", r.code)
	}
	first := e.activeIssuanceID(t, id, 12)

	// The confirmation page, then the cancellation itself.
	page := e.get(t, e.syndic, "/finance/issuances/"+first+"/cancel?budget="+id)
	if page.code != http.StatusOK {
		t.Fatalf("cancel page = %d\n%s", page.code, page.body)
	}
	r := e.post(t, e.syndic, "/finance/issuances/"+first+"/cancel", url.Values{"budget_id": {id}})
	if r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=issuance_canceled") {
		t.Fatalf("cancel = %d Location=%q\n%s", r.code, r.location(), r.body)
	}
	var st string
	if err := e.db.QueryRow(`SELECT status FROM quota_issuances WHERE id=$1`, first).Scan(&st); err != nil || st != "canceled" {
		t.Fatalf("issuance status = %q (%v), want canceled", st, err)
	}
	for _, q := range e.quotaRows(t, first) {
		if q.Status != "canceled" {
			t.Errorf("unit %s receivable status %s, want canceled", q.Unit, q.Status)
		}
	}
	list := e.get(t, e.syndic, "/finance/budgets/"+id+"/view")
	mustContain(t, "issuances list", list.body, "Canceled")

	// The same period can be issued again; the canceled quotas stay in history.
	if r := e.issue(t, id, 12); r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=issued") {
		t.Fatalf("re-issue after cancel = %d Location=%q", r.code, r.location())
	}
	second := e.activeIssuanceID(t, id, 12)
	if second == "" || second == first {
		t.Fatalf("new issuance id %q (first %q)", second, first)
	}
	if n := e.count(t, `SELECT count(*) FROM finance_accounts WHERE category='condo_fee' AND status='canceled'`); n != 3 {
		t.Errorf("canceled receivables = %d, want 3 kept in history", n)
	}
	if n := e.count(t, `SELECT count(*) FROM finance_accounts WHERE category='condo_fee' AND status='pending'`); n != 3 {
		t.Errorf("pending receivables = %d, want 3", n)
	}

	// With one settled quota the cancellation is refused and nothing changes.
	acct := e.quotaAccountID(t, second, "A")
	if r := e.post(t, e.syndic, "/finance/"+acct+"/settle", url.Values{"settlement_date": {"2027-12-10"}}); r.code != http.StatusSeeOther {
		t.Fatalf("settle = %d\n%s", r.code, r.body)
	}
	r = e.post(t, e.syndic, "/finance/issuances/"+second+"/cancel", url.Values{"budget_id": {id}})
	if r.code != http.StatusConflict {
		t.Fatalf("cancel with a settled quota = %d, want 409", r.code)
	}
	mustContain(t, "settled refusal", r.body, "already settled")
	if err := e.db.QueryRow(`SELECT status FROM quota_issuances WHERE id=$1`, second).Scan(&st); err != nil || st != "issued" {
		t.Fatalf("issuance status after refusal = %q (%v), want issued", st, err)
	}
	pending := 0
	for _, q := range e.quotaRows(t, second) {
		if q.Status == "pending" {
			pending++
		}
	}
	if pending != 2 {
		t.Errorf("pending quotas after refused cancel = %d, want 2", pending)
	}
	if n := e.auditCount(t, "quota_issuance_canceled", e.syndicAID, ""); n != 1 {
		t.Errorf("quota_issuance_canceled events = %d, want 1", n)
	}
}

// AC-5, AC-21 (reopen): a budget with an active issuance cannot be reopened;
// after canceling the issuance it can.
func TestBudgetReopenRules(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")
	if r := e.issue(t, id, 12); r.code != http.StatusSeeOther {
		t.Fatalf("issue = %d", r.code)
	}
	iss := e.activeIssuanceID(t, id, 12)

	status := func() string {
		var s string
		if err := e.db.QueryRow(`SELECT status FROM quota_budgets WHERE id=$1`, id).Scan(&s); err != nil {
			t.Fatalf("budget status: %v", err)
		}
		return s
	}
	r := e.post(t, e.syndic, "/finance/budgets/"+id+"/reopen", nil)
	if r.code != http.StatusConflict {
		t.Fatalf("reopen with an active issuance = %d, want 409", r.code)
	}
	mustContain(t, "reopen refusal", r.body, "The budget has active issuances. Cancel them first.")
	if status() != "approved" {
		t.Fatalf("budget status = %s after refused reopen, want approved", status())
	}

	if c := e.post(t, e.syndic, "/finance/issuances/"+iss+"/cancel", url.Values{"budget_id": {id}}); c.code != http.StatusSeeOther {
		t.Fatalf("cancel = %d", c.code)
	}
	r = e.post(t, e.syndic, "/finance/budgets/"+id+"/reopen", nil)
	if r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=reopened") {
		t.Fatalf("reopen after cancel = %d Location=%q\n%s", r.code, r.location(), r.body)
	}
	if status() != "draft" {
		t.Fatalf("budget status = %s, want draft", status())
	}
	if n := e.auditCount(t, "budget_reopened", e.syndicAID, ""); n != 1 {
		t.Errorf("budget_reopened events = %d, want 1", n)
	}
}

// AC-21: every audited action of FR-23 leaves an event of its type.
func TestQuotaAuditEvents(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	if r := e.post(t, e.syndic, "/condominium/units", url.Values{"code": {"Z"}, "share": {"0"}}); r.code != http.StatusSeeOther {
		t.Fatalf("create unit = %d", r.code)
	}
	var zID string
	if err := e.db.QueryRow(`SELECT id FROM units WHERE code='Z'`).Scan(&zID); err != nil {
		t.Fatalf("unit Z: %v", err)
	}
	if r := e.post(t, e.syndic, "/condominium/units/"+zID+"/edit", url.Values{"code": {"Z"}, "share": {"0.5"}}); r.code != http.StatusSeeOther {
		t.Fatalf("edit unit = %d", r.code)
	}
	// Unit Z now has 0.5 per mille: restore 100% by shrinking C.
	if _, err := e.db.Exec(`UPDATE units SET share_ppm=249500 WHERE condominium_id=$1 AND code='C'`, e.condoA); err != nil {
		t.Fatalf("shrink C: %v", err)
	}
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")
	if r := e.issue(t, id, 3); r.code != http.StatusSeeOther {
		t.Fatalf("issue = %d\n%s", r.code, r.body)
	}
	iss := e.activeIssuanceID(t, id, 3)
	if r := e.post(t, e.syndic, "/finance/issuances/"+iss+"/cancel", url.Values{"budget_id": {id}}); r.code != http.StatusSeeOther {
		t.Fatalf("cancel = %d", r.code)
	}
	if r := e.post(t, e.syndic, "/finance/budgets/"+id+"/reopen", nil); r.code != http.StatusSeeOther {
		t.Fatalf("reopen = %d", r.code)
	}
	for _, ev := range []string{"unit_created", "unit_updated", "budget_created", "budget_approved",
		"quota_issued", "quota_issuance_canceled", "budget_reopened"} {
		if n := e.auditCount(t, ev, e.syndicAID, ""); n != 1 {
			t.Errorf("%s events = %d, want 1", ev, n)
		}
	}
}

// AC-14 and the list part of AC-25: the generic list hides edit and cancel for a
// quota and refuses direct requests; settlement works and then the Recibo link
// appears, only on settled receivables.
func TestQuotaReceivableGenericScreensBlocked(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")
	if r := e.issue(t, id, 12); r.code != http.StatusSeeOther {
		t.Fatalf("issue = %d", r.code)
	}
	iss := e.activeIssuanceID(t, id, 12)
	q := e.quotaAccountID(t, iss, "A")
	plainPending := e.insertReceivable(t, "", "ZZ-PLAIN-PENDING", 5000, "2027-05-01", "pending", "")
	plainSettled := e.insertReceivable(t, "", "ZZ-PLAIN-SETTLED", 5000, "2027-05-01", "settled", "2027-05-02")
	var payable string
	if err := e.db.QueryRow(`
		INSERT INTO finance_accounts (condominium_id, account_type, title, category, amount_cents, due_date, status, settlement_date, supplier_payee, created_by)
		VALUES ($1, 'payable', 'ZZ-PAYABLE', 'maintenance', 7000, '2027-05-01', 'settled', '2027-05-02', 'Acme', $2) RETURNING id`,
		e.condoA, e.syndicAID).Scan(&payable); err != nil {
		t.Fatalf("insert payable: %v", err)
	}

	link := func(id, action string) string { return `href="/finance/` + id + `/` + action + `"` }
	list := e.get(t, e.syndic, "/finance")
	if list.code != http.StatusOK {
		t.Fatalf("list = %d", list.code)
	}
	mustContain(t, "quota row", list.body, link(q, "settle"))
	mustNotContain(t, "quota row", list.body, link(q, "edit"), link(q, "cancel"), link(q, "recibo"))
	mustContain(t, "plain pending row", list.body, link(plainPending, "settle"), link(plainPending, "edit"), link(plainPending, "cancel"))
	mustNotContain(t, "plain pending row", list.body, link(plainPending, "recibo"))
	mustContain(t, "plain settled row", list.body, link(plainSettled, "recibo"))
	mustNotContain(t, "payable row", list.body, link(payable, "recibo"))

	// Direct requests are refused with 409 and change nothing.
	for _, c := range []struct {
		name string
		r    resp
	}{
		{"GET edit", e.get(t, e.syndic, "/finance/"+q+"/edit")},
		{"POST edit", e.post(t, e.syndic, "/finance/"+q+"/edit", url.Values{
			"type": {"receivable"}, "title": {"hacked"}, "category": {"condo_fee"}, "amount": {"1.00"},
			"due_date": {"2027-12-09"}, "status": {"pending"}})},
		{"GET cancel", e.get(t, e.syndic, "/finance/"+q+"/cancel")},
		{"POST cancel", e.post(t, e.syndic, "/finance/"+q+"/cancel", nil)},
	} {
		if c.r.code != http.StatusConflict {
			t.Errorf("%s of a quota = %d, want 409", c.name, c.r.code)
		}
	}
	var title, status string
	var cents int64
	var dueDate string
	if err := e.db.QueryRow(`SELECT title, status, amount_cents, to_char(due_date,'YYYY-MM-DD') FROM finance_accounts WHERE id=$1`, q).Scan(&title, &status, &cents, &dueDate); err != nil {
		t.Fatalf("read quota: %v", err)
	}
	if title != "Quota 2027-12 A" || status != "pending" || cents != 44000 || dueDate != "2027-12-08" {
		t.Errorf("quota after refused requests = %q/%s/%d/%s, want unchanged", title, status, cents, dueDate)
	}

	// The plain receivable still edits and cancels through the generic screens.
	if r := e.get(t, e.syndic, "/finance/"+plainPending+"/edit"); r.code != http.StatusOK {
		t.Errorf("GET edit of a plain receivable = %d, want 200", r.code)
	}

	// Settlement works for a quota and the Recibo link appears afterwards.
	r := e.post(t, e.syndic, "/finance/"+q+"/settle", url.Values{"settlement_date": {"2027-12-10"}})
	if r.code != http.StatusSeeOther {
		t.Fatalf("settle quota = %d\n%s", r.code, r.body)
	}
	if err := e.db.QueryRow(`SELECT status FROM finance_accounts WHERE id=$1`, q).Scan(&status); err != nil || status != "settled" {
		t.Fatalf("quota status = %q (%v), want settled", status, err)
	}
	list = e.get(t, e.syndic, "/finance")
	mustContain(t, "settled quota row", list.body, link(q, "recibo"))
	mustNotContain(t, "settled quota row", list.body, link(q, "edit"), link(q, "cancel"))
	// The attached proof of payables still lives at /receipt, syndic only.
	if r := e.get(t, e.owner, "/finance/"+payable+"/receipt"); r.code != http.StatusForbidden {
		t.Errorf("owner GET payable /receipt = %d, want 403", r.code)
	}
}

// AC-15: changing a share after an issuance keeps the issued quota and the
// frozen share of the detail.
func TestIssuedQuotaKeepsFrozenShare(t *testing.T) {
	e := newQuotaEnv(t)
	aID, _, _ := e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")
	if r := e.issue(t, id, 12); r.code != http.StatusSeeOther {
		t.Fatalf("issue = %d", r.code)
	}
	iss := e.activeIssuanceID(t, id, 12)

	if r := e.post(t, e.syndic, "/condominium/units/"+aID+"/edit", url.Values{"code": {"A"}, "share": {"500"}}); r.code != http.StatusSeeOther {
		t.Fatalf("edit share = %d\n%s", r.code, r.body)
	}
	var ppm int
	if err := e.db.QueryRow(`SELECT share_ppm FROM units WHERE id=$1`, aID).Scan(&ppm); err != nil || ppm != 500000 {
		t.Fatalf("unit A share = %d (%v), want 500000", ppm, err)
	}
	for _, q := range e.quotaRows(t, iss) {
		if q.Unit != "A" {
			continue
		}
		if q.Cents != 44000 || q.SharePPM != 400000 || q.Base != 40000 || q.Reserve != 4000 {
			t.Errorf("quota of A after share change = %+v, want 44000 with frozen share 400000", q)
		}
	}
	// Existing issuances stay; the next one is refused until the sum is 100% again.
	if r := e.issue(t, id, 11); r.code != http.StatusUnprocessableEntity {
		t.Errorf("issue with shares at 115%% = %d, want 422", r.code)
	}
	if r := e.issue(t, id, 12); r.code != http.StatusSeeOther || !strings.Contains(r.location(), "already_issued") {
		t.Errorf("re-issue of the issued period after a share change = %d Location=%q, want already_issued", r.code, r.location())
	}
}

// Fixtures of the Conta corrente tests: tenant "tenant-inv" occupies T1 since
// 2026-10-01 with a pending receivable of August and settled ones of September
// and November; unit "OTH" belongs to nobody the viewers know.
type statementFixture struct {
	t1, oth                   string
	aug, sep, nov, othSettled string
}

func (e *quotaEnv) statementFixture(t *testing.T) statementFixture {
	t.Helper()
	f := statementFixture{t1: e.addUnit(t, "T1", 0), oth: e.addUnit(t, "OTH", 0)}
	e.occupy(t, "tenant-inv@example.com", f.t1, "tenant", "2026-10-01")
	f.aug = e.insertReceivable(t, f.t1, "ZZ-AUG-PENDING", 10000, "2026-08-10", "pending", "")
	f.sep = e.insertReceivable(t, f.t1, "ZZ-SEP-SETTLED", 10000, "2026-09-10", "settled", "2026-09-15")
	f.nov = e.insertReceivable(t, f.t1, "ZZ-NOV-SETTLED", 10000, "2026-11-05", "settled", "2026-11-10")
	f.othSettled = e.insertReceivable(t, f.oth, "ZZ-OTH-SETTLED", 10000, "2026-11-05", "settled", "2026-11-10")
	return f
}

// AC-16: the tenant sees the pending of August and the settled of November, not
// the one of September (before the occupancy); the syndic sees all three.
func TestStatementVisibilityByOccupancy(t *testing.T) {
	e := newQuotaEnv(t)
	f := e.statementFixture(t)

	r := e.get(t, e.tenant, "/finance/my-charges")
	if r.code != http.StatusOK {
		t.Fatalf("tenant statement = %d\n%s", r.code, r.body)
	}
	mustContain(t, "tenant statement", r.body, "ZZ-AUG-PENDING", "ZZ-NOV-SETTLED", `href="/finance/`+f.nov+`/recibo"`)
	mustNotContain(t, "tenant statement", r.body, "ZZ-SEP-SETTLED", f.sep, "ZZ-OTH-SETTLED")

	s := e.get(t, e.syndic, "/finance/my-charges?unit="+f.t1)
	if s.code != http.StatusOK {
		t.Fatalf("syndic statement = %d", s.code)
	}
	mustContain(t, "syndic statement", s.body, "ZZ-AUG-PENDING", "ZZ-SEP-SETTLED", "ZZ-NOV-SETTLED")
}

// A settlement on the same day the occupancy started is visible (edge case).
func TestStatementSameDayAsOccupancyStartIsVisible(t *testing.T) {
	e := newQuotaEnv(t)
	f := e.statementFixture(t)
	same := e.insertReceivable(t, f.t1, "ZZ-SAME-DAY", 10000, "2026-09-30", "settled", "2026-10-01")
	r := e.get(t, e.tenant, "/finance/my-charges")
	mustContain(t, "same-day settlement", r.body, "ZZ-SAME-DAY")
	if c := e.get(t, e.tenant, "/finance/"+same+"/recibo"); c.code != http.StatusOK {
		t.Errorf("recibo of a same-day settlement = %d, want 200", c.code)
	}
}

// AC-17: an owner of several units sees the first by code and a selector, and
// choosing another unit shows its accounts.
func TestStatementUnitSelector(t *testing.T) {
	e := newQuotaEnv(t)
	aID := e.addUnit(t, "A", 0)
	gID := e.addUnit(t, "G1", 0)
	e.occupy(t, "resident@example.com", aID, "owner", "2020-01-01")
	e.occupy(t, "resident@example.com", gID, "owner", "2020-01-01")
	e.insertReceivable(t, aID, "ZZ-UNIT-A-PENDING", 10000, "2027-02-10", "pending", "")
	e.insertReceivable(t, gID, "ZZ-UNIT-G1-PENDING", 20000, "2027-02-10", "pending", "")

	r := e.get(t, e.owner, "/finance/my-charges")
	if r.code != http.StatusOK {
		t.Fatalf("statement = %d", r.code)
	}
	mustContain(t, "default unit", r.body, "ZZ-UNIT-A-PENDING", `id="statement-unit"`, `<option value="`+gID+`"`)
	mustNotContain(t, "default unit", r.body, "ZZ-UNIT-G1-PENDING")

	g := e.get(t, e.owner, "/finance/my-charges?unit="+gID)
	if g.code != http.StatusOK {
		t.Fatalf("statement of G1 = %d", g.code)
	}
	mustContain(t, "G1 unit", g.body, "ZZ-UNIT-G1-PENDING")
	mustNotContain(t, "G1 unit", g.body, "ZZ-UNIT-A-PENDING")
}

// AC-18 and AC-21 (access denied): another unit's statement or receipt, and a
// receipt of a settlement before the occupancy, return 403 and are audited.
func TestStatementAndReceiptAccessDenied(t *testing.T) {
	e := newQuotaEnv(t)
	f := e.statementFixture(t)
	tenantID := e.userID(t, "tenant-inv@example.com")

	denied := func(resource string) int { return e.auditCount(t, "access_denied", tenantID, resource) }

	if r := e.get(t, e.tenant, "/finance/my-charges?unit="+f.oth); r.code != http.StatusForbidden {
		t.Fatalf("statement of another unit = %d, want 403", r.code)
	}
	if n := denied("statement"); n != 1 {
		t.Errorf("statement access_denied events = %d, want 1", n)
	}
	if r := e.get(t, e.tenant, "/finance/"+f.othSettled+"/recibo"); r.code != http.StatusForbidden {
		t.Fatalf("recibo of another unit = %d, want 403", r.code)
	}
	if r := e.get(t, e.tenant, "/finance/"+f.sep+"/recibo"); r.code != http.StatusForbidden {
		t.Fatalf("recibo of a settlement before the occupancy = %d, want 403", r.code)
	}
	if n := denied("receipt"); n != 2 {
		t.Errorf("receipt access_denied events = %d, want 2", n)
	}
	if r := e.get(t, e.tenant, "/finance/"+f.nov+"/recibo"); r.code != http.StatusOK {
		t.Errorf("recibo of a settlement after the occupancy = %d, want 200", r.code)
	}
	// An ended occupancy loses access to the unit.
	if _, err := e.db.Exec(`UPDATE unit_occupancies SET ended_at = now() WHERE unit_id=$1`, f.t1); err != nil {
		t.Fatalf("end occupancy: %v", err)
	}
	if r := e.get(t, e.tenant, "/finance/"+f.nov+"/recibo"); r.code != http.StatusForbidden {
		t.Errorf("recibo after the occupancy ended = %d, want 403", r.code)
	}
	if r := e.get(t, e.tenant, "/finance/my-charges"); r.code != http.StatusOK || !strings.Contains(r.body, "You need an active unit") {
		t.Errorf("statement without an active unit = %d, want the no-unit message", r.code)
	}
}

var receiptNumberRe = regexp.MustCompile(`font-mono font-semibold text-ink-900">([^<]+)</span>`)

// AC-19 and AC-25: the Recibo of a settled quota shows the condominium, unit,
// period, base, reserve, amount, date and number; pending and canceled receivables
// have no Recibo (404); a settled receivable that is not a quota has none of the
// period data.
func TestReceiptContentAndNotFound(t *testing.T) {
	e := newQuotaEnv(t)
	aID, _, _ := e.addStdUnits(t)
	e.occupy(t, "resident@example.com", aID, "owner", "2020-01-01")
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")
	if r := e.issue(t, id, 12); r.code != http.StatusSeeOther {
		t.Fatalf("issue 12 = %d", r.code)
	}
	dec := e.activeIssuanceID(t, id, 12)
	qA := e.quotaAccountID(t, dec, "A")
	qB := e.quotaAccountID(t, dec, "B")

	// Pending: 404 for the syndic and for the occupant of the unit.
	if r := e.get(t, e.syndic, "/finance/"+qA+"/recibo"); r.code != http.StatusNotFound {
		t.Errorf("syndic recibo of a pending quota = %d, want 404", r.code)
	}
	if r := e.get(t, e.owner, "/finance/"+qA+"/recibo"); r.code != http.StatusNotFound {
		t.Errorf("owner recibo of a pending quota of their unit = %d, want 404", r.code)
	}

	if r := e.post(t, e.syndic, "/finance/"+qA+"/settle", url.Values{"settlement_date": {"2027-12-10"}}); r.code != http.StatusSeeOther {
		t.Fatalf("settle = %d\n%s", r.code, r.body)
	}
	r := e.get(t, e.owner, "/finance/"+qA+"/recibo")
	if r.code != http.StatusOK {
		t.Fatalf("owner recibo = %d\n%s", r.code, r.body)
	}
	number := strings.ToUpper(strings.ReplaceAll(qA, "-", ""))[:8]
	mustContain(t, "recibo", r.body,
		"IT Condo", `text-ink-900">A</dd>`, "December 2027", "Condo fee December 2027 — Unit A", "R$ 400.00 / R$ 40.00", "R$ 440.00",
		"December 10, 2027", number)
	if m := receiptNumberRe.FindStringSubmatch(r.body); m == nil || m[1] != number {
		t.Errorf("recibo number = %v, want %s", m, number)
	}
	mustContain(t, "recibo print styles", r.body, "receipt-sheet")
	if p := e.get(t, e.syndic, "/finance/"+qA+"/recibo"); p.code != http.StatusOK {
		t.Errorf("syndic recibo of any settled receivable = %d, want 200", p.code)
	}
	// The owner of A cannot open the settled receipt of unit B.
	if _, err := e.db.Exec(`UPDATE finance_accounts SET status='settled', settlement_date='2027-12-11' WHERE id=$1`, qB); err != nil {
		t.Fatalf("settle B: %v", err)
	}
	if r := e.get(t, e.owner, "/finance/"+qB+"/recibo"); r.code != http.StatusForbidden {
		t.Errorf("owner recibo of unit B = %d, want 403", r.code)
	}

	// Canceled quotas (a canceled issuance) have no Recibo.
	if r := e.issue(t, id, 11); r.code != http.StatusSeeOther {
		t.Fatalf("issue 11 = %d", r.code)
	}
	nov := e.activeIssuanceID(t, id, 11)
	canceledQ := e.quotaAccountID(t, nov, "B")
	if r := e.post(t, e.syndic, "/finance/issuances/"+nov+"/cancel", url.Values{"budget_id": {id}}); r.code != http.StatusSeeOther {
		t.Fatalf("cancel issuance 11 = %d", r.code)
	}
	if r := e.get(t, e.syndic, "/finance/"+canceledQ+"/recibo"); r.code != http.StatusNotFound {
		t.Errorf("syndic recibo of a canceled quota = %d, want 404", r.code)
	}
	if r := e.get(t, e.syndic, "/finance/00000000-0000-0000-0000-000000000000/recibo"); r.code != http.StatusNotFound {
		t.Errorf("recibo of an unknown id = %d, want 404", r.code)
	}

	// A settled receivable that is not a quota: shown without period or split.
	plain := e.insertReceivable(t, aID, "ZZ-PLAIN-FEE", 12345, "2027-03-01", "settled", "2027-03-02")
	pr := e.get(t, e.syndic, "/finance/"+plain+"/recibo")
	if pr.code != http.StatusOK {
		t.Fatalf("recibo of a plain receivable = %d\n%s", pr.code, pr.body)
	}
	mustContain(t, "plain recibo", pr.body, "ZZ-PLAIN-FEE", "R$ 123.45", "March 2, 2027")
	mustNotContain(t, "plain recibo", pr.body, "Period</dt>")
	// Without a unit only the syndic may open it.
	noUnit := e.insertReceivable(t, "", "ZZ-NO-UNIT", 100, "2027-03-01", "settled", "2027-03-02")
	if r := e.get(t, e.owner, "/finance/"+noUnit+"/recibo"); r.code != http.StatusForbidden {
		t.Errorf("owner recibo of a receivable without unit = %d, want 403", r.code)
	}
	if r := e.get(t, e.syndic, "/finance/"+noUnit+"/recibo"); r.code != http.StatusOK {
		t.Errorf("syndic recibo of a receivable without unit = %d, want 200", r.code)
	}
	pend := e.insertReceivable(t, aID, "ZZ-PLAIN-PENDING", 100, "2027-03-01", "pending", "")
	if r := e.get(t, e.syndic, "/finance/"+pend+"/recibo"); r.code != http.StatusNotFound {
		t.Errorf("recibo of a pending plain receivable = %d, want 404", r.code)
	}
}

// AC-23: owners and tenants receive 403 on budget and issuance screens and
// actions, and nothing changes.
func TestBudgetsForbiddenForNonSyndic(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2027, "monthly", 8, "10")
	if r := e.issue(t, id, 12); r.code != http.StatusSeeOther {
		t.Fatalf("issue = %d", r.code)
	}
	iss := e.activeIssuanceID(t, id, 12)
	var lineID string
	if err := e.db.QueryRow(`SELECT id FROM quota_budget_lines WHERE budget_id=$1 LIMIT 1`, id).Scan(&lineID); err != nil {
		t.Fatalf("line: %v", err)
	}

	gets := []string{
		"/finance/budgets", "/finance/budgets/new", "/finance/budgets/" + id + "/view",
		"/finance/budgets/" + id + "/issuances/11/preview", "/finance/issuances/" + iss + "/cancel?budget=" + id,
	}
	posts := map[string]url.Values{
		"/finance/budgets":                                        {"year": {"2028"}, "periodicity": {"monthly"}, "due_day": {"5"}, "reserve_rate": {"0"}},
		"/finance/budgets/" + id + "/edit":                        {"periodicity": {"monthly"}, "due_day": {"1"}, "reserve_rate": {"0"}},
		"/finance/budgets/" + id + "/lines":                       {"description": {"X"}, "category": {"other"}, "amount": {"1.00"}},
		"/finance/budgets/" + id + "/lines/" + lineID + "/edit":   {"description": {"X"}, "category": {"other"}, "amount": {"1.00"}},
		"/finance/budgets/" + id + "/lines/" + lineID + "/delete": nil,
		"/finance/budgets/" + id + "/approve":                     nil,
		"/finance/budgets/" + id + "/reopen":                      nil,
		"/finance/budgets/" + id + "/issuances/11":                nil,
		"/finance/issuances/" + iss + "/cancel":                   {"budget_id": {id}},
	}
	for name, c := range map[string]*http.Client{"owner": e.owner, "tenant": e.tenant} {
		for _, p := range gets {
			if r := e.get(t, c, p); r.code != http.StatusForbidden {
				t.Errorf("%s GET %s = %d, want 403", name, p, r.code)
			}
		}
		for p, v := range posts {
			cp := url.Values{}
			for k, vv := range v {
				cp[k] = vv
			}
			if r := e.post(t, c, p, cp); r.code != http.StatusForbidden {
				t.Errorf("%s POST %s = %d, want 403", name, p, r.code)
			}
		}
	}
	if n := e.count(t, `SELECT count(*) FROM quota_budgets WHERE condominium_id=$1`, e.condoA); n != 1 {
		t.Errorf("budgets = %d, want 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM quota_issuances`); n != 1 {
		t.Errorf("issuances = %d, want 1", n)
	}
	var st string
	if err := e.db.QueryRow(`SELECT status FROM quota_issuances WHERE id=$1`, iss).Scan(&st); err != nil || st != "issued" {
		t.Errorf("issuance status = %q (%v), want issued", st, err)
	}
	if err := e.db.QueryRow(`SELECT status FROM quota_budgets WHERE id=$1`, id).Scan(&st); err != nil || st != "approved" {
		t.Errorf("budget status = %q (%v), want approved", st, err)
	}
}

// --- races -----------------------------------------------------------------

const raceRounds = 10

// raceBudget creates an approved monthly budget of the given year with period 1
// free for issuing.
func (e *quotaEnv) raceBudget(t *testing.T, year int) string {
	t.Helper()
	return e.newApprovedBudget(t, year, "monthly", 8, "10")
}

// runConcurrently runs the functions at the same time and waits for all of them.
func runConcurrently(fns ...func()) {
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, fn := range fns {
		wg.Add(1)
		go func(fn func()) {
			defer wg.Done()
			<-start
			fn()
		}(fn)
	}
	close(start)
	wg.Wait()
}

// Concurrent Issue and Reopen never leave a draft budget with an active issuance.
func TestRaceIssueVersusReopen(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	token := e.csrf(t, e.syndic)
	for i := 0; i < raceRounds; i++ {
		id := e.raceBudget(t, 2030+i)
		var issueCode, reopenCode int
		var issueErr, reopenErr error
		runConcurrently(
			func() {
				issueCode, _, issueErr = e.postRaw(e.syndic, token, "/finance/budgets/"+id+"/issuances/1", nil)
			},
			func() {
				reopenCode, _, reopenErr = e.postRaw(e.syndic, token, "/finance/budgets/"+id+"/reopen", nil)
			},
		)
		if issueErr != nil || reopenErr != nil {
			t.Fatalf("round %d: %v / %v", i, issueErr, reopenErr)
		}
		var status string
		if err := e.db.QueryRow(`SELECT status FROM quota_budgets WHERE id=$1`, id).Scan(&status); err != nil {
			t.Fatalf("round %d: budget status: %v", i, err)
		}
		active := e.count(t, `SELECT count(*) FROM quota_issuances WHERE budget_id=$1 AND status='issued'`, id)
		if status == "draft" && active > 0 {
			t.Errorf("round %d: draft budget with %d active issuance(s) (issue=%d reopen=%d)", i, active, issueCode, reopenCode)
		}
		// Reopen and issue exclude each other: exactly one of them succeeded.
		okIssue := issueCode == http.StatusSeeOther
		okReopen := reopenCode == http.StatusSeeOther
		if okIssue == okReopen {
			t.Errorf("round %d: issue=%d reopen=%d, want exactly one to succeed (status %s, active %d)", i, issueCode, reopenCode, status, active)
		}
		if okIssue && active != 1 || !okIssue && active != 0 {
			t.Errorf("round %d: issue=%d but %d active issuance(s)", i, issueCode, active)
		}
	}
}

// consistentIssuance counts inconsistencies of one issuance: a canceled issuance
// with a settled quota or with a quota that is not canceled, and an active
// issuance with a canceled quota.
func (e *quotaEnv) inconsistencies(t *testing.T, issuanceID string) int {
	t.Helper()
	return e.count(t, `
		SELECT count(*) FROM quota_issuances i
		JOIN quota_charges c ON c.issuance_id = i.id
		JOIN finance_accounts a ON a.id = c.finance_account_id
		WHERE i.id = $1 AND (
			(i.status = 'canceled' AND a.status <> 'canceled') OR
			(i.status = 'issued' AND a.status = 'canceled'))`, issuanceID)
}

// Concurrent Cancel and Settle never leave a canceled issuance with a settled
// quota.
func TestRaceCancelVersusSettle(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	token := e.csrf(t, e.syndic)
	for i := 0; i < raceRounds; i++ {
		id := e.raceBudget(t, 2030+i)
		if r := e.issue(t, id, 1); r.code != http.StatusSeeOther {
			t.Fatalf("round %d: issue = %d", i, r.code)
		}
		iss := e.activeIssuanceID(t, id, 1)
		acct := e.quotaAccountID(t, iss, "A")
		var cancelCode, settleCode int
		var cancelErr, settleErr error
		runConcurrently(
			func() {
				cancelCode, _, cancelErr = e.postRaw(e.syndic, token, "/finance/issuances/"+iss+"/cancel", url.Values{"budget_id": {id}})
			},
			func() {
				settleCode, _, settleErr = e.postRaw(e.syndic, token, "/finance/"+acct+"/settle", url.Values{"settlement_date": {"2027-01-20"}})
			},
		)
		if cancelErr != nil || settleErr != nil {
			t.Fatalf("round %d: %v / %v", i, cancelErr, settleErr)
		}
		if n := e.inconsistencies(t, iss); n != 0 {
			t.Errorf("round %d: %d inconsistent quota(s) (cancel=%d settle=%d)", i, n, cancelCode, settleCode)
		}
		var st string
		if err := e.db.QueryRow(`SELECT status FROM quota_issuances WHERE id=$1`, iss).Scan(&st); err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		if st == "canceled" {
			if n := e.count(t, `SELECT count(*) FROM quota_charges c JOIN finance_accounts a ON a.id=c.finance_account_id
				WHERE c.issuance_id=$1 AND a.status='settled'`, iss); n != 0 {
				t.Errorf("round %d: canceled issuance with %d settled quota(s)", i, n)
			}
		}
	}
}

// Concurrent Reopen and Cancel end consistent: the issuance is canceled and the
// budget is either approved (reopen refused) or draft (cancel won first).
func TestRaceReopenVersusCancel(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	token := e.csrf(t, e.syndic)
	for i := 0; i < raceRounds; i++ {
		id := e.raceBudget(t, 2030+i)
		if r := e.issue(t, id, 1); r.code != http.StatusSeeOther {
			t.Fatalf("round %d: issue = %d", i, r.code)
		}
		iss := e.activeIssuanceID(t, id, 1)
		var cancelCode, reopenCode int
		var cancelErr, reopenErr error
		runConcurrently(
			func() {
				cancelCode, _, cancelErr = e.postRaw(e.syndic, token, "/finance/issuances/"+iss+"/cancel", url.Values{"budget_id": {id}})
			},
			func() {
				reopenCode, _, reopenErr = e.postRaw(e.syndic, token, "/finance/budgets/"+id+"/reopen", nil)
			},
		)
		if cancelErr != nil || reopenErr != nil {
			t.Fatalf("round %d: %v / %v", i, cancelErr, reopenErr)
		}
		var bStatus, iStatus string
		if err := e.db.QueryRow(`SELECT status FROM quota_budgets WHERE id=$1`, id).Scan(&bStatus); err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		if err := e.db.QueryRow(`SELECT status FROM quota_issuances WHERE id=$1`, iss).Scan(&iStatus); err != nil {
			t.Fatalf("round %d: %v", i, err)
		}
		active := e.count(t, `SELECT count(*) FROM quota_issuances WHERE budget_id=$1 AND status='issued'`, id)
		if bStatus == "draft" && active > 0 {
			t.Errorf("round %d: draft budget with an active issuance (cancel=%d reopen=%d)", i, cancelCode, reopenCode)
		}
		if iStatus != "canceled" || cancelCode != http.StatusSeeOther {
			t.Errorf("round %d: issuance %s, cancel=%d, want canceled with 303", i, iStatus, cancelCode)
		}
		// Reopen succeeded only when the cancellation was already committed.
		if (reopenCode == http.StatusSeeOther) != (bStatus == "draft") {
			t.Errorf("round %d: reopen=%d but budget is %s", i, reopenCode, bStatus)
		}
		if n := e.inconsistencies(t, iss); n != 0 {
			t.Errorf("round %d: %d inconsistent quota(s)", i, n)
		}
	}
}
