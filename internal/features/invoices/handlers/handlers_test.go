package handlers

import (
	"context"
	"errors"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/services"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
)

// --- fakes ---

type fakeRoles struct{ roles []model.Role }

func (f *fakeRoles) ActiveRolesForUser(context.Context, string, string) ([]model.Role, error) {
	return f.roles, nil
}

type fakeSessionReader struct{ sess model.Session }

func (f fakeSessionReader) Read(*http.Request) (model.Session, error) { return f.sess, nil }

type fakeUserLoader struct{ u model.User }

func (f fakeUserLoader) GetUserByID(context.Context, string) (model.User, error) { return f.u, nil }

// fakeInvoices keeps invoices of one condominium, "c1"; any other condominium
// id gets ErrNotFound, like the real service.
type fakeInvoices struct {
	invoices map[string]domain.Invoice
	saveErr  error
	// deleteErr is returned by DeleteDraft.
	deleteErr error
	saveRes   services.SaveResult
	saved     []domain.Invoice
	register  bool
	attPath   string
	att       domain.Attachment
}

func (f *fakeInvoices) SaveDraft(_ context.Context, _, _ string, inv domain.Invoice) (services.SaveResult, error) {
	f.saved, f.register = append(f.saved, inv), false
	return f.saveRes, f.saveErr
}

func (f *fakeInvoices) Register(_ context.Context, _, _ string, inv domain.Invoice) (services.SaveResult, error) {
	f.saved, f.register = append(f.saved, inv), true
	return f.saveRes, f.saveErr
}

func (f *fakeInvoices) Edit(context.Context, string, string, string, domain.Invoice, bool) error {
	return f.saveErr
}
func (f *fakeInvoices) DeleteDraft(context.Context, string, string, string) error { return f.deleteErr }
func (f *fakeInvoices) Annul(context.Context, string, string, string, string) error {
	return nil
}
func (f *fakeInvoices) AddPayment(context.Context, string, string, string, domain.Payment) (domain.Payment, error) {
	return domain.Payment{}, nil
}
func (f *fakeInvoices) AddAttachments(context.Context, string, string, string, []*multipart.FileHeader) ([]domain.Attachment, error) {
	return nil, nil
}
func (f *fakeInvoices) RemoveAttachment(context.Context, string, string, string, string) error {
	return nil
}

func (f *fakeInvoices) OpenAttachment(_ context.Context, condo, _, _ string) (*os.File, domain.Attachment, error) {
	if condo != "c1" || f.attPath == "" {
		return nil, domain.Attachment{}, services.ErrNotFound
	}
	file, err := os.Open(f.attPath)
	return file, f.att, err
}

func (f *fakeInvoices) Get(_ context.Context, condo, id string) (domain.Invoice, error) {
	inv, ok := f.invoices[id]
	if !ok || condo != "c1" {
		return domain.Invoice{}, services.ErrNotFound
	}
	return inv, nil
}

func (f *fakeInvoices) List(context.Context, string, domain.ListFilter) (domain.ListResult, error) {
	return domain.ListResult{Page: 1, PageSize: 25}, nil
}

func (f *fakeInvoices) Preview(region domain.FiscalRegion, lines []domain.Line, w int64) (domain.Totals, error) {
	return domain.ComputeTotals(region, lines, w)
}

type fakeSuppliers struct {
	list      map[string]domain.Supplier
	searchErr error
}

func (f *fakeSuppliers) Create(_ context.Context, _, _ string, in domain.Supplier) (domain.Supplier, error) {
	if strings.TrimSpace(in.Name) == "" {
		return domain.Supplier{}, domain.FieldErrors{"name": domain.KeyRequired}
	}
	in.ID = "s-new"
	return in, nil
}
func (f *fakeSuppliers) Update(context.Context, string, string, string, domain.Supplier) error {
	return nil
}
func (f *fakeSuppliers) Get(_ context.Context, condo, id string) (domain.Supplier, error) {
	s, ok := f.list[id]
	if !ok || condo != "c1" {
		return domain.Supplier{}, services.ErrNotFound
	}
	return s, nil
}
func (f *fakeSuppliers) Search(context.Context, string, string, int) ([]domain.Supplier, error) {
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	var out []domain.Supplier
	for _, s := range f.list {
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeSuppliers) SearchPage(ctx context.Context, condo, q string, requested int) ([]domain.Supplier, pagination.Page, error) {
	list, err := f.Search(ctx, condo, q, 0)
	if err != nil {
		return nil, pagination.Page{}, err
	}
	items, p := pagination.Slice(list, requested)
	return items, p, nil
}

type auditSpy struct{ events []model.AuditEventType }

func (a *auditSpy) RecordEvent(_ context.Context, _ *string, t model.AuditEventType, _ map[string]any) error {
	a.events = append(a.events, t)
	return nil
}

// --- harness ---

type harness struct {
	inv  *fakeInvoices
	sup  *fakeSuppliers
	aud  *auditSpy
	lang string // the signed-in user's language preference; "" is English
}

func newHarness() *harness {
	return &harness{
		inv: &fakeInvoices{invoices: map[string]domain.Invoice{
			"i1": {ID: "i1", CondominiumID: "c1", State: domain.StateRegistered, Number: "FT 1", Supplier: &domain.Supplier{Name: "ACME"}},
		}},
		sup: &fakeSuppliers{list: map[string]domain.Supplier{
			"s1": {ID: "s1", CondominiumID: "c1", Name: "ACME Lda", NIF: "501964843", Country: "PT"},
		}},
		aud: &auditSpy{},
	}
}

// router builds the handlers behind the session middleware for the roles; a
// nil roles slice means no session at all.
func (h *harness) router(roles []model.Role) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Roles: &fakeRoles{roles: roles}, Audit: h.aud, Invoices: h.inv, Suppliers: h.sup})
	var root http.Handler = mux
	if roles != nil {
		user := model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive, LanguagePreference: h.lang}
		sess := model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "tok"}
		root = middleware.WithLocale(root)
		root = middleware.WithUser(fakeSessionReader{sess: sess}, fakeUserLoader{u: user})(root)
	}
	return root
}

func (h *harness) syndic() http.Handler { return h.router([]model.Role{model.RoleSyndic}) }

func do(router http.Handler, method, target string, form url.Values, hx bool) *httptest.ResponseRecorder {
	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// validForm is a complete product invoice with one line.
func validForm() url.Values {
	return url.Values{
		"action": {"register"}, "idempotency_key": {"key-1"},
		"document_type": {"fatura"}, "number": {"FT 2026/1"}, "issue_date": {"2026-03-01"},
		"supplier_id": {"s1"}, "expense_type": {"compra_produtos"}, "category": {"limpeza"},
		"description": {"Detergente"}, "fiscal_region": {"continental"},
		"lines[0].product": {"Detergente"}, "lines[0].qty": {"2.5"}, "lines[0].unit_price": {"1,234.56"},
		"lines[0].treatment": {"rate"}, "lines[0].rate": {"2300"},
	}
}

// --- access control ---

func TestNonSyndicRolesGet403OnEveryRouteKind(t *testing.T) {
	h := newHarness()
	for _, role := range []model.Role{model.RoleOwner, model.RoleTenant} {
		router := h.router([]model.Role{role})
		for _, c := range []struct{ method, path string }{
			{"GET", "/invoices"}, {"GET", "/invoices/new"}, {"POST", "/invoices"},
			{"GET", "/invoices/i1"}, {"GET", "/invoices/i1/edit"}, {"POST", "/invoices/i1/annul"},
			{"GET", "/invoices/i1/attachments/a1"}, {"GET", "/suppliers"}, {"POST", "/suppliers"},
		} {
			var form url.Values
			if c.method == "POST" {
				form = url.Values{}
			}
			if rec := do(router, c.method, c.path, form, false); rec.Code != http.StatusForbidden {
				t.Errorf("%s %s %s = %d, want 403", role, c.method, c.path, rec.Code)
			}
		}
		for _, c := range []struct{ method, path string }{
			{"GET", "/invoices/form/section"}, {"POST", "/invoices/form/lines"},
			{"POST", "/invoices/form/preview"}, {"GET", "/suppliers/search?q=a"}, {"POST", "/suppliers/inline"},
		} {
			var form url.Values
			if c.method == "POST" {
				form = url.Values{}
			}
			rec := do(router, c.method, c.path, form, true)
			if rec.Code != http.StatusForbidden || rec.Body.Len() != 0 {
				t.Errorf("%s HX %s %s = %d (%d bytes), want empty 403", role, c.method, c.path, rec.Code, rec.Body.Len())
			}
		}
	}
	if len(h.inv.saved) != 0 {
		t.Fatal("a denied request must not reach the service")
	}
}

func TestExpiredSessionOnFragmentsRedirectsHX(t *testing.T) {
	router := newHarness().router(nil)
	for _, c := range []struct{ method, path string }{
		{"GET", "/invoices/form/section"}, {"POST", "/invoices/form/lines"}, {"POST", "/invoices/form/preview"},
		{"GET", "/suppliers/search?q=a"}, {"POST", "/suppliers/inline"},
	} {
		var form url.Values
		if c.method == "POST" {
			form = url.Values{}
		}
		rec := do(router, c.method, c.path, form, true)
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("HX-Redirect") != "/login" {
			t.Errorf("HX %s %s = %d HX-Redirect %q, want 401 /login", c.method, c.path, rec.Code, rec.Header().Get("HX-Redirect"))
		}
		rec = do(router, c.method, c.path, form, false)
		if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
			t.Errorf("plain %s %s = %d %q, want 303 /login", c.method, c.path, rec.Code, rec.Header().Get("Location"))
		}
	}
}

// --- scoping ---

func TestOtherCondominiumRecordIs404(t *testing.T) {
	h := newHarness()
	router := h.syndic()
	// "i-other" belongs to another condominium: the service answers ErrNotFound.
	for _, c := range []struct{ method, path string }{
		{"GET", "/invoices/i-other"}, {"GET", "/invoices/i-other/edit"},
		{"POST", "/invoices/i-other/edit"}, {"POST", "/invoices/i-other/annul"},
		{"POST", "/invoices/i-other/payments"}, {"POST", "/invoices/i-other/attachments"},
		{"GET", "/invoices/i1/attachments/a-foreign"}, {"GET", "/suppliers/s-other/edit"},
	} {
		var form url.Values
		if c.method == "POST" {
			form = url.Values{}
		}
		if rec := do(router, c.method, c.path, form, false); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s = %d, want 404", c.method, c.path, rec.Code)
		}
	}
}

func TestSyndicSeesOwnInvoice(t *testing.T) {
	rec := do(newHarness().syndic(), "GET", "/invoices/i1", nil, false)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "FT 1") {
		t.Fatalf("view = %d, want 200 with the number", rec.Code)
	}
}

// --- create ---

func TestCreateParsesIndexedLinesAndRedirects(t *testing.T) {
	h := newHarness()
	h.inv.saveRes = services.SaveResult{ID: "i9"}
	h.lang = "pt-br" // "2,5" and "1.234,56" are Brazilian-format inputs
	form := validForm()
	form.Set("lines[0].qty", "2,5")
	form.Set("lines[0].unit_price", "1.234,56")
	rec := do(h.syndic(), "POST", "/invoices", form, false)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/invoices/i9?flash=registered" {
		t.Fatalf("create = %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if !h.inv.register || len(h.inv.saved) != 1 {
		t.Fatalf("Register calls = %d (register=%v)", len(h.inv.saved), h.inv.register)
	}
	inv := h.inv.saved[0]
	if inv.IdempotencyKey != "key-1" || inv.SupplierID != "s1" || inv.Number != "FT 2026/1" {
		t.Fatalf("invoice header = %+v", inv)
	}
	if len(inv.Lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(inv.Lines))
	}
	ln := inv.Lines[0]
	if ln.QuantityMilli != 2500 || ln.UnitPriceCents != 123456 || ln.RateBP != 2300 || ln.Treatment != domain.TreatmentRate {
		t.Fatalf("line = %+v, want qty 2500, price 123456, 23%%", ln)
	}
}

func TestCreateDraftAction(t *testing.T) {
	h := newHarness()
	h.inv.saveRes = services.SaveResult{ID: "i9"}
	form := validForm()
	form.Set("action", "draft")
	rec := do(h.syndic(), "POST", "/invoices", form, false)
	if h.inv.register || len(h.inv.saved) != 1 {
		t.Fatalf("action=draft must call SaveDraft once (saved %d)", len(h.inv.saved))
	}
	if rec.Header().Get("Location") != "/invoices/i9?flash=saved" {
		t.Fatalf("Location = %q", rec.Header().Get("Location"))
	}
}

func TestCreateIdempotentReplayRedirectsToExisting(t *testing.T) {
	h := newHarness()
	h.inv.saveRes = services.SaveResult{ID: "i-first", Existing: true}
	rec := do(h.syndic(), "POST", "/invoices", validForm(), false)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/invoices/i-first" {
		t.Fatalf("replay = %d %q, want 303 /invoices/i-first", rec.Code, rec.Header().Get("Location"))
	}
}

func TestCreateDuplicateShowsMessageAndLink(t *testing.T) {
	h := newHarness()
	h.inv.saveErr = &services.DuplicateInvoiceError{ExistingID: "i-dup"}
	rec := do(h.syndic(), "POST", "/invoices", validForm(), false)
	body := rec.Body.String()
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate = %d, want 422", rec.Code)
	}
	if !strings.Contains(body, i18n.T(i18n.LanguageEN, "invoices.flash.duplicate")) {
		t.Fatal("body lacks the duplicate message")
	}
	if !strings.Contains(body, `href="/invoices/i-dup"`) {
		t.Fatal("body lacks the link to the existing invoice")
	}
	if !strings.Contains(body, `value="FT 2026/1"`) {
		t.Fatal("the user's input must be preserved")
	}
}

func TestCreateValidationErrorsAreInlineWith422(t *testing.T) {
	h := newHarness()
	h.inv.saveErr = domain.FieldErrors{"number": domain.KeyRequired, "lines[0].qty": domain.KeyInvalidQuantity}
	rec := do(h.syndic(), "POST", "/invoices", validForm(), false)
	body := rec.Body.String()
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	msg := i18n.T(i18n.LanguageEN, "invoices.error.required")
	if !strings.Contains(body, msg) {
		t.Fatal("missing the inline number error")
	}
	// The domain key is the name of the form input, "qty".
	if !strings.Contains(body, i18n.T(i18n.LanguageEN, "invoices.error.invalid_quantity")) {
		t.Fatal("missing the inline quantity error")
	}
	if !strings.Contains(body, `value="Detergente"`) {
		t.Fatal("input not preserved")
	}
}

func TestCreateUnparsableMoneyIsRejectedBeforeTheService(t *testing.T) {
	h := newHarness()
	form := validForm()
	form.Set("lines[0].unit_price", "abc")
	rec := do(h.syndic(), "POST", "/invoices", form, false)
	if rec.Code != http.StatusUnprocessableEntity || len(h.inv.saved) != 0 {
		t.Fatalf("status = %d, service calls = %d; want 422 and none", rec.Code, len(h.inv.saved))
	}
	if !strings.Contains(rec.Body.String(), i18n.T(i18n.LanguageEN, "invoices.error.invalid_amount")) {
		t.Fatal("missing the inline amount error")
	}
}

func TestCreateServiceFailureIs500(t *testing.T) {
	h := newHarness()
	h.inv.saveErr = errors.New("db down")
	if rec := do(h.syndic(), "POST", "/invoices", validForm(), false); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// --- fragments ---

func TestFragmentsReturnNoFullPage(t *testing.T) {
	router := newHarness().syndic()
	cases := []struct {
		method, path string
		form         url.Values
	}{
		{"GET", "/invoices/form/section?expense_type=prestacao_servicos", nil},
		{"POST", "/invoices/form/lines", url.Values{"op": {"add"}}},
		{"POST", "/invoices/form/preview", url.Values{}},
		{"GET", "/suppliers/search?q=acme", nil},
		{"GET", "/suppliers/s1/selected", nil},
	}
	for _, c := range cases {
		rec := do(router, c.method, c.path, c.form, true)
		if rec.Code != http.StatusOK {
			t.Errorf("%s %s = %d, want 200", c.method, c.path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "<html") {
			t.Errorf("%s %s returned a full page", c.method, c.path)
		}
	}
}

func TestSectionSwapsByExpenseType(t *testing.T) {
	router := newHarness().syndic()
	body := do(router, "GET", "/invoices/form/section?expense_type=prestacao_servicos", nil, true).Body.String()
	if !strings.Contains(body, `data-section="prestacao_servicos"`) {
		t.Fatal("services section missing")
	}
	body = do(router, "GET", "/invoices/form/section?expense_type=despesa_mensal", nil, true).Body.String()
	if !strings.Contains(body, `data-section="despesa_mensal"`) || strings.Contains(body, "prestacao_servicos") {
		t.Fatal("monthly section expected alone")
	}
	if body := do(router, "GET", "/invoices/form/section?expense_type=manutencao", nil, true).Body.String(); strings.TrimSpace(body) != "" {
		t.Fatalf("a type without conditional fields renders nothing, got %q", body)
	}
}

func TestLinesAddAndRemove(t *testing.T) {
	router := newHarness().syndic()
	form := url.Values{
		"expense_type": {"compra_produtos"}, "fiscal_region": {"continental"},
		"lines[0].product": {"A"}, "lines[0].qty": {"1"}, "lines[0].unit_price": {"10"},
		"lines[1].product": {"B"}, "lines[1].qty": {"2"}, "lines[1].unit_price": {"5"},
		"op": {"add"},
	}
	body := do(router, "POST", "/invoices/form/lines", form, true).Body.String()
	for _, i := range []string{"0", "1", "2"} {
		if !strings.Contains(body, `data-line="`+i+`"`) {
			t.Errorf("after add, line %s missing", i)
		}
	}
	form.Set("op", "remove")
	form.Set("index", "0")
	body = do(router, "POST", "/invoices/form/lines", form, true).Body.String()
	if strings.Contains(body, `data-line="1"`) || !strings.Contains(body, `data-line="0"`) {
		t.Fatal("after remove, two lines expected renumbered 0..1")
	}
	if strings.Contains(body, `value="A"`) || !strings.Contains(body, `value="B"`) {
		t.Fatal("line 0 should be removed and B kept")
	}
}

func TestPreviewComputesOnServerAndToleratesBadInput(t *testing.T) {
	router := newHarness().syndic()

	form := url.Values{
		"expense_type": {"manutencao"}, "fiscal_region": {"continental"},
		"lines[0].description": {"Reparo"}, "lines[0].base": {"100"},
		"lines[0].treatment": {"rate"}, "lines[0].rate": {"2300"},
	}
	rec := do(router, "POST", "/invoices/form/preview", form, true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "123.00") {
		t.Fatalf("preview = %d, want 200 with the 123.00 total; body %s", rec.Code, rec.Body.String())
	}

	for name, bad := range map[string]url.Values{
		"empty":   {},
		"garbage": {"expense_type": {"manutencao"}, "lines[0].base": {"x"}},
		"region":  {"fiscal_region": {"mars"}, "lines[0].base": {"1"}},
	} {
		rec := do(router, "POST", "/invoices/form/preview", bad, true)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: preview = %d, want 200", name, rec.Code)
		}
	}
	rec = do(router, "POST", "/invoices/form/preview", url.Values{"expense_type": {"manutencao"}, "lines[0].base": {"x"}}, true)
	if !strings.Contains(rec.Body.String(), i18n.T(i18n.LanguageEN, "invoices.error.invalid_amount")) {
		t.Fatal("an unparsable amount should be reported inside the summary")
	}
}

func TestSupplierSelectAndInline(t *testing.T) {
	router := newHarness().syndic()
	body := do(router, "GET", "/suppliers/s1/selected", nil, true).Body.String()
	if !strings.Contains(body, `name="supplier_id" value="s1"`) || !strings.Contains(body, "ACME Lda") {
		t.Fatalf("selected fragment = %s", body)
	}
	rec404 := do(router, "GET", "/suppliers/s-other/selected", nil, true)
	if rec404.Code != http.StatusNotFound {
		t.Fatalf("a supplier of another condominium = %d, want 404", rec404.Code)
	}
	// HX gets the fragment with the supplier message, not a full error page.
	if b := rec404.Body.String(); !strings.Contains(b, i18n.T(i18n.LanguageEN, "suppliers.error.not_found")) ||
		!strings.Contains(b, `name="supplier_id" value=""`) || strings.Contains(b, "<html") {
		t.Fatalf("HX 404 body = %s", b)
	}
	if rec := do(router, "GET", "/suppliers/s-other/selected", nil, false); rec.Code != http.StatusNotFound ||
		!strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("plain 404 = %d, want the full error page", rec.Code)
	}
	if rec := do(router, "GET", "/suppliers/s1/selected?x=1", nil, false); rec.Code != http.StatusOK {
		t.Fatalf("plain request = %d, want 200", rec.Code)
	}
	member := newHarness().router([]model.Role{model.RoleOwner})
	if rec := do(member, "GET", "/suppliers/s1/selected", nil, true); rec.Code != http.StatusForbidden || rec.Body.Len() != 0 {
		t.Fatalf("member HX = %d (%q), want an empty 403", rec.Code, rec.Body.String())
	}
	// The select= mode of the search is gone: it renders nothing without q.
	if body := do(router, "GET", "/suppliers/search?select=s1", nil, true).Body.String(); strings.Contains(body, "ACME") {
		t.Fatalf("select= must not select a supplier: %s", body)
	}

	rec := do(router, "POST", "/suppliers/inline", url.Values{"supplier_name": {"Novo"}, "supplier_nif": {"501964843"}}, true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `value="s-new"`) {
		t.Fatalf("inline create = %d %s", rec.Code, rec.Body.String())
	}
	rec = do(router, "POST", "/suppliers/inline", url.Values{"supplier_name": {""}}, true)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), i18n.T(i18n.LanguageEN, "invoices.error.required")) {
		t.Fatalf("invalid inline supplier = %d %s", rec.Code, rec.Body.String())
	}
}

// --- attachments ---

func TestAttachmentDownloadHeaders(t *testing.T) {
	h := newHarness()
	path := filepath.Join(t.TempDir(), "stored")
	if err := os.WriteFile(path, []byte("%PDF-1.4 data"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.inv.attPath = path
	h.inv.att = domain.Attachment{ID: "a1", OriginalName: "inv\"oice\r\n.pdf", ContentType: "application/pdf"}

	rec := do(h.syndic(), "GET", "/invoices/i1/attachments/a1", nil, false)
	if rec.Code != http.StatusOK {
		t.Fatalf("download = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("Content-Type = %q", ct)
	}
	cd := rec.Header().Get("Content-Disposition")
	if cd != `attachment; filename=invoice.pdf` {
		t.Fatalf("Content-Disposition = %q, want the sanitized name as an attachment", cd)
	}
	if rec.Body.String() != "%PDF-1.4 data" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestUnnamedAttachmentDownloadsAsAttachment(t *testing.T) {
	h := newHarness()
	path := filepath.Join(t.TempDir(), "stored")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.inv.attPath = path
	h.inv.att = domain.Attachment{ID: "a1", OriginalName: "", ContentType: "application/pdf"}
	rec := do(h.syndic(), "GET", "/invoices/i1/attachments/a1", nil, false)
	if cd := rec.Header().Get("Content-Disposition"); rec.Code != http.StatusOK || cd != `attachment; filename=attachment` {
		t.Fatalf("status %d, Content-Disposition = %q, want the neutral name", rec.Code, cd)
	}
}

func TestDeleteNonDraftIs409WithNotDraftMessage(t *testing.T) {
	h := newHarness()
	h.inv.deleteErr = services.ErrNotDraft
	rec := do(h.syndic(), "POST", "/invoices/i1/delete", url.Values{}, false)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), i18n.T(i18n.LanguageEN, "invoices.error.not_draft")) {
		t.Fatalf("delete non-draft = %d, want 409 with the not_draft message", rec.Code)
	}
}

func TestEditConcurrentTransitionIs409WithConflictMessage(t *testing.T) {
	h := newHarness()
	h.inv.saveErr = services.ErrInvalidTransition
	rec := do(h.syndic(), "POST", "/invoices/i1/edit", validForm(), false)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), i18n.T(i18n.LanguageEN, "invoices.error.conflict")) {
		t.Fatalf("conflict = %d, want 409 with the conflict message", rec.Code)
	}
	// The invoice is reloaded: the form shows the stored number, not the posted one.
	if !strings.Contains(rec.Body.String(), `value="FT 1"`) {
		t.Fatal("the stored invoice should be reloaded into the form")
	}
}

type captureHandler struct{ records *[]slog.Record }

func (captureHandler) Enabled(context.Context, slog.Level) bool { return true }
func (c captureHandler) Handle(_ context.Context, r slog.Record) error {
	*c.records = append(*c.records, r)
	return nil
}
func (c captureHandler) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c captureHandler) WithGroup(string) slog.Handler      { return c }

func TestSupplierSearchErrorIsLoggedAndRendersEmpty(t *testing.T) {
	var records []slog.Record
	prev := slog.Default()
	slog.SetDefault(slog.New(captureHandler{records: &records}))
	defer slog.SetDefault(prev)

	h := newHarness()
	h.sup.searchErr = errors.New("db down")
	rec := do(h.syndic(), "GET", "/suppliers/search?q=acme", nil, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("search = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "ACME") {
		t.Fatal("no results expected")
	}
	logged := false
	for _, r := range records {
		if r.Level == slog.LevelError && strings.Contains(r.Message, "supplier search") {
			logged = true
		}
	}
	if !logged {
		t.Fatalf("the search error was not logged: %v", records)
	}
}

func TestUploadWithoutFilesMapsToNoFilesMessage(t *testing.T) {
	h := newHarness()
	var body strings.Builder
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("csrf_token", "tok")
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/invoices/i1/attachments", strings.NewReader(body.String()))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	h2 := &uploadErrInvoices{fakeInvoices: h.inv, err: services.ErrNoFiles}
	mux := http.NewServeMux()
	RegisterRoutes(mux, Deps{Roles: &fakeRoles{roles: []model.Role{model.RoleSyndic}}, Invoices: h2, Suppliers: h.sup})
	var root http.Handler = mux
	root = middleware.WithLocale(root)
	root = middleware.WithUser(fakeSessionReader{sess: model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "tok"}},
		fakeUserLoader{u: model.User{ID: "u1", Email: "s@example.com", Status: model.UserStatusActive}})(root)
	root.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), i18n.T(i18n.LanguageEN, "invoices.error.no_files")) {
		t.Fatalf("upload = %d (Location %q), want 422 with the no_files message", rec.Code, rec.Header().Get("Location"))
	}
}

type uploadErrInvoices struct {
	*fakeInvoices
	err error
}

func (u *uploadErrInvoices) AddAttachments(context.Context, string, string, string, []*multipart.FileHeader) ([]domain.Attachment, error) {
	return nil, u.err
}

// --- helpers ---

func TestFormatting(t *testing.T) {
	if s := centsText(i18n.LanguagePTBR, 123456, false); s != "1234,56" {
		t.Errorf("centsText pt = %q", s)
	}
	if s := centsText(i18n.LanguageEN, 5, false); s != "0.05" {
		t.Errorf("centsText en = %q", s)
	}
	if s := milliText(i18n.LanguagePTBR, 2500); s != "2,5" {
		t.Errorf("milliText pt = %q", s)
	}
	if s := milliText(i18n.LanguageEN, 3000); s != "3" {
		t.Errorf("milliText en = %q", s)
	}
}

func TestIdempotencyKeyIsUUIDv4Shaped(t *testing.T) {
	a, b := newIdempotencyKey(), newIdempotencyKey()
	if a == b || len(a) != 36 || a[14] != '4' {
		t.Fatalf("keys %q %q are not distinct UUID v4 values", a, b)
	}
}
