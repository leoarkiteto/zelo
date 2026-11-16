package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

var receivableCategories = map[string]bool{
	"condo_fee": true, "fine_interest": true, "common_area_reservation": true, "extraordinary_income": true,
}

var payableCategories = map[string]bool{
	"maintenance": true, "cleaning": true, "utilities": true, "payroll": true,
	"third_party_services": true, "other": true,
}

func TestSeedHistoryCoversTwelveMonths(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 31, 23, 0, 0, 0, time.UTC),
	} {
		today := now.UTC().Truncate(24 * time.Hour)
		type totals struct{ in, out int64 }
		months := map[string]*totals{}
		for _, h := range seedHistory(now) {
			if h.amountCents <= 0 {
				t.Fatalf("non-positive amount: %+v", h)
			}
			if h.settlementDate.After(today) {
				t.Fatalf("settlement in the future: %+v", h)
			}
			key := h.settlementDate.Format("2006-01")
			if months[key] == nil {
				months[key] = &totals{}
			}
			switch h.accountType {
			case "receivable":
				if !receivableCategories[h.category] {
					t.Fatalf("invalid receivable category %q", h.category)
				}
				months[key].in += h.amountCents
			case "payable":
				if !payableCategories[h.category] {
					t.Fatalf("invalid payable category %q", h.category)
				}
				if h.unit != "" || h.paymentCode != "" {
					t.Fatalf("payable must not carry unit/payment code: %+v", h)
				}
				months[key].out += h.amountCents
			default:
				t.Fatalf("unknown account type %q", h.accountType)
			}
		}
		if len(months) != seedHistoryMonths {
			t.Fatalf("now=%s: got %d months, want %d", now.Format("2006-01-02"), len(months), seedHistoryMonths)
		}
		deficit := false
		for k, m := range months {
			if m.in == 0 || m.out == 0 {
				t.Errorf("month %s missing entradas or saídas: %+v", k, *m)
			}
			if m.out > m.in {
				deficit = true
			}
		}
		if !deficit {
			t.Errorf("now=%s: no month with saídas > entradas", now.Format("2006-01-02"))
		}
	}
}

// TestSeedNoDuplicateFeesAcrossBaseAndHistory combines the base fees with the
// history rows the seed actually inserts and rejects any repeated
// (unit, month) condo fee or payment code. Since the quota layer owns months
// -1..+1, it also requires that no manual condo fee falls in that window.
func TestSeedNoDuplicateFeesAcrossBaseAndHistory(t *testing.T) {
	for _, now := range []time.Time{
		time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
	} {
		unitMonths := map[string]bool{}
		codes := map[string]bool{}
		add := func(unit, month, code string) {
			key := unit + "|" + month
			if unitMonths[key] {
				t.Errorf("now=%s: duplicate condo fee for %s", now.Format("2006-01-02"), key)
			}
			unitMonths[key] = true
			if codes[code] {
				t.Errorf("now=%s: duplicate payment code %s", now.Format("2006-01-02"), code)
			}
			codes[code] = true
		}
		window := map[string]bool{}
		for _, off := range []int{-1, 0, 1} {
			window[monthDay(now, off, 1).Format("2006-01")] = true
		}
		for _, f := range seedFeesToInsert() {
			month := seedFeeDueDate(f, now).Format("2006-01")
			if window[month] {
				t.Errorf("now=%s: base fee %s in quota month %s", now.Format("2006-01-02"), f.unit, month)
			}
			add(f.unit, month, seedFeePaymentCode(month, f.unit))
		}
		older := 0
		for _, h := range seedHistoryToInsert(now) {
			if h.accountType != "receivable" || h.category != "condo_fee" {
				continue
			}
			month := h.dueDate.Format("2006-01")
			if window[month] || seedIsQuotaMonth(h.monthOffset) {
				t.Errorf("now=%s: history fee %s in quota month %s", now.Format("2006-01-02"), h.unit, month)
			}
			older++
			add(h.unit, month, h.paymentCode)
		}
		// Older history (months -11..-2) keeps one fee per unit per month.
		if want := len(seedFees) * (seedHistoryMonths - 2); older != want {
			t.Errorf("now=%s: %d older history fees, want %d", now.Format("2006-01-02"), older, want)
		}
	}
}

func TestSeedFeesToInsertEmptyInQuotaWindow(t *testing.T) {
	if got := seedFeesToInsert(); len(got) != 0 {
		t.Fatalf("base fees are all in months -1/+1, got %d manual fees", len(got))
	}
	for off, want := range map[int]bool{-2: false, -1: true, 0: true, 1: true, 2: false} {
		if seedIsQuotaMonth(off) != want {
			t.Errorf("seedIsQuotaMonth(%d) = %v, want %v", off, !want, want)
		}
	}
}

func TestSeedSharesSumTo100Percent(t *testing.T) {
	sum := 0
	for _, code := range seedUnits {
		ppm, ok := seedSharePPM[code]
		if !ok || ppm <= 0 {
			t.Fatalf("unit %s has no positive share", code)
		}
		sum += ppm
	}
	if sum != 1000000 || len(seedSharePPM) != len(seedUnits) {
		t.Fatalf("shares sum %d over %d units, want 1000000 over %d", sum, len(seedSharePPM), len(seedUnits))
	}
}

func TestSeedBudgetLinesTotal(t *testing.T) {
	var total int64
	for _, l := range seedBudgetLines {
		if !payableCategories[string(l.category)] || l.amountCents <= 0 {
			t.Fatalf("invalid budget line %+v", l)
		}
		total += l.amountCents
	}
	if total != 15600000 || len(seedBudgetLines) != 6 {
		t.Fatalf("budget total %d over %d lines, want 15600000 over 6", total, len(seedBudgetLines))
	}
}

func TestSeedQuotaPeriods(t *testing.T) {
	cases := []struct {
		now        time.Time
		prev, cur  seedQuotaPeriod
		prevSettle string
		curSettle  string
	}{
		{time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC), seedQuotaPeriod{2026, 10}, seedQuotaPeriod{2026, 11}, "2026-10-20", "2026-11-05"},
		{time.Date(2027, 1, 3, 9, 0, 0, 0, time.UTC), seedQuotaPeriod{2026, 12}, seedQuotaPeriod{2027, 1}, "2026-12-20", "2027-01-03"},
		{time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), seedQuotaPeriod{2026, 2}, seedQuotaPeriod{2026, 3}, "2026-02-20", "2026-03-01"},
	}
	for _, c := range cases {
		prev, cur := seedQuotaPeriods(c.now)
		if prev != c.prev || cur != c.cur {
			t.Errorf("now=%s: periods %v/%v, want %v/%v", c.now.Format("2006-01-02"), prev, cur, c.prev, c.cur)
		}
		if got := seedPrevSettleDate(c.now).Format("2006-01-02"); got != c.prevSettle {
			t.Errorf("now=%s: prev settle %s, want %s", c.now.Format("2006-01-02"), got, c.prevSettle)
		}
		if got := seedCurrentSettleDate(c.now); got.Format("2006-01-02") != c.curSettle ||
			got.After(c.now.UTC().Truncate(24*time.Hour)) {
			t.Errorf("now=%s: current settle %s, want %s and not in the future", c.now.Format("2006-01-02"), got.Format("2006-01-02"), c.curSettle)
		}
	}
}

func TestSeedLegacyFeeCodePatterns(t *testing.T) {
	got := seedLegacyFeeCodePatterns(time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC))
	want := [3]string{"BOLETO-2026-11-%", "BOLETO-2026-12-%", "BOLETO-2027-01-%"}
	if got != want {
		t.Fatalf("patterns %v, want %v", got, want)
	}
}

func TestSeedPeriodIndex(t *testing.T) {
	if seedPeriodIndex("monthly", 11) != 11 || seedPeriodIndex("quarterly", 11) != 4 || seedPeriodIndex("quarterly", 1) != 1 {
		t.Fatal("unexpected period index")
	}
}

func TestSeedHistoryPaymentCodesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, h := range seedHistory(time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)) {
		if h.paymentCode == "" {
			continue
		}
		key := h.paymentCode
		if seen[key] {
			t.Fatalf("duplicate payment code %s", key)
		}
		seen[key] = true
	}
}

// seedTestNow is the pinned clock of the DB-backed tests (not January).
var seedTestNow = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

// seedTestDB opens the dedicated seed test database with an emptied, migrated
// schema (never the app data). It skips without TEST_DATABASE_URL.
func seedTestDB(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	url := testutil.TestDatabaseURL("seed")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping DB-backed seed test")
	}
	db, err := store.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	if err := store.Migrate(ctx, db, "../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db, ctx
}

func seedTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func seedScalar(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	var s sql.NullString
	if err := db.QueryRow(query, args...).Scan(&s); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return s.String
}

// seedQuotaSnapshot is a deterministic text dump of the whole quota state, so
// two runs can be compared for "no change".
func seedQuotaSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	queries := []string{
		`SELECT 'units:' || string_agg(code || '=' || share_ppm, ',' ORDER BY code) FROM units`,
		`SELECT 'budgets:' || coalesce(string_agg(year || '/' || status || '/' || periodicity || '/' || due_day || '/' || reserve_rate_bp, ',' ORDER BY year), '') FROM quota_budgets`,
		`SELECT 'lines:' || count(*) FROM quota_budget_lines`,
		`SELECT 'issuances:' || coalesce(string_agg(period_index || '/' || status, ',' ORDER BY period_index), '') FROM quota_issuances`,
		`SELECT 'charges:' || coalesce(string_agg(i.period_index || '/' || u.code || '/' || fa.status || '/' || coalesce(fa.settlement_date::text, '-') || '/' || fa.amount_cents,
			',' ORDER BY i.period_index, u.code), '')
		 FROM quota_charges qc JOIN quota_issuances i ON i.id = qc.issuance_id
		 JOIN finance_accounts fa ON fa.id = qc.finance_account_id JOIN units u ON u.id = fa.unit_id`,
		`SELECT 'accounts:' || count(*) FROM finance_accounts`,
		`SELECT 'audit:' || count(*) FROM audit_events`,
	}
	var out []string
	for _, q := range queries {
		out = append(out, seedScalar(t, db, q))
	}
	return strings.Join(out, "\n")
}

// assertSeedQuotaState checks AC-1..AC-4 for seedTestNow (October = previous,
// November = current period).
func assertSeedQuotaState(t *testing.T, db *sql.DB) {
	t.Helper()
	if got := seedScalar(t, db, `SELECT sum(share_ppm)::text FROM units`); got != "1000000" {
		t.Errorf("share sum = %s, want 1000000", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text FROM units WHERE share_ppm > 0`); got != "12" {
		t.Errorf("units with share = %s, want 12", got)
	}
	if got := seedScalar(t, db, `SELECT count(*) || '/' || min(status) || '/' || min(periodicity) || '/' || min(due_day) || '/' || min(reserve_rate_bp) FROM quota_budgets WHERE year = 2026`); got != "1/approved/monthly/25/1000" {
		t.Errorf("budget 2026 = %s", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text || '/' || sum(amount_cents) FROM quota_budget_lines`); got != "6/15600000" {
		t.Errorf("budget lines = %s, want 6/15600000", got)
	}
	if got := seedScalar(t, db, `SELECT string_agg(period_index || '/' || status, ',' ORDER BY period_index) FROM quota_issuances`); got != "10/issued,11/issued,12/issued" {
		t.Errorf("issuances = %s", got)
	}
	// Receivables of each issuance sum to its total.
	if got := seedScalar(t, db, `SELECT count(*)::text FROM quota_issuances i
		WHERE i.total_cents <> (SELECT sum(fa.amount_cents) FROM quota_charges qc JOIN finance_accounts fa ON fa.id = qc.finance_account_id WHERE qc.issuance_id = i.id)`); got != "0" {
		t.Errorf("%s issuances whose receivables do not sum to the total", got)
	}
	const charges = `FROM quota_charges qc JOIN quota_issuances i ON i.id = qc.issuance_id
		JOIN finance_accounts fa ON fa.id = qc.finance_account_id JOIN units u ON u.id = fa.unit_id WHERE i.period_index = $1`
	if got := seedScalar(t, db, `SELECT count(*)::text `+charges, 10); got != "12" {
		t.Errorf("previous period charges = %s, want 12", got)
	}
	if got := seedScalar(t, db, `SELECT string_agg(u.code, ',' ORDER BY u.code) `+charges+` AND fa.status = 'pending'`, 10); got != "A-301" {
		t.Errorf("previous period pending = %q, want A-301", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text `+charges+` AND fa.status = 'settled' AND fa.settlement_date BETWEEN '2026-10-01' AND '2026-10-31'`, 10); got != "11" {
		t.Errorf("previous period settled in October = %s, want 11", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text `+charges+` AND fa.status = 'pending' AND fa.due_date < $2`, 10, seedTestNow); got != "1" {
		t.Errorf("overdue previous quotas = %s, want 1", got)
	}
	if got := seedScalar(t, db, `SELECT string_agg(u.code, ',' ORDER BY u.code) `+charges+` AND fa.status = 'settled' AND fa.settlement_date <= $2`, 11, seedTestNow); got != "A-101,A-102,B-101" {
		t.Errorf("current period settled = %q", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text `+charges+` AND fa.status = 'pending'`, 11); got != "9" {
		t.Errorf("current period pending = %s, want 9", got)
	}
	// Next period (I2): issued, every quota pending, so month +1 has charges.
	if got := seedScalar(t, db, `SELECT count(*)::text || '/' || count(*) FILTER (WHERE fa.status = 'pending') `+charges, 12); got != "12/12" {
		t.Errorf("next period charges/pending = %s, want 12/12", got)
	}
	// No manual condo fee in months -1..+1; older history is kept.
	if got := seedScalar(t, db, `SELECT count(*)::text FROM finance_accounts
		WHERE category = 'condo_fee' AND id NOT IN (SELECT finance_account_id FROM quota_charges)
		AND due_date >= '2026-10-01' AND due_date < '2027-01-01'`); got != "0" {
		t.Errorf("manual condo fees in the quota window = %s, want 0", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text FROM finance_accounts
		WHERE category = 'condo_fee' AND id NOT IN (SELECT finance_account_id FROM quota_charges)`); got != "100" {
		t.Errorf("older manual condo fees = %s, want 100 (10 units x months -11..-2)", got)
	}
}

func TestSeedQuotaLayerFreshAndRepeat(t *testing.T) {
	db, ctx := seedTestDB(t)
	logger := seedTestLogger()
	if err := seedDemoDataAt(ctx, logger, db, "pepper", seedTestNow); err != nil {
		t.Fatalf("fresh seed: %v", err)
	}
	assertSeedQuotaState(t, db)
	first := seedQuotaSnapshot(t, db)

	if err := seedDemoDataAt(ctx, logger, db, "pepper", seedTestNow); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if second := seedQuotaSnapshot(t, db); second != first {
		t.Fatalf("second run changed the state:\n--- first\n%s\n--- second\n%s", first, second)
	}
}

func TestSeedQuotaLayerOverOldSeed(t *testing.T) {
	db, ctx := seedTestDB(t)
	logger := seedTestLogger()
	// Simulate a DB seeded by the old seed: run the base step (which no longer
	// makes fees for months -1..+1) and add the legacy manual condo fees of
	// months -1, 0 and +1 with the seed payment-code pattern.
	if err := seedBase(ctx, logger, db, "pepper", seedTestNow); err != nil {
		t.Fatalf("base seed: %v", err)
	}
	for _, off := range []int{-1, 0, 1} {
		month := monthDay(seedTestNow, off, 1)
		for _, code := range seedUnits {
			if _, err := db.Exec(`INSERT INTO finance_accounts
				(condominium_id, account_type, title, category, amount_cents, due_date, status, unit_id, payment_code, created_by)
				SELECT c.id, 'receivable', 'Taxa condominial', 'condo_fee', 100000, $1, 'pending', u.id, $2, (SELECT id FROM users LIMIT 1)
				FROM condominiums c JOIN units u ON u.condominium_id = c.id AND u.code = $3`,
				monthDay(seedTestNow, off, 10), seedFeePaymentCode(month.Format("2006-01"), code), code); err != nil {
				t.Fatalf("legacy fee %s %s: %v", month.Format("2006-01"), code, err)
			}
		}
	}
	// A manual condo fee that does not match the seed pattern must survive.
	if _, err := db.Exec(`INSERT INTO finance_accounts
		(condominium_id, account_type, title, category, amount_cents, due_date, status, unit_id, payment_code, created_by)
		SELECT c.id, 'receivable', 'Taxa avulsa', 'condo_fee', 5000, '2026-11-12', 'pending', u.id, 'AVULSA-1', (SELECT id FROM users LIMIT 1)
		FROM condominiums c JOIN units u ON u.condominium_id = c.id AND u.code = 'A-101'`); err != nil {
		t.Fatalf("keep row: %v", err)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text FROM finance_accounts WHERE payment_code LIKE 'BOLETO-2026-1%' AND due_date >= '2026-10-01'`); got != fmt.Sprint(3*len(seedUnits)) {
		t.Fatalf("legacy setup rows = %s", got)
	}

	if err := seedQuotas(ctx, logger, db, seedTestNow); err != nil {
		t.Fatalf("quota layer: %v", err)
	}
	// Only the non-matching manual row is left in the window, so the AC-4
	// assertion (no manual condo fee there) is checked after removing it.
	if got := seedScalar(t, db, `SELECT payment_code FROM finance_accounts WHERE category = 'condo_fee'
		AND id NOT IN (SELECT finance_account_id FROM quota_charges) AND due_date >= '2026-10-01'`); got != "AVULSA-1" {
		t.Fatalf("manual condo fee left in the window = %q, want only AVULSA-1", got)
	}
	if _, err := db.Exec(`DELETE FROM finance_accounts WHERE payment_code = 'AVULSA-1'`); err != nil {
		t.Fatal(err)
	}
	assertSeedQuotaState(t, db)

	// A further full run (base skipped, quota layer again) changes nothing.
	if _, err := db.Exec(`INSERT INTO finance_accounts
		(condominium_id, account_type, title, category, amount_cents, due_date, status, unit_id, payment_code, created_by)
		SELECT c.id, 'receivable', 'Taxa avulsa', 'condo_fee', 5000, '2026-11-12', 'pending', u.id, 'AVULSA-1', (SELECT id FROM users LIMIT 1)
		FROM condominiums c JOIN units u ON u.condominium_id = c.id AND u.code = 'A-101'`); err != nil {
		t.Fatalf("re-add keep row: %v", err)
	}
	before := seedQuotaSnapshot(t, db)
	if err := seedDemoDataAt(ctx, logger, db, "pepper", seedTestNow); err != nil {
		t.Fatalf("repeat seed: %v", err)
	}
	if after := seedQuotaSnapshot(t, db); after != before {
		t.Fatalf("repeat run changed the state:\n--- before\n%s\n--- after\n%s", before, after)
	}
}

// TestSeedQuotaLayerJanuary (I3): in January the previous period belongs to the
// previous year, so the seed needs two budgets; the next period is February.
func TestSeedQuotaLayerJanuary(t *testing.T) {
	db, ctx := seedTestDB(t)
	logger := seedTestLogger()
	now := time.Date(2027, 1, 14, 10, 0, 0, 0, time.UTC)
	if err := seedDemoDataAt(ctx, logger, db, "pepper", now); err != nil {
		t.Fatalf("January seed: %v", err)
	}
	if got := seedScalar(t, db, `SELECT string_agg(year || '/' || status, ',' ORDER BY year) FROM quota_budgets`); got != "2026/approved,2027/approved" {
		t.Errorf("budgets = %s, want 2026 and 2027 approved", got)
	}
	if got := seedScalar(t, db, `SELECT string_agg(b.year || '-' || i.period_index || '/' || i.status, ',' ORDER BY b.year, i.period_index)
		FROM quota_issuances i JOIN quota_budgets b ON b.id = i.budget_id`); got != "2026-12/issued,2027-1/issued,2027-2/issued" {
		t.Errorf("issuances = %s", got)
	}
	const charges = `FROM quota_charges qc JOIN quota_issuances i ON i.id = qc.issuance_id
		JOIN quota_budgets b ON b.id = i.budget_id
		JOIN finance_accounts fa ON fa.id = qc.finance_account_id JOIN units u ON u.id = fa.unit_id
		WHERE b.year = $1 AND i.period_index = $2`
	if got := seedScalar(t, db, `SELECT string_agg(u.code, ',' ORDER BY u.code) `+charges+` AND fa.status = 'pending'`, 2026, 12); got != "A-301" {
		t.Errorf("December pending = %q, want A-301", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text `+charges+` AND fa.status = 'settled' AND fa.settlement_date = '2026-12-20'`, 2026, 12); got != "11" {
		t.Errorf("December settled on 2026-12-20 = %s, want 11", got)
	}
	if got := seedScalar(t, db, `SELECT string_agg(u.code, ',' ORDER BY u.code) `+charges+` AND fa.status = 'settled' AND fa.settlement_date = '2027-01-05'`, 2027, 1); got != "A-101,A-102,B-101" {
		t.Errorf("January settled = %q", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text || '/' || count(*) FILTER (WHERE fa.status = 'pending') `+charges, 2027, 2); got != "12/12" {
		t.Errorf("February charges/pending = %s, want 12/12", got)
	}

	first := seedQuotaSnapshot(t, db)
	if err := seedDemoDataAt(ctx, logger, db, "pepper", now); err != nil {
		t.Fatalf("repeat January seed: %v", err)
	}
	if second := seedQuotaSnapshot(t, db); second != first {
		t.Fatalf("repeat run changed the state:\n--- first\n%s\n--- second\n%s", first, second)
	}
}

// TestSeedNextQuotaPeriod: the next period rolls over the year in December.
func TestSeedNextQuotaPeriod(t *testing.T) {
	for _, c := range []struct {
		now  time.Time
		want seedQuotaPeriod
	}{
		{time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), seedQuotaPeriod{Year: 2026, Month: 12}},
		{time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), seedQuotaPeriod{Year: 2027, Month: 1}},
		{time.Date(2027, 1, 14, 0, 0, 0, 0, time.UTC), seedQuotaPeriod{Year: 2027, Month: 2}},
	} {
		if got := seedNextQuotaPeriod(c.now); got != c.want {
			t.Errorf("seedNextQuotaPeriod(%s) = %+v, want %+v", c.now.Format("2006-01-02"), got, c.want)
		}
	}
}

// TestSeedQuotaLayerSkipsWhenSharesNot100 (I1): with shares that do not sum to
// 100% the quota layer returns before deleting the legacy fees or creating a
// budget, so the demo is never left with neither fees nor quotas.
func TestSeedQuotaLayerSkipsWhenSharesNot100(t *testing.T) {
	db, ctx := seedTestDB(t)
	logger := seedTestLogger()
	if err := seedBase(ctx, logger, db, "pepper", seedTestNow); err != nil {
		t.Fatalf("base seed: %v", err)
	}
	// Non-zero shares summing to 60%: left untouched, so the sum stays wrong.
	if _, err := db.Exec(`UPDATE units SET share_ppm = 300000 WHERE code IN ('A-101', 'B-101')`); err != nil {
		t.Fatal(err)
	}
	// Legacy manual fees of the quota window, as the old seed left them.
	for _, off := range []int{-1, 0, 1} {
		month := monthDay(seedTestNow, off, 1)
		if _, err := db.Exec(`INSERT INTO finance_accounts
			(condominium_id, account_type, title, category, amount_cents, due_date, status, unit_id, payment_code, created_by)
			SELECT c.id, 'receivable', 'Taxa condominial', 'condo_fee', 100000, $1, 'pending', u.id, $2, (SELECT id FROM users LIMIT 1)
			FROM condominiums c JOIN units u ON u.condominium_id = c.id AND u.code = 'A-101'`,
			monthDay(seedTestNow, off, 10), seedFeePaymentCode(month.Format("2006-01"), "A-101")); err != nil {
			t.Fatalf("legacy fee: %v", err)
		}
	}

	if err := seedDemoDataAt(ctx, logger, db, "pepper", seedTestNow); err != nil {
		t.Fatalf("seed with shares not at 100%% returned an error: %v", err)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text FROM finance_accounts WHERE payment_code LIKE 'BOLETO-%-A-101' AND due_date >= '2026-10-01'`); got != "3" {
		t.Errorf("legacy fees in the quota window = %s, want 3 (kept)", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text FROM quota_budgets`); got != "0" {
		t.Errorf("budgets = %s, want 0", got)
	}
	if got := seedScalar(t, db, `SELECT count(*)::text FROM quota_issuances`); got != "0" {
		t.Errorf("issuances = %s, want 0", got)
	}
	if got := seedScalar(t, db, `SELECT sum(share_ppm)::text FROM units`); got != "600000" {
		t.Errorf("shares changed: sum = %s, want 600000", got)
	}
}

// TestSeedQuotaLayerKeepsExistingShares: non-zero shares are never overwritten.
func TestSeedQuotaLayerKeepsExistingShares(t *testing.T) {
	db, ctx := seedTestDB(t)
	logger := seedTestLogger()
	if err := seedBase(ctx, logger, db, "pepper", seedTestNow); err != nil {
		t.Fatalf("base seed: %v", err)
	}
	if _, err := db.Exec(`UPDATE units SET share_ppm = 500000 WHERE code IN ('A-101', 'B-101')`); err != nil {
		t.Fatal(err)
	}
	if err := seedQuotas(ctx, logger, db, seedTestNow); err != nil {
		t.Fatalf("quota layer with 100%% custom shares: %v", err)
	}
	if got := seedScalar(t, db, `SELECT string_agg(code || '=' || share_ppm, ',' ORDER BY code) FROM units WHERE share_ppm > 0`); got != "A-101=500000,B-101=500000" {
		t.Fatalf("custom shares changed: %s", got)
	}
}
