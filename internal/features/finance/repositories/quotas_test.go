package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
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

	svc := &services.QuotaService{Store: qs, Now: time.Now}
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
