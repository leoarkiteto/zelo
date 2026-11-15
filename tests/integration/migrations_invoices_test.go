package integration

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

const migrationsDir = "../../migrations"

// freshMigrationDB opens a dedicated, emptied database (never the app data).
func freshMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	url := testutil.TestDatabaseURL("integration_migrations")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := store.Open(url)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	return db
}

func migrationFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no migrations found (%v)", err)
	}
	sort.Strings(files)
	return files
}

func strSet(t *testing.T, db *sql.DB, query string, args ...any) map[string]bool {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[s] = true
	}
	return out
}

// AC-31: the migrations apply cleanly on an empty database (and again, as a
// no-op), and create the listed indexes and foreign keys.
func TestMigrationsEmptyDatabaseSchema(t *testing.T) {
	db := freshMigrationDB(t)
	ctx := context.Background()
	if err := store.Migrate(ctx, db, migrationsDir); err != nil {
		t.Fatalf("migrate empty database: %v", err)
	}
	if err := store.Migrate(ctx, db, migrationsDir); err != nil {
		t.Fatalf("second migrate run: %v", err)
	}
	var applied int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil || applied != len(migrationFiles(t)) {
		t.Fatalf("applied migrations = %d (%v), want %d", applied, err, len(migrationFiles(t)))
	}

	indexes := strSet(t, db, `SELECT indexname FROM pg_indexes WHERE schemaname='public'`)
	for _, ix := range []string{
		"idx_suppliers_condo_nif", "idx_suppliers_condo_name",
		"idx_invoices_condo_issue_date", "idx_invoices_condo_supplier", "idx_invoices_condo_number", "idx_invoices_duplicate",
		"idx_invoice_items_invoice", "idx_invoice_taxes_invoice", "idx_invoice_payments_invoice", "idx_invoice_attachments_invoice",
	} {
		if !indexes[ix] {
			t.Errorf("index %s is missing", ix)
		}
	}
	var def string
	if err := db.QueryRow(`SELECT indexdef FROM pg_indexes WHERE indexname='idx_invoices_duplicate'`).Scan(&def); err != nil {
		t.Fatalf("duplicate index: %v", err)
	}
	for _, part := range []string{"UNIQUE", "condominium_id", "supplier_id", "document_type", "number", "deleted_at IS NULL"} {
		if !strings.Contains(def, part) {
			t.Errorf("idx_invoices_duplicate = %q, want it to contain %q", def, part)
		}
	}

	// Foreign keys: name -> "table>referenced table/column count/delete action".
	type fk struct {
		table, ref string
		cols       int
		onDelete   string
	}
	wantFK := map[string]fk{
		"invoices_supplier_fk":           {"invoices", "suppliers", 2, "a"},
		"invoices_original_fk":           {"invoices", "invoices", 2, "a"},
		"invoice_items_invoice_fk":       {"invoice_items", "invoices", 2, "c"},
		"invoice_taxes_invoice_fk":       {"invoice_taxes", "invoices", 2, "c"},
		"invoice_payments_invoice_fk":    {"invoice_payments", "invoices", 2, "c"},
		"invoice_attachments_invoice_fk": {"invoice_attachments", "invoices", 2, "c"},
	}
	rows, err := db.Query(`
		SELECT conname, conrelid::regclass::text, confrelid::regclass::text, array_length(conkey, 1), confdeltype::text
		FROM pg_constraint WHERE contype = 'f'`)
	if err != nil {
		t.Fatalf("foreign keys: %v", err)
	}
	defer rows.Close()
	got := map[string]fk{}
	for rows.Next() {
		var name string
		var f fk
		if err := rows.Scan(&name, &f.table, &f.ref, &f.cols, &f.onDelete); err != nil {
			t.Fatalf("scan fk: %v", err)
		}
		got[name] = f
	}
	for name, want := range wantFK {
		if g, ok := got[name]; !ok {
			t.Errorf("foreign key %s is missing", name)
		} else if g != want {
			t.Errorf("foreign key %s = %+v, want %+v", name, g, want)
		}
	}
	// The condominium and user references of every new table.
	for _, table := range []string{"suppliers", "invoices"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM pg_constraint WHERE contype='f' AND conrelid=$1::regclass AND confrelid='condominiums'::regclass`, table).Scan(&n); err != nil || n != 1 {
			t.Errorf("%s -> condominiums foreign keys = %d (%v), want 1", table, n, err)
		}
	}
	for _, table := range []string{"suppliers", "invoices", "invoice_items", "invoice_taxes", "invoice_payments", "invoice_attachments"} {
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM pg_constraint WHERE contype='f' AND conrelid=$1::regclass AND confrelid='users'::regclass`, table).Scan(&n); err != nil || n < 2 {
			t.Errorf("%s -> users (created_by, updated_by) foreign keys = %d (%v), want 2", table, n, err)
		}
	}

	// FR-34 / AC-28: credit note type reserved, nullable original reference.
	var nullable string
	if err := db.QueryRow(`SELECT is_nullable FROM information_schema.columns WHERE table_name='invoices' AND column_name='original_invoice_id'`).Scan(&nullable); err != nil || nullable != "YES" {
		t.Errorf("invoices.original_invoice_id nullable = %q (%v), want YES", nullable, err)
	}
	var check string
	if err := db.QueryRow(`SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conname='invoices_document_type_check'`).Scan(&check); err != nil || !strings.Contains(check, "nota_credito") {
		t.Errorf("document type check = %q (%v), want nota_credito allowed", check, err)
	}
	var curDefault, curNull string
	if err := db.QueryRow(`SELECT column_default, is_nullable FROM information_schema.columns WHERE table_name='condominiums' AND column_name='currency'`).Scan(&curDefault, &curNull); err != nil ||
		!strings.Contains(curDefault, "BRL") || curNull != "NO" {
		t.Errorf("condominiums.currency default/nullable = %q/%q (%v), want BRL / NO", curDefault, curNull, err)
	}

	// The composite foreign key keeps a row from pointing at another
	// condominium's supplier (FR-31).
	var condoA, condoB, user, supplier string
	for _, c := range []struct {
		dst  *string
		name string
	}{{&condoA, "A"}, {&condoB, "B"}} {
		if err := db.QueryRow(`INSERT INTO condominiums (name) VALUES ($1) RETURNING id`, "Schema "+c.name).Scan(c.dst); err != nil {
			t.Fatalf("condominium %s: %v", c.name, err)
		}
	}
	if err := db.QueryRow(`INSERT INTO users (email, password_hash) VALUES ('schema@example.com', 'x') RETURNING id`).Scan(&user); err != nil {
		t.Fatalf("user: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO suppliers (condominium_id, name, created_by, updated_by) VALUES ($1, 'S', $2, $2) RETURNING id`, condoA, user).Scan(&supplier); err != nil {
		t.Fatalf("supplier: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO invoices (condominium_id, supplier_id, created_by, updated_by) VALUES ($1, $2, $3, $3)`, condoA, supplier, user); err != nil {
		t.Fatalf("invoice of the supplier's own condominium: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO invoices (condominium_id, supplier_id, created_by, updated_by) VALUES ($1, $2, $3, $3)`, condoB, supplier, user); err == nil {
		t.Errorf("an invoice of condominium B accepted the supplier of condominium A")
	}
}

// AC-1 and AC-31: the new migrations apply on a database that already holds
// data. Existing condominiums become BRL, existing rows survive and the
// finance pages keep rendering "R$" amounts.
func TestMigrationsOnExistingData(t *testing.T) {
	db := freshMigrationDB(t)
	redisURL := testutil.TestRedisURL("integration")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL not set; skipping integration test")
	}
	ctx := context.Background()

	// The schema as it was before the invoices module: migrations 0001-0012.
	oldDir := t.TempDir()
	for _, f := range migrationFiles(t) {
		name := filepath.Base(f)
		if name >= "0013" {
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
		t.Fatalf("migrate to the pre-invoices schema: %v", err)
	}

	hash, err := security.NewPasswordHasher("integration-test-pepper").Hash("legacy-pass-123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	var condo, user string
	if err := db.QueryRow(`INSERT INTO condominiums (name) VALUES ('Legacy Condo') RETURNING id`).Scan(&condo); err != nil {
		t.Fatalf("legacy condominium: %v", err)
	}
	if err := db.QueryRow(`INSERT INTO users (email, password_hash) VALUES ('legacy@example.com', $1) RETURNING id`, hash).Scan(&user); err != nil {
		t.Fatalf("legacy user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_roles (user_id, condominium_id, role) VALUES ($1, $2, 'syndic')`, user, condo); err != nil {
		t.Fatalf("legacy role: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO finance_accounts (condominium_id, account_type, title, category, amount_cents, due_date, status, created_by)
		VALUES ($1, 'receivable', 'Legacy fee', 'condo_fee', 12345, now() + interval '20 days', 'pending', $2)`, condo, user); err != nil {
		t.Fatalf("legacy finance account: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO audit_events (user_id, event_type, details) VALUES ($1, 'sign_in', '{}')`, user); err != nil {
		t.Fatalf("legacy audit event: %v", err)
	}

	if err := store.Migrate(ctx, db, migrationsDir); err != nil {
		t.Fatalf("migrate existing data: %v", err)
	}

	var currency string
	if err := db.QueryRow(`SELECT currency FROM condominiums WHERE id=$1`, condo).Scan(&currency); err != nil || currency != "BRL" {
		t.Fatalf("existing condominium currency = %q (%v), want BRL", currency, err)
	}
	var title string
	if err := db.QueryRow(`SELECT title FROM finance_accounts WHERE condominium_id=$1`, condo).Scan(&title); err != nil || title != "Legacy fee" {
		t.Fatalf("existing finance account after migration = %q (%v)", title, err)
	}
	if n := (&invEnv{db: db}).count(t, `SELECT count(*) FROM audit_events WHERE event_type='sign_in'`); n != 1 {
		t.Fatalf("existing audit events = %d, want 1", n)
	}
	// The audit allow-list now accepts the module's events and still the old ones.
	for _, ev := range []string{"invoice_registered", "supplier_created", "condominium_currency_changed", "ticket_created"} {
		if _, err := db.Exec(`INSERT INTO audit_events (user_id, event_type, details) VALUES ($1, $2, '{}')`, user, ev); err != nil {
			t.Errorf("audit event %s rejected after migration: %v", ev, err)
		}
	}

	// The composed app on the migrated database still renders R$ amounts.
	redisClient, err := store.OpenRedis(redisURL)
	if err != nil {
		t.Fatalf("open redis: %v", err)
	}
	t.Cleanup(func() { _ = redisClient.Close() })
	if err := redisClient.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}
	ts := httptest.NewServer(composeApp(t, db, redisClient))
	defer ts.Close()
	c := loginAs(t, ts.URL, "legacy@example.com", "legacy-pass-123")
	code, _, body := get(t, c, ts.URL+"/finance", nil)
	if code != http.StatusOK {
		t.Fatalf("legacy finance page = %d", code)
	}
	if !strings.Contains(body, "R$ 123.45") {
		t.Errorf("finance page lacks the legacy amount R$ 123.45\n%s", body)
	}
	if strings.Contains(body, "€") {
		t.Errorf("finance page renders euro for a migrated condominium")
	}
}
