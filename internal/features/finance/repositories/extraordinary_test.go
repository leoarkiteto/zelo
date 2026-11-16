package repositories

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

var (
	_ services.LockedExtraordinary = (*lockedExtraordinary)(nil)
)

func TestExtraordinaryStoreLifecycle(t *testing.T) {
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
	svc := &services.QuotaService{Store: qs, Units: store.NewUnitStore(db), Now: time.Now}

	// An ordinary budget/issuance coexists with the extraordinary quota.
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
	ordIss, _, err := svc.Issue(ctx, userID, condoID, budgetID, 1)
	if err != nil {
		t.Fatal(err)
	}

	in := services.ExtraordinaryInput{Description: "Roof", TotalCents: 100001, Division: services.DivisionShare,
		Installments: 3, FirstDueDate: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)}
	id, err := svc.CreateExtraordinary(ctx, userID, condoID, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	var n int
	if err := db.QueryRowContext(ctx, `
		SELECT count(*) FROM finance_accounts WHERE category = 'extraordinary_income' AND status = 'pending'`).Scan(&n); err != nil || n != 6 {
		t.Fatalf("receivables = %d, %v", n, err)
	}
	var reserve, base, total int64
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(c.reserve_cents), 0), SUM(c.base_cents),(SELECT SUM(amount_cents) FROM finance_accounts WHERE category = 'extraordinary_income')
		FROM quota_charges c JOIN quota_issuances i ON i.id = c.issuance_id WHERE i.extraordinary_id = $1`, id).Scan(&reserve, &base, &total); err != nil {
		t.Fatal(err)
	}
	if reserve != 0 || base != 100001 || total != 100001 {
		t.Fatalf("reserve=%d base=%d total=%d", reserve, base, total)
	}

	// Ordinary paths never see the installments (R1).
	list, err := qs.ListIssuances(ctx, condoID, budgetID)
	if err != nil || len(list) != 1 || list[0].ID != ordIss {
		t.Fatalf("ListIssuances = %+v, %v", list, err)
	}
	d, err := qs.GetExtraordinary(ctx, condoID, id)
	if err != nil || len(d.Issuances) != 3 || d.ChargeCount != 6 || d.SettledCount != 0 || d.Status != domain.IssuanceIssued {
		t.Fatalf("GetExtraordinary = %+v, %v", d, err)
	}
	instID := d.Issuances[1].ID
	if d.Issuances[1].ExtraordinaryID != id || d.Issuances[1].PeriodIndex != 2 || d.Issuances[1].BudgetID != "" {
		t.Fatalf("installment = %+v", d.Issuances[1])
	}
	if err := svc.CancelIssuance(ctx, userID, condoID, instID); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("ordinary cancel of installment = %v, want ErrNotFound", err)
	}
	if err := qs.WithLockedIssuance(ctx, condoID, instID, func(context.Context, services.LockedIssuance) error { return nil }); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("WithLockedIssuance(installment) = %v", err)
	}
	if _, ok, err := qs.ActiveIssuanceFor(ctx, condoID, budgetID, 2); err != nil || ok {
		t.Fatalf("ActiveIssuanceFor = %v, %v", ok, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM finance_accounts WHERE status = 'canceled'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("canceled after refused ordinary cancel = %d, %v", n, err)
	}

	// Kind-aware reads.
	var accID string
	if err := db.QueryRowContext(ctx, `
		SELECT c.finance_account_id FROM quota_charges c JOIN quota_issuances i ON i.id = c.issuance_id
		WHERE i.id = $1 AND c.unit_id = $2`, instID, unitID).Scan(&accID); err != nil {
		t.Fatal(err)
	}
	links, err := qs.QuotaPeriods(ctx, []string{accID})
	p := links[accID]
	if err != nil || p.Kind != domain.QuotaKindExtraordinary || p.Description != "Roof" || p.Installment != 2 || p.Installments != 3 || p.UnitCode == "" {
		t.Fatalf("QuotaPeriods = %+v, %v", p, err)
	}
	rows, err := qs.UnitReceivables(ctx, condoID, unitID)
	if err != nil {
		t.Fatal(err)
	}
	var ext, ord int
	for _, r := range rows {
		if !r.IsQuota {
			continue
		}
		switch r.Period.Kind {
		case domain.QuotaKindExtraordinary:
			ext++
			if r.Period.Description != "Roof" || r.Period.Installments != 3 || r.Period.Installment < 1 {
				t.Errorf("statement period = %+v", r.Period)
			}
		case domain.QuotaKindOrdinary:
			ord++
			if r.Period.Year != 2026 || r.Period.Periodicity != domain.PeriodicityMonthly {
				t.Errorf("ordinary period = %+v", r.Period)
			}
		}
	}
	if ext != 3 || ord != 1 {
		t.Fatalf("statement ext=%d ord=%d", ext, ord)
	}
	if _, err := db.ExecContext(ctx, `UPDATE finance_accounts SET status = 'settled', settlement_date = CURRENT_DATE WHERE id = $1`, accID); err != nil {
		t.Fatal(err)
	}
	rc, err := qs.ReceiptData(ctx, condoID, accID)
	if err != nil || !rc.IsQuota || rc.Period.Kind != domain.QuotaKindExtraordinary || rc.Period.Installment != 2 || rc.ReserveCents != 0 || rc.BaseCents != rc.AmountCents {
		t.Fatalf("ReceiptData = %+v, %v", rc, err)
	}
	card, err := qs.CardData(ctx, condoID, []string{unitID}, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), false)
	if err != nil || card.NextPending == nil {
		t.Fatalf("CardData = %+v, %v", card, err)
	}

	// A settled receivable refuses the whole cancel and rolls it back.
	if err := svc.CancelExtraordinary(ctx, userID, condoID, id); !errors.Is(err, domain.ErrExtraordinaryHasSettled) {
		t.Fatalf("cancel with settled = %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM finance_accounts WHERE category = 'extraordinary_income' AND status = 'pending'`).Scan(&n); err != nil || n != 5 {
		t.Fatalf("pending after rollback = %d, %v", n, err)
	}
	ls, err := qs.ListExtraordinaries(ctx, condoID)
	if err != nil || len(ls) != 1 || ls[0].SettledCount != 1 || ls[0].Status != domain.IssuanceIssued {
		t.Fatalf("ListExtraordinaries = %+v, %v", ls, err)
	}

	if _, err := db.ExecContext(ctx, `UPDATE finance_accounts SET status = 'pending', settlement_date = NULL WHERE id = $1`, accID); err != nil {
		t.Fatal(err)
	}
	if err := svc.CancelExtraordinary(ctx, userID, condoID, id); err != nil {
		t.Fatalf("cancel = %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM finance_accounts WHERE category = 'extraordinary_income' AND status = 'canceled'`).Scan(&n); err != nil || n != 6 {
		t.Fatalf("canceled = %d, %v", n, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM quota_issuances WHERE extraordinary_id = $1 AND status = 'canceled' AND canceled_at IS NOT NULL`, id).Scan(&n); err != nil || n != 3 {
		t.Fatalf("installment issuances canceled = %d, %v", n, err)
	}
	d, err = qs.GetExtraordinary(ctx, condoID, id)
	if err != nil || d.Status != domain.IssuanceCanceled || d.CanceledAt == nil {
		t.Fatalf("after cancel = %+v, %v", d, err)
	}
	if err := svc.CancelExtraordinary(ctx, userID, condoID, id); !errors.Is(err, services.ErrIssuanceNotActive) {
		t.Fatalf("second cancel = %v", err)
	}
	// The ordinary issuance is untouched.
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM quota_issuances WHERE id = $1 AND status = 'issued'`, ordIss).Scan(&n); err != nil || n != 1 {
		t.Fatalf("ordinary issuance = %d, %v", n, err)
	}
	if _, err := qs.GetExtraordinary(ctx, condoID, "00000000-0000-0000-0000-000000000000"); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
	if err := svc.CancelExtraordinary(ctx, userID, condoID, "not-a-uuid"); !errors.Is(err, services.ErrNotFound) {
		t.Fatalf("bad id = %v", err)
	}

	// Shares not at 100 % refuse a by-share quota with the difference.
	if _, err := db.ExecContext(ctx, `UPDATE units SET share_ppm = 500000 WHERE id = $1`, unitID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreateExtraordinary(ctx, userID, condoID, in)
	var se services.SharesError
	if !errors.As(err, &se) || se.DiffPPM() != -100000 {
		t.Fatalf("shares refusal = %v", err)
	}
}
