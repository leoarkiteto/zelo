package handlers

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/core/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/core/ports"
	"github.com/leoarkiteto/zelo/internal/features/finance/core/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

type fakeRoles struct{ roles []model.Role }

func (f *fakeRoles) ActiveRolesForUser(_ context.Context, _, _ string) ([]model.Role, error) {
	return f.roles, nil
}

type fakeSessionReader struct{ sess model.Session }

func (f fakeSessionReader) Read(*http.Request) (model.Session, error) { return f.sess, nil }

type fakeUserLoader struct{ u model.User }

func (f fakeUserLoader) GetUserByID(_ context.Context, _ string) (model.User, error) { return f.u, nil }

type fakeAccountReader struct {
	account domain.FinancialAccount
	err     error
}

func (f *fakeAccountReader) GetByID(_ context.Context, _ string) (domain.FinancialAccount, error) {
	return f.account, f.err
}

type fakeUnitLister struct{ units []model.Unit }

func (f *fakeUnitLister) ListUnitsForCondominium(context.Context, string) ([]model.Unit, error) {
	return f.units, nil
}

type fakeFinance struct {
	createErr   error
	editErr     error
	settleErr   error
	cancelErr   error
	list        []domain.FinancialAccount
	sum         ports.MonthSummary
	charges     []domain.FinancialAccount
	chargesErr  error
	health      ports.MonthSummary
	created     bool
	lastCreate  services.AccountInput
	lastSettle  string
	lastCancel  string
	lastEdit    string
	lastFilter  ports.AccountFilter
}

func (f *fakeFinance) CreateAccount(_ context.Context, _, _ string, in services.AccountInput) (domain.FinancialAccount, error) {
	f.created = true
	f.lastCreate = in
	if f.createErr != nil {
		return domain.FinancialAccount{}, f.createErr
	}
	return domain.FinancialAccount{ID: "a1", Title: in.Title, AmountCents: 35000, Status: domain.StatusPending}, nil
}

func (f *fakeFinance) EditAccount(_ context.Context, _, id, _ string, in services.AccountInput) error {
	f.lastEdit = id
	return f.editErr
}

func (f *fakeFinance) SettleAccount(_ context.Context, _, id, _, date string) error {
	f.lastSettle = id
	return f.settleErr
}

func (f *fakeFinance) CancelAccount(_ context.Context, _, id, _ string) error {
	f.lastCancel = id
	return f.cancelErr
}

func (f *fakeFinance) ListAccounts(_ context.Context, _ string, filter ports.AccountFilter) ([]domain.FinancialAccount, error) {
	f.lastFilter = filter
	return f.list, nil
}

func (f *fakeFinance) Summary(context.Context, string, time.Time) (ports.MonthSummary, error) {
	return f.sum, nil
}

func (f *fakeFinance) ResidentCharges(context.Context, string, string) ([]domain.FinancialAccount, error) {
	return f.charges, f.chargesErr
}

func (f *fakeFinance) Health(context.Context, string, time.Time) (ports.MonthSummary, error) {
	return f.health, nil
}

func financeRouter(user *model.User, sess *model.Session, deps Deps) http.Handler {
	mux := http.NewServeMux()
	RegisterRoutes(mux, deps)
	var root http.Handler = mux
	if user != nil && sess != nil {
		root = middleware.WithLocale(root)
		root = middleware.WithUser(fakeSessionReader{sess: *sess}, fakeUserLoader{u: *user})(root)
	}
	return root
}

func syndicSession() (*model.User, *model.Session) {
	user := &model.User{ID: "u1", Email: "syndic@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	return user, sess
}

func baseDeps(fin *fakeFinance) Deps {
	return Deps{
		Roles:   &fakeRoles{roles: []model.Role{model.RoleSyndic}},
		Units:   &fakeUnitLister{},
		Finance: fin,
	}
}

func TestFinanceUnauthenticatedRedirects(t *testing.T) {
	router := financeRouter(nil, nil, baseDeps(&fakeFinance{}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Fatalf("GET /finance = %d %q, want 303 /login", rec.Code, rec.Header().Get("Location"))
	}
}

func TestFinanceIsSyndicOnly(t *testing.T) {
	user := &model.User{ID: "u1", Email: "owner@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u1", CondominiumID: "c1", CSRFToken: "x"}
	deps := baseDeps(&fakeFinance{})
	deps.Roles = &fakeRoles{roles: []model.Role{model.RoleOwner}}
	router := financeRouter(user, sess, deps)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("owner on /finance = %d, want 403", rec.Code)
	}
}

func TestFinanceDashboardRenders(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{
		list: []domain.FinancialAccount{{
			ID: "a1", CondominiumID: "c1", Type: domain.AccountTypePayable,
			Title: "Manutenção do portão", Category: domain.CategoryMaintenance,
			AmountCents: 35000, DueDate: time.Now().Add(7 * 24 * time.Hour), Status: domain.StatusPending,
		}},
		sum: ports.MonthSummary{TotalReceivableCents: 50000, TotalPayableCents: 35000, ProjectedBalanceCents: 15000},
	}
	router := financeRouter(user, sess, baseDeps(fin))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /finance = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Manutenção do portão", "R$ 350.00", "Pending", "Maintenance"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard is missing %q", want)
		}
	}
	if !strings.Contains(body, "Accounts payable &amp; receivable") {
		t.Errorf("dashboard is missing the page title")
	}
}

func TestFinanceNewFormRenders(t *testing.T) {
	user, sess := syndicSession()
	router := financeRouter(user, sess, baseDeps(&fakeFinance{}))

	for _, tt := range []struct{ query, want string }{
		{"type=payable", "Register an expense"},
		{"type=receivable", "Register revenue"},
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance/new?"+tt.query, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /finance/new?%s = %d, want 200", tt.query, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), tt.want) {
			t.Errorf("form for %q missing %q", tt.query, tt.want)
		}
	}
}

func TestFinanceCreatePOST(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{}
	router := financeRouter(user, sess, baseDeps(fin))

	form := url.Values{
		"type": {"payable"}, "title": {"Manutenção do portão"},
		"category": {"maintenance"}, "amount": {"350.00"}, "due_date": {"2026-08-30"},
		"status": {"pending"}, "csrf_token": {"x"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/finance", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/finance?flash=created" {
		t.Fatalf("POST /finance = %d %q, want 303 /finance?flash=created", rec.Code, rec.Header().Get("Location"))
	}
	if !fin.created || fin.lastCreate.Title != "Manutenção do portão" {
		t.Fatalf("service not called correctly: %+v", fin.lastCreate)
	}
}

func TestFinanceCreateValidationErrorRerenders(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{createErr: services.ErrInvalidAmount}
	router := financeRouter(user, sess, baseDeps(fin))

	form := url.Values{
		"type": {"payable"}, "title": {"Manutenção do portão"},
		"category": {"maintenance"}, "amount": {"0"}, "due_date": {"2026-08-30"},
		"status": {"pending"}, "csrf_token": {"x"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/finance", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /finance invalid = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Enter a valid amount") {
		t.Errorf("validation error not shown: %s", rec.Body.String())
	}
}

func TestFinanceSettleRequiresDate(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{settleErr: services.ErrSettlementDateRequired}
	deps := baseDeps(fin)
	deps.Accounts = &fakeAccountReader{account: domain.FinancialAccount{
		ID: "a1", CondominiumID: "c1", Status: domain.StatusPending,
	}}
	router := financeRouter(user, sess, deps)

	form := url.Values{"settlement_date": {""}, "csrf_token": {"x"}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/finance/a1/settle", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST settle without date = %d, want 200 re-render", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "A transaction date is required") {
		t.Errorf("settle date error not shown")
	}
}

func TestFinanceReceiptMissingReturns404(t *testing.T) {
	user, sess := syndicSession()
	deps := baseDeps(&fakeFinance{})
	deps.Accounts = &fakeAccountReader{account: domain.FinancialAccount{
		ID: "a1", CondominiumID: "c1", Status: domain.StatusSettled,
	}}
	router := financeRouter(user, sess, deps)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance/a1/receipt", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET receipt without file = %d, want 404", rec.Code)
	}
}

func TestMyChargesNoUnit(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{chargesErr: services.ErrNoUnit}
	router := financeRouter(user, sess, baseDeps(fin))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance/my-charges", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /finance/my-charges = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "You need an active unit") {
		t.Errorf("no-unit message missing")
	}
}

func TestHealthRendersAggregates(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{health: ports.MonthSummary{
		RealizedPayableCents: 35000, RealizedReceivableCents: 50000,
	}}
	router := financeRouter(user, sess, baseDeps(fin))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /finance/health = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Financial health", "R$ 350.00", "R$ 500.00"} {
		if !strings.Contains(body, want) {
			t.Errorf("health page missing %q", want)
		}
	}
}

func TestFinanceInadimplentesFilter(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{}
	router := financeRouter(user, sess, baseDeps(fin))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance?status=inadimplentes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /finance?status=inadimplentes = %d, want 200", rec.Code)
	}
	if fin.lastFilter.Type != domain.AccountTypeReceivable || fin.lastFilter.Status != domain.StatusOverdue {
		t.Fatalf("inadimplentes filter = %+v, want receivable+overdue", fin.lastFilter)
	}
	if !strings.Contains(rec.Body.String(), `value="inadimplentes" selected`) {
		t.Errorf("inadimplentes option is not shown as selected")
	}
}

// accountMultipart builds a multipart POST /finance request with the given
// receipt part (contentType "" means no receipt part).
func accountMultipart(t *testing.T, contentType string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	writeField := func(k, v string) {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	writeField("type", "payable")
	writeField("title", "Manutenção do portão")
	writeField("category", "maintenance")
	writeField("amount", "350.00")
	writeField("due_date", "2026-08-30")
	writeField("status", "pending")
	writeField("csrf_token", "x")
	if contentType != "" {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="receipt"; filename="recibo"`)
		h.Set("Content-Type", contentType)
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		if _, err := part.Write(content); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/finance", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

var pngBytes = append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, []byte("filler data")...)

func TestFinanceCreatePOSTWithReceipt(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{}
	deps := baseDeps(fin)
	deps.UploadDir = t.TempDir()
	router := financeRouter(user, sess, deps)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, accountMultipart(t, "image/png", pngBytes))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /finance with receipt = %d, want 303", rec.Code)
	}
	if fin.lastCreate.ReceiptPath == "" || !strings.HasSuffix(fin.lastCreate.ReceiptPath, ".png") {
		t.Fatalf("receipt not stored: %+v", fin.lastCreate)
	}
	entries, err := os.ReadDir(deps.UploadDir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 stored receipt, got %v (%v)", entries, err)
	}
}

func TestFinanceCreatePOSTRejectsSpoofedReceipt(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{}
	deps := baseDeps(fin)
	deps.UploadDir = t.TempDir()
	router := financeRouter(user, sess, deps)

	// Client claims image/png but the content sniffs as text/plain (T057).
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, accountMultipart(t, "image/png", []byte("this is definitely not an image")))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("spoofed receipt = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Receipt must be a PDF") {
		t.Errorf("receipt error not shown")
	}
	if entries, _ := os.ReadDir(deps.UploadDir); len(entries) != 0 {
		t.Fatalf("spoofed receipt must not be stored, found %d files", len(entries))
	}
}

func TestFinanceCreatePOSTRejectsOversizeReceipt(t *testing.T) {
	user, sess := syndicSession()
	fin := &fakeFinance{}
	deps := baseDeps(fin)
	deps.UploadDir = t.TempDir()
	router := financeRouter(user, sess, deps)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, accountMultipart(t, "image/png", make([]byte, 5<<20+1)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversize receipt = %d, want 400", rec.Code)
	}
}

func TestFinanceReceiptDownloadSyndic(t *testing.T) {
	user, sess := syndicSession()
	dir := t.TempDir()
	payload := []byte("%PDF-1.4 fake pdf body")
	if err := os.WriteFile(filepath.Join(dir, "abc.pdf"), payload, 0o644); err != nil {
		t.Fatalf("write receipt: %v", err)
	}
	deps := baseDeps(&fakeFinance{})
	deps.UploadDir = dir
	deps.Accounts = &fakeAccountReader{account: domain.FinancialAccount{
		ID: "a1", CondominiumID: "c1", Status: domain.StatusSettled,
		ReceiptPath: "abc.pdf", ReceiptName: "recibo.pdf",
	}}
	router := financeRouter(user, sess, deps)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance/a1/receipt", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET receipt = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), payload) {
		t.Errorf("receipt body mismatch")
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "recibo.pdf") {
		t.Errorf("Content-Disposition = %q, want attachment with recibo.pdf", cd)
	}
}

func TestFinanceReceiptDownloadResidentForbidden(t *testing.T) {
	user := &model.User{ID: "u2", Email: "owner@example.com", Status: model.UserStatusActive}
	sess := &model.Session{TokenHash: "th", UserID: "u2", CondominiumID: "c1", CSRFToken: "x"}
	deps := baseDeps(&fakeFinance{})
	deps.Roles = &fakeRoles{roles: []model.Role{model.RoleOwner}}
	router := financeRouter(user, sess, deps)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/finance/a1/receipt", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("resident on receipt = %d, want 403", rec.Code)
	}
}
