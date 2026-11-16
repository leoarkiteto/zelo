package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/store"
)

// AC-4 (follow-up I7): migration 0018 applies on a database that already
// holds quota data created under 0017. The existing charge, issuance and
// receivable survive, the single-column FKs are replaced by the
// condominium-scoped composite FKs, and deleting an issuance still cascades.
func TestMigration0018OnExistingQuotaData(t *testing.T) {
	db := freshMigrationDB(t)
	ctx := context.Background()

	// The schema as it was after phase 1 of the quotas: migrations 0001-0017.
	oldDir := t.TempDir()
	for _, f := range migrationFiles(t) {
		name := filepath.Base(f)
		if name >= "0018" {
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
	if err := store.Migrate(ctx, db, oldDir); err != nil {
		t.Fatalf("migrate to the 0017 schema: %v", err)
	}

	var condo, user, unit, budget, issuance, account, charge string
	mustScan := func(what, query string, dest *string, args ...any) {
		t.Helper()
		if err := db.QueryRow(query, args...).Scan(dest); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	mustScan("condominium", `INSERT INTO condominiums (name) VALUES ('Quota Legacy') RETURNING id`, &condo)
	mustScan("user", `INSERT INTO users (email, password_hash) VALUES ('quota-legacy@example.com', 'x') RETURNING id`, &user)
	mustScan("unit", `INSERT INTO units (condominium_id, code, share_ppm) VALUES ($1, 'A', 1000000) RETURNING id`, &unit, condo)
	mustScan("budget", `
		INSERT INTO quota_budgets (condominium_id, year, status, periodicity, due_day, reserve_rate_bp, approved_at, created_by)
		VALUES ($1, 2026, 'approved', 'monthly', 8, 1000, now(), $2) RETURNING id`, &budget, condo, user)
	mustScan("issuance", `
		INSERT INTO quota_issuances (condominium_id, budget_id, period_index, status, total_cents, base_cents, reserve_cents, created_by)
		VALUES ($1, $2, 12, 'issued', 110000, 100000, 10000, $3) RETURNING id`, &issuance, condo, budget, user)
	mustScan("receivable", `
		INSERT INTO finance_accounts (condominium_id, account_type, title, category, amount_cents, due_date, status, unit_id, created_by)
		VALUES ($1, 'receivable', 'Quota 2026-12 A', 'condo_fee', 110000, '2026-12-08', 'pending', $2, $3) RETURNING id`, &account, condo, unit, user)
	mustScan("charge", `
		INSERT INTO quota_charges (condominium_id, issuance_id, unit_id, finance_account_id, share_ppm, base_cents, reserve_cents)
		VALUES ($1, $2, $3, $4, 1000000, 100000, 10000) RETURNING id`, &charge, condo, issuance, unit, account)

	if err := store.Migrate(ctx, db, migrationsDir); err != nil {
		t.Fatalf("migrate existing quota data to 0018: %v", err)
	}

	// Existing rows survive unchanged.
	var gotIssuance, gotAccount string
	var base, reserve int64
	if err := db.QueryRow(`SELECT issuance_id, finance_account_id, base_cents, reserve_cents FROM quota_charges WHERE id=$1`, charge).
		Scan(&gotIssuance, &gotAccount, &base, &reserve); err != nil {
		t.Fatalf("existing charge after 0018: %v", err)
	}
	if gotIssuance != issuance || gotAccount != account || base != 100000 || reserve != 10000 {
		t.Fatalf("charge changed: issuance=%s account=%s base=%d reserve=%d", gotIssuance, gotAccount, base, reserve)
	}
	var amount int64
	if err := db.QueryRow(`SELECT amount_cents FROM finance_accounts WHERE id=$1`, account).Scan(&amount); err != nil || amount != 110000 {
		t.Fatalf("existing receivable after 0018: amount=%d err=%v", amount, err)
	}

	// The composite FKs exist and the old single-column FKs are gone.
	cons := strSet(t, db, `SELECT conname FROM pg_constraint WHERE conrelid = 'quota_charges'::regclass AND contype = 'f'`)
	for _, want := range []string{"quota_charges_issuance_fk", "quota_charges_finance_account_fk", "quota_charges_unit_fk"} {
		if !cons[want] {
			t.Errorf("foreign key %s missing after 0018; have %v", want, cons)
		}
	}
	for _, gone := range []string{"quota_charges_issuance_id_fkey", "quota_charges_finance_account_id_fkey"} {
		if cons[gone] {
			t.Errorf("old single-column foreign key %s still present after 0018", gone)
		}
	}

	// Deleting the issuance still cascades to its charge.
	if _, err := db.Exec(`DELETE FROM quota_issuances WHERE id=$1`, issuance); err != nil {
		t.Fatalf("delete issuance: %v", err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM quota_charges WHERE id=$1`, charge).Scan(&n); err != nil || n != 0 {
		t.Fatalf("charge not cascaded on issuance delete: count=%d err=%v", n, err)
	}
}
