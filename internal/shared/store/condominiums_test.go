package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

func TestCondominiumStoreLateFeeSettings(t *testing.T) {
	db := openStoreTestDB(t)
	ctx := context.Background()
	var id string
	if err := db.QueryRowContext(ctx, `INSERT INTO condominiums (name) VALUES ('Late Fee Test') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("insert condominium: %v", err)
	}
	cs := NewCondominiumStore(db)

	got, err := cs.GetLateFeeSettings(ctx, id)
	if err != nil || got != (model.LateFeeSettings{FeeBP: 200, InterestBP: 100, GraceDays: 0}) {
		t.Fatalf("default settings = %+v, %v; want 200/100/0", got, err)
	}
	want := model.LateFeeSettings{FeeBP: 150, InterestBP: 250, GraceDays: 5}
	cap := model.MaxBRLLateFeeBP
	if err := cs.SetLateFeeSettings(ctx, id, want, cap); err != nil {
		t.Fatalf("set settings: %v", err)
	}
	if got, err = cs.GetLateFeeSettings(ctx, id); err != nil || got != want {
		t.Fatalf("settings = %+v, %v; want %+v", got, err, want)
	}

	// BRL: the cap is enforced atomically in the UPDATE.
	over := model.LateFeeSettings{FeeBP: cap + 1, InterestBP: 300, GraceDays: 7}
	if err := cs.SetLateFeeSettings(ctx, id, over, cap); !errors.Is(err, ErrLateFeeAboveBRLCap) {
		t.Fatalf("BRL fee %d = %v, want ErrLateFeeAboveBRLCap", cap+1, err)
	}
	if got, _ = cs.GetLateFeeSettings(ctx, id); got != want {
		t.Fatalf("settings after refusal = %+v, want unchanged %+v", got, want)
	}
	atCap := model.LateFeeSettings{FeeBP: cap, InterestBP: 300, GraceDays: 7}
	if err := cs.SetLateFeeSettings(ctx, id, atCap, cap); err != nil {
		t.Fatalf("BRL fee at cap: %v", err)
	}
	if got, _ = cs.GetLateFeeSettings(ctx, id); got != atCap {
		t.Fatalf("settings = %+v, want %+v", got, atCap)
	}

	// EUR is not capped; the CHECK constraints still apply.
	if err := cs.SetCurrency(ctx, id, money.EUR); err != nil {
		t.Fatalf("set EUR: %v", err)
	}
	if err := cs.SetLateFeeSettings(ctx, id, model.LateFeeSettings{FeeBP: 10001}, cap); err == nil || errors.Is(err, ErrLateFeeAboveBRLCap) {
		t.Fatalf("fee above 10000 bp: err = %v, want CHECK error", err)
	}
	if err := cs.SetLateFeeSettings(ctx, id, model.LateFeeSettings{GraceDays: 61}, cap); err == nil {
		t.Fatal("expected CHECK error for grace above 60 days")
	}
	missing := "00000000-0000-0000-0000-000000000000"
	if _, err := cs.GetLateFeeSettings(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing = %v, want ErrNotFound", err)
	}
	if err := cs.SetLateFeeSettings(ctx, missing, want, cap); !errors.Is(err, ErrNotFound) {
		t.Fatalf("set missing = %v, want ErrNotFound", err)
	}

	// EUR keeps a stored fee above the cap; switching to BRL clamps it.
	if err := cs.SetLateFeeSettings(ctx, id, model.LateFeeSettings{FeeBP: 500, InterestBP: 100}, cap); err != nil {
		t.Fatalf("EUR fee 500: %v", err)
	}
	if err := cs.SetCurrencyClampingLateFee(ctx, id, money.EUR, model.MaxBRLLateFeeBP); err != nil {
		t.Fatalf("set EUR: %v", err)
	}
	if got, _ = cs.GetLateFeeSettings(ctx, id); got.FeeBP != 500 {
		t.Fatalf("EUR fee = %d, want 500 (not clamped)", got.FeeBP)
	}
	if err := cs.SetCurrencyClampingLateFee(ctx, id, money.BRL, model.MaxBRLLateFeeBP); err != nil {
		t.Fatalf("set BRL: %v", err)
	}
	if got, _ = cs.GetLateFeeSettings(ctx, id); got.FeeBP != model.MaxBRLLateFeeBP || got.InterestBP != 100 {
		t.Fatalf("BRL settings = %+v, want fee clamped to 200 and interest 100", got)
	}
	if c, _ := cs.GetCurrency(ctx, id); c != money.BRL {
		t.Fatalf("currency = %q, want BRL", c)
	}
	if err := cs.SetCurrencyClampingLateFee(ctx, id, money.Currency("USD"), 200); err == nil {
		t.Fatal("expected error for invalid currency")
	}
	if err := cs.SetCurrencyClampingLateFee(ctx, missing, money.BRL, 200); !errors.Is(err, ErrNotFound) {
		t.Fatalf("clamp missing = %v, want ErrNotFound", err)
	}
}

// Migration 0019 applies on a database that already holds phase-1 quota data
// (0017/0018): ordinary issuances keep working, the late-fee defaults and the
// EUR backfill apply, and the issuance owner CHECK rejects both/neither.
func TestMigration0019OnExistingQuotaData(t *testing.T) {
	url := testutil.TestDatabaseURL("store_migration_0019")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	ctx := context.Background()
	db, err := Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.ExecContext(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}

	const dir = "../../../migrations"
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found (%v)", err)
	}
	sort.Strings(files)
	oldDir := t.TempDir()
	for _, f := range files {
		name := filepath.Base(f)
		if name >= "0019" {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(oldDir, name), b, 0o600); err != nil {
			t.Fatalf("copy %s: %v", name, err)
		}
	}
	if err := Migrate(ctx, db, oldDir); err != nil {
		t.Fatalf("migrate to the 0018 schema: %v", err)
	}

	var brl, eur, user, unit, budget, issuance, account string
	mustScan := func(what, query string, dest *string, args ...any) {
		t.Helper()
		if err := db.QueryRowContext(ctx, query, args...).Scan(dest); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	mustScan("brl condominium", `INSERT INTO condominiums (name, currency) VALUES ('Legacy BRL', 'BRL') RETURNING id`, &brl)
	mustScan("eur condominium", `INSERT INTO condominiums (name, currency) VALUES ('Legacy EUR', 'EUR') RETURNING id`, &eur)
	mustScan("user", `INSERT INTO users (email, password_hash) VALUES ('mig0019@example.com', 'x') RETURNING id`, &user)
	mustScan("unit", `INSERT INTO units (condominium_id, code, share_ppm) VALUES ($1, 'A', 1000000) RETURNING id`, &unit, brl)
	mustScan("budget", `
		INSERT INTO quota_budgets (condominium_id, year, status, periodicity, due_day, reserve_rate_bp, approved_at, created_by)
		VALUES ($1, 2026, 'approved', 'monthly', 8, 1000, now(), $2) RETURNING id`, &budget, brl, user)
	mustScan("issuance", `
		INSERT INTO quota_issuances (condominium_id, budget_id, period_index, status, total_cents, base_cents, reserve_cents, created_by)
		VALUES ($1, $2, 12, 'issued', 110000, 100000, 10000, $3) RETURNING id`, &issuance, brl, budget, user)
	mustScan("receivable", `
		INSERT INTO finance_accounts (condominium_id, account_type, title, category, amount_cents, due_date, status, unit_id, created_by)
		VALUES ($1, 'receivable', 'Quota 2026-12 A', 'condo_fee', 110000, '2026-12-08', 'pending', $2, $3) RETURNING id`, &account, brl, unit, user)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO quota_charges (condominium_id, issuance_id, unit_id, finance_account_id, share_ppm, base_cents, reserve_cents)
		VALUES ($1, $2, $3, $4, 1000000, 100000, 10000)`, brl, issuance, unit, account); err != nil {
		t.Fatalf("charge: %v", err)
	}

	if err := Migrate(ctx, db, dir); err != nil {
		t.Fatalf("migrate existing quota data to 0019: %v", err)
	}

	cs := NewCondominiumStore(db)
	if got, err := cs.GetLateFeeSettings(ctx, brl); err != nil || got != (model.LateFeeSettings{FeeBP: 200, InterestBP: 100}) {
		t.Fatalf("existing BRL settings = %+v, %v; want 200/100/0", got, err)
	}
	if got, err := cs.GetLateFeeSettings(ctx, eur); err != nil || got != (model.LateFeeSettings{}) {
		t.Fatalf("existing EUR settings = %+v, %v; want 0/0/0", got, err)
	}
	var fresh string
	mustScan("new condominium", `INSERT INTO condominiums (name) VALUES ('New BRL') RETURNING id`, &fresh)
	if got, err := cs.GetLateFeeSettings(ctx, fresh); err != nil || got != (model.LateFeeSettings{FeeBP: 200, InterestBP: 100}) {
		t.Fatalf("new BRL settings = %+v, %v; want 200/100/0", got, err)
	}

	// The existing ordinary issuance survived, and ordinary issuing still works.
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM quota_issuances WHERE id=$1 AND budget_id=$2 AND extraordinary_id IS NULL`, issuance, budget).Scan(&n); err != nil || n != 1 {
		t.Fatalf("existing issuance after 0019: count=%d err=%v", n, err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO quota_issuances (condominium_id, budget_id, period_index, status, total_cents, base_cents, reserve_cents, created_by)
		VALUES ($1, $2, 11, 'issued', 110000, 100000, 10000, $3)`, brl, budget, user); err != nil {
		t.Fatalf("ordinary issuance after 0019: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO quota_issuances (condominium_id, budget_id, period_index, status, total_cents, base_cents, reserve_cents, created_by)
		VALUES ($1, $2, 11, 'issued', 110000, 100000, 10000, $3)`, brl, budget, user); err == nil {
		t.Fatal("second active issuance of the same period must still be rejected")
	}

	// The owner CHECK: exactly one of budget_id / extraordinary_id.
	var extra string
	mustScan("extraordinary", `
		INSERT INTO quota_extraordinaries (condominium_id, description, total_cents, division, installments, first_due_date, created_by)
		VALUES ($1, 'Roof', 50000, 'share', 2, '2027-01-10', $2) RETURNING id`, &extra, brl, user)
	const insertIssuance = `
		INSERT INTO quota_issuances (condominium_id, budget_id, extraordinary_id, period_index, status, total_cents, base_cents, reserve_cents, created_by)
		VALUES ($1, $2, $3, $4, 'issued', 25000, 25000, 0, $5)`
	if _, err := db.ExecContext(ctx, insertIssuance, brl, budget, extra, 1, user); err == nil {
		t.Fatal("issuance with both budget_id and extraordinary_id must be rejected")
	}
	if _, err := db.ExecContext(ctx, insertIssuance, brl, nil, nil, 1, user); err == nil {
		t.Fatal("issuance with neither budget_id nor extraordinary_id must be rejected")
	}
	if _, err := db.ExecContext(ctx, insertIssuance, brl, nil, extra, 1, user); err != nil {
		t.Fatalf("extraordinary installment issuance: %v", err)
	}
	if _, err := db.ExecContext(ctx, insertIssuance, brl, nil, extra, 1, user); err == nil {
		t.Fatal("duplicate installment number must be rejected")
	}
	// A different condominium cannot reference this extraordinary quota.
	if _, err := db.ExecContext(ctx, insertIssuance, eur, nil, extra, 2, user); err == nil {
		t.Fatal("cross-condominium extraordinary reference must be rejected")
	}
}

func TestCondominiumStoreCurrency(t *testing.T) {
	db := openStoreTestDB(t)
	ctx := context.Background()
	var id string
	if err := db.QueryRowContext(ctx, `INSERT INTO condominiums (name) VALUES ('Currency Test') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("insert condominium: %v", err)
	}
	cs := NewCondominiumStore(db)

	got, err := cs.GetCurrency(ctx, id)
	if err != nil || got != money.BRL {
		t.Fatalf("default currency = %q, %v; want BRL", got, err)
	}
	if err := cs.SetCurrency(ctx, id, money.EUR); err != nil {
		t.Fatalf("set currency: %v", err)
	}
	got, err = cs.GetCurrency(ctx, id)
	if err != nil || got != money.EUR {
		t.Fatalf("currency = %q, %v; want EUR", got, err)
	}
	if err := cs.SetCurrency(ctx, id, money.Currency("USD")); err == nil {
		t.Fatal("expected error for invalid currency")
	}
	missing := "00000000-0000-0000-0000-000000000000"
	if _, err := cs.GetCurrency(ctx, missing); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing = %v, want ErrNotFound", err)
	}
	if err := cs.SetCurrency(ctx, missing, money.EUR); !errors.Is(err, ErrNotFound) {
		t.Fatalf("set missing = %v, want ErrNotFound", err)
	}
}
