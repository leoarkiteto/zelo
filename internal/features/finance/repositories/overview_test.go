package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
)

func utcDate(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// seedOverviewAccount creates an account and applies the requested status.
func seedOverviewAccount(t *testing.T, store *AccountStore, condoID, userID string, typ domain.AccountType,
	cat domain.Category, cents int64, due time.Time, status domain.AccountStatus, settled time.Time) {
	t.Helper()
	ctx := context.Background()
	a := baseAccount(condoID, userID)
	a.Type = typ
	a.Category = cat
	a.AmountCents = cents
	a.DueDate = due
	a.Status = domain.StatusPending
	id, err := store.Create(ctx, a)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	switch status {
	case domain.StatusSettled:
		if err := store.SetStatus(ctx, id, condoID, domain.StatusSettled, &settled); err != nil {
			t.Fatalf("settle: %v", err)
		}
	case domain.StatusCanceled:
		if err := store.SetStatus(ctx, id, condoID, domain.StatusCanceled, nil); err != nil {
			t.Fatalf("cancel: %v", err)
		}
	}
}

func TestOverviewStoreRealized(t *testing.T) {
	db := openFinanceTestDB(t)
	ctx := context.Background()
	condoID, _, userID := seedFinanceFixture(t, db)
	var otherCondo string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO condominiums (name) VALUES ('Other Condo') RETURNING id`).Scan(&otherCondo); err != nil {
		t.Fatalf("seed other condo: %v", err)
	}
	store := NewAccountStore(db)

	rec, pay := domain.AccountTypeReceivable, domain.AccountTypePayable
	settled, pending, canceled := domain.StatusSettled, domain.StatusPending, domain.StatusCanceled

	// Due in March, settled in April: counts in April.
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 1000, utcDate(2026, 3, 10), settled, utcDate(2026, 4, 2))
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 500, utcDate(2026, 4, 10), settled, utcDate(2026, 4, 30))
	seedOverviewAccount(t, store, condoID, userID, pay, domain.CategoryMaintenance, 300, utcDate(2026, 4, 5), settled, utcDate(2026, 4, 5))
	seedOverviewAccount(t, store, condoID, userID, pay, domain.CategoryCleaning, 200, utcDate(2026, 5, 5), settled, utcDate(2026, 5, 1))
	// Excluded: pending, canceled, out of range, other condominium.
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 9999, utcDate(2026, 4, 10), pending, time.Time{})
	seedOverviewAccount(t, store, condoID, userID, pay, domain.CategoryCleaning, 8888, utcDate(2026, 4, 10), canceled, time.Time{})
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 7777, utcDate(2026, 6, 1), settled, utcDate(2026, 6, 1))
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 6666, utcDate(2025, 12, 31), settled, utcDate(2025, 12, 31))
	seedOverviewAccount(t, store, otherCondo, userID, rec, domain.CategoryCondoFee, 5555, utcDate(2026, 4, 1), settled, utcDate(2026, 4, 1))

	start, end := utcDate(2026, 4, 1), utcDate(2026, 6, 1)

	flows, err := store.MonthlyRealized(ctx, condoID, start, end)
	if err != nil {
		t.Fatalf("MonthlyRealized() = %v", err)
	}
	if len(flows) != 2 {
		t.Fatalf("flows = %+v, want 2 months", flows)
	}
	want := []struct {
		month   time.Time
		in, out int64
	}{
		{utcDate(2026, 4, 1), 1500, 300},
		{utcDate(2026, 5, 1), 0, 200},
	}
	for i, w := range want {
		f := flows[i]
		if !f.Month.Equal(w.month) || f.Month.Location() != time.UTC {
			t.Errorf("flows[%d].Month = %v (%v), want %v UTC", i, f.Month, f.Month.Location(), w.month)
		}
		if f.InCents != w.in || f.OutCents != w.out {
			t.Errorf("flows[%d] = in %d out %d, want in %d out %d", i, f.InCents, f.OutCents, w.in, w.out)
		}
	}

	cats, err := store.RealizedByCategory(ctx, condoID, start, end)
	if err != nil {
		t.Fatalf("RealizedByCategory() = %v", err)
	}
	got := map[string]int64{}
	for _, c := range cats {
		got[string(c.Type)+"/"+string(c.Category)] = c.Cents
	}
	wantCats := map[string]int64{
		"receivable/condo_fee": 1500,
		"payable/maintenance":  300,
		"payable/cleaning":     200,
	}
	if len(got) != len(wantCats) {
		t.Fatalf("categories = %v, want %v", got, wantCats)
	}
	for k, v := range wantCats {
		if got[k] != v {
			t.Errorf("category %s = %d, want %d", k, got[k], v)
		}
	}

	years, err := store.SettlementYears(ctx, condoID)
	if err != nil {
		t.Fatalf("SettlementYears() = %v", err)
	}
	if len(years) != 2 || years[0] != 2026 || years[1] != 2025 {
		t.Fatalf("years = %v, want [2026 2025]", years)
	}
	otherYears, err := store.SettlementYears(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil || len(otherYears) != 0 {
		t.Fatalf("unknown condo years = %v, %v", otherYears, err)
	}
}

func TestOverviewStoreCurrentPosition(t *testing.T) {
	db := openFinanceTestDB(t)
	ctx := context.Background()
	condoID, _, userID := seedFinanceFixture(t, db)
	var otherCondo string
	if err := db.QueryRowContext(ctx,
		`INSERT INTO condominiums (name) VALUES ('Other Condo') RETURNING id`).Scan(&otherCondo); err != nil {
		t.Fatalf("seed other condo: %v", err)
	}
	store := NewAccountStore(db)

	rec, pay := domain.AccountTypeReceivable, domain.AccountTypePayable
	settled, pending, canceled := domain.StatusSettled, domain.StatusPending, domain.StatusCanceled
	today := utcDate(2026, 11, 14)
	monthStart, monthEnd := utcDate(2026, 11, 1), utcDate(2026, 12, 1)

	// In month: pending receivable overdue (Nov 3), due today (Nov 14), future (Nov 20).
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 100, utcDate(2026, 11, 3), pending, time.Time{})
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 200, today, pending, time.Time{})
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 400, utcDate(2026, 11, 20), pending, time.Time{})
	// Overdue from an earlier month.
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 800, utcDate(2026, 9, 10), pending, time.Time{})
	// Payables.
	seedOverviewAccount(t, store, condoID, userID, pay, domain.CategoryCleaning, 50, utcDate(2026, 11, 25), pending, time.Time{})
	seedOverviewAccount(t, store, condoID, userID, pay, domain.CategoryCleaning, 5000, utcDate(2026, 10, 25), pending, time.Time{})
	// Excluded.
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 9000, utcDate(2026, 11, 5), canceled, time.Time{})
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 9001, utcDate(2026, 11, 5), settled, utcDate(2026, 11, 6))
	seedOverviewAccount(t, store, condoID, userID, pay, domain.CategoryCleaning, 9002, utcDate(2026, 11, 5), canceled, time.Time{})
	seedOverviewAccount(t, store, otherCondo, userID, rec, domain.CategoryCondoFee, 9003, utcDate(2026, 11, 3), pending, time.Time{})
	seedOverviewAccount(t, store, otherCondo, userID, pay, domain.CategoryCleaning, 9004, utcDate(2026, 11, 3), pending, time.Time{})

	pos, err := store.CurrentPosition(ctx, condoID, monthStart, monthEnd, today)
	if err != nil {
		t.Fatalf("CurrentPosition() = %v", err)
	}
	if pos.ReceivableCents != 700 {
		t.Errorf("ReceivableCents = %d, want 700", pos.ReceivableCents)
	}
	if pos.PayableCents != 50 {
		t.Errorf("PayableCents = %d, want 50", pos.PayableCents)
	}
	if pos.OverdueCents != 900 {
		t.Errorf("OverdueCents = %d, want 900 (due today is not overdue)", pos.OverdueCents)
	}
}

func TestOverviewStoreCurrentPositionNonMidnightToday(t *testing.T) {
	db := openFinanceTestDB(t)
	ctx := context.Background()
	condoID, _, userID := seedFinanceFixture(t, db)
	store := NewAccountStore(db)

	rec, pending := domain.AccountTypeReceivable, domain.StatusPending
	today := utcDate(2026, 11, 14)
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 200, today, pending, time.Time{})
	seedOverviewAccount(t, store, condoID, userID, rec, domain.CategoryCondoFee, 100, utcDate(2026, 11, 13), pending, time.Time{})

	now := time.Date(2026, 11, 14, 15, 30, 0, 0, time.UTC)
	pos, err := store.CurrentPosition(ctx, condoID, utcDate(2026, 11, 1), utcDate(2026, 12, 1), now)
	if err != nil {
		t.Fatalf("CurrentPosition() = %v", err)
	}
	if pos.OverdueCents != 100 {
		t.Errorf("OverdueCents = %d, want 100 (due today is not overdue, due yesterday is)", pos.OverdueCents)
	}
}
