package repositories

import (
	"context"
	"testing"
	"time"
)

func TestCardDataIncludeDebtors(t *testing.T) {
	db := openFinanceTestDB(t)
	ctx := context.Background()
	condoID, unitID, userID := seedFinanceFixture(t, db)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO finance_accounts (condominium_id, account_type, title, category, amount_cents, due_date, status, unit_id, created_by)
		VALUES ($1, 'receivable', 'Salão', 'common_area_reservation', 9000, '2026-02-01', 'pending', $2, $3)`,
		condoID, unitID, userID); err != nil {
		t.Fatal(err)
	}
	qs := NewQuotaStore(db)
	today := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	off, err := qs.CardData(ctx, condoID, nil, today, false)
	if err != nil {
		t.Fatal(err)
	}
	if off.DebtorUnits != 0 || off.DebtorTotalCents != 0 {
		t.Fatalf("includeDebtors=false: debtors = %d units, %d cents, want zero", off.DebtorUnits, off.DebtorTotalCents)
	}
	on, err := qs.CardData(ctx, condoID, nil, today, true)
	if err != nil {
		t.Fatal(err)
	}
	if on.DebtorUnits != 1 || on.DebtorTotalCents != 9000 {
		t.Fatalf("includeDebtors=true: debtors = %d units, %d cents, want 1 and 9000", on.DebtorUnits, on.DebtorTotalCents)
	}
}
