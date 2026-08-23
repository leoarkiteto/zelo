package repositories

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/core/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/core/ports"
	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

func openFinanceTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := testutil.TestDatabaseURL("finance")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	db, err := sharedstore.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := sharedstore.Migrate(ctx, db, "../../../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		TRUNCATE finance_accounts, unit_occupancies, user_roles, units,
		condominiums, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return db
}

func seedFinanceFixture(t *testing.T, db *sql.DB) (condoID, unitID, userID string) {
	t.Helper()
	ctx := context.Background()
	if err := db.QueryRowContext(ctx,
		`INSERT INTO condominiums (name) VALUES ('Fin Condo') RETURNING id`).Scan(&condoID); err != nil {
		t.Fatalf("seed condo: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO units (condominium_id, code) VALUES ($1, 'A-1') RETURNING id`, condoID).Scan(&unitID); err != nil {
		t.Fatalf("seed unit: %v", err)
	}
	if err := db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('fin@example.com', 'x') RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return condoID, unitID, userID
}

func baseAccount(condoID, userID string) domain.FinancialAccount {
	return domain.FinancialAccount{
		CondominiumID: condoID,
		Type:          domain.AccountTypePayable,
		Title:         "Manutenção do portão",
		Category:      domain.CategoryMaintenance,
		AmountCents:   35000,
		DueDate:       time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC),
		Status:        domain.StatusPending,
		CreatedBy:     userID,
	}
}

func TestAccountStoreCreateGetAndSettle(t *testing.T) {
	db := openFinanceTestDB(t)
	ctx := context.Background()
	condoID, _, userID := seedFinanceFixture(t, db)
	store := NewAccountStore(db)

	id, err := store.Create(ctx, baseAccount(condoID, userID))
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	got, err := store.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID() = %v", err)
	}
	if got.Title != "Manutenção do portão" || got.AmountCents != 35000 || got.Status != domain.StatusPending {
		t.Fatalf("unexpected account: %+v", got)
	}

	// Settle with a date (FR-010).
	settled := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	if err := store.SetStatus(ctx, id, condoID, domain.StatusSettled, &settled); err != nil {
		t.Fatalf("SetStatus(settled) = %v", err)
	}
	got, _ = store.GetByID(ctx, id)
	if got.Status != domain.StatusSettled || got.SettlementDate == nil || !got.SettlementDate.Equal(settled) {
		t.Fatalf("settle did not record date: %+v", got)
	}

	// Settled accounts are terminal: a second transition fails with not found.
	err = store.SetStatus(ctx, id, condoID, domain.StatusCanceled, nil)
	if !errors.Is(err, sharedstore.ErrNotFound) {
		t.Fatalf("transition on settled account = %v, want ErrNotFound", err)
	}
}

func TestAccountStoreListFiltersOverdue(t *testing.T) {
	db := openFinanceTestDB(t)
	ctx := context.Background()
	condoID, unitID, userID := seedFinanceFixture(t, db)
	store := NewAccountStore(db)

	pastDue := baseAccount(condoID, userID)
	pastDue.Title = "Conta vencida"
	pastDue.DueDate = time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	if _, err := store.Create(ctx, pastDue); err != nil {
		t.Fatalf("create past due: %v", err)
	}
	receivable := baseAccount(condoID, userID)
	receivable.Type = domain.AccountTypeReceivable
	receivable.Category = domain.CategoryCondoFee
	receivable.Title = "Cota agosto"
	receivable.AmountCents = 50000
	receivable.UnitID = unitID
	if _, err := store.Create(ctx, receivable); err != nil {
		t.Fatalf("create receivable: %v", err)
	}

	// overdue filter expands to pending + past due (FR-009).
	overdue, err := store.List(ctx, condoID, ports.AccountFilter{Status: domain.StatusOverdue})
	if err != nil {
		t.Fatalf("List(overdue) = %v", err)
	}
	if len(overdue) != 1 || overdue[0].Title != "Conta vencida" {
		t.Fatalf("overdue list = %+v, want only the past-due account", overdue)
	}

	// type + category filter.
	rec, err := store.List(ctx, condoID, ports.AccountFilter{
		Type: domain.AccountTypeReceivable, Category: domain.CategoryCondoFee,
	})
	if err != nil {
		t.Fatalf("List(receivable) = %v", err)
	}
	if len(rec) != 1 || rec[0].Title != "Cota agosto" {
		t.Fatalf("receivable list = %+v", rec)
	}

	// keyword search.
	search, err := store.List(ctx, condoID, ports.AccountFilter{Query: "vencida"})
	if err != nil {
		t.Fatalf("List(q) = %v", err)
	}
	if len(search) != 1 {
		t.Fatalf("search results = %d, want 1", len(search))
	}
}

func TestAccountStorePendingForUnitAndSummary(t *testing.T) {
	db := openFinanceTestDB(t)
	ctx := context.Background()
	condoID, unitID, userID := seedFinanceFixture(t, db)
	store := NewAccountStore(db)

	receivable := baseAccount(condoID, userID)
	receivable.Type = domain.AccountTypeReceivable
	receivable.Category = domain.CategoryCondoFee
	receivable.Title = "Cota setembro"
	receivable.AmountCents = 50000
	receivable.UnitID = unitID
	if _, err := store.Create(ctx, receivable); err != nil {
		t.Fatalf("create receivable: %v", err)
	}
	if _, err := store.Create(ctx, baseAccount(condoID, userID)); err != nil {
		t.Fatalf("create payable: %v", err)
	}

	// Resident pendências: only the unit's pending receivable (FR-016).
	charges, err := store.ListPendingForUnit(ctx, condoID, unitID)
	if err != nil {
		t.Fatalf("ListPendingForUnit() = %v", err)
	}
	if len(charges) != 1 || charges[0].Title != "Cota setembro" {
		t.Fatalf("unit charges = %+v", charges)
	}

	// Summary for the due month (R5): 50000 receivable vs 35000 payable.
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	sum, err := store.SummaryForMonth(ctx, condoID, month)
	if err != nil {
		t.Fatalf("SummaryForMonth() = %v", err)
	}
	if sum.TotalReceivableCents != 50000 || sum.TotalPayableCents != 35000 {
		t.Fatalf("summary totals = %+v", sum)
	}
	if sum.ProjectedBalanceCents != 15000 {
		t.Fatalf("projected balance = %d, want 15000", sum.ProjectedBalanceCents)
	}
	if sum.RealizedBalanceCents != 0 {
		t.Fatalf("realized balance = %d, want 0 (nothing settled)", sum.RealizedBalanceCents)
	}
}
