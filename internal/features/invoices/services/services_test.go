package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/repositories"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// ---- fakes ----

type fakeRepo struct {
	invoices    map[string]*domain.Invoice
	suppliers   map[string]domain.Supplier // by id
	seq         int
	saveErr     error
	attachErr   error
	saveCalls   int
	lastSaved   domain.Invoice
	lastUpdated domain.Invoice
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{invoices: map[string]*domain.Invoice{}, suppliers: map[string]domain.Supplier{}}
}

func (f *fakeRepo) nextID(prefix string) string { f.seq++; return fmt.Sprintf("%s-%d", prefix, f.seq) }

func (f *fakeRepo) addSupplier(condo, id, name string) {
	f.suppliers[id] = domain.Supplier{ID: id, CondominiumID: condo, Name: name}
}

// supplier repo
func (f *fakeRepo) CreateSupplier(_ context.Context, s domain.Supplier, _ string) (domain.Supplier, error) {
	for _, o := range f.suppliers {
		if o.CondominiumID == s.CondominiumID && o.NIF != "" && o.NIF == s.NIF {
			return domain.Supplier{}, repositories.ErrDuplicateNIF
		}
	}
	s.ID = f.nextID("sup")
	f.suppliers[s.ID] = s
	return s, nil
}
func (f *fakeRepo) UpdateSupplier(_ context.Context, s domain.Supplier, _ string) error {
	f.suppliers[s.ID] = s
	return nil
}
func (f *fakeRepo) GetSupplier(_ context.Context, condo, id string) (domain.Supplier, error) {
	s, ok := f.suppliers[id]
	if !ok || s.CondominiumID != condo {
		return domain.Supplier{}, repositories.ErrNotFound
	}
	return s, nil
}
func (f *fakeRepo) SearchSuppliers(_ context.Context, condo, q string, _ int) ([]domain.Supplier, error) {
	var out []domain.Supplier
	for _, s := range f.suppliers {
		if s.CondominiumID == condo {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeRepo) get(condo, id string) (*domain.Invoice, error) {
	inv, ok := f.invoices[id]
	if !ok || inv.CondominiumID != condo {
		return nil, repositories.ErrNotFound
	}
	return inv, nil
}

// invoice repo
func (f *fakeRepo) SaveInvoice(_ context.Context, inv domain.Invoice) (string, error) {
	f.saveCalls++
	if f.saveErr != nil {
		return "", f.saveErr
	}
	inv.ID = f.nextID("inv")
	f.lastSaved = inv
	f.invoices[inv.ID] = &inv
	return inv.ID, nil
}
func (f *fakeRepo) UpdateInvoice(_ context.Context, inv domain.Invoice) error {
	cur, err := f.get(inv.CondominiumID, inv.ID)
	if err != nil {
		return err
	}
	f.lastUpdated = inv
	inv.Payments, inv.Attachments = cur.Payments, cur.Attachments
	f.invoices[inv.ID] = &inv
	return nil
}
func (f *fakeRepo) GetInvoice(_ context.Context, condo, id string) (domain.Invoice, error) {
	inv, err := f.get(condo, id)
	if err != nil {
		return domain.Invoice{}, err
	}
	return *inv, nil
}
func (f *fakeRepo) List(context.Context, string, domain.ListFilter) (domain.ListResult, error) {
	return domain.ListResult{}, nil
}
func (f *fakeRepo) FindByIdempotencyKey(_ context.Context, condo, key string) (string, error) {
	for _, inv := range f.invoices {
		if inv.CondominiumID == condo && inv.IdempotencyKey == key && key != "" {
			return inv.ID, nil
		}
	}
	return "", repositories.ErrNotFound
}
func (f *fakeRepo) FindDuplicate(_ context.Context, condo, sup string, dt domain.DocumentType, number, exclude string) (string, error) {
	for _, inv := range f.invoices {
		if inv.CondominiumID == condo && inv.SupplierID == sup && inv.DocumentType == dt &&
			inv.Number == number && inv.ID != exclude {
			return inv.ID, nil
		}
	}
	return "", repositories.ErrNotFound
}
func (f *fakeRepo) AddPayment(_ context.Context, condo, id string, p domain.Payment) (domain.Payment, error) {
	inv, err := f.get(condo, id)
	if err != nil {
		return domain.Payment{}, err
	}
	if err := domain.CheckPayment(inv.State, inv.Totals.PayableCents, inv.PaidCents(), p.AmountCents, p.Justification); err != nil {
		return domain.Payment{}, err
	}
	p.ID = f.nextID("pay")
	inv.Payments = append(inv.Payments, p)
	return p, nil
}
func (f *fakeRepo) Annul(_ context.Context, condo, id, reason, _ string) error {
	inv, err := f.get(condo, id)
	if err != nil {
		return err
	}
	if err := domain.CheckAnnul(inv.State, reason); err != nil {
		return err
	}
	inv.State = domain.StateAnnulled
	return nil
}
func (f *fakeRepo) SoftDeleteDraft(_ context.Context, condo, id, _ string) error {
	inv, err := f.get(condo, id)
	if err != nil {
		return err
	}
	if inv.State != domain.StateDraft {
		return repositories.ErrNotDraft
	}
	delete(f.invoices, id)
	return nil
}
func (f *fakeRepo) AddAttachments(_ context.Context, condo, id string, atts []domain.Attachment) ([]domain.Attachment, error) {
	if f.attachErr != nil {
		return nil, f.attachErr
	}
	inv, err := f.get(condo, id)
	if err != nil {
		return nil, err
	}
	if err := domain.CheckAttachmentCount(len(inv.Attachments), len(atts)); err != nil {
		return nil, err
	}
	for i := range atts {
		atts[i].ID = f.nextID("att")
		inv.Attachments = append(inv.Attachments, atts[i])
	}
	return atts, nil
}
func (f *fakeRepo) RemoveAttachment(_ context.Context, condo, id, attID string) (domain.Attachment, error) {
	inv, err := f.get(condo, id)
	if err != nil {
		return domain.Attachment{}, err
	}
	for i, a := range inv.Attachments {
		if a.ID == attID {
			inv.Attachments = append(inv.Attachments[:i], inv.Attachments[i+1:]...)
			return a, nil
		}
	}
	return domain.Attachment{}, repositories.ErrNotFound
}
func (f *fakeRepo) GetAttachment(_ context.Context, condo, id, attID string) (domain.Attachment, error) {
	inv, err := f.get(condo, id)
	if err != nil {
		return domain.Attachment{}, err
	}
	for _, a := range inv.Attachments {
		if a.ID == attID {
			return a, nil
		}
	}
	return domain.Attachment{}, repositories.ErrNotFound
}

type auditEvent struct {
	user    string
	event   model.AuditEventType
	details map[string]any
}

type fakeAudit struct{ events []auditEvent }

func (a *fakeAudit) RecordEvent(_ context.Context, uid *string, ev model.AuditEventType, d map[string]any) error {
	a.events = append(a.events, auditEvent{user: *uid, event: ev, details: d})
	return nil
}

func (a *fakeAudit) last(t *testing.T) auditEvent {
	t.Helper()
	if len(a.events) == 0 {
		t.Fatal("no audit event recorded")
	}
	return a.events[len(a.events)-1]
}

// ---- helpers ----

const (
	condoA = "condo-a"
	condoB = "condo-b"
	actor  = "user-1"
)

type env struct {
	repo  *fakeRepo
	audit *fakeAudit
	svc   *InvoiceService
	dir   string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	repo, audit := newFakeRepo(), &fakeAudit{}
	dir := t.TempDir()
	repo.addSupplier(condoA, "sup-a", "ACME")
	repo.addSupplier(condoB, "sup-b", "Other")
	fixed := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	return &env{repo: repo, audit: audit, dir: dir, svc: &InvoiceService{
		Invoices: repo, Suppliers: repo, Audit: audit, UploadDir: dir,
		Now: func() time.Time { return fixed },
	}}
}

func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func validInvoice() domain.Invoice {
	return domain.Invoice{
		IdempotencyKey: "key-1",
		DocumentType:   domain.DocumentFatura,
		ExpenseType:    domain.ExpenseManutencao,
		Number:         "  FT   2026/1 ",
		IssueDate:      day(2026, 11, 1),
		DueDate:        day(2026, 11, 30),
		SupplierID:     "sup-a",
		Description:    "Elevator maintenance",
		Category:       domain.CategoryManutencao,
		Lines: []domain.Line{{
			Description: "service", BaseCents: 10000,
			Treatment: domain.TreatmentRate, RateBP: 2300,
		}},
	}
}

func (e *env) registered(t *testing.T) string {
	t.Helper()
	res, err := e.svc.Register(context.Background(), actor, condoA, validInvoice())
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	e.audit.events = nil
	return res.ID
}

func pdfHeader(t *testing.T, name string, content []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, name))
	part, err := w.CreatePart(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(content)
	_ = w.Close()
	form, err := multipart.NewReader(&buf, w.Boundary()).ReadForm(1 << 20)
	if err != nil {
		t.Fatal(err)
	}
	return form.File["file"][0]
}

var pdfBytes = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n")

func filesIn(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, "invoices"))
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

// ---- invoice tests ----

func TestRegisterSetsTotalsActorAndAudit(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.Register(context.Background(), actor, condoA, validInvoice())
	if err != nil {
		t.Fatal(err)
	}
	got := e.repo.lastSaved
	if got.Totals.TotalCents != 12300 || got.Totals.VATCents != 2300 {
		t.Errorf("totals not computed from lines: %+v", got.Totals)
	}
	if got.Number != "FT 2026/1" {
		t.Errorf("number not normalized: %q", got.Number)
	}
	if got.CreatedBy != actor || got.UpdatedBy != actor || got.CondominiumID != condoA {
		t.Errorf("actor or condo not set: %+v", got)
	}
	if got.State != domain.StateRegistered {
		t.Errorf("state = %s", got.State)
	}
	ev := e.audit.last(t)
	if ev.event != model.AuditInvoiceRegistered || ev.user != actor ||
		ev.details["condominium_id"] != condoA || ev.details["invoice_id"] != res.ID {
		t.Errorf("bad audit event: %+v", ev)
	}
}

func TestRegisterIgnoresClientTotals(t *testing.T) {
	e := newEnv(t)
	inv := validInvoice()
	inv.Totals = domain.Totals{TotalCents: 1, PayableCents: 1, WithholdingCents: 0}
	if _, err := e.svc.Register(context.Background(), actor, condoA, inv); err != nil {
		t.Fatal(err)
	}
	if e.repo.lastSaved.Totals.PayableCents != 12300 {
		t.Errorf("payable = %d", e.repo.lastSaved.Totals.PayableCents)
	}
}

func TestSaveDraftRecordsCreated(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.SaveDraft(context.Background(), actor, condoA, domain.Invoice{DocumentType: domain.DocumentFatura, IdempotencyKey: "d"})
	if err != nil {
		t.Fatal(err)
	}
	if e.repo.lastSaved.State != domain.StateDraft {
		t.Errorf("state = %s", e.repo.lastSaved.State)
	}
	if ev := e.audit.last(t); ev.event != model.AuditInvoiceCreated || ev.details["invoice_id"] != res.ID {
		t.Errorf("bad audit event: %+v", ev)
	}
}

// brokenLineDrafts are drafts whose lines or withholding fail ComputeTotals.
func brokenLineDrafts() map[string]domain.Invoice {
	base := func() domain.Invoice {
		return domain.Invoice{DocumentType: domain.DocumentFatura, ExpenseType: domain.ExpenseManutencao}
	}
	exempt := base()
	exempt.Lines = []domain.Line{{BaseCents: 1000, Treatment: domain.TreatmentExempt}}
	rate := base()
	rate.Lines = []domain.Line{{BaseCents: 1000, Treatment: domain.TreatmentRate, RateBP: 1500}}
	neg := base()
	neg.Lines = []domain.Line{{BaseCents: -5, Treatment: domain.TreatmentRate, RateBP: 2300}}
	withh := base()
	withh.Lines = []domain.Line{{BaseCents: 1000, Treatment: domain.TreatmentRate, RateBP: 2300}}
	withh.Totals.WithholdingCents = 999999
	return map[string]domain.Invoice{"exempt": exempt, "rate": rate, "negative": neg, "withholding": withh}
}

func TestDraftWithInvalidLinesIsSavedWithZeroTotals(t *testing.T) {
	for name, inv := range brokenLineDrafts() {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			wantLines, wantWithh := inv.Lines, inv.Totals.WithholdingCents
			if _, err := e.svc.SaveDraft(context.Background(), actor, condoA, inv); err != nil {
				t.Fatalf("draft must save: %v", err)
			}
			got := e.repo.lastSaved
			if got.State != domain.StateDraft {
				t.Errorf("state = %s", got.State)
			}
			if got.Totals.TotalCents != 0 || got.Totals.PayableCents != 0 || got.Totals.VATCents != 0 || got.Totals.BaseCents != 0 {
				t.Errorf("header totals must be 0: %+v", got.Totals)
			}
			if got.Totals.WithholdingCents != wantWithh {
				t.Errorf("withholding = %d, want %d", got.Totals.WithholdingCents, wantWithh)
			}
			if len(got.Lines) != len(wantLines) || got.Lines[0] != wantLines[0] {
				t.Errorf("lines must be stored as entered: %+v", got.Lines)
			}

			// The same content cannot be registered.
			reg := inv
			reg.Number, reg.IssueDate, reg.SupplierID = "FT 1", day(2026, 11, 1), "sup-a"
			reg.Description, reg.Category = "x", domain.CategoryManutencao
			_, err := e.svc.Register(context.Background(), actor, condoA, reg)
			var fe domain.FieldErrors
			if !errors.As(err, &fe) || len(fe) == 0 {
				t.Fatalf("register must be rejected with field errors, got %v", err)
			}
		})
	}
}

// I1 (follow-up): values the schema rejects are field errors even for drafts,
// so they never reach the database as a 500.
func TestDraftRejectsSchemaViolatingLines(t *testing.T) {
	base := func() domain.Invoice {
		return domain.Invoice{DocumentType: domain.DocumentFatura, ExpenseType: domain.ExpenseManutencao}
	}
	unknown := base()
	unknown.Lines = []domain.Line{{BaseCents: 1000, Treatment: domain.VATTreatment("foo")}}
	negRate := base()
	negRate.Lines = []domain.Line{{BaseCents: 1000, Treatment: domain.TreatmentRate, RateBP: -100}}
	cases := map[string]struct {
		inv   domain.Invoice
		field string
	}{
		"unknown treatment": {unknown, "lines[0].treatment"},
		"negative rate":     {negRate, "lines[0].rate"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t)
			_, err := e.svc.SaveDraft(context.Background(), actor, condoA, tc.inv)
			var fe domain.FieldErrors
			if !errors.As(err, &fe) {
				t.Fatalf("draft must be rejected with field errors, got %v", err)
			}
			if fe[tc.field] == "" {
				t.Fatalf("missing error on %s: %v", tc.field, fe)
			}
			if e.repo.lastSaved.ID != "" || len(e.repo.lastSaved.Lines) != 0 {
				t.Fatalf("nothing must be saved: %+v", e.repo.lastSaved)
			}
		})
	}
}

func TestEditDraftWithInvalidLinesStaysDraft(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	res, _ := e.svc.SaveDraft(ctx, actor, condoA, domain.Invoice{DocumentType: domain.DocumentFatura, IdempotencyKey: "k"})
	inv := brokenLineDrafts()["negative"]
	if err := e.svc.Edit(ctx, actor, condoA, res.ID, inv, false); err != nil {
		t.Fatalf("draft edit must save: %v", err)
	}
	if got := e.repo.invoices[res.ID]; got.State != domain.StateDraft || got.Totals.TotalCents != 0 || len(got.Lines) != 1 {
		t.Errorf("stored = %+v", got)
	}
	// Editing a registered invoice keeps the full validation.
	rid := e.registered(t)
	bad := validInvoice()
	bad.Lines = inv.Lines
	var fe domain.FieldErrors
	if err := e.svc.Edit(ctx, actor, condoA, rid, bad, false); !errors.As(err, &fe) {
		t.Fatalf("registered edit must be validated, got %v", err)
	}
}

func TestRegisterValidationErrors(t *testing.T) {
	e := newEnv(t)
	inv := validInvoice()
	inv.Number = ""
	inv.Lines = nil
	_, err := e.svc.Register(context.Background(), actor, condoA, inv)
	var fe domain.FieldErrors
	if !errors.As(err, &fe) || fe["number"] == "" || fe["lines"] == "" {
		t.Fatalf("want field errors for number and lines, got %v", err)
	}
	if e.repo.saveCalls != 0 || len(e.audit.events) != 0 {
		t.Error("nothing must be saved or audited on validation failure")
	}
}

func TestIdempotentCreateReturnsExisting(t *testing.T) {
	e := newEnv(t)
	first, err := e.svc.Register(context.Background(), actor, condoA, validInvoice())
	if err != nil || first.Existing {
		t.Fatalf("first: %+v %v", first, err)
	}
	second, err := e.svc.Register(context.Background(), actor, condoA, validInvoice())
	if err != nil {
		t.Fatal(err)
	}
	if !second.Existing || second.ID != first.ID {
		t.Errorf("second = %+v, want existing %s", second, first.ID)
	}
	if e.repo.saveCalls != 1 || len(e.audit.events) != 1 {
		t.Errorf("saves=%d audits=%d, want 1 and 1", e.repo.saveCalls, len(e.audit.events))
	}
}

func TestIdempotentCreateOnRepositoryKeyViolation(t *testing.T) {
	e := newEnv(t)
	e.repo.saveErr = fmt.Errorf("save invoice: %w", &repositories.DuplicateKeyError{ExistingID: "inv-existing"})
	res, err := e.svc.Register(context.Background(), actor, condoA, validInvoice())
	if err != nil {
		t.Fatal(err)
	}
	if !res.Existing || res.ID != "inv-existing" {
		t.Errorf("res = %+v", res)
	}
	if len(e.audit.events) != 0 {
		t.Error("no audit for an idempotent replay")
	}
}

func TestDuplicateInvoiceCarriesExistingID(t *testing.T) {
	e := newEnv(t)
	first := e.registered(t)
	inv := validInvoice()
	inv.IdempotencyKey = "key-2"
	inv.Number = "FT 2026/1" // already normalized variant of the first
	_, err := e.svc.Register(context.Background(), actor, condoA, inv)
	var dup *DuplicateInvoiceError
	if !errors.As(err, &dup) || dup.ExistingID != first {
		t.Fatalf("want DuplicateInvoiceError{%s}, got %v", first, err)
	}
	if !errors.Is(err, ErrDuplicateInvoice) {
		t.Error("errors.Is(ErrDuplicateInvoice) should be true")
	}
}

func TestDuplicateInvoiceFromSaveIsMapped(t *testing.T) {
	e := newEnv(t)
	e.repo.saveErr = fmt.Errorf("save invoice: %w", &repositories.DuplicateInvoiceError{ExistingID: "inv-x"})
	_, err := e.svc.Register(context.Background(), actor, condoA, validInvoice())
	var dup *DuplicateInvoiceError
	if !errors.As(err, &dup) || dup.ExistingID != "inv-x" {
		t.Fatalf("got %v", err)
	}
}

func TestForeignSupplierRejected(t *testing.T) {
	e := newEnv(t)
	inv := validInvoice()
	inv.SupplierID = "sup-b" // belongs to condo B
	_, err := e.svc.Register(context.Background(), actor, condoA, inv)
	var fe domain.FieldErrors
	if !errors.As(err, &fe) || fe["supplier_id"] != KeyUnknownSupplier {
		t.Fatalf("want supplier field error, got %v", err)
	}
	if e.repo.saveCalls != 0 {
		t.Error("must not save")
	}
	// A draft with a foreign supplier is rejected as well.
	d := domain.Invoice{DocumentType: domain.DocumentFatura, SupplierID: "sup-b"}
	if _, err := e.svc.SaveDraft(context.Background(), actor, condoA, d); !errors.As(err, &fe) {
		t.Fatalf("draft: got %v", err)
	}
}

func TestInvalidReferenceFromRepositoryIsFieldError(t *testing.T) {
	e := newEnv(t)
	e.repo.saveErr = fmt.Errorf("save invoice: %w", repositories.ErrInvalidReference)
	_, err := e.svc.Register(context.Background(), actor, condoA, validInvoice())
	var fe domain.FieldErrors
	if !errors.As(err, &fe) || fe["supplier_id"] == "" {
		t.Fatalf("got %v", err)
	}
}

func TestPaymentRules(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.registered(t) // payable 12300

	p := domain.Payment{AmountCents: 5000, Method: domain.MethodMultibanco}
	if _, err := e.svc.AddPayment(ctx, actor, condoA, id, p); err != nil {
		t.Fatal(err)
	}
	ev := e.audit.last(t)
	if ev.event != model.AuditInvoicePaymentAdded || ev.user != actor ||
		ev.details["invoice_id"] != id || ev.details["condominium_id"] != condoA {
		t.Errorf("bad audit event: %+v", ev)
	}
	if e.repo.invoices[id].Payments[0].Date.IsZero() {
		t.Error("date should default to now")
	}

	n := len(e.audit.events)
	over := domain.Payment{AmountCents: 9000, Method: domain.MethodMultibanco}
	if _, err := e.svc.AddPayment(ctx, actor, condoA, id, over); !errors.Is(err, domain.ErrOverpayment) {
		t.Fatalf("want ErrOverpayment, got %v", err)
	}
	over.Justification = "advance"
	if _, err := e.svc.AddPayment(ctx, actor, condoA, id, over); err != nil {
		t.Fatalf("justified overpayment: %v", err)
	}
	if len(e.audit.events) != n+1 {
		t.Error("only the accepted payment is audited")
	}

	if _, err := e.svc.AddPayment(ctx, actor, condoA, id, domain.Payment{AmountCents: 1}); err == nil {
		t.Error("missing method must be rejected")
	}
}

func TestPaymentOnDraftRejected(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.SaveDraft(context.Background(), actor, condoA, domain.Invoice{DocumentType: domain.DocumentFatura})
	if err != nil {
		t.Fatal(err)
	}
	e.audit.events = nil
	_, err = e.svc.AddPayment(context.Background(), actor, condoA, res.ID, domain.Payment{AmountCents: 100, Method: domain.MethodOutro})
	if !errors.Is(err, domain.ErrDraftNoPayments) {
		t.Fatalf("got %v", err)
	}
	if len(e.audit.events) != 0 {
		t.Error("no audit on rejection")
	}
}

func TestPaymentOnAnnulledRejected(t *testing.T) {
	e := newEnv(t)
	id := e.registered(t)
	if err := e.svc.Annul(context.Background(), actor, condoA, id, "wrong supplier"); err != nil {
		t.Fatal(err)
	}
	_, err := e.svc.AddPayment(context.Background(), actor, condoA, id, domain.Payment{AmountCents: 100, Method: domain.MethodOutro})
	if !errors.Is(err, domain.ErrAnnulled) {
		t.Fatalf("got %v", err)
	}
}

func TestPaymentOtherCondominiumNotFound(t *testing.T) {
	e := newEnv(t)
	id := e.registered(t)
	_, err := e.svc.AddPayment(context.Background(), actor, condoB, id, domain.Payment{AmountCents: 100, Method: domain.MethodOutro})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestEditBelowPaidRejected(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.registered(t)
	if _, err := e.svc.AddPayment(ctx, actor, condoA, id, domain.Payment{AmountCents: 10000, Method: domain.MethodOutro}); err != nil {
		t.Fatal(err)
	}
	e.audit.events = nil

	inv := validInvoice()
	inv.Lines[0].BaseCents = 5000 // payable 6150 < 10000 paid
	if err := e.svc.Edit(ctx, actor, condoA, id, inv, true); !errors.Is(err, domain.ErrPayableBelowPaid) {
		t.Fatalf("got %v", err)
	}
	if len(e.audit.events) != 0 {
		t.Error("no audit on rejection")
	}

	inv.Lines[0].BaseCents = 20000
	if err := e.svc.Edit(ctx, actor, condoA, id, inv, true); err != nil {
		t.Fatal(err)
	}
	ev := e.audit.last(t)
	if ev.event != model.AuditInvoiceUpdated || ev.details["invoice_id"] != id {
		t.Errorf("bad audit event: %+v", ev)
	}
	if e.repo.lastUpdated.UpdatedBy != actor || e.repo.lastUpdated.IdempotencyKey != "key-1" {
		t.Errorf("update metadata: %+v", e.repo.lastUpdated)
	}
}

func TestEditAnnulledRejected(t *testing.T) {
	e := newEnv(t)
	id := e.registered(t)
	_ = e.svc.Annul(context.Background(), actor, condoA, id, "reason")
	if err := e.svc.Edit(context.Background(), actor, condoA, id, validInvoice(), true); !errors.Is(err, domain.ErrAnnulled) {
		t.Fatalf("got %v", err)
	}
}

func TestEditDraftIntoRegisteredRecordsRegistered(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	res, _ := e.svc.SaveDraft(ctx, actor, condoA, domain.Invoice{DocumentType: domain.DocumentFatura, IdempotencyKey: "k"})
	e.audit.events = nil

	// Incomplete: full validation applies when completing.
	if err := e.svc.Edit(ctx, actor, condoA, res.ID, domain.Invoice{DocumentType: domain.DocumentFatura}, true); err == nil {
		t.Fatal("incomplete draft must not register")
	}
	if err := e.svc.Edit(ctx, actor, condoA, res.ID, validInvoice(), true); err != nil {
		t.Fatal(err)
	}
	if ev := e.audit.last(t); ev.event != model.AuditInvoiceRegistered {
		t.Errorf("event = %s", ev.event)
	}
	if e.repo.invoices[res.ID].State != domain.StateRegistered {
		t.Error("state should be registered")
	}
}

func TestDeleteDraftAndRegisteredCannotBeDeleted(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	d, _ := e.svc.SaveDraft(ctx, actor, condoA, domain.Invoice{DocumentType: domain.DocumentFatura})
	e.audit.events = nil
	if err := e.svc.DeleteDraft(ctx, actor, condoA, d.ID); err != nil {
		t.Fatal(err)
	}
	if ev := e.audit.last(t); ev.event != model.AuditInvoiceDraftDeleted || ev.details["invoice_id"] != d.ID {
		t.Errorf("bad audit event: %+v", ev)
	}

	id := e.registered(t)
	if err := e.svc.DeleteDraft(ctx, actor, condoA, id); !errors.Is(err, ErrNotDraft) {
		t.Fatalf("got %v", err)
	}
	if len(e.audit.events) != 0 {
		t.Error("no audit on rejection")
	}
}

func TestAnnulRequiresReasonAndAudits(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.registered(t)
	if err := e.svc.Annul(ctx, actor, condoA, id, "  "); !errors.Is(err, domain.ErrAnnulReasonRequired) {
		t.Fatalf("got %v", err)
	}
	if len(e.audit.events) != 0 {
		t.Fatal("no audit on rejection")
	}
	if err := e.svc.Annul(ctx, actor, condoA, id, "duplicated"); err != nil {
		t.Fatal(err)
	}
	if ev := e.audit.last(t); ev.event != model.AuditInvoiceAnnulled || ev.user != actor {
		t.Errorf("bad audit event: %+v", ev)
	}
}

func TestPreview(t *testing.T) {
	e := newEnv(t)
	tot, err := e.svc.Preview("", validInvoice().Lines, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if tot.TotalCents != 12300 || tot.PayableCents != 11300 {
		t.Errorf("totals = %+v", tot)
	}
	bad := []domain.Line{{BaseCents: 100, Treatment: domain.TreatmentRate, RateBP: 1234}}
	if _, err := e.svc.Preview(domain.RegionContinental, bad, 0); err == nil {
		t.Error("invalid rate must return field errors")
	}
	if e.repo.saveCalls != 0 || len(e.audit.events) != 0 {
		t.Error("preview must not persist or audit")
	}
}

// ---- attachments ----

func TestAddAttachmentsStoresFilesAndAudits(t *testing.T) {
	e := newEnv(t)
	id := e.registered(t)
	saved, err := e.svc.AddAttachments(context.Background(), actor, condoA, id,
		[]*multipart.FileHeader{pdfHeader(t, "a.pdf", pdfBytes)})
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].ContentType != "application/pdf" || saved[0].OriginalName != "a.pdf" {
		t.Fatalf("saved = %+v", saved)
	}
	if filesIn(t, e.dir) != 1 {
		t.Error("file should be stored under UploadDir/invoices")
	}
	ev := e.audit.last(t)
	if ev.event != model.AuditInvoiceAttachmentAdded || ev.details["invoice_id"] != id ||
		ev.details["condominium_id"] != condoA || ev.user != actor {
		t.Errorf("bad audit event: %+v", ev)
	}
}

func TestAttachmentLimit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.registered(t)
	var files []*multipart.FileHeader
	for i := 0; i < domain.MaxAttachments; i++ {
		files = append(files, pdfHeader(t, "f.pdf", pdfBytes))
	}
	if _, err := e.svc.AddAttachments(ctx, actor, condoA, id, files); err != nil {
		t.Fatal(err)
	}
	e.audit.events = nil
	_, err := e.svc.AddAttachments(ctx, actor, condoA, id, []*multipart.FileHeader{pdfHeader(t, "x.pdf", pdfBytes)})
	if !errors.Is(err, domain.ErrTooManyAttachments) {
		t.Fatalf("got %v", err)
	}
	if filesIn(t, e.dir) != domain.MaxAttachments {
		t.Errorf("rejected upload left %d files", filesIn(t, e.dir))
	}
	if len(e.audit.events) != 0 {
		t.Error("no audit on rejection")
	}
}

func TestAttachmentRepositoryFailureRemovesFiles(t *testing.T) {
	e := newEnv(t)
	id := e.registered(t)
	e.repo.attachErr = errors.New("db down")
	_, err := e.svc.AddAttachments(context.Background(), actor, condoA, id, []*multipart.FileHeader{
		pdfHeader(t, "a.pdf", pdfBytes), pdfHeader(t, "b.pdf", pdfBytes),
	})
	if err == nil {
		t.Fatal("want error")
	}
	if n := filesIn(t, e.dir); n != 0 {
		t.Errorf("%d orphan files left", n)
	}
	if len(e.audit.events) != 0 {
		t.Error("no audit on failure")
	}
}

func TestAttachmentInvalidTypeRemovesEarlierFiles(t *testing.T) {
	e := newEnv(t)
	id := e.registered(t)
	_, err := e.svc.AddAttachments(context.Background(), actor, condoA, id, []*multipart.FileHeader{
		pdfHeader(t, "a.pdf", pdfBytes), pdfHeader(t, "evil.exe", []byte("MZ\x90\x00 not allowed")),
	})
	if !errors.Is(err, ErrInvalidFile) {
		t.Fatalf("got %v", err)
	}
	if n := filesIn(t, e.dir); n != 0 {
		t.Errorf("%d files left", n)
	}
}

func TestAttachmentOnAnnulledOrForeignInvoiceWritesNothing(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.registered(t)
	files := []*multipart.FileHeader{pdfHeader(t, "a.pdf", pdfBytes)}
	if _, err := e.svc.AddAttachments(ctx, actor, condoB, id, files); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign: %v", err)
	}
	_ = e.svc.Annul(ctx, actor, condoA, id, "r")
	if _, err := e.svc.AddAttachments(ctx, actor, condoA, id, files); !errors.Is(err, domain.ErrAnnulled) {
		t.Fatalf("annulled: %v", err)
	}
	if filesIn(t, e.dir) != 0 {
		t.Error("no file should be written")
	}
	if _, err := e.svc.AddAttachments(ctx, actor, condoA, id, nil); !errors.Is(err, ErrNoFiles) {
		t.Errorf("no files: %v", err)
	}
}

func TestRemoveAttachmentDeletesFileAfterCommit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.registered(t)
	saved, err := e.svc.AddAttachments(ctx, actor, condoA, id, []*multipart.FileHeader{pdfHeader(t, "a.pdf", pdfBytes)})
	if err != nil {
		t.Fatal(err)
	}
	e.audit.events = nil
	if err := e.svc.RemoveAttachment(ctx, actor, condoA, id, saved[0].ID); err != nil {
		t.Fatal(err)
	}
	if filesIn(t, e.dir) != 0 {
		t.Error("file should be removed")
	}
	if ev := e.audit.last(t); ev.event != model.AuditInvoiceAttachmentRemoved || ev.details["attachment_id"] != saved[0].ID {
		t.Errorf("bad audit event: %+v", ev)
	}
}

func TestRemoveAttachmentKeepsFileWhenRepositoryFails(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.registered(t)
	if _, err := e.svc.AddAttachments(ctx, actor, condoA, id, []*multipart.FileHeader{pdfHeader(t, "a.pdf", pdfBytes)}); err != nil {
		t.Fatal(err)
	}
	e.audit.events = nil
	if err := e.svc.RemoveAttachment(ctx, actor, condoA, id, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if filesIn(t, e.dir) != 1 || len(e.audit.events) != 0 {
		t.Error("file must stay and nothing is audited")
	}
}

func TestOpenAttachment(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	id := e.registered(t)
	saved, _ := e.svc.AddAttachments(ctx, actor, condoA, id, []*multipart.FileHeader{pdfHeader(t, "a.pdf", pdfBytes)})

	f, att, err := e.svc.OpenAttachment(ctx, condoA, id, saved[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	body, _ := io.ReadAll(f)
	if !bytes.Equal(body, pdfBytes) || att.OriginalName != "a.pdf" || att.ContentType != "application/pdf" {
		t.Errorf("att=%+v body=%q", att, body)
	}

	if _, _, err := e.svc.OpenAttachment(ctx, condoB, id, saved[0].ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("foreign condo: %v", err)
	}

	// A stored path that escapes the directory is refused.
	e.repo.invoices[id].Attachments[0].StoredPath = "../../etc/passwd"
	if _, _, err := e.svc.OpenAttachment(ctx, condoA, id, saved[0].ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("traversal: %v", err)
	}
}

// ---- suppliers ----

func TestSupplierCreateNormalizesNIFAndAudits(t *testing.T) {
	repo, audit := newFakeRepo(), &fakeAudit{}
	svc := &SupplierService{Suppliers: repo, Audit: audit}
	out, err := svc.Create(context.Background(), actor, condoA, domain.Supplier{Name: " ACME ", NIF: "PT 501 964 843"})
	if err != nil {
		t.Fatal(err)
	}
	if out.NIF != "501964843" || out.Country != "PT" || out.Name != "ACME" {
		t.Errorf("out = %+v", out)
	}
	ev := audit.last(t)
	if ev.event != model.AuditSupplierCreated || ev.details["supplier_id"] != out.ID || ev.details["condominium_id"] != condoA {
		t.Errorf("bad audit event: %+v", ev)
	}
}

func TestSupplierValidationAndDuplicateNIF(t *testing.T) {
	repo, audit := newFakeRepo(), &fakeAudit{}
	svc := &SupplierService{Suppliers: repo, Audit: audit}
	ctx := context.Background()

	_, err := svc.Create(ctx, actor, condoA, domain.Supplier{Name: "X", NIF: "123456788"})
	var fe domain.FieldErrors
	if !errors.As(err, &fe) || fe["nif"] != domain.KeyInvalidNIF {
		t.Fatalf("invalid nif: %v", err)
	}
	if _, err := svc.Create(ctx, actor, condoA, domain.Supplier{Name: "A", NIF: "501964843"}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.Create(ctx, actor, condoA, domain.Supplier{Name: "B", NIF: "501 964 843"})
	if !errors.As(err, &fe) || fe["nif"] != KeyDuplicateNIF {
		t.Fatalf("duplicate nif: %v", err)
	}
	if len(audit.events) != 1 {
		t.Errorf("audit events = %d, want 1", len(audit.events))
	}
}

func TestSupplierUpdateScopedAndAudited(t *testing.T) {
	repo, audit := newFakeRepo(), &fakeAudit{}
	repo.addSupplier(condoA, "s1", "Old")
	svc := &SupplierService{Suppliers: repo, Audit: audit}
	ctx := context.Background()

	if err := svc.Update(ctx, actor, condoB, "s1", domain.Supplier{Name: "New"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign update: %v", err)
	}
	if err := svc.Update(ctx, actor, condoA, "s1", domain.Supplier{Name: "New", Country: "ES", NIF: "B123"}); err != nil {
		t.Fatal(err)
	}
	if repo.suppliers["s1"].Name != "New" {
		t.Error("not updated")
	}
	if ev := audit.last(t); ev.event != model.AuditSupplierUpdated || ev.details["supplier_id"] != "s1" {
		t.Errorf("bad audit event: %+v", ev)
	}
	if got, _ := svc.Search(ctx, condoA, "", 0); len(got) != 1 {
		t.Errorf("search = %d", len(got))
	}
	if _, err := svc.Get(ctx, condoB, "s1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("foreign get: %v", err)
	}
}
