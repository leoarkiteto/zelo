package repositories

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

var _ services.LockedQuotaAccount = (*lockedQuotaAccount)(nil)
var _ services.QuotaStore = (*QuotaStore)(nil)
var _ services.LateFeeLinks = (*QuotaStore)(nil)

// TestLateFeeSettleApplyWaiveDebtorsAndReserve uses two units (60 % / 40 %) and
// a monthly budget of 1200.00 a year with a 10 % reserve, so a period is 11000:
// A-1 pays 6600 (reserve 600) and A-2 4400 (reserve 400); periods 1 and 2 of
// 2026 are issued, due on the 5th.
func TestLateFeeSettleApplyWaiveDebtorsAndReserve(t *testing.T) {
	sqldb := openFinanceTestDB(t)
	ctx := context.Background()
	condoID, unit1, userID := seedFinanceFixture(t, sqldb)
	var unit2 string
	if err := sqldb.QueryRowContext(ctx,
		`INSERT INTO units (condominium_id, code, share_ppm) VALUES ($1, 'A-2', 400000) RETURNING id`, condoID).Scan(&unit2); err != nil {
		t.Fatal(err)
	}
	if _, err := sqldb.ExecContext(ctx, `UPDATE units SET share_ppm = 600000 WHERE id = $1`, unit1); err != nil {
		t.Fatal(err)
	}
	// Unit A-1 has two active occupants (owner and tenant) and one who left;
	// the debtors query must neither duplicate nor lose rows because of them.
	occ := func(email string, typ string, ended bool) {
		var uid string
		if err := sqldb.QueryRowContext(ctx,
			`INSERT INTO users (email, password_hash) VALUES ($1, 'x') RETURNING id`, email).Scan(&uid); err != nil {
			t.Fatal(err)
		}
		q := `INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type) VALUES ($1, $2, $3)`
		if ended {
			q = `INSERT INTO unit_occupancies (user_id, unit_id, occupancy_type, ended_at) VALUES ($1, $2, $3, now())`
		}
		if _, err := sqldb.ExecContext(ctx, q, uid, unit1, typ); err != nil {
			t.Fatal(err)
		}
	}
	occ("owner@example.com", "owner", false)
	occ("tenant@example.com", "tenant", false)
	occ("gone@example.com", "tenant", true)

	qs := NewQuotaStore(sqldb)
	svc := &services.QuotaService{
		Store: qs, Units: store.NewUnitStore(sqldb), Occupancies: store.NewUnitStore(sqldb),
		Currency: func(context.Context, string) (money.Currency, error) { return money.BRL, nil },
		LateFees: func(context.Context, string) (model.LateFeeSettings, error) {
			return model.LateFeeSettings{FeeBP: 200, InterestBP: 100}, nil
		},
		Now: func() time.Time { return time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC) },
	}
	budgetID, err := qs.CreateBudget(ctx, services.Budget{CondominiumID: condoID, Year: 2026,
		Periodicity: domain.PeriodicityMonthly, DueDay: 5, ReserveRateBP: 1000, CreatedBy: userID})
	if err != nil {
		t.Fatal(err)
	}
	if err := qs.WithLockedBudget(ctx, condoID, budgetID, func(_ context.Context, b services.LockedBudget) error {
		if _, err := b.AddLine(services.BudgetLine{Description: "Limpeza", Category: domain.CategoryCleaning, AmountCents: 120000}); err != nil {
			return err
		}
		return b.SetStatus(domain.BudgetApproved)
	}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []int{1, 2} {
		if _, _, err := svc.Issue(ctx, userID, condoID, budgetID, p); err != nil {
			t.Fatal(err)
		}
	}
	// quotaOf returns the account id of a unit's quota of a period.
	quotaOf := func(unitID string, period int) string {
		t.Helper()
		var id string
		if err := sqldb.QueryRowContext(ctx, `
			SELECT c.finance_account_id FROM quota_charges c JOIN quota_issuances i ON i.id = c.issuance_id
			WHERE c.unit_id = $1 AND i.period_index = $2`, unitID, period).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	a1p1, a1p2, a2p1, a2p2 := quotaOf(unit1, 1), quotaOf(unit1, 2), quotaOf(unit2, 1), quotaOf(unit2, 2)
	jan15 := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	// ---- locked account seam ----
	if err := qs.WithLockedQuotaAccount(ctx, condoID, "not-a-uuid", func(context.Context, services.LockedQuotaAccount) error { return nil }); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("malformed id = %v, want ErrNotFound", err)
	}
	if _, err := qs.QuotaAccount(ctx, condoID, a1p1); err != nil {
		t.Fatal(err)
	}
	other := ""
	if err := sqldb.QueryRowContext(ctx, `INSERT INTO condominiums (name) VALUES ('Other') RETURNING id`).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if _, err := qs.QuotaAccount(ctx, other, a1p1); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("other condominium = %v, want ErrNotFound", err)
	}

	// ---- late quota without a decision changes nothing ----
	if err := svc.SettleQuota(ctx, userID, condoID, a1p1, jan15, ""); !errors.Is(err, services.ErrLateFeeDecisionRequired) {
		t.Fatalf("no decision = %v", err)
	}
	var n int
	if err := sqldb.QueryRowContext(ctx, `SELECT count(*) FROM finance_accounts WHERE status = 'settled'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("settled after refused settle = %d, %v", n, err)
	}

	// ---- apply: quota settled + pending fine_interest receivable linked to it ----
	// A-1 p1: 6600, due 5 Jan, settled 15 Jan: 10 days; fine 132, interest 6600*100*10/300000 = 22.
	if err := svc.SettleQuota(ctx, userID, condoID, a1p1, jan15, "apply"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var lateID string
	var multa, juros int64
	var days, mbp, jbp, grace int
	if err := sqldb.QueryRowContext(ctx, `
		SELECT late_fee_account_id, multa_cents, juros_cents, days_late, multa_bp, juros_bp, grace_days
		FROM quota_late_fees WHERE quota_account_id = $1`, a1p1).Scan(&lateID, &multa, &juros, &days, &mbp, &jbp, &grace); err != nil {
		t.Fatal(err)
	}
	if multa != 132 || juros != 22 || days != 10 || mbp != 200 || jbp != 100 || grace != 0 {
		t.Fatalf("late fee row = %d %d %d %d %d %d", multa, juros, days, mbp, jbp, grace)
	}
	var cat, status, unitOf string
	var amount int64
	var due time.Time
	if err := sqldb.QueryRowContext(ctx, `
		SELECT category, status, unit_id::text, amount_cents, due_date FROM finance_accounts WHERE id = $1`, lateID).
		Scan(&cat, &status, &unitOf, &amount, &due); err != nil {
		t.Fatal(err)
	}
	if cat != "fine_interest" || status != "pending" || unitOf != unit1 || amount != 154 || !due.Equal(jan15) {
		t.Fatalf("late fee account = %s %s %s %d %v", cat, status, unitOf, amount, due)
	}
	var qStatus string
	var qAmount int64
	if err := sqldb.QueryRowContext(ctx, `SELECT status, amount_cents FROM finance_accounts WHERE id = $1`, a1p1).Scan(&qStatus, &qAmount); err != nil ||
		qStatus != "settled" || qAmount != 6600 {
		t.Fatalf("quota after apply = %s %d, %v (the quota itself must not change)", qStatus, qAmount, err)
	}
	// A second settlement is refused and creates no second late fee.
	if err := svc.SettleQuota(ctx, userID, condoID, a1p1, jan15, "apply"); !errors.Is(err, services.ErrNotEditable) {
		t.Fatalf("second settle = %v, want ErrNotEditable", err)
	}
	// A late fee is not a quota charge: it is not settled through SettleQuota.
	if err := svc.SettleQuota(ctx, userID, condoID, lateID, jan15, "apply"); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("SettleQuota(late fee) = %v, want ErrNotFound", err)
	}
	// The DB itself refuses a second late fee for the same quota.
	err = qs.WithLockedQuotaAccount(ctx, condoID, a1p1, func(_ context.Context, l services.LockedQuotaAccount) error {
		_, err := l.InsertLateFee(services.NewLateFee{Title: "dup", DueDate: jan15, CreatedBy: userID,
			Detail: domain.LateFeeDetail{FineCents: 1, InterestCents: 1, DaysLate: 1}})
		return err
	})
	if !errors.Is(err, domain.ErrNotEditable) {
		t.Fatalf("duplicate late fee = %v, want ErrNotEditable (UNIQUE quota_account_id)", err)
	}

	// ---- links: statement, receipt, batch origins ----
	rows, err := qs.UnitReceivables(ctx, condoID, unit1)
	if err != nil {
		t.Fatal(err)
	}
	var sawLate, sawQuotaP1 bool
	for _, r := range rows {
		switch r.Account.ID {
		case lateID:
			sawLate = true
			if !r.IsLateFee || r.IsQuota || r.LateFee == nil || r.LateFee.FineCents != 132 || r.LateFee.InterestCents != 22 ||
				r.LateFee.DaysLate != 10 || r.LateFee.FeeBP != 200 || r.LateFeeOrigin.Index != 1 || r.LateFeeOrigin.Year != 2026 || r.LateFeeOrigin.UnitCode != "A-1" {
				t.Fatalf("late fee statement row = %+v", r)
			}
		case a1p1:
			sawQuotaP1 = true
			if r.IsLateFee || !r.IsQuota {
				t.Fatalf("quota statement row = %+v", r)
			}
		}
	}
	if !sawLate || !sawQuotaP1 {
		t.Fatalf("statement is missing rows: late=%v quota=%v", sawLate, sawQuotaP1)
	}
	if _, err := sqldb.ExecContext(ctx, `UPDATE finance_accounts SET status = 'settled', settlement_date = $2 WHERE id = $1`, lateID, jan15); err != nil {
		t.Fatal(err)
	}
	rc, err := qs.ReceiptData(ctx, condoID, lateID)
	if err != nil || !rc.IsLateFee || rc.IsQuota || rc.LateFee == nil || rc.LateFee.FineCents != 132 || rc.LateFeeOrigin.Index != 1 || rc.LateFeeOrigin.UnitCode != "A-1" {
		t.Fatalf("late fee receipt = %+v, %v", rc, err)
	}
	if rc, err = qs.ReceiptData(ctx, condoID, a1p1); err != nil || rc.IsLateFee || !rc.IsQuota {
		t.Fatalf("quota receipt = %+v, %v", rc, err)
	}
	origins, err := qs.LateFeeOrigins(ctx, []string{lateID, a1p1, a2p1})
	if err != nil || len(origins) != 1 || origins[lateID].Origin.Index != 1 || origins[lateID].Detail.InterestCents != 22 {
		t.Fatalf("LateFeeOrigins = %+v, %v", origins, err)
	}
	if o, err := qs.LateFeeOrigins(ctx, nil); err != nil || len(o) != 0 {
		t.Fatalf("empty LateFeeOrigins = %v, %v", o, err)
	}
	fin := &services.FinanceService{Accounts: NewAccountStore(sqldb), Units: store.NewUnitStore(sqldb), QuotaLinks: qs, LateFeeLinks: qs}
	if late, err := fin.IsLateFeeAccount(ctx, lateID); err != nil || !late {
		t.Fatalf("IsLateFeeAccount = %v, %v", late, err)
	}

	// ---- waive: settles without a late fee ----
	if err := svc.SettleQuota(ctx, userID, condoID, a1p2, time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC), "waive"); err != nil {
		t.Fatalf("waive: %v", err)
	}
	if err := sqldb.QueryRowContext(ctx, `SELECT count(*) FROM quota_late_fees`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("late fees after waive = %d, %v", n, err)
	}

	// ---- concurrent double settle: one settlement, at most one late fee ----
	const racers = 8
	var wg sync.WaitGroup
	results := make([]error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = svc.SettleQuota(ctx, userID, condoID, a2p1, jan15, "apply")
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, e := range results {
		switch {
		case e == nil:
			ok++
		case !errors.Is(e, services.ErrNotEditable):
			t.Fatalf("racer error = %v, want nil or ErrNotEditable", e)
		}
	}
	if ok != 1 {
		t.Fatalf("successful settlements = %d, want exactly 1", ok)
	}
	if err := sqldb.QueryRowContext(ctx, `SELECT count(*) FROM quota_late_fees WHERE quota_account_id = $1`, a2p1).Scan(&n); err != nil || n != 1 {
		t.Fatalf("late fees of the raced quota = %d, %v", n, err)
	}
	if err := sqldb.QueryRowContext(ctx, `
		SELECT count(*) FROM finance_accounts WHERE category = 'fine_interest' AND unit_id = $1`, unit2).Scan(&n); err != nil || n != 1 {
		t.Fatalf("fine_interest accounts of the raced unit = %d, %v", n, err)
	}

	// ---- debtors (today 2026-03-10) ----
	// Pending and overdue now: A-2 p2 (4400, due 5 Feb); the A-2 late fee of p1
	// (due 15 Jan, pending); A-1 has only settled quotas and its settled late fee.
	today := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	debtors, err := svc.Debtors(ctx, condoID, today)
	if err != nil || len(debtors) != 1 || debtors[0].UnitCode != "A-2" {
		t.Fatalf("debtors = %+v, %v", debtors, err)
	}
	d := debtors[0]
	var lateOfA2 int64
	if err := sqldb.QueryRowContext(ctx, `SELECT amount_cents FROM finance_accounts WHERE category = 'fine_interest' AND unit_id = $1`, unit2).Scan(&lateOfA2); err != nil {
		t.Fatal(err)
	}
	if d.OverdueCount != 2 || d.OverdueTotalCents != 4400+lateOfA2 || !d.OldestDue.Equal(jan15) || d.DaysLate != 54 {
		t.Fatalf("debtor = %+v", d)
	}
	// The estimate covers the quota only (not the late fee receivable): 4400 due 5 Feb, 33 days late.
	wantEst := domain.ComputeLateFee(4400, time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC), today, domain.LateFeeRates{FeeBP: 200, InterestBP: 100}).TotalCents
	if wantEst == 0 || d.EstimatedLateFeeCents != wantEst {
		t.Fatalf("estimate = %d, want %d", d.EstimatedLateFeeCents, wantEst)
	}

	// Make A-1 a debtor with two occupants and two overdue receivables, then
	// check there is no fan-out and the sort is by total descending.
	if _, err := sqldb.ExecContext(ctx, `
		INSERT INTO finance_accounts (condominium_id, account_type, title, category, amount_cents, due_date, status, unit_id, created_by)
		VALUES ($1, 'receivable', 'Salão', 'common_area_reservation', 9000, '2026-02-01', 'pending', $2, $3),
		       ($1, 'receivable', 'Chave', 'common_area_reservation', 1000, '2026-02-02', 'pending', $2, $3),
		       ($1, 'receivable', 'Futuro', 'common_area_reservation', 5000, '2026-04-01', 'pending', $2, $3)`,
		condoID, unit1, userID); err != nil {
		t.Fatal(err)
	}
	debtors, err = svc.Debtors(ctx, condoID, today)
	if err != nil || len(debtors) != 2 {
		t.Fatalf("debtors = %+v, %v", debtors, err)
	}
	if debtors[0].UnitCode != "A-1" || debtors[0].OverdueCount != 2 || debtors[0].OverdueTotalCents != 10000 ||
		!debtors[0].OldestDue.Equal(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("A-1 debtor = %+v (two occupants must not duplicate its receivables)", debtors[0])
	}
	if debtors[0].EstimatedLateFeeCents != 0 {
		t.Fatalf("A-1 estimate = %d, want 0 (no overdue quota)", debtors[0].EstimatedLateFeeCents)
	}
	if debtors[1].UnitCode != "A-2" || debtors[0].OverdueTotalCents < debtors[1].OverdueTotalCents {
		t.Fatalf("order = %+v", debtors)
	}
	if len(debtors[0].Occupants) != 2 || debtors[0].Occupants[0].Email != "owner@example.com" || debtors[0].Occupants[0].Type != model.OccupancyOwner ||
		debtors[0].Occupants[1].Email != "tenant@example.com" || debtors[0].Occupants[1].Type != model.OccupancyTenant {
		t.Fatalf("occupants = %+v (active only, with type)", debtors[0].Occupants)
	}
	if len(debtors[1].Occupants) != 0 {
		t.Fatalf("A-2 occupants = %+v, want none", debtors[1].Occupants)
	}

	// Card: debtor units and total across both.
	card, err := svc.Card(ctx, services.Viewer{UserID: userID, IsSyndic: true}, condoID, today)
	if err != nil || card.Syndic == nil || card.Syndic.DebtorUnits != 2 ||
		card.Syndic.DebtorTotalCents != debtors[0].OverdueTotalCents+debtors[1].OverdueTotalCents {
		t.Fatalf("card = %+v, %v", card.Syndic, err)
	}

	// ---- reserve balance ----
	// Settled so far: A-1 p1 (600), A-1 p2 (600), A-2 p1 (400); pending: A-2 p2 (400).
	bal, err := svc.ReserveBalance(ctx, condoID, today)
	if err != nil || bal.RealizedCents != 1600 || bal.ForecastCents != 2000 || bal.RealizedYearCents != 1600 {
		t.Fatalf("reserve = %+v, %v", bal, err)
	}
	// A settlement dated in 2025 counts since the start but not in 2026.
	if _, err := sqldb.ExecContext(ctx, `UPDATE finance_accounts SET settlement_date = '2025-12-20' WHERE id = $1`, a2p1); err != nil {
		t.Fatal(err)
	}
	if bal, err = svc.ReserveBalance(ctx, condoID, today); err != nil || bal.RealizedCents != 1600 || bal.RealizedYearCents != 1200 {
		t.Fatalf("reserve with a 2025 settlement = %+v, %v", bal, err)
	}
	// A canceled charge counts nowhere.
	if _, err := sqldb.ExecContext(ctx, `UPDATE finance_accounts SET status = 'canceled' WHERE id = $1`, a2p2); err != nil {
		t.Fatal(err)
	}
	if bal, err = svc.ReserveBalance(ctx, condoID, today); err != nil || bal.RealizedCents != 1600 || bal.ForecastCents != 1600 {
		t.Fatalf("reserve with a canceled charge = %+v, %v", bal, err)
	}
}
