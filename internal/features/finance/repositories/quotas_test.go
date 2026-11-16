package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

// Compile-time contract checks: the store satisfies the services seams.
var (
	_ services.QuotaStore     = (*QuotaStore)(nil)
	_ services.QuotaLinks     = (*QuotaStore)(nil)
	_ services.LockedBudget   = (*lockedBudget)(nil)
	_ services.LockedIssue    = (*lockedIssue)(nil)
	_ services.LockedIssuance = (*lockedIssuance)(nil)
)

// TestQuotaStoreLifecycle exercises budgets, issue, idempotency, cancel and the
// quota link lookup against PostgreSQL (skipped without TEST_DATABASE_URL).
func TestQuotaStoreLifecycle(t *testing.T) {
	db := openFinanceTestDB(t)
	ctx := context.Background()
	condoID, unitID, userID := seedFinanceFixture(t, db)
	var unit2 string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO units (condominium_id, code, share_ppm) VALUES ($1, 'A-2', 400000) RETURNING id`, condoID).Scan(&unit2); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE units SET share_ppm = 600000 WHERE id = $1`, unitID); err != nil {
		t.Fatal(err)
	}
	qs := NewQuotaStore(db)

	budgetID, err := qs.CreateBudget(ctx, services.Budget{CondominiumID: condoID, Year: 2026,
		Periodicity: domain.PeriodicityMonthly, DueDay: 5, ReserveRateBP: 1000, CreatedBy: userID})
	if err != nil {
		t.Fatalf("CreateBudget: %v", err)
	}
	if _, err := qs.CreateBudget(ctx, services.Budget{CondominiumID: condoID, Year: 2026,
		Periodicity: domain.PeriodicityMonthly, DueDay: 5, CreatedBy: userID}); !errors.Is(err, domain.ErrBudgetExists) {
		t.Fatalf("duplicate year = %v, want ErrBudgetExists", err)
	}

	err = qs.WithLockedBudget(ctx, condoID, budgetID, func(_ context.Context, b services.LockedBudget) error {
		if _, err := b.AddLine(services.BudgetLine{Description: "Limpeza", Category: domain.CategoryCleaning, AmountCents: 120000}); err != nil {
			return err
		}
		return b.SetStatus(domain.BudgetApproved)
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if err := qs.WithLockedBudget(ctx, condoID, "00000000-0000-0000-0000-000000000000", func(context.Context, services.LockedBudget) error { return nil }); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("missing budget = %v", err)
	}

	svc := &services.QuotaService{Store: qs, Units: store.NewUnitStore(db), Now: time.Now}
	id, created, err := svc.Issue(ctx, userID, condoID, budgetID, 1)
	if err != nil || !created {
		t.Fatalf("Issue = %q %v %v", id, created, err)
	}
	id2, created2, err := svc.Issue(ctx, userID, condoID, budgetID, 1)
	if err != nil || created2 || id2 != id {
		t.Fatalf("re-issue = %q %v %v", id2, created2, err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM quota_charges WHERE issuance_id = $1`, id).Scan(&n); err != nil || n != 2 {
		t.Fatalf("charges = %d, %v", n, err)
	}
	var accID string
	if err := db.QueryRowContext(ctx, `SELECT finance_account_id FROM quota_charges WHERE issuance_id = $1 LIMIT 1`, id).Scan(&accID); err != nil {
		t.Fatal(err)
	}
	links, err := qs.QuotaPeriods(ctx, []string{accID, unitID})
	if _, isQuota := links[accID]; err != nil || !isQuota || len(links) != 1 {
		t.Fatalf("QuotaPeriods = %v, %v", links, err)
	}
	if p := links[accID]; p.Year != 2026 || p.Periodicity != domain.PeriodicityMonthly || p.Index != 1 || p.UnitCode == "" {
		t.Fatalf("QuotaPeriods metadata = %+v", p)
	}

	// A settled quota blocks the cancellation and rolls everything back.
	if _, err := db.ExecContext(ctx, `UPDATE finance_accounts SET status = 'settled', settlement_date = CURRENT_DATE WHERE id = $1`, accID); err != nil {
		t.Fatal(err)
	}
	if err := svc.CancelIssuance(ctx, userID, condoID, id); !errors.Is(err, domain.ErrIssuanceHasSettled) {
		t.Fatalf("cancel with settled = %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM finance_accounts WHERE status = 'pending'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("pending after rollback = %d, %v", n, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE finance_accounts SET status = 'pending', settlement_date = NULL WHERE id = $1`, accID); err != nil {
		t.Fatal(err)
	}
	if err := svc.CancelIssuance(ctx, userID, condoID, id); err != nil {
		t.Fatalf("cancel = %v", err)
	}
	if _, created, err := svc.Issue(ctx, userID, condoID, budgetID, 1); err != nil || !created {
		t.Fatalf("issue after cancel = %v %v", created, err)
	}
}

// TestQuotaChargesCondominiumFKs checks migration 0018: a quota_charges row
// cannot reference another condominium's issuance or finance account.
func TestQuotaChargesCondominiumFKs(t *testing.T) {
	db := openFinanceTestDB(t)
	ctx := context.Background()
	condoA, unitA, userID := seedFinanceFixture(t, db)
	var condoB, unitB string
	if err := db.QueryRowContext(ctx, `INSERT INTO condominiums (name) VALUES ('Other Condo') RETURNING id`).Scan(&condoB); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO units (condominium_id, code) VALUES ($1, 'B-1') RETURNING id`, condoB).Scan(&unitB); err != nil {
		t.Fatal(err)
	}

	issuance := func(condo string) string {
		var budget, iss string
		if err := db.QueryRowContext(ctx, `INSERT INTO quota_budgets (condominium_id, year, periodicity, due_day, created_by)
			VALUES ($1, 2026, 'monthly', 5, $2) RETURNING id`, condo, userID).Scan(&budget); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRowContext(ctx, `INSERT INTO quota_issuances (condominium_id, budget_id, period_index, total_cents, base_cents, reserve_cents, created_by)
			VALUES ($1, $2, 1, 100, 100, 0, $3) RETURNING id`, condo, budget, userID).Scan(&iss); err != nil {
			t.Fatal(err)
		}
		return iss
	}
	account := func(condo, unit string) string {
		a := baseAccount(condo, userID)
		a.Type = domain.AccountTypeReceivable
		a.Category = domain.CategoryCondoFee
		a.UnitID = unit
		id, err := NewAccountStore(db).Create(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	issA, issB := issuance(condoA), issuance(condoB)
	accA, accB := account(condoA, unitA), account(condoB, unitB)

	insert := func(condo, iss, unit, acc string) error {
		_, err := db.ExecContext(ctx, `INSERT INTO quota_charges (condominium_id, issuance_id, unit_id, finance_account_id, share_ppm, base_cents, reserve_cents)
			VALUES ($1, $2, $3, $4, 1000000, 100, 0)`, condo, iss, unit, acc)
		return err
	}
	isFK := func(err error) bool {
		var pe *pgconn.PgError
		return errors.As(err, &pe) && pe.Code == "23503"
	}
	if err := insert(condoA, issB, unitA, accA); !isFK(err) {
		t.Fatalf("charge with another condominium's issuance = %v, want FK violation", err)
	}
	if err := insert(condoA, issA, unitA, accB); !isFK(err) {
		t.Fatalf("charge with another condominium's account = %v, want FK violation", err)
	}
	if err := insert(condoA, issA, unitA, accA); err != nil {
		t.Fatalf("same-condominium charge = %v", err)
	}
	// ON DELETE CASCADE from the issuance is preserved.
	if _, err := db.ExecContext(ctx, `DELETE FROM quota_issuances WHERE id = $1`, issA); err != nil {
		t.Fatalf("delete issuance: %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM quota_charges WHERE issuance_id = $1`, issA).Scan(&n); err != nil || n != 0 {
		t.Fatalf("charges after issuance delete = %d, %v", n, err)
	}
}
