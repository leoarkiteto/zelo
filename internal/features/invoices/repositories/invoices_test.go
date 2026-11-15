package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	sharedstore "github.com/leoarkiteto/zelo/internal/shared/store"
	"github.com/leoarkiteto/zelo/internal/shared/testutil"
)

type fixture struct {
	db        *sql.DB
	suppliers *SupplierStore
	invoices  *InvoiceStore
	condo     string
	otherCond string
	user      string
	supplier  domain.Supplier
	seq       int
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	url := testutil.TestDatabaseURL("invoices")
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
		TRUNCATE invoice_attachments, invoice_payments, invoice_taxes, invoice_items,
		invoices, suppliers, unit_occupancies, user_roles, units, condominiums, users
		RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	f := &fixture{db: db, suppliers: NewSupplierStore(db), invoices: NewInvoiceStore(db)}
	must := func(q string, dest *string, args ...any) {
		if err := db.QueryRowContext(ctx, q, args...).Scan(dest); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	must(`INSERT INTO condominiums (name) VALUES ('Inv Condo') RETURNING id`, &f.condo)
	must(`INSERT INTO condominiums (name) VALUES ('Other Condo') RETURNING id`, &f.otherCond)
	must(`INSERT INTO users (email, password_hash) VALUES ('inv@example.com', 'x') RETURNING id`, &f.user)
	f.supplier = f.newSupplier(t, f.condo, "Fornecedor Um", "501964843")
	return f
}

func (f *fixture) newSupplier(t *testing.T, condo, name, nif string) domain.Supplier {
	t.Helper()
	s, err := f.suppliers.CreateSupplier(context.Background(),
		domain.Supplier{CondominiumID: condo, Name: name, NIF: nif}, f.user)
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	return s
}

func date(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return &t
}

// newInvoice builds a registered invoice with one rate line of baseCents at
// 23 % and a unique number.
func (f *fixture) newInvoice(t *testing.T, baseCents, withholding int64) domain.Invoice {
	t.Helper()
	f.seq++
	lines := []domain.Line{{Description: "Servico", BaseCents: baseCents,
		Treatment: domain.TreatmentRate, RateBP: 2300}}
	totals, err := domain.ComputeTotals(domain.RegionContinental, lines, withholding)
	if err != nil {
		t.Fatalf("compute totals: %v", err)
	}
	return domain.Invoice{
		CondominiumID: f.condo, State: domain.StateRegistered, CreatedBy: f.user, UpdatedBy: f.user,
		DocumentType: domain.DocumentFatura, ExpenseType: domain.ExpensePrestacaoServicos,
		Number: fmt.Sprintf("FT 2026/%d", f.seq), IssueDate: date("2026-03-10"), DueDate: date("2026-04-10"),
		SupplierID: f.supplier.ID, Description: "Descricao", Category: domain.CategoryLimpeza,
		Lines: lines, Totals: totals,
	}
}

func (f *fixture) save(t *testing.T, inv domain.Invoice) string {
	t.Helper()
	id, err := f.invoices.SaveInvoice(context.Background(), inv)
	if err != nil {
		t.Fatalf("save invoice: %v", err)
	}
	return id
}

func (f *fixture) pay(t *testing.T, id string, cents int64) {
	t.Helper()
	_, err := f.invoices.AddPayment(context.Background(), f.condo, id, domain.Payment{
		Date: time.Now(), AmountCents: cents, Method: domain.MethodMultibanco, CreatedBy: f.user})
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
}

func TestMoneyRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	lines := []domain.Line{
		{Description: "Tinta", Product: "Tinta 5L", Reference: "T5", Unit: "un",
			QuantityMilli: 2500, UnitPriceCents: 1999, DiscountCents: 100,
			Treatment: domain.TreatmentRate, RateBP: 2300},
		{Description: "Isento", BaseCents: 5000, Treatment: domain.TreatmentExempt,
			ExemptionCode: "M01", ExemptionReason: "Art. 16"},
	}
	totals, err := domain.ComputeTotals(domain.RegionContinental, lines, 123)
	if err != nil {
		t.Fatal(err)
	}
	inv := f.newInvoice(t, 1, 0)
	inv.Lines, inv.Totals = lines, totals
	inv.Monthly = domain.MonthlyDetails{ServiceType: "agua", Recurring: true, StartDate: date("2026-01-01")}
	inv.Service = domain.ServiceDetails{Contract: "C-1", EndDate: date("2026-12-31")}
	inv.ReferencePeriod = "2026-03"
	id := f.save(t, inv)

	got, err := f.invoices.GetInvoice(ctx, f.condo, id)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Totals, totals) {
		t.Fatalf("totals = %+v, want %+v", got.Totals, totals)
	}
	if len(got.Lines) != 2 || got.Lines[0].QuantityMilli != 2500 || got.Lines[0].UnitPriceCents != 1999 ||
		got.Lines[0].DiscountCents != 100 || got.Lines[1].ExemptionCode != "M01" {
		t.Fatalf("lines = %+v", got.Lines)
	}
	wantBase := int64(2500*1999+500)/1000 - 100 // round half up
	if got.Lines[0].BaseCents != wantBase {
		t.Fatalf("stored base = %d, want %d", got.Lines[0].BaseCents, wantBase)
	}
	if got.Number != inv.Number || got.Supplier == nil || got.Supplier.NIF != "501964843" ||
		got.Monthly.ServiceType != "agua" || !got.Monthly.Recurring || got.Monthly.StartDate == nil ||
		got.Service.Contract != "C-1" || got.ReferencePeriod != "2026-03" || got.State != domain.StateRegistered {
		t.Fatalf("header mismatch: %+v", got)
	}
	if got.IssueDate == nil || got.IssueDate.Format("2006-01-02") != "2026-03-10" {
		t.Fatalf("issue date = %v", got.IssueDate)
	}

	// Another condominium cannot read it (FR-31).
	if _, err := f.invoices.GetInvoice(ctx, f.otherCond, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-condo get = %v, want ErrNotFound", err)
	}
	if _, err := f.invoices.GetInvoice(ctx, f.condo, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("malformed id = %v, want ErrNotFound", err)
	}
}

func TestDuplicateKeysAndIdempotency(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	inv := f.newInvoice(t, 10000, 0)
	inv.IdempotencyKey = "key-1"
	id := f.save(t, inv)

	got, err := f.invoices.FindByIdempotencyKey(ctx, f.condo, "key-1")
	if err != nil || got != id {
		t.Fatalf("find by key = %q, %v; want %q", got, err, id)
	}
	if _, err := f.invoices.FindByIdempotencyKey(ctx, f.otherCond, "key-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other condo key = %v", err)
	}

	// Same key again: typed error with the existing id.
	again := f.newInvoice(t, 10000, 0)
	again.IdempotencyKey = "key-1"
	_, err = f.invoices.SaveInvoice(ctx, again)
	var dk *DuplicateKeyError
	if !errors.Is(err, ErrDuplicateIdempotencyKey) || !errors.As(err, &dk) || dk.ExistingID != id {
		t.Fatalf("duplicate key err = %v", err)
	}

	// Same document key, with whitespace noise in the number.
	dup := f.newInvoice(t, 5000, 0)
	dup.Number = "  " + inv.Number + " "
	_, err = f.invoices.SaveInvoice(ctx, dup)
	var di *DuplicateInvoiceError
	if !errors.Is(err, ErrDuplicateInvoice) || !errors.As(err, &di) || di.ExistingID != id {
		t.Fatalf("duplicate invoice err = %v", err)
	}
	if found, err := f.invoices.FindDuplicate(ctx, f.condo, inv.SupplierID, inv.DocumentType, inv.Number, ""); err != nil || found != id {
		t.Fatalf("FindDuplicate = %q, %v", found, err)
	}
	if _, err := f.invoices.FindDuplicate(ctx, f.condo, inv.SupplierID, inv.DocumentType, inv.Number, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindDuplicate excluding self = %v", err)
	}

	// A different document type or supplier does not clash.
	other := f.newInvoice(t, 5000, 0)
	other.Number, other.DocumentType = inv.Number, domain.DocumentNotaDebito
	f.save(t, other)

	// Annulled invoices keep the key (FR-23).
	f.pay(t, id, 1)
	if err := f.invoices.Annul(ctx, f.condo, id, "erro", f.user); err != nil {
		t.Fatal(err)
	}
	if _, err := f.invoices.SaveInvoice(ctx, dup); !errors.Is(err, ErrDuplicateInvoice) {
		t.Fatalf("annulled keeps key: %v", err)
	}

	// A soft-deleted draft frees both keys.
	d := f.newInvoice(t, 0, 0)
	d.State, d.Totals, d.Lines, d.IdempotencyKey = domain.StateDraft, domain.Totals{}, nil, "key-draft"
	d.Number = "DRAFT-1"
	did := f.save(t, d)
	if err := f.invoices.SoftDeleteDraft(ctx, f.condo, did, f.user); err != nil {
		t.Fatal(err)
	}
	if _, err := f.invoices.GetInvoice(ctx, f.condo, did); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted draft visible: %v", err)
	}
	f.save(t, d)

	// Registered invoices cannot be soft-deleted.
	if err := f.invoices.SoftDeleteDraft(ctx, f.condo, id, f.user); !errors.Is(err, ErrNotDraft) {
		t.Fatalf("delete annulled = %v, want ErrNotDraft", err)
	}
}

func TestSupplierNIFAndSearch(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.suppliers.CreateSupplier(ctx,
		domain.Supplier{CondominiumID: f.condo, Name: "Outro", NIF: "PT 501 964 843"}, f.user)
	if !errors.Is(err, ErrDuplicateNIF) {
		t.Fatalf("duplicate nif (normalized) = %v", err)
	}
	// Same NIF in another condominium is fine.
	f.newSupplier(t, f.otherCond, "Fornecedor Um", "501964843")
	// Suppliers without NIF may repeat.
	f.newSupplier(t, f.condo, "Sem NIF A", "")
	f.newSupplier(t, f.condo, "Sem NIF B", "")

	for _, tc := range []struct {
		q    string
		want int
	}{{"", 3}, {"fornecedor", 1}, {"501964", 1}, {"sem nif", 2}, {"%", 0}} {
		got, err := f.suppliers.SearchSuppliers(ctx, f.condo, tc.q, 10)
		if err != nil || len(got) != tc.want {
			t.Fatalf("search %q = %d, %v; want %d", tc.q, len(got), err, tc.want)
		}
	}

	s := f.supplier
	s.Name = "Renomeado"
	if err := f.suppliers.UpdateSupplier(ctx, s, f.user); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.suppliers.GetSupplier(ctx, f.condo, s.ID); got.Name != "Renomeado" {
		t.Fatalf("name = %q", got.Name)
	}
	if _, err := f.suppliers.GetSupplier(ctx, f.otherCond, s.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-condo supplier = %v", err)
	}
	s.CondominiumID = f.otherCond
	if err := f.suppliers.UpdateSupplier(ctx, s, f.user); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-condo update = %v", err)
	}
}

func TestCrossCondominiumSupplierRejected(t *testing.T) {
	f := newFixture(t)
	foreign := f.newSupplier(t, f.otherCond, "Estrangeiro", "500100144")
	inv := f.newInvoice(t, 10000, 0)
	inv.SupplierID = foreign.ID
	if _, err := f.invoices.SaveInvoice(context.Background(), inv); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("save with foreign supplier = %v, want ErrInvalidReference", err)
	}

	// Raw SQL is rejected by the composite FK too (defence in depth).
	_, err := f.db.Exec(`
		INSERT INTO invoices (condominium_id, state, supplier_id, created_by, updated_by)
		VALUES ($1, 'draft', $2, $3, $3)`, f.condo, foreign.ID, f.user)
	if pe := pgError(err); pe == nil || pe.Code != "23503" {
		t.Fatalf("raw insert = %v, want FK violation", err)
	}

	// Editing to a foreign supplier is rejected as well.
	id := f.save(t, f.newInvoice(t, 10000, 0))
	upd := f.newInvoice(t, 10000, 0)
	upd.ID, upd.SupplierID = id, foreign.ID
	if err := f.invoices.UpdateInvoice(context.Background(), upd); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("update with foreign supplier = %v", err)
	}
	got, _ := f.invoices.GetInvoice(context.Background(), f.condo, id)
	if got.SupplierID != f.supplier.ID {
		t.Fatalf("supplier changed despite rollback: %s", got.SupplierID)
	}
}

func TestListFiltersAndTotals(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	other := f.newSupplier(t, f.condo, "Eletricidade SA", "500100144")

	mk := func(mod func(*domain.Invoice), base int64) string {
		inv := f.newInvoice(t, base, 0)
		mod(&inv)
		return f.save(t, inv)
	}
	a := mk(func(i *domain.Invoice) { i.IssueDate = date("2026-01-05"); i.DueDate = date("2026-02-05") }, 10000) // 12300
	b := mk(func(i *domain.Invoice) {
		i.IssueDate, i.DueDate, i.SupplierID = date("2026-02-05"), date("2026-03-05"), other.ID
		i.Category, i.ExpenseType, i.ReferencePeriod = domain.CategoryEletricidade, domain.ExpenseDespesaMensal, "2026-02"
	}, 20000) // 24600
	c := mk(func(i *domain.Invoice) { i.IssueDate = date("2026-03-05") }, 30000) // annulled, 36900
	mk(func(i *domain.Invoice) {                                                 // draft
		i.State, i.Totals, i.Lines, i.Number = domain.StateDraft, domain.Totals{}, nil, "RASC"
		i.IssueDate = date("2026-04-05")
	}, 0)
	f.pay(t, a, 5000)
	f.pay(t, b, 24600)
	f.pay(t, c, 100)
	if err := f.invoices.Annul(ctx, f.condo, c, "motivo", f.user); err != nil {
		t.Fatal(err)
	}
	// Invoice in another condominium must never show up.
	foreign := f.newSupplier(t, f.otherCond, "Fora", "501964843")
	fi := f.newInvoice(t, 99900, 0)
	fi.CondominiumID, fi.SupplierID = f.otherCond, foreign.ID
	f.save(t, fi)

	res, err := f.invoices.List(ctx, f.condo, domain.ListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowCount != 4 || len(res.Rows) != 4 || res.Page != 1 || res.PageSize != domain.DefaultPageSize {
		t.Fatalf("rowcount=%d rows=%d page=%d size=%d", res.RowCount, len(res.Rows), res.Page, res.PageSize)
	}
	// Default order: issue date desc.
	if res.Rows[0].Status != domain.StatusRascunho || res.Rows[1].ID != c || res.Rows[2].ID != b || res.Rows[3].ID != a {
		t.Fatalf("order/status wrong: %+v", res.Rows)
	}
	if res.Rows[1].Status != domain.StatusAnulado || res.Rows[2].Status != domain.StatusPago ||
		res.Rows[3].Status != domain.StatusParcialmentePago {
		t.Fatalf("statuses: %+v", res.Rows)
	}
	// Totals exclude the draft and the annulled invoice.
	want := domain.ListTotals{TotalCents: 12300 + 24600, PayableCents: 12300 + 24600,
		PaidCents: 5000 + 24600, DebtCents: 12300 - 5000, Count: 2}
	if res.Totals != want {
		t.Fatalf("totals = %+v, want %+v", res.Totals, want)
	}
	if r := res.Rows[3]; r.SupplierName != "Fornecedor Um" || r.SupplierNIF != "501964843" ||
		r.PaidCents != 5000 || r.DebtCents != 7300 {
		t.Fatalf("row a = %+v", r)
	}

	filters := []struct {
		name string
		f    domain.ListFilter
		want int
	}{
		{"supplier name", domain.ListFilter{SupplierName: "eletric"}, 1},
		{"nif", domain.ListFilter{NIF: "501 964"}, 3},
		{"number", domain.ListFilter{Number: "/1"}, 1},
		{"document type", domain.ListFilter{DocumentType: "nota_debito"}, 0},
		{"expense type", domain.ListFilter{ExpenseType: "despesa_mensal"}, 1},
		{"category", domain.ListFilter{Category: "eletricidade"}, 1},
		{"status draft", domain.ListFilter{Status: "rascunho"}, 1},
		{"status annulled", domain.ListFilter{Status: "anulado"}, 1},
		{"status pago", domain.ListFilter{Status: "pago"}, 1},
		{"issue range", domain.ListFilter{IssueFrom: "2026-02-01", IssueTo: "2026-03-31"}, 2},
		{"due range", domain.ListFilter{DueFrom: "2026-03-01", DueTo: "2026-03-31"}, 1},
		{"reference period", domain.ListFilter{ReferencePeriod: "2026-02"}, 1},
		{"AND combination", domain.ListFilter{SupplierName: "um", Status: "pago"}, 0},
		{"AND combination hit", domain.ListFilter{SupplierName: "um", Status: "parcialmente_pago"}, 1},
	}
	for _, tc := range filters {
		got, err := f.invoices.List(ctx, f.condo, tc.f)
		if err != nil || got.RowCount != tc.want || len(got.Rows) != tc.want {
			t.Errorf("%s: rowcount=%d rows=%d err=%v, want %d", tc.name, got.RowCount, len(got.Rows), err, tc.want)
		}
	}

	// Totals cover the whole filtered set, not the page.
	p1, err := f.invoices.List(ctx, f.condo, domain.ListFilter{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(p1.Rows) != 1 || p1.RowCount != 4 || p1.Totals != want {
		t.Fatalf("page 1: %d rows, rowcount %d, totals %+v", len(p1.Rows), p1.RowCount, p1.Totals)
	}
	p4, _ := f.invoices.List(ctx, f.condo, domain.ListFilter{Page: 4, PageSize: 1})
	if len(p4.Rows) != 1 || p4.Rows[0].ID != a {
		t.Fatalf("page 4 = %+v", p4.Rows)
	}
	p5, _ := f.invoices.List(ctx, f.condo, domain.ListFilter{Page: 5, PageSize: 1})
	if len(p5.Rows) != 0 || p5.RowCount != 4 {
		t.Fatalf("page 5 = %+v", p5)
	}

	// The other condominium sees only its own invoice.
	ores, _ := f.invoices.List(ctx, f.otherCond, domain.ListFilter{})
	if ores.RowCount != 1 || ores.Totals.TotalCents != 99900*123/100 {
		t.Fatalf("other condo list = %+v", ores)
	}
}

func TestStatusSQLParity(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	type tc struct {
		name          string
		state         domain.State
		base, withh   int64
		pay           int64
		overJustify   bool
		annulAfterPay bool
	}
	// payable = base*1.23 - withholding
	cases := []tc{
		{name: "draft", state: domain.StateDraft},
		{name: "annulled", state: domain.StateRegistered, base: 10000, pay: 100, annulAfterPay: true},
		{name: "payable zero no payments", state: domain.StateRegistered, base: 10000, withh: 12300},
		{name: "no payments", state: domain.StateRegistered, base: 10000},
		{name: "partial", state: domain.StateRegistered, base: 10000, pay: 5000},
		{name: "exact", state: domain.StateRegistered, base: 10000, pay: 12300},
		{name: "overpaid", state: domain.StateRegistered, base: 10000, pay: 13000, overJustify: true},
	}
	ids := map[string]string{}
	for _, c := range cases {
		inv := f.newInvoice(t, c.base, c.withh)
		if c.state == domain.StateDraft {
			inv.State, inv.Totals, inv.Lines = domain.StateDraft, domain.Totals{}, nil
		}
		id := f.save(t, inv)
		ids[c.name] = id
		if c.pay > 0 {
			j := ""
			if c.overJustify {
				j = "pagamento a mais"
			}
			if _, err := f.invoices.AddPayment(ctx, f.condo, id, domain.Payment{Date: time.Now(),
				AmountCents: c.pay, Method: domain.MethodDinheiro, Justification: j, CreatedBy: f.user}); err != nil {
				t.Fatalf("%s: pay: %v", c.name, err)
			}
		}
		if c.annulAfterPay {
			if err := f.invoices.Annul(ctx, f.condo, id, "x", f.user); err != nil {
				t.Fatal(err)
			}
		}
	}
	res, err := f.invoices.List(ctx, f.condo, domain.ListFilter{PageSize: 100})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]domain.ListRow{}
	for _, r := range res.Rows {
		byID[r.ID] = r
	}
	for _, c := range cases {
		row := byID[ids[c.name]]
		inv, err := f.invoices.GetInvoice(ctx, f.condo, ids[c.name])
		if err != nil {
			t.Fatal(err)
		}
		want := domain.Status(inv.State, inv.Totals.PayableCents, inv.PaidCents())
		if row.Status != want {
			t.Errorf("%s: SQL status %q, domain %q", c.name, row.Status, want)
		}
		// The status filter must agree with the row status.
		fr, err := f.invoices.List(ctx, f.condo, domain.ListFilter{Status: string(want), PageSize: 100})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range fr.Rows {
			if r.ID == ids[c.name] {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: not returned by status filter %q", c.name, want)
		}
	}
	for name, want := range map[string]domain.PaymentStatus{
		"draft": domain.StatusRascunho, "annulled": domain.StatusAnulado,
		"payable zero no payments": domain.StatusPago, "no payments": domain.StatusPendente,
		"partial": domain.StatusParcialmentePago, "exact": domain.StatusPago, "overpaid": domain.StatusPago,
	} {
		if got := byID[ids[name]].Status; got != want {
			t.Errorf("%s: status %q, want %q", name, got, want)
		}
	}
	if d := byID[ids["overpaid"]].DebtCents; d != -700 {
		t.Errorf("overpaid debt = %d, want -700", d)
	}
}

func TestConcurrentPaymentsDoNotExceedPayable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.save(t, f.newInvoice(t, 10000, 0)) // payable 12300

	var ok, over int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.invoices.AddPayment(ctx, f.condo, id, domain.Payment{Date: time.Now(),
				AmountCents: 3000, Method: domain.MethodMBWay, CreatedBy: f.user})
			switch {
			case err == nil:
				atomic.AddInt32(&ok, 1)
			case errors.Is(err, domain.ErrOverpayment):
				atomic.AddInt32(&over, 1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if ok != 4 || over != 8 {
		t.Fatalf("ok=%d over=%d, want 4 and 8", ok, over)
	}
	inv, _ := f.invoices.GetInvoice(ctx, f.condo, id)
	if inv.PaidCents() != 12000 || len(inv.Payments) != 4 {
		t.Fatalf("paid=%d payments=%d", inv.PaidCents(), len(inv.Payments))
	}
}

// The edit blocks on the row lock held by a payment and then sees the
// committed payment, so payable below paid is rejected (FR-19).
func TestEditWaitsForPaymentAndIsRejected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.save(t, f.newInvoice(t, 10000, 0)) // payable 12300

	locked := make(chan struct{})
	release := make(chan struct{})
	payDone := make(chan error, 1)
	go func() {
		payDone <- f.invoices.WithLockedInvoice(ctx, f.condo, id, func(ctx context.Context, l *LockedInvoice) error {
			if _, err := l.AddPayment(ctx, domain.Payment{Date: time.Now(), AmountCents: 10000,
				Method: domain.MethodDinheiro, CreatedBy: f.user}); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked

	upd := f.newInvoice(t, 5000, 0) // payable 6150 < 10000 paid
	upd.ID = id
	editDone := make(chan error, 1)
	go func() { editDone <- f.invoices.UpdateInvoice(ctx, upd) }()

	select {
	case err := <-editDone:
		t.Fatalf("edit did not wait for the lock: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	if err := <-payDone; err != nil {
		t.Fatal(err)
	}
	if err := <-editDone; !errors.Is(err, domain.ErrPayableBelowPaid) {
		t.Fatalf("edit = %v, want ErrPayableBelowPaid", err)
	}
	inv, _ := f.invoices.GetInvoice(ctx, f.condo, id)
	if inv.Totals.PayableCents != 12300 || inv.PaidCents() != 10000 {
		t.Fatalf("payable=%d paid=%d", inv.Totals.PayableCents, inv.PaidCents())
	}
}

// Racing edit and payment always end consistent: paid never exceeds payable.
func TestEditVersusPaymentRace(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for i := 0; i < 15; i++ {
		id := f.save(t, f.newInvoice(t, 10000, 0)) // payable 12300
		upd := f.newInvoice(t, 5000, 0)            // payable 6150
		upd.ID = id

		var wg sync.WaitGroup
		var editErr, payErr error
		wg.Add(2)
		go func() { defer wg.Done(); editErr = f.invoices.UpdateInvoice(ctx, upd) }()
		go func() {
			defer wg.Done()
			_, payErr = f.invoices.AddPayment(ctx, f.condo, id, domain.Payment{Date: time.Now(),
				AmountCents: 10000, Method: domain.MethodDinheiro, CreatedBy: f.user})
		}()
		wg.Wait()

		inv, err := f.invoices.GetInvoice(ctx, f.condo, id)
		if err != nil {
			t.Fatal(err)
		}
		if inv.PaidCents() > inv.Totals.PayableCents {
			t.Fatalf("iteration %d: paid %d > payable %d (edit=%v pay=%v)",
				i, inv.PaidCents(), inv.Totals.PayableCents, editErr, payErr)
		}
		if editErr != nil && !errors.Is(editErr, domain.ErrPayableBelowPaid) {
			t.Fatalf("edit err = %v", editErr)
		}
		if payErr != nil && !errors.Is(payErr, domain.ErrOverpayment) {
			t.Fatalf("pay err = %v", payErr)
		}
		if editErr == nil && payErr == nil {
			t.Fatalf("iteration %d: both succeeded", i)
		}
	}
}

func TestEditAnnulAndDraftRules(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// Draft: no payments (FR-20); completing it registers it.
	d := f.newInvoice(t, 0, 0)
	d.State, d.Totals, d.Lines = domain.StateDraft, domain.Totals{}, nil
	did := f.save(t, d)
	if _, err := f.invoices.AddPayment(ctx, f.condo, did, domain.Payment{Date: time.Now(),
		AmountCents: 1, Method: domain.MethodOutro, CreatedBy: f.user}); !errors.Is(err, domain.ErrDraftNoPayments) {
		t.Fatalf("draft payment = %v", err)
	}
	full := f.newInvoice(t, 10000, 0)
	full.ID, full.Number = did, d.Number
	if err := f.invoices.UpdateInvoice(ctx, full); err != nil {
		t.Fatalf("complete draft: %v", err)
	}
	got, _ := f.invoices.GetInvoice(ctx, f.condo, did)
	if got.State != domain.StateRegistered || len(got.Lines) != 1 || len(got.Totals.Taxes) != 1 {
		t.Fatalf("completed = %+v", got)
	}
	// Edit replaces (not appends) lines and taxes.
	full.Lines = append(full.Lines, domain.Line{Description: "B", BaseCents: 1000, Treatment: domain.TreatmentRate, RateBP: 600})
	full.Totals, _ = domain.ComputeTotals(domain.RegionContinental, full.Lines, 0)
	if err := f.invoices.UpdateInvoice(ctx, full); err != nil {
		t.Fatal(err)
	}
	got, _ = f.invoices.GetInvoice(ctx, f.condo, did)
	if len(got.Lines) != 2 || len(got.Totals.Taxes) != 2 {
		t.Fatalf("after edit lines=%d taxes=%d", len(got.Lines), len(got.Totals.Taxes))
	}
	// Registered cannot go back to draft.
	back := full
	back.State = domain.StateDraft
	if err := f.invoices.UpdateInvoice(ctx, back); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("back to draft = %v", err)
	}

	// Annulment: needs a reason, keeps data, blocks edits, payments, attachments.
	f.pay(t, did, 500)
	if err := f.invoices.Annul(ctx, f.condo, did, " ", f.user); !errors.Is(err, domain.ErrAnnulReasonRequired) {
		t.Fatalf("annul without reason = %v", err)
	}
	if err := f.invoices.Annul(ctx, f.otherCond, did, "x", f.user); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-condo annul = %v", err)
	}
	if err := f.invoices.Annul(ctx, f.condo, did, "duplicada", f.user); err != nil {
		t.Fatal(err)
	}
	if err := f.invoices.Annul(ctx, f.condo, did, "again", f.user); !errors.Is(err, domain.ErrAnnulled) {
		t.Fatalf("second annul = %v", err)
	}
	if err := f.invoices.UpdateInvoice(ctx, full); !errors.Is(err, domain.ErrAnnulled) {
		t.Fatalf("edit annulled = %v", err)
	}
	if _, err := f.invoices.AddPayment(ctx, f.condo, did, domain.Payment{Date: time.Now(),
		AmountCents: 1, Method: domain.MethodOutro, CreatedBy: f.user}); !errors.Is(err, domain.ErrAnnulled) {
		t.Fatalf("pay annulled = %v", err)
	}
	if _, err := f.invoices.AddAttachments(ctx, f.condo, did, []domain.Attachment{{StoredPath: "p", OriginalName: "a.pdf",
		ContentType: "application/pdf", SizeBytes: 1, CreatedBy: f.user}}); !errors.Is(err, domain.ErrAnnulled) {
		t.Fatalf("attach annulled = %v", err)
	}
	got, _ = f.invoices.GetInvoice(ctx, f.condo, did)
	if got.State != domain.StateAnnulled || got.AnnulReason != "duplicada" || got.AnnulledAt == nil ||
		len(got.Lines) != 2 || len(got.Payments) != 1 {
		t.Fatalf("annulled data = %+v", got)
	}

	// Overpayment needs a justification (FR-17).
	id := f.save(t, f.newInvoice(t, 10000, 0))
	over := domain.Payment{Date: time.Now(), AmountCents: 20000, Method: domain.MethodOutro, CreatedBy: f.user}
	if _, err := f.invoices.AddPayment(ctx, f.condo, id, over); !errors.Is(err, domain.ErrOverpayment) {
		t.Fatalf("overpay = %v", err)
	}
	over.Justification = "adiantamento"
	if p, err := f.invoices.AddPayment(ctx, f.condo, id, over); err != nil || p.ID == "" {
		t.Fatalf("justified overpay = %+v, %v", p, err)
	}
}

func TestAttachmentsLimitAndLifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.save(t, f.newInvoice(t, 10000, 0))
	mk := func(n int) []domain.Attachment {
		var out []domain.Attachment
		for i := 0; i < n; i++ {
			out = append(out, domain.Attachment{StoredPath: fmt.Sprintf("invoices/%d-%d", n, i),
				OriginalName: "f.pdf", ContentType: "application/pdf", SizeBytes: 10, CreatedBy: f.user})
		}
		return out
	}
	saved, err := f.invoices.AddAttachments(ctx, f.condo, id, mk(8))
	if err != nil || len(saved) != 8 {
		t.Fatalf("add 8 = %d, %v", len(saved), err)
	}
	if _, err := f.invoices.AddAttachments(ctx, f.condo, id, mk(3)); !errors.Is(err, domain.ErrTooManyAttachments) {
		t.Fatalf("add 3 more = %v", err)
	}
	inv, _ := f.invoices.GetInvoice(ctx, f.condo, id)
	if len(inv.Attachments) != 8 {
		t.Fatalf("attachments = %d (partial insert not rolled back)", len(inv.Attachments))
	}

	// Concurrent adds never exceed 10.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = f.invoices.AddAttachments(ctx, f.condo, id, mk(1)) }()
	}
	wg.Wait()
	inv, _ = f.invoices.GetInvoice(ctx, f.condo, id)
	if len(inv.Attachments) != domain.MaxAttachments {
		t.Fatalf("attachments = %d, want %d", len(inv.Attachments), domain.MaxAttachments)
	}

	att := inv.Attachments[0]
	if got, err := f.invoices.GetAttachment(ctx, f.condo, id, att.ID); err != nil || got.StoredPath != att.StoredPath {
		t.Fatalf("get attachment = %+v, %v", got, err)
	}
	if _, err := f.invoices.GetAttachment(ctx, f.otherCond, id, att.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-condo attachment = %v", err)
	}
	removed, err := f.invoices.RemoveAttachment(ctx, f.condo, id, att.ID)
	if err != nil || removed.StoredPath != att.StoredPath {
		t.Fatalf("remove = %+v, %v", removed, err)
	}
	if _, err := f.invoices.RemoveAttachment(ctx, f.condo, id, att.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("remove twice = %v", err)
	}
}

func TestSchemaConstraints(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	// A registered invoice must be complete (state-conditional CHECK).
	_, err := f.db.ExecContext(ctx, `
		INSERT INTO invoices (condominium_id, state, created_by, updated_by)
		VALUES ($1, 'registered', $2, $2)`, f.condo, f.user)
	if pe := pgError(err); pe == nil || pe.Code != "23514" {
		t.Fatalf("incomplete registered = %v, want CHECK violation", err)
	}
	// Withholding cannot exceed the total.
	inv := f.newInvoice(t, 10000, 0)
	inv.Totals.WithholdingCents = inv.Totals.TotalCents + 1
	inv.Totals.PayableCents = -1
	if _, err := f.invoices.SaveInvoice(ctx, inv); err == nil {
		t.Fatal("withholding above total accepted")
	}
	// Unknown enum values are rejected.
	inv = f.newInvoice(t, 10000, 0)
	inv.Category = "nao_existe"
	if _, err := f.invoices.SaveInvoice(ctx, inv); err == nil {
		t.Fatal("unknown category accepted")
	}
	// nota_credito is a stored value (FR-34).
	inv = f.newInvoice(t, 10000, 0)
	inv.DocumentType = domain.DocumentNotaCredito
	f.save(t, inv)
}
