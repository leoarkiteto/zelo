package integration

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// Phase 2 of the condominium quotas: late fees (mora), debtors, extraordinary
// quotas and the reserve fund balance. Fixtures use direct inserts only where
// the UI path is not under test (backdating due dates, second occupants).

var (
	tagRe        = regexp.MustCompile(`<[^>]*>`)
	cellRe       = regexp.MustCompile(`(?s)<td[^>]*>(.*?)</td>`)
	extraLocRe   = regexp.MustCompile(`^/finance/extraordinaries/([0-9a-f-]{36})/view`)
	breakdownTag = `data-late-fee="breakdown"`
)

// plain strips the tags of an HTML fragment and collapses whitespace.
func plain(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tagRe.ReplaceAllString(s, " "))), " ")
}

// tableRows returns the text of the cells of every body row of the page.
func cellRows(body string) [][]string {
	var out [][]string
	for _, chunk := range strings.Split(body, "<tr>")[1:] {
		if end := strings.Index(chunk, "</tr>"); end >= 0 {
			chunk = chunk[:end]
		}
		var cells []string
		for _, m := range cellRe.FindAllStringSubmatch(chunk, -1) {
			cells = append(cells, plain(m[1]))
		}
		if len(cells) > 0 {
			out = append(out, cells)
		}
	}
	return out
}

// rowStarting returns the row whose first cell equals first (nil if none) and its index.
func rowStarting(rows [][]string, first string) ([]string, int) {
	for i, r := range rows {
		if r[0] == first {
			return r, i
		}
	}
	return nil, -1
}

// ddAfter returns the text of the <dd> that follows the <dt> with the label.
func ddAfter(body, label string) (string, bool) {
	re := regexp.MustCompile(`(?s)<dt[^>]*>` + regexp.QuoteMeta(label) + `</dt>\s*<dd[^>]*>(.*?)</dd>`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		return "", false
	}
	return plain(m[1]), true
}

func enT(key string) string { return i18n.T(i18n.LanguageEN, i18n.MessageKey(key)) }

// lateFx is one issued ordinary period whose quotas are long overdue: budget
// 2026, monthly, due on the 25th, 10% reserve, period 10 (due 2026-10-25).
// Units: A 40% (440.00), B 35% (385.00), C 25% (275.00).
type lateFx struct {
	budget, issuance string
	a, b, c          string // receivable ids
}

func (e *quotaEnv) lateFixture(t *testing.T) lateFx {
	t.Helper()
	e.addStdUnits(t)
	id := e.newApprovedBudget(t, 2026, "monthly", 25, "10")
	if r := e.issue(t, id, 10); r.code != http.StatusSeeOther {
		t.Fatalf("issue period 10 = %d\n%s", r.code, r.body)
	}
	iss := e.activeIssuanceID(t, id, 10)
	f := lateFx{budget: id, issuance: iss,
		a: e.quotaAccountID(t, iss, "A"), b: e.quotaAccountID(t, iss, "B"), c: e.quotaAccountID(t, iss, "C")}
	for _, q := range e.quotaRows(t, iss) {
		if q.Due != "2026-10-25" {
			t.Fatalf("unit %s due %s, want 2026-10-25", q.Unit, q.Due)
		}
		if q.Unit == "B" && q.Cents != 38500 {
			t.Fatalf("unit B quota = %d, want 38500", q.Cents)
		}
	}
	return f
}

// setLateFee posts the late-fee form of the settings page as the syndic of A.
func (e *quotaEnv) setLateFee(t *testing.T, fee, interest, grace string) {
	t.Helper()
	r := e.post(t, e.syndic, "/condominium/settings", url.Values{
		"form": {"late_fee"}, "late_fee": {fee}, "late_interest": {interest}, "late_grace_days": {grace},
	})
	if r.code != http.StatusSeeOther {
		t.Fatalf("save late fee %s/%s/%s = %d\n%s", fee, interest, grace, r.code, r.body)
	}
}

func (e *quotaEnv) settle(t *testing.T, acct, date, decision string) resp {
	t.Helper()
	v := url.Values{"settlement_date": {date}}
	if decision != "" {
		v.Set("late_fee_decision", decision)
	}
	return e.post(t, e.syndic, "/finance/"+acct+"/settle", v)
}

// fragment is the HX late-fee fragment of the settle form for a date.
func (e *quotaEnv) fragment(t *testing.T, acct, date string) resp {
	t.Helper()
	return e.get(t, e.syndic, "/finance/"+acct+"/settle/late-fee?settlement_date="+date, hx)
}

// acctState returns the status and settlement date ("" if none) of a receivable.
func (e *quotaEnv) acctState(t *testing.T, id string) (string, string) {
	t.Helper()
	var st, on string
	if err := e.db.QueryRow(`SELECT status, COALESCE(to_char(settlement_date,'YYYY-MM-DD'),'') FROM finance_accounts WHERE id=$1`, id).Scan(&st, &on); err != nil {
		t.Fatalf("account %s: %v", id, err)
	}
	return st, on
}

// lateFeeRow is the late fee receivable linked to a quota with its stored calculation.
type lateFeeRow struct {
	ID                                  string
	Multa, Juros, Amount                int64
	Days, MultaBP, JurosBP, Grace       int
	Category, Status, Type, Due, UnitCd string
}

func (e *quotaEnv) lateFeeOf(t *testing.T, quota string) (lateFeeRow, bool) {
	t.Helper()
	var r lateFeeRow
	err := e.db.QueryRow(`
		SELECT lf.late_fee_account_id, lf.multa_cents, lf.juros_cents, lf.days_late, lf.multa_bp, lf.juros_bp, lf.grace_days,
		       a.amount_cents, a.category, a.status, a.account_type, to_char(a.due_date,'YYYY-MM-DD'), u.code
		FROM quota_late_fees lf
		JOIN finance_accounts a ON a.id = lf.late_fee_account_id
		JOIN units u ON u.id = a.unit_id
		WHERE lf.quota_account_id = $1`, quota).
		Scan(&r.ID, &r.Multa, &r.Juros, &r.Days, &r.MultaBP, &r.JurosBP, &r.Grace,
			&r.Amount, &r.Category, &r.Status, &r.Type, &r.Due, &r.UnitCd)
	if errors.Is(err, sql.ErrNoRows) {
		return r, false
	}
	if err != nil {
		t.Fatalf("late fee of %s: %v", quota, err)
	}
	return r, true
}

func (e *quotaEnv) auditDetail(t *testing.T, event, key, val string) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM audit_events WHERE event_type=$1 AND details->>$2=$3`, event, key, val)
}

// breakdownFields asserts the fragment shows the late fee breakdown.
func checkBreakdown(t *testing.T, what, body, fine, interest, days, total string) {
	t.Helper()
	mustContain(t, what, body, breakdownTag)
	for label, want := range map[string]string{"Fine": fine, "Interest": interest, "Days late": days, "Late fee total": total} {
		if got, ok := ddAfter(body, label); !ok || got != want {
			t.Errorf("%s: %s = %q (present %v), want %q", what, label, got, ok, want)
		}
	}
}

// AC-3, AC-17 (settings), AC-18 (settings): the late-fee settings.
func TestLateFeeSettings(t *testing.T) {
	e := newQuotaEnv(t)
	read := func(condo string) (int, int, int) {
		var fee, interest, grace int
		if err := e.db.QueryRow(`SELECT late_fee_bp, late_interest_bp, late_grace_days FROM condominiums WHERE id=$1`, condo).Scan(&fee, &interest, &grace); err != nil {
			t.Fatalf("read settings: %v", err)
		}
		return fee, interest, grace
	}
	if f, i, g := read(e.condoA); f != 200 || i != 100 || g != 0 {
		t.Fatalf("BRL defaults = %d/%d/%d, want 200/100/0", f, i, g)
	}
	form := func(fee, interest, grace string) url.Values {
		return url.Values{"form": {"late_fee"}, "late_fee": {fee}, "late_interest": {interest}, "late_grace_days": {grace}}
	}

	// BRL: 2.5% is above the cap and nothing is stored or audited.
	r := e.post(t, e.syndic, "/condominium/settings", form("2.5", "1", "0"))
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("BRL fee 2.5 = %d, want 422", r.code)
	}
	mustContain(t, "BRL cap", r.body, enT("settings.error.fee_above_cap"))
	if f, _, _ := read(e.condoA); f != 200 {
		t.Errorf("fee after refusal = %d, want 200", f)
	}
	if n := e.auditCount(t, "late_fee_settings_changed", "", ""); n != 0 {
		t.Errorf("settings events after refusal = %d, want 0", n)
	}
	// Malformed values are refused too.
	for name, v := range map[string]url.Values{
		"fee text":       form("abc", "1", "0"),
		"fee 3 decimal":  form("1.234", "1", "0"),
		"interest 101":   form("2", "101", "0"),
		"grace 61":       form("2", "1", "61"),
		"grace negative": form("2", "1", "-1"),
	} {
		if r := e.post(t, e.syndic, "/condominium/settings", v); r.code != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d, want 422", name, r.code)
		}
	}

	// 2% and 1% a month with 3 grace days are stored and audited.
	r = e.post(t, e.syndic, "/condominium/settings", form("2", "1", "3"))
	if r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=late_fee_saved") {
		t.Fatalf("save 2/1/3 = %d Location=%q\n%s", r.code, r.location(), r.body)
	}
	if f, i, g := read(e.condoA); f != 200 || i != 100 || g != 3 {
		t.Errorf("stored = %d/%d/%d, want 200/100/3", f, i, g)
	}
	if n := e.auditCount(t, "late_fee_settings_changed", e.syndicAID, ""); n != 1 {
		t.Errorf("late_fee_settings_changed events = %d, want 1", n)
	}
	page := e.get(t, e.syndic, "/condominium/settings")
	if v, ok := inputValue(page.body, "late_grace_days"); !ok || v != "3" {
		t.Errorf("grace input = %q (present %v), want 3", v, ok)
	}

	// EUR condominium: 5% is accepted.
	if _, err := e.db.Exec(`UPDATE condominiums SET currency='EUR' WHERE id=$1`, e.condoB); err != nil {
		t.Fatalf("set EUR: %v", err)
	}
	r = e.post(t, e.syndicB, "/condominium/settings", form("5", "0.5", "0"))
	if r.code != http.StatusSeeOther {
		t.Fatalf("EUR fee 5 = %d\n%s", r.code, r.body)
	}
	if f, i, g := read(e.condoB); f != 500 || i != 50 || g != 0 {
		t.Errorf("EUR stored = %d/%d/%d, want 500/50/0", f, i, g)
	}

	// AC-18: residents cannot read or change the settings.
	for name, c := range map[string]*http.Client{"owner": e.owner, "tenant": e.tenant} {
		if r := e.get(t, c, "/condominium/settings"); r.code != http.StatusForbidden {
			t.Errorf("%s GET settings = %d, want 403", name, r.code)
		}
		if r := e.post(t, c, "/condominium/settings", form("0", "0", "0")); r.code != http.StatusForbidden {
			t.Errorf("%s POST settings = %d, want 403", name, r.code)
		}
	}
	if f, i, g := read(e.condoA); f != 200 || i != 100 || g != 3 {
		t.Errorf("stored after resident attempts = %d/%d/%d, want 200/100/3", f, i, g)
	}
}

// AC-4 (stored numbers), AC-5, AC-6, AC-7, AC-8, AC-16 (Recibo), AC-17 (apply,
// waive): the settle flow of a late quota.
func TestLateFeeApplyAndWaive(t *testing.T) {
	e := newQuotaEnv(t)
	f := e.lateFixture(t)

	// The fragment for the AC-4 date shows fine 7.70, interest 2.57 over 20 days.
	r := e.fragment(t, f.b, "2026-11-14")
	if r.code != http.StatusOK {
		t.Fatalf("fragment = %d\n%s", r.code, r.body)
	}
	checkBreakdown(t, "fragment 2026-11-14", r.body, fmtMoney(770), fmtMoney(257), "20", fmtMoney(1027))
	// AC-5: on the due date and before it there is no late fee.
	for _, d := range []string{"2026-10-25", "2026-10-10"} {
		if r := e.fragment(t, f.b, d); r.code != http.StatusOK || strings.Contains(r.body, breakdownTag) {
			t.Errorf("fragment %s = %d, want 200 without a late fee\n%s", d, r.code, r.body)
		}
	}

	// A late settlement needs a decision and changes nothing without it.
	r = e.settle(t, f.b, "2026-11-14", "")
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("settle without decision = %d, want 422 (settle page)", r.code)
	}
	mustContain(t, "decision required", r.body, enT("quotas.error.late_fee_decision_required"))
	if st, _ := e.acctState(t, f.b); st != "pending" {
		t.Errorf("quota after refused settle = %s, want pending", st)
	}
	if r := e.settle(t, f.b, "2026-11-14", "maybe"); r.code != http.StatusBadRequest {
		t.Errorf("settle with an invalid decision = %d, want 400", r.code)
	} else {
		mustContain(t, "invalid decision", r.body, enT("quotas.error.invalid_decision"))
	}
	if st, _ := e.acctState(t, f.b); st != "pending" {
		t.Errorf("quota after invalid decision = %s, want pending", st)
	}

	// AC-6: apply.
	if r := e.settle(t, f.b, "2026-11-14", "apply"); r.code != http.StatusSeeOther {
		t.Fatalf("settle and apply = %d\n%s", r.code, r.body)
	}
	if st, on := e.acctState(t, f.b); st != "settled" || on != "2026-11-14" {
		t.Errorf("quota = %s on %s, want settled on 2026-11-14", st, on)
	}
	lf, ok := e.lateFeeOf(t, f.b)
	if !ok {
		t.Fatal("no quota_late_fees row after apply")
	}
	if lf.Multa != 770 || lf.Juros != 257 || lf.Days != 20 || lf.MultaBP != 200 || lf.JurosBP != 100 || lf.Grace != 0 {
		t.Errorf("stored calculation = %+v, want 770/257 over 20 days at 200/100bp grace 0", lf)
	}
	if lf.Amount != 1027 || lf.Category != "fine_interest" || lf.Status != "pending" || lf.Type != "receivable" ||
		lf.Due != "2026-11-14" || lf.UnitCd != "B" {
		t.Errorf("late fee receivable = %+v, want 1027 fine_interest pending receivable due 2026-11-14 of unit B", lf)
	}
	if n := e.auditDetail(t, "late_fee_applied", "amount_cents", "1027"); n != 1 {
		t.Errorf("late_fee_applied events (1027) = %d, want 1", n)
	}
	if n := e.auditCount(t, "late_fee_applied", e.syndicAID, ""); n != 1 {
		t.Errorf("late_fee_applied by the syndic = %d, want 1", n)
	}

	// The late fee shows its localised title in the generic list and cannot be edited.
	list := e.get(t, e.syndic, "/finance")
	mustContain(t, "generic list", list.body, "Late fee and interest — Condo fee")
	if r := e.get(t, e.syndic, "/finance/"+lf.ID+"/edit"); r.code != http.StatusConflict {
		t.Errorf("GET edit of a late fee = %d, want 409", r.code)
	}
	if r := e.post(t, e.syndic, "/finance/"+lf.ID+"/edit", url.Values{
		"type": {"receivable"}, "title": {"hacked"}, "category": {"fine_interest"}, "amount": {"1.00"},
		"due_date": {"2026-12-01"}, "status": {"pending"}}); r.code != http.StatusConflict {
		t.Errorf("POST edit of a late fee = %d, want 409", r.code)
	}

	// AC-8: settling the late fee late (generic flow) never creates a second mora.
	if r := e.settle(t, lf.ID, "2026-11-20", ""); r.code != http.StatusSeeOther {
		t.Fatalf("settle late fee = %d\n%s", r.code, r.body)
	}
	if st, _ := e.acctState(t, lf.ID); st != "settled" {
		t.Errorf("late fee status = %s, want settled", st)
	}
	if n := e.count(t, `SELECT count(*) FROM quota_late_fees`); n != 1 {
		t.Errorf("quota_late_fees rows = %d, want 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM finance_accounts WHERE category='fine_interest'`); n != 1 {
		t.Errorf("fine_interest receivables = %d, want 1", n)
	}
	// A settled late fee cannot be canceled.
	if r := e.post(t, e.syndic, "/finance/"+lf.ID+"/cancel", nil); r.code == http.StatusSeeOther {
		t.Errorf("cancel of a settled late fee = 303, want a refusal")
	}
	if st, _ := e.acctState(t, lf.ID); st != "settled" {
		t.Errorf("late fee status after cancel attempt = %s, want settled", st)
	}
	// The Recibo of the late fee shows the origin quota and the breakdown.
	rec := e.get(t, e.syndic, "/finance/"+lf.ID+"/recibo")
	if rec.code != http.StatusOK {
		t.Fatalf("late fee recibo = %d\n%s", rec.code, rec.body)
	}
	mustContain(t, "late fee recibo", rec.body, "Late fee and interest — Condo fee")
	if got, ok := ddAfter(rec.body, "Fine"); !ok || !strings.HasPrefix(got, fmtMoney(770)) {
		t.Errorf("recibo fine = %q (present %v), want %s", got, ok, fmtMoney(770))
	}
	if got, ok := ddAfter(rec.body, "Interest"); !ok || !strings.HasPrefix(got, fmtMoney(257)) {
		t.Errorf("recibo interest = %q (present %v), want %s", got, ok, fmtMoney(257))
	}
	if got, ok := ddAfter(rec.body, "Days late"); !ok || got != "20" {
		t.Errorf("recibo days late = %q (present %v), want 20", got, ok)
	}
	if got, ok := ddAfter(rec.body, "Origin quota"); !ok || !strings.Contains(got, "Condo fee") {
		t.Errorf("recibo origin = %q (present %v), want the quota title", got, ok)
	}

	// AC-7: waive on unit C (275.00: fine 5.50 + interest 1.83 = 7.33).
	if r := e.settle(t, f.c, "2026-11-14", "waive"); r.code != http.StatusSeeOther {
		t.Fatalf("settle and waive = %d\n%s", r.code, r.body)
	}
	if st, on := e.acctState(t, f.c); st != "settled" || on != "2026-11-14" {
		t.Errorf("waived quota = %s on %s, want settled on 2026-11-14", st, on)
	}
	if _, ok := e.lateFeeOf(t, f.c); ok {
		t.Error("late fee row exists after waive")
	}
	if n := e.count(t, `SELECT count(*) FROM finance_accounts WHERE category='fine_interest'`); n != 1 {
		t.Errorf("fine_interest receivables after waive = %d, want still 1", n)
	}
	if n := e.auditDetail(t, "late_fee_waived", "amount_cents", "733"); n != 1 {
		t.Errorf("late_fee_waived events (733) = %d, want 1", n)
	}
	if n := e.auditDetail(t, "late_fee_waived", "stage", "settlement"); n != 1 {
		t.Errorf("late_fee_waived at settlement = %d, want 1", n)
	}
}

// AC-4 (grace), AC-5 (grace), edge case "mora at 0,00": grace days move the
// start of the late fee and zero rates leave the settle flow as in phase 1.
func TestLateFeeGraceAndZeroRates(t *testing.T) {
	e := newQuotaEnv(t)
	f := e.lateFixture(t)
	e.setLateFee(t, "2", "1", "5")

	// 20 days minus 5 grace days: fine 7.70, interest 385.00 x 1% x 15/30 = 1.925 -> 1.93.
	r := e.fragment(t, f.b, "2026-11-14")
	checkBreakdown(t, "grace 5", r.body, fmtMoney(770), fmtMoney(193), "15", fmtMoney(963))
	// Within the grace (5 days after the due date) there is no late fee, from the day after there is.
	if r := e.fragment(t, f.b, "2026-10-30"); strings.Contains(r.body, breakdownTag) {
		t.Errorf("fragment inside the grace shows a late fee\n%s", r.body)
	}
	if r := e.fragment(t, f.b, "2026-10-31"); !strings.Contains(r.body, breakdownTag) {
		t.Errorf("fragment 1 day after the grace shows no late fee\n%s", r.body)
	}
	if r := e.settle(t, f.a, "2026-10-30", ""); r.code != http.StatusSeeOther {
		t.Fatalf("settle inside the grace without decision = %d\n%s", r.code, r.body)
	}
	if st, _ := e.acctState(t, f.a); st != "settled" {
		t.Errorf("quota inside the grace = %s, want settled", st)
	}
	if n := e.count(t, `SELECT count(*) FROM quota_late_fees`) + e.auditCount(t, "late_fee_waived", "", ""); n != 0 {
		t.Errorf("late fee rows and waivers inside the grace = %d, want 0", n)
	}

	// Rates at zero: a very late quota settles without a decision.
	e.setLateFee(t, "0", "0", "0")
	if r := e.fragment(t, f.c, "2026-11-14"); strings.Contains(r.body, breakdownTag) {
		t.Errorf("fragment with zero rates shows a late fee\n%s", r.body)
	}
	if r := e.settle(t, f.c, "2026-11-14", ""); r.code != http.StatusSeeOther {
		t.Fatalf("settle with zero rates = %d\n%s", r.code, r.body)
	}
	if _, ok := e.lateFeeOf(t, f.c); ok {
		t.Error("late fee created with zero rates")
	}

	// A change of the settings between preview and settlement counts at settlement.
	e.setLateFee(t, "2", "1", "0")
	if r := e.settle(t, f.b, "2026-11-14", ""); r.code != http.StatusUnprocessableEntity {
		t.Fatalf("settle B after changing the settings back = %d, want the decision page (422)", r.code)
	}
}

// AC-8 (cancel), AC-17 (waiver by cancellation): canceling a pending late fee
// is a later waiver.
func TestLateFeeCancelIsLaterWaiver(t *testing.T) {
	e := newQuotaEnv(t)
	f := e.lateFixture(t)
	if r := e.settle(t, f.a, "2026-11-14", "apply"); r.code != http.StatusSeeOther {
		t.Fatalf("settle and apply = %d\n%s", r.code, r.body)
	}
	lf, ok := e.lateFeeOf(t, f.a)
	if !ok || lf.Amount != 1173 {
		t.Fatalf("late fee of A = %+v (found %v), want 1173", lf, ok)
	}
	page := e.get(t, e.syndic, "/finance/"+lf.ID+"/cancel")
	if page.code != http.StatusOK {
		t.Fatalf("cancel page of a late fee = %d\n%s", page.code, page.body)
	}
	mustContain(t, "cancel page", page.body, enT("quotas.late_fee.cancel_copy"))

	if r := e.post(t, e.syndic, "/finance/"+lf.ID+"/cancel", nil); r.code != http.StatusSeeOther {
		t.Fatalf("cancel late fee = %d\n%s", r.code, r.body)
	}
	if st, _ := e.acctState(t, lf.ID); st != "canceled" {
		t.Errorf("late fee status = %s, want canceled", st)
	}
	if st, _ := e.acctState(t, f.a); st != "settled" {
		t.Errorf("origin quota status = %s, want settled", st)
	}
	if n := e.auditDetail(t, "late_fee_waived", "stage", "cancel"); n != 1 {
		t.Errorf("late_fee_waived by cancellation = %d, want 1", n)
	}
	if n := e.auditDetail(t, "late_fee_waived", "amount_cents", "1173"); n != 1 {
		t.Errorf("late_fee_waived (1173) = %d, want 1", n)
	}
	if n := e.auditDetail(t, "account_canceled", "account_id", lf.ID); n != 1 {
		t.Errorf("account_canceled of the late fee = %d, want 1", n)
	}
	// A canceled late fee cannot be canceled again, and the quota keeps its refusal.
	if r := e.post(t, e.syndic, "/finance/"+lf.ID+"/cancel", nil); r.code == http.StatusSeeOther {
		t.Error("second cancel = 303, want a refusal")
	}
	if r := e.get(t, e.syndic, "/finance/"+f.b+"/cancel"); r.code != http.StatusConflict {
		t.Errorf("generic cancel of a quota = %d, want 409", r.code)
	}
}

// debtorsFixture builds AC-1 and AC-16: A-301 with one quota of 1.000,00 due
// 30 days ago; B-201 with a quota of 1.000,00 due 30 days ago plus a receivable
// of 1.400,00 due 10 days ago (2 overdue, 2.400,00) and two occupants; C-101
// with only a receivable that is not due yet.
func (e *quotaEnv) debtorsFixture(t *testing.T) (a, b, c string, d30 time.Time) {
	t.Helper()
	a = e.addUnit(t, "A-301", 500000)
	b = e.addUnit(t, "B-201", 500000)
	c = e.addUnit(t, "C-101", 0)
	id := e.createBudget(t, 2026, "monthly", 25, "0")
	if r := e.addLine(t, id, "Maintenance", "maintenance", "24000.00"); r.code != http.StatusSeeOther {
		t.Fatalf("add line = %d\n%s", r.code, r.body)
	}
	if r := e.approve(t, id); r.code != http.StatusSeeOther {
		t.Fatalf("approve = %d", r.code)
	}
	if r := e.issue(t, id, 10); r.code != http.StatusSeeOther {
		t.Fatalf("issue = %d\n%s", r.code, r.body)
	}
	iss := e.activeIssuanceID(t, id, 10)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	d30 = today.AddDate(0, 0, -30)
	for _, code := range []string{"A-301", "B-201"} {
		q := e.quotaAccountID(t, iss, code)
		if _, err := e.db.Exec(`UPDATE finance_accounts SET due_date=$1 WHERE id=$2`, d30.Format("2006-01-02"), q); err != nil {
			t.Fatalf("backdate %s: %v", code, err)
		}
	}
	e.insertReceivable(t, b, "ZZ-B-OLD", 140000, today.AddDate(0, 0, -10).Format("2006-01-02"), "pending", "")
	e.insertReceivable(t, c, "ZZ-C-FUTURE", 5000, today.AddDate(0, 0, 10).Format("2006-01-02"), "pending", "")
	// Two active occupants of B-201; A-301 has none.
	e.occupy(t, "resident@example.com", b, "owner", "2026-01-01")
	e.occupy(t, "tenant-inv@example.com", b, "tenant", "2026-01-01")
	return a, b, c, d30
}

// AC-1, AC-2, AC-16, a debtor with two occupants, a debtor without occupants.
func TestDebtorsListAndCard(t *testing.T) {
	e := newQuotaEnv(t)
	_, b, _, d30 := e.debtorsFixture(t)

	page := e.get(t, e.syndic, "/finance/debtors")
	if page.code != http.StatusOK {
		t.Fatalf("debtors = %d\n%s", page.code, page.body)
	}
	rows := cellRows(page.body)
	ra, ia := rowStarting(rows, "A-301")
	rb, ib := rowStarting(rows, "B-201")
	if ra == nil || rb == nil {
		t.Fatalf("debtors rows A-301=%v B-201=%v\n%v", ra, rb, rows)
	}
	if _, ic := rowStarting(rows, "C-101"); ic >= 0 {
		t.Error("C-101 (nothing overdue) is listed")
	}
	if ib > ia {
		t.Errorf("B-201 (2.400,00) is at row %d after A-301 (1.000,00) at row %d", ib, ia)
	}
	oldest := i18n.FormatDate(i18n.LanguageEN, d30)
	// Columns: unit, occupants, overdue count, total, oldest due, days late, estimated late fee.
	// Estimated late fee: 1000.00 x 2% = 20.00 plus 1000.00 x 1% x 30/30 = 10.00.
	wantB := []string{"B-201", "", "2", fmtMoney(240000), oldest, "30", fmtMoney(3000)}
	wantA := []string{"A-301", enT("quotas.debtors.no_occupants"), "1", fmtMoney(100000), oldest, "30", fmtMoney(3000)}
	for _, c := range []struct {
		name      string
		got, want []string
	}{{"B-201", rb, wantB}, {"A-301", ra, wantA}} {
		if len(c.got) != len(c.want) {
			t.Fatalf("%s cells = %v, want %d columns", c.name, c.got, len(c.want))
		}
		for i := range c.want {
			if i == 1 && c.name == "B-201" {
				continue
			}
			if c.got[i] != c.want[i] {
				t.Errorf("%s column %d = %q, want %q", c.name, i, c.got[i], c.want[i])
			}
		}
	}
	// Both occupants of B-201 are shown with their role.
	for _, want := range []string{"resident@example.com (owner)", "tenant-inv@example.com (tenant)"} {
		if !strings.Contains(rb[1], want) {
			t.Errorf("B-201 occupants = %q, missing %q", rb[1], want)
		}
	}
	// Each row links to the statement of the unit.
	mustContain(t, "statement link", page.body, `href="/finance/my-charges?unit=`+b+`"`)

	// AC-16: the dashboard card of the syndic.
	card := e.get(t, e.syndic, "/finance/quotas/card", hx)
	if card.code != http.StatusOK {
		t.Fatalf("card = %d\n%s", card.code, card.body)
	}
	mustContain(t, "debtors line", card.body, "2 units overdue — "+fmtMoney(340000), `href="/finance/debtors"`)

	// AC-2: residents get 403 for the list and no debtors line on their card.
	for name, c := range map[string]*http.Client{"owner": e.owner, "tenant": e.tenant} {
		if r := e.get(t, c, "/finance/debtors"); r.code != http.StatusForbidden {
			t.Errorf("%s GET debtors = %d, want 403", name, r.code)
		}
	}
	if r := e.get(t, e.owner, "/finance/quotas/card", hx); strings.Contains(r.body, "units overdue") || strings.Contains(r.body, "data-quotas-card-debtors") {
		t.Errorf("resident card shows the debtors line\n%s", r.body)
	}
	// The resident statement shows no estimated late fee.
	st := e.get(t, e.owner, "/finance/my-charges?unit="+b)
	if st.code != http.StatusOK {
		t.Fatalf("resident statement = %d", st.code)
	}
	mustNotContain(t, "resident statement", st.body, enT("quotas.debtors.col.estimated"))
	// Another condominium sees nothing of it.
	if r := e.get(t, e.syndicB, "/finance/debtors"); !strings.Contains(r.body, enT("quotas.debtors.empty_title")) {
		t.Errorf("syndic of B debtors page is not empty\n%s", r.body)
	}

	// Without overdue receivables the unit leaves the list and the card line disappears.
	if _, err := e.db.Exec(`UPDATE finance_accounts SET status='settled', settlement_date=CURRENT_DATE
		WHERE condominium_id=$1 AND status='pending' AND due_date < CURRENT_DATE`, e.condoA); err != nil {
		t.Fatalf("settle overdue: %v", err)
	}
	page = e.get(t, e.syndic, "/finance/debtors")
	mustContain(t, "empty debtors", page.body, enT("quotas.debtors.empty_title"))
	card = e.get(t, e.syndic, "/finance/quotas/card", hx)
	mustNotContain(t, "card without overdue", card.body, "data-quotas-card-debtors", "units overdue")
}

// FR-21: the debtors list is named per language.
func TestDebtorsTitlePerLanguage(t *testing.T) {
	e := newQuotaEnv(t)
	for lang, want := range map[string]string{"pt-br": "Inadimplentes", "pt-pt": "Devedores", "en": "Debtors"} {
		if _, err := e.db.Exec(`UPDATE users SET language_preference=$1 WHERE email='syndic@example.com'`, lang); err != nil {
			t.Fatalf("set language: %v", err)
		}
		r := e.get(t, e.syndic, "/finance/debtors")
		if r.code != http.StatusOK || !strings.Contains(r.body, "<h1 class=\"page-title\">"+want+"</h1>") {
			t.Errorf("%s debtors page = %d, want title %q", lang, r.code, want)
		}
	}
}

// AC-18: a resident gets 403 on extraordinary quotas, the reserve fund balance
// and the late-fee fragment (settings are covered in TestLateFeeSettings).
func TestPhase2ForbiddenForResidents(t *testing.T) {
	e := newQuotaEnv(t)
	f := e.lateFixture(t)
	const someID = "00000000-0000-0000-0000-000000000001"
	form := url.Values{"description": {"X"}, "total": {"100.00"}, "division": {"equal"}, "installments": {"1"}, "first_due": {"2026-12-10"}}
	for name, c := range map[string]*http.Client{"owner": e.owner, "tenant": e.tenant} {
		for _, p := range []string{
			"/finance/extraordinaries", "/finance/extraordinaries/new", "/finance/extraordinaries/" + someID + "/view",
			"/finance/extraordinaries/" + someID + "/cancel", "/finance/budgets/" + f.budget + "/view", "/finance/debtors",
		} {
			if r := e.get(t, c, p); r.code != http.StatusForbidden {
				t.Errorf("%s GET %s = %d, want 403", name, p, r.code)
			}
		}
		for _, p := range []string{"/finance/extraordinaries", "/finance/extraordinaries/preview", "/finance/extraordinaries/" + someID + "/cancel"} {
			if r := e.post(t, c, p, form); r.code != http.StatusForbidden {
				t.Errorf("%s POST %s = %d, want 403", name, p, r.code)
			}
		}
		if r := e.get(t, c, "/finance/"+f.b+"/settle/late-fee?settlement_date=2026-11-14", hx); r.code != http.StatusForbidden || strings.Contains(r.body, breakdownTag) {
			t.Errorf("%s late fee fragment = %d, want 403 without a breakdown", name, r.code)
		}
		// The overview panel is open to members but the reserve fund balance is not.
		ov := e.get(t, c, "/finance/overview", hx)
		if ov.code != http.StatusOK {
			t.Errorf("%s overview = %d, want 200", name, ov.code)
		}
		mustNotContain(t, name+" overview", ov.body, "data-reserve-fund", enT("quotas.reserve_fund.title"))
	}
	if n := e.count(t, `SELECT count(*) FROM quota_extraordinaries`); n != 0 {
		t.Errorf("extraordinary quotas after denied requests = %d, want 0", n)
	}
}

// AC-9 (preview values), FR-10.
func TestExtraordinaryPreview(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	prev := func(desc, total, division, n string) resp {
		return e.post(t, e.syndic, "/finance/extraordinaries/preview", url.Values{
			"description": {desc}, "total": {total}, "division": {division}, "installments": {n}, "first_due": {"2026-12-10"},
		}, hx)
	}
	// 12.000,00 in 6 installments by share: 2.000,00 each; A 40%, B 35%, C 25%.
	r := prev("Obra da fachada", "12000.00", "share", "6")
	if r.code != http.StatusOK {
		t.Fatalf("preview = %d\n%s", r.code, r.body)
	}
	rows := cellRows(r.body)
	for i := 1; i <= 6; i++ {
		row, _ := rowStarting(rows, strconv.Itoa(i))
		if row == nil || len(row) != 3 || row[2] != fmtMoney(200000) {
			t.Errorf("installment %d row = %v, want total %s", i, row, fmtMoney(200000))
		}
	}
	for code, per := range map[string]int64{"A": 80000, "B": 70000, "C": 50000} {
		row, _ := rowStarting(rows, code)
		// code, share, 6 installments, total
		if len(row) != 9 {
			t.Fatalf("unit %s row = %v, want 9 cells", code, row)
		}
		for i := 2; i < 8; i++ {
			if row[i] != fmtMoney(per) {
				t.Errorf("unit %s installment cell %d = %q, want %s", code, i-1, row[i], fmtMoney(per))
			}
		}
		if row[8] != fmtMoney(per*6) {
			t.Errorf("unit %s total = %q, want %s", code, row[8], fmtMoney(per*6))
		}
	}
	// Equal parts: 100,00 among 3 units is 33,34 / 33,33 / 33,33 (the smallest code takes the cent).
	r = prev("Equal", "100.00", "equal", "1")
	if r.code != http.StatusOK {
		t.Fatalf("equal preview = %d\n%s", r.code, r.body)
	}
	rows = cellRows(r.body)
	for code, want := range map[string]int64{"A": 3334, "B": 3333, "C": 3333} {
		row, _ := rowStarting(rows, code)
		if len(row) != 4 || row[2] != fmtMoney(want) || row[3] != fmtMoney(want) {
			t.Errorf("equal unit %s row = %v, want %s", code, row, fmtMoney(want))
		}
	}
	if row, _ := rowStarting(rows, "B-1"); row != nil {
		t.Errorf("the zero-share unit B-1 is in the preview: %v", row)
	}
	// Nothing is created by a preview.
	if n := e.count(t, `SELECT count(*) FROM quota_extraordinaries`); n != 0 {
		t.Errorf("extraordinary quotas after previews = %d, want 0", n)
	}
}

// AC-10: shares that do not add to 100% refuse the share division but not equal parts.
func TestExtraordinaryValidation(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	if _, err := e.db.Exec(`UPDATE units SET share_ppm=230000 WHERE condominium_id=$1 AND code='C'`, e.condoA); err != nil {
		t.Fatalf("shrink C: %v", err)
	}
	create := func(division, n, due string) resp {
		return e.post(t, e.syndic, "/finance/extraordinaries", url.Values{
			"description": {"Obra"}, "total": {"900.00"}, "division": {division}, "installments": {n}, "first_due": {due},
		})
	}
	if r := create("share", "3", "2026-12-10"); r.code != http.StatusUnprocessableEntity {
		t.Errorf("share division at 98%% = %d, want 422", r.code)
	} else {
		mustContain(t, "98% refusal", r.body, enT("quotas.error.shares_not_100"))
	}
	for name, r := range map[string]resp{
		"25 installments":  create("equal", "25", "2026-12-10"),
		"0 installments":   create("equal", "0", "2026-12-10"),
		"first due day 29": create("equal", "3", "2026-12-29"),
		"no first due":     create("equal", "3", ""),
		"bad division":     create("lump", "3", "2026-12-10"),
	} {
		if r.code != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d, want 422", name, r.code)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM quota_extraordinaries`); n != 0 {
		t.Fatalf("extraordinary quotas after refusals = %d, want 0", n)
	}
	if r := create("equal", "3", "2026-12-10"); r.code != http.StatusSeeOther {
		t.Errorf("equal parts at 98%% = %d, want 303\n%s", r.code, r.body)
	}
}

// extraRow is one receivable of an extraordinary quota.
type extraRow struct {
	ID, Unit, Due, Status, Category, Type string
	Cents                                 int64
	Installment                           int
}

func (e *quotaEnv) createExtra(t *testing.T, desc, total, division string, n int, firstDue string) string {
	t.Helper()
	r := e.post(t, e.syndic, "/finance/extraordinaries", url.Values{
		"description": {desc}, "total": {total}, "division": {division},
		"installments": {strconv.Itoa(n)}, "first_due": {firstDue},
	})
	if r.code != http.StatusSeeOther {
		t.Fatalf("create extraordinary %q = %d\n%s", desc, r.code, r.body)
	}
	m := extraLocRe.FindStringSubmatch(r.location())
	if m == nil || !strings.Contains(r.location(), "flash=extra_created") {
		t.Fatalf("create extraordinary: unexpected Location %q", r.location())
	}
	return m[1]
}

func (e *quotaEnv) extraRows(t *testing.T, extraID string) []extraRow {
	t.Helper()
	rows, err := e.db.Query(`
		SELECT a.id, u.code, to_char(a.due_date,'YYYY-MM-DD'), a.status, a.category, a.account_type, a.amount_cents, i.period_index
		FROM quota_issuances i
		JOIN quota_charges c ON c.issuance_id = i.id
		JOIN finance_accounts a ON a.id = c.finance_account_id
		JOIN units u ON u.id = c.unit_id
		WHERE i.extraordinary_id = $1 ORDER BY i.period_index, u.code`, extraID)
	if err != nil {
		t.Fatalf("extra rows: %v", err)
	}
	defer rows.Close()
	var out []extraRow
	for rows.Next() {
		var r extraRow
		if err := rows.Scan(&r.ID, &r.Unit, &r.Due, &r.Status, &r.Category, &r.Type, &r.Cents, &r.Installment); err != nil {
			t.Fatalf("scan extra row: %v", err)
		}
		out = append(out, r)
	}
	return out
}

func (e *quotaEnv) installmentAccount(t *testing.T, extraID string, n int, unit string) string {
	t.Helper()
	for _, r := range e.extraRows(t, extraID) {
		if r.Installment == n && r.Unit == unit {
			return r.ID
		}
	}
	t.Fatalf("no installment %d of unit %s in %s", n, unit, extraID)
	return ""
}

// AC-11, AC-12, AC-17 (extraordinary events), the ordinary cancel route
// refusing an installment issuance.
func TestExtraordinaryCreateAndCancel(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)

	id := e.createExtra(t, "Reforma", "3000.00", "share", 3, "2026-12-10")
	rows := e.extraRows(t, id)
	if len(rows) != 9 {
		t.Fatalf("receivables = %d, want 9 (3 units x 3 installments)", len(rows))
	}
	dues := map[string]int{}
	var sum int64
	perUnit := map[string]int64{"A": 40000, "B": 35000, "C": 25000}
	for _, r := range rows {
		dues[r.Due]++
		sum += r.Cents
		if r.Status != "pending" || r.Category != "extraordinary_income" || r.Type != "receivable" {
			t.Errorf("receivable %+v, want pending extraordinary_income receivable", r)
		}
		if r.Unit == "B-1" || r.Cents != perUnit[r.Unit] {
			t.Errorf("unit %s installment %d = %d, want %d (no B-1)", r.Unit, r.Installment, r.Cents, perUnit[r.Unit])
		}
	}
	for _, d := range []string{"2026-12-10", "2027-01-10", "2027-02-10"} {
		if dues[d] != 3 {
			t.Errorf("receivables due %s = %d, want 3 (%v)", d, dues[d], dues)
		}
	}
	if sum != 300000 {
		t.Errorf("sum of receivables = %d, want 300000", sum)
	}
	if n := e.count(t, `SELECT count(*) FROM quota_issuances WHERE extraordinary_id=$1 AND budget_id IS NULL AND status='issued'`, id); n != 3 {
		t.Errorf("installment issuances = %d, want 3", n)
	}
	if n := e.auditCount(t, "extraordinary_quota_created", e.syndicAID, ""); n != 1 {
		t.Errorf("extraordinary_quota_created events = %d, want 1", n)
	}
	list := e.get(t, e.syndic, "/finance/extraordinaries")
	mustContain(t, "extraordinary list", list.body, "Reforma", fmtMoney(300000), "0 / 3")
	view := e.get(t, e.syndic, "/finance/extraordinaries/"+id+"/view")
	if view.code != http.StatusOK {
		t.Fatalf("view = %d", view.code)
	}
	mustContain(t, "extraordinary view", view.body, "Reforma", "0 / 9")

	// The ordinary cancel route does not know an installment issuance.
	var inst string
	if err := e.db.QueryRow(`SELECT id FROM quota_issuances WHERE extraordinary_id=$1 AND period_index=1`, id).Scan(&inst); err != nil {
		t.Fatalf("installment issuance: %v", err)
	}
	if r := e.post(t, e.syndic, "/finance/issuances/"+inst+"/cancel", url.Values{"budget_id": {""}}); r.code != http.StatusNotFound {
		t.Errorf("ordinary cancel of an installment issuance = %d, want 404", r.code)
	}
	if n := e.count(t, `SELECT count(*) FROM finance_accounts WHERE status='canceled'`); n != 0 {
		t.Errorf("canceled receivables after the refused route = %d, want 0", n)
	}

	// AC-12: without settled installments the whole quota is canceled.
	if r := e.get(t, e.syndic, "/finance/extraordinaries/"+id+"/cancel"); r.code != http.StatusOK {
		t.Fatalf("cancel page = %d\n%s", r.code, r.body)
	}
	r := e.post(t, e.syndic, "/finance/extraordinaries/"+id+"/cancel", nil)
	if r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=extra_canceled") {
		t.Fatalf("cancel = %d Location=%q\n%s", r.code, r.location(), r.body)
	}
	for _, row := range e.extraRows(t, id) {
		if row.Status != "canceled" {
			t.Errorf("receivable %s %d status %s, want canceled", row.Unit, row.Installment, row.Status)
		}
	}
	var st string
	if err := e.db.QueryRow(`SELECT status FROM quota_extraordinaries WHERE id=$1`, id).Scan(&st); err != nil || st != "canceled" {
		t.Errorf("extraordinary status = %q (%v), want canceled", st, err)
	}
	if n := e.auditCount(t, "extraordinary_quota_canceled", e.syndicAID, ""); n != 1 {
		t.Errorf("extraordinary_quota_canceled events = %d, want 1", n)
	}
	if r := e.post(t, e.syndic, "/finance/extraordinaries/"+id+"/cancel", nil); r.code != http.StatusConflict {
		t.Errorf("second cancel = %d, want 409", r.code)
	}

	// With a settled installment the cancellation is refused and nothing changes.
	id2 := e.createExtra(t, "Telhado", "3000.00", "share", 3, "2026-12-10")
	if r := e.settle(t, e.installmentAccount(t, id2, 1, "A"), "2026-11-14", ""); r.code != http.StatusSeeOther {
		t.Fatalf("settle installment = %d\n%s", r.code, r.body)
	}
	r = e.post(t, e.syndic, "/finance/extraordinaries/"+id2+"/cancel", nil)
	if r.code != http.StatusConflict {
		t.Fatalf("cancel with a settled installment = %d, want 409\n%s", r.code, r.body)
	}
	mustContain(t, "settled refusal", r.body, enT("quotas.error.extra_has_settled"))
	pending := 0
	for _, row := range e.extraRows(t, id2) {
		if row.Status == "pending" {
			pending++
		}
	}
	if pending != 8 {
		t.Errorf("pending receivables after the refused cancel = %d, want 8", pending)
	}
	if err := e.db.QueryRow(`SELECT status FROM quota_extraordinaries WHERE id=$1`, id2).Scan(&st); err != nil || st != "issued" {
		t.Errorf("extraordinary status after refusal = %q (%v), want issued", st, err)
	}
	if n := e.auditCount(t, "extraordinary_quota_canceled", "", ""); n != 1 {
		t.Errorf("extraordinary_quota_canceled events after the refusal = %d, want 1", n)
	}
}

// AC-13, AC-14, AC-16 (Recibo): overdue installments carry a late fee; the
// resident sees installments and applied late fees in pt-br, with the Recibo of
// the late fee and no estimated late fee.
func TestExtraordinaryInstallmentLateFeeAndStatement(t *testing.T) {
	e := newQuotaEnv(t)
	f := e.lateFixture(t) // ordinary quota of A: 440.00 due 2026-10-25
	id := e.createExtra(t, "Obra da fachada", "12000.00", "share", 6, "2026-09-10")
	inst1 := e.installmentAccount(t, id, 1, "A") // 800.00 due 2026-09-10
	inst2 := e.installmentAccount(t, id, 2, "A") // 800.00 due 2026-10-10

	// AC-13: the late installment shows the breakdown: 65 days, fine 16.00, interest 800.00 x 1% x 65/30 = 17.33.
	fr := e.fragment(t, inst1, "2026-11-14")
	checkBreakdown(t, "installment fragment", fr.body, fmtMoney(1600), fmtMoney(1733), "65", fmtMoney(3333))
	if r := e.settle(t, inst1, "2026-11-14", ""); r.code != http.StatusUnprocessableEntity {
		t.Fatalf("settle installment without decision = %d, want the decision page (422)", r.code)
	} else {
		mustContain(t, "installment decision", r.body, enT("quotas.error.late_fee_decision_required"))
	}
	// Generic edit and cancel of an installment are refused.
	for name, r := range map[string]resp{
		"GET edit":    e.get(t, e.syndic, "/finance/"+inst2+"/edit"),
		"GET cancel":  e.get(t, e.syndic, "/finance/"+inst2+"/cancel"),
		"POST cancel": e.post(t, e.syndic, "/finance/"+inst2+"/cancel", nil),
		"POST edit": e.post(t, e.syndic, "/finance/"+inst2+"/edit", url.Values{
			"type": {"receivable"}, "title": {"hacked"}, "category": {"extraordinary_income"}, "amount": {"1.00"},
			"due_date": {"2026-12-01"}, "status": {"pending"}}),
	} {
		if r.code != http.StatusConflict {
			t.Errorf("%s of an installment = %d, want 409", name, r.code)
		}
	}
	if st, _ := e.acctState(t, inst2); st != "pending" {
		t.Errorf("installment 2 status = %s, want pending", st)
	}

	if r := e.settle(t, inst1, "2026-11-14", "apply"); r.code != http.StatusSeeOther {
		t.Fatalf("settle installment and apply = %d\n%s", r.code, r.body)
	}
	lfx, ok := e.lateFeeOf(t, inst1)
	if !ok || lfx.Multa != 1600 || lfx.Juros != 1733 || lfx.Days != 65 || lfx.Amount != 3333 {
		t.Fatalf("installment late fee = %+v (found %v), want 1600/1733 over 65 days = 3333", lfx, ok)
	}
	// The ordinary quota of A also pays with an applied late fee (880 + 293).
	if r := e.settle(t, f.a, "2026-11-14", "apply"); r.code != http.StatusSeeOther {
		t.Fatalf("settle ordinary and apply = %d\n%s", r.code, r.body)
	}
	lfo, ok := e.lateFeeOf(t, f.a)
	if !ok || lfo.Amount != 1173 {
		t.Fatalf("ordinary late fee = %+v (found %v), want 1173", lfo, ok)
	}

	// AC-14: the resident of unit A reads the statement in pt-br.
	var unitA string
	if err := e.db.QueryRow(`SELECT id FROM units WHERE condominium_id=$1 AND code='A'`, e.condoA).Scan(&unitA); err != nil {
		t.Fatalf("unit A: %v", err)
	}
	e.occupy(t, "resident@example.com", unitA, "owner", "2026-01-01")
	if _, err := e.db.Exec(`UPDATE users SET language_preference='pt-br' WHERE email='resident@example.com'`); err != nil {
		t.Fatalf("set language: %v", err)
	}
	st := e.get(t, e.owner, "/finance/my-charges?unit="+unitA)
	if st.code != http.StatusOK {
		t.Fatalf("resident statement = %d\n%s", st.code, st.body)
	}
	mustContain(t, "pt-br statement", st.body,
		"Quota extraordinária — Obra da fachada — parcela 2/6",
		"Quota extraordinária — Obra da fachada — parcela 1/6",
		"Multa e juros — Taxa condominial",
		"Multa e juros — Quota extraordinária — Obra da fachada — parcela 1/6")
	mustNotContain(t, "pt-br statement", st.body, "Mora estimada", "Estimated late fee")

	// The Recibo of a settled late fee shows the origin, the fine, the interest and the days.
	if r := e.settle(t, lfo.ID, "2026-11-20", ""); r.code != http.StatusSeeOther {
		t.Fatalf("settle late fee = %d\n%s", r.code, r.body)
	}
	rec := e.get(t, e.owner, "/finance/"+lfo.ID+"/recibo")
	if rec.code != http.StatusOK {
		t.Fatalf("resident recibo of the late fee = %d\n%s", rec.code, rec.body)
	}
	mustContain(t, "late fee recibo", rec.body, "Multa e juros — Taxa condominial")
	if got, ok := ddAfter(rec.body, "Multa"); !ok || !strings.HasPrefix(got, money.Format(i18n.LanguagePTBR, money.BRL, 880)) {
		t.Errorf("recibo multa = %q (present %v), want %s", got, ok, money.Format(i18n.LanguagePTBR, money.BRL, 880))
	}
	if got, ok := ddAfter(rec.body, "Juros"); !ok || !strings.HasPrefix(got, money.Format(i18n.LanguagePTBR, money.BRL, 293)) {
		t.Errorf("recibo juros = %q (present %v), want %s", got, ok, money.Format(i18n.LanguagePTBR, money.BRL, 293))
	}
	if got, ok := ddAfter(rec.body, "Dias de atraso"); !ok || got != "20" {
		t.Errorf("recibo dias de atraso = %q (present %v), want 20", got, ok)
	}
}

// reserveFigures reads the three figures of the reserve fund card of a page.
func reserveFigures(t *testing.T, what, body string) [3]string {
	t.Helper()
	if !strings.Contains(body, `data-reserve-fund="card"`) {
		t.Fatalf("%s: no reserve fund card", what)
	}
	var out [3]string
	for i, key := range []string{"quotas.reserve_fund.realized", "quotas.reserve_fund.forecast", "quotas.reserve_fund.realized_year"} {
		got, ok := ddAfter(body, enT(key))
		if !ok {
			t.Fatalf("%s: no figure %q", what, enT(key))
		}
		out[i] = got
	}
	return out
}

// AC-15, AC-18 (reserve): the reserve fund balance.
func TestReserveFundBalance(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	year := time.Now().UTC().Year()
	prev := e.newApprovedBudget(t, year-1, "monthly", 8, "10")
	cur := e.newApprovedBudget(t, year, "monthly", 8, "10")
	for _, p := range []struct {
		budget string
		period int
	}{{prev, 12}, {cur, 1}, {cur, 2}} {
		if r := e.issue(t, p.budget, p.period); r.code != http.StatusSeeOther {
			t.Fatalf("issue %s/%d = %d\n%s", p.budget, p.period, r.code, r.body)
		}
	}
	// One settled quota last year, one this year; the rest is pending.
	prevA := e.quotaAccountID(t, e.activeIssuanceID(t, prev, 12), "A")
	curB := e.quotaAccountID(t, e.activeIssuanceID(t, cur, 1), "B")
	if r := e.settle(t, prevA, fmt.Sprintf("%d-12-20", year-1), "waive"); r.code != http.StatusSeeOther {
		t.Fatalf("settle last year = %d\n%s", r.code, r.body)
	}
	if r := e.settle(t, curB, fmt.Sprintf("%d-01-31", year), "waive"); r.code != http.StatusSeeOther {
		t.Fatalf("settle this year = %d\n%s", r.code, r.body)
	}

	sumReserve := func(extra string, args ...any) int64 {
		var n int64
		q := `SELECT COALESCE(SUM(c.reserve_cents),0) FROM quota_charges c JOIN finance_accounts a ON a.id=c.finance_account_id WHERE a.condominium_id=$1 ` + extra
		if err := e.db.QueryRow(q, append([]any{e.condoA}, args...)...).Scan(&n); err != nil {
			t.Fatalf("sum reserve: %v", err)
		}
		return n
	}
	realized := sumReserve(`AND a.status='settled'`)
	pending := sumReserve(`AND a.status='pending'`)
	realizedYear := sumReserve(`AND a.status='settled' AND EXTRACT(YEAR FROM a.settlement_date)=$2`, year)
	if realizedYear <= 0 || realized <= realizedYear || pending <= 0 {
		t.Fatalf("fixture reserve realized=%d year=%d pending=%d, want 0 < year < realized and pending > 0", realized, realizedYear, pending)
	}
	want := [3]string{fmtMoney(realized), fmtMoney(realized + pending), fmtMoney(realizedYear)}

	ov := e.get(t, e.syndic, "/finance/overview", hx)
	if ov.code != http.StatusOK {
		t.Fatalf("overview = %d", ov.code)
	}
	if got := reserveFigures(t, "overview", ov.body); got != want {
		t.Errorf("overview reserve figures = %v, want %v", got, want)
	}
	bv := e.get(t, e.syndic, "/finance/budgets/"+cur+"/view")
	if bv.code != http.StatusOK {
		t.Fatalf("budget view = %d", bv.code)
	}
	if got := reserveFigures(t, "budget page", bv.body); got != want {
		t.Errorf("budget page reserve figures = %v, want %v", got, want)
	}

	// Extraordinary installments have no reserve: the figures do not move.
	e.createExtra(t, "Obra", "3000.00", "share", 3, "2026-12-10")
	ov = e.get(t, e.syndic, "/finance/overview", hx)
	if got := reserveFigures(t, "overview after extraordinary", ov.body); got != want {
		t.Errorf("reserve figures after an extraordinary quota = %v, want %v", got, want)
	}
	// Another condominium has its own balance.
	if r := e.get(t, e.syndicB, "/finance/overview", hx); strings.Contains(r.body, fmtMoney(realized)) && realized != 0 {
		t.Errorf("condominium B overview shows the reserve of A")
	}
}

// AC-19: simultaneous "apply" settlements of one quota produce one settlement
// and at most one late fee, over several rounds.
func TestLateFeeConcurrentApply(t *testing.T) {
	e := newQuotaEnv(t)
	e.addStdUnits(t)
	budget := e.newApprovedBudget(t, 2026, "monthly", 25, "10")
	clients := []*http.Client{e.syndic,
		loginAs(t, e.base, "syndic@example.com", "syndic-pass-123"),
		loginAs(t, e.base, "syndic@example.com", "syndic-pass-123")}
	tokens := make([]string, len(clients))
	for i, c := range clients {
		tokens[i] = e.csrf(t, c)
	}
	const rounds = 6
	for round := 1; round <= rounds; round++ {
		if r := e.issue(t, budget, round); r.code != http.StatusSeeOther {
			t.Fatalf("round %d: issue = %d", round, r.code)
		}
		acct := e.quotaAccountID(t, e.activeIssuanceID(t, budget, round), "A")
		before := e.count(t, `SELECT count(*) FROM finance_accounts WHERE category='fine_interest'`)
		codes := make([]int, len(clients))
		errs := make([]error, len(clients))
		fns := make([]func(), len(clients))
		for i := range clients {
			fns[i] = func() {
				codes[i], _, errs[i] = e.postRaw(clients[i], tokens[i], "/finance/"+acct+"/settle",
					url.Values{"settlement_date": {"2026-11-14"}, "late_fee_decision": {"apply"}})
			}
		}
		runConcurrently(fns...)
		wins := 0
		for i, code := range codes {
			if errs[i] != nil {
				t.Fatalf("round %d: request %d: %v", round, i, errs[i])
			}
			if code == http.StatusSeeOther {
				wins++
			} else if code >= 500 {
				t.Errorf("round %d: request %d = %d, want a clean refusal", round, i, code)
			}
		}
		if wins != 1 {
			t.Errorf("round %d: %d requests succeeded (%v), want exactly 1", round, wins, codes)
		}
		if n := e.auditDetail(t, "account_settled", "account_id", acct); n != 1 {
			t.Errorf("round %d: account_settled events = %d, want 1", round, n)
		}
		if n := e.count(t, `SELECT count(*) FROM quota_late_fees WHERE quota_account_id=$1`, acct); n != 1 {
			t.Errorf("round %d: quota_late_fees rows = %d, want 1", round, n)
		}
		if n := e.count(t, `SELECT count(*) FROM finance_accounts WHERE category='fine_interest'`); n != before+1 {
			t.Errorf("round %d: late fee receivables = %d, want %d", round, n, before+1)
		}
		if n := e.auditDetail(t, "late_fee_applied", "quota_account_id", acct); n != 1 {
			t.Errorf("round %d: late_fee_applied events = %d, want 1", round, n)
		}
	}
}

// R14: quota_issuances belongs to exactly one of a budget and an extraordinary quota.
func TestQuotaIssuanceOwnerCheck(t *testing.T) {
	e := newQuotaEnv(t)
	budget := e.createBudget(t, 2026, "monthly", 25, "")
	var extra string
	if err := e.db.QueryRow(`
		INSERT INTO quota_extraordinaries (condominium_id, description, total_cents, division, installments, first_due_date, created_by)
		VALUES ($1, 'Obra', 100000, 'equal', 2, '2026-12-10', $2) RETURNING id`, e.condoA, e.syndicAID).Scan(&extra); err != nil {
		t.Fatalf("insert extraordinary: %v", err)
	}
	insert := func(budgetID, extraID any, period int) error {
		_, err := e.db.Exec(`
			INSERT INTO quota_issuances (condominium_id, budget_id, extraordinary_id, period_index, total_cents, base_cents, reserve_cents, created_by)
			VALUES ($1, $2, $3, $4, 100000, 100000, 0, $5)`, e.condoA, budgetID, extraID, period, e.syndicAID)
		return err
	}
	for name, err := range map[string]error{
		"both budget_id and extraordinary_id": insert(budget, extra, 1),
		"neither":                             insert(nil, nil, 2),
	} {
		if err == nil {
			t.Errorf("issuance with %s was accepted", name)
		} else if !strings.Contains(err.Error(), "quota_issuances_owner_check") {
			t.Errorf("issuance with %s failed with %v, want quota_issuances_owner_check", name, err)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM quota_issuances`); n != 0 {
		t.Fatalf("issuances after the refused inserts = %d, want 0", n)
	}
	// The valid shapes are accepted, so the refusals above are the CHECK and nothing else.
	if err := insert(budget, nil, 1); err != nil {
		t.Errorf("issuance of a budget only: %v", err)
	}
	if err := insert(nil, extra, 1); err != nil {
		t.Errorf("issuance of an extraordinary quota only: %v", err)
	}
}
