package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// invEnv is the composed app with two condominiums, each with a syndic, plus an
// owner and a tenant of condominium A, all signed in.
type invEnv struct {
	base string
	db   *sql.DB

	condoA, condoB  string
	syndicAID       string
	syndicBID       string
	uploadDir       string
	syndic, syndicB *http.Client // syndic of A and of B
	owner, tenant   *http.Client // members of A without the syndic role
	csrfTokens      map[*http.Client]string
}

func newInvEnv(t *testing.T) *invEnv {
	t.Helper()
	router, db := newAppDB(t)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	e := &invEnv{
		base:       ts.URL,
		db:         db,
		condoA:     os.Getenv("TEST_CONDOMINIUM"),
		condoB:     os.Getenv("TEST_CONDOMINIUM_B"),
		uploadDir:  os.Getenv("TEST_UPLOAD_DIR"),
		csrfTokens: map[*http.Client]string{},
	}
	ctx := context.Background()
	if err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE email='syndic@example.com'`).Scan(&e.syndicAID); err != nil {
		t.Fatalf("syndic A id: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE email=$1`, syndicBEmail).Scan(&e.syndicBID); err != nil {
		t.Fatalf("syndic B id: %v", err)
	}
	insertUserWithRole(t, db, "tenant-inv@example.com", "tenant", e.condoA)

	e.syndic = loginAs(t, e.base, "syndic@example.com", "syndic-pass-123")
	e.syndicB = loginAs(t, e.base, syndicBEmail, syndicBPass)
	e.owner = loginAs(t, e.base, "resident@example.com", "resident-pass-123")
	e.tenant = loginAs(t, e.base, "tenant-inv@example.com", memberPass)
	return e
}

// resp is one HTTP response.
type resp struct {
	code   int
	header http.Header
	body   string
}

func (r resp) location() string { return r.header.Get("Location") }

// csrf returns the session CSRF token of a signed-in client (read from the
// profile page, which every member can open).
func (e *invEnv) csrf(t *testing.T, c *http.Client) string {
	t.Helper()
	if tok, ok := e.csrfTokens[c]; ok {
		return tok
	}
	tok := csrfFrom(t, c, e.base, "/profile")
	e.csrfTokens[c] = tok
	return tok
}

func (e *invEnv) do(t *testing.T, c *http.Client, method, path string, body io.Reader, hdr map[string]string) resp {
	t.Helper()
	req, err := http.NewRequest(method, e.base+path, body)
	if err != nil {
		t.Fatalf("build %s %s: %v", method, path, err)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return resp{code: res.StatusCode, header: res.Header, body: string(b)}
}

// get issues a GET (with optional headers).
func (e *invEnv) get(t *testing.T, c *http.Client, path string, hdr ...map[string]string) resp {
	t.Helper()
	h := map[string]string{}
	for _, m := range hdr {
		for k, v := range m {
			h[k] = v
		}
	}
	return e.do(t, c, http.MethodGet, path, nil, h)
}

// hx is the header set of an htmx request.
var hx = map[string]string{"HX-Request": "true"}

// post sends an url-encoded form. The CSRF token is added unless v has one.
func (e *invEnv) post(t *testing.T, c *http.Client, path string, v url.Values, hdr ...map[string]string) resp {
	t.Helper()
	if v == nil {
		v = url.Values{}
	}
	if v.Get("csrf_token") == "" {
		v.Set("csrf_token", e.csrf(t, c))
	}
	h := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
	for _, m := range hdr {
		for k, val := range m {
			h[k] = val
		}
	}
	return e.do(t, c, http.MethodPost, path, strings.NewReader(v.Encode()), h)
}

// upFile is one file of a multipart upload.
type upFile struct {
	name    string
	content []byte
}

// upload sends a multipart/form-data POST with the given files in field "files".
func (e *invEnv) upload(t *testing.T, c *http.Client, path string, files []upFile) resp {
	t.Helper()
	return e.multipart(t, c, path, nil, "files", files)
}

func (e *invEnv) multipart(t *testing.T, c *http.Client, path string, fields map[string]string, fileField string, files []upFile) resp {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("csrf_token", e.csrf(t, c)); err != nil {
		t.Fatalf("multipart csrf: %v", err)
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("multipart field %s: %v", k, err)
		}
	}
	for _, f := range files {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, fileField, f.name))
		h.Set("Content-Type", "application/octet-stream")
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("multipart part: %v", err)
		}
		if _, err := part.Write(f.content); err != nil {
			t.Fatalf("multipart write: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("multipart close: %v", err)
	}
	return e.do(t, c, http.MethodPost, path, &buf, map[string]string{"Content-Type": w.FormDataContentType()})
}

// pdfBytes is a minimal file that sniffs as application/pdf, padded to size bytes.
func pdfBytes(size int) []byte {
	b := bytes.Repeat([]byte(" "), size)
	copy(b, "%PDF-1.4\n")
	return b
}

// pngBytes is a minimal file that sniffs as image/png, padded to size bytes.
func pngBytes(size int) []byte {
	b := make([]byte, size)
	copy(b, "\x89PNG\r\n\x1a\n")
	return b
}

// exeBytes is a Windows executable header: it never sniffs as an allowed type.
func exeBytes() []byte {
	b := make([]byte, 2048)
	copy(b, "MZ\x90\x00\x03\x00\x00\x00")
	return b
}

// fmtMoney renders cents the way English pages show amounts in the default
// currency (BRL).
func fmtMoney(cents int64) string { return money.Format(i18n.LanguageEN, money.BRL, cents) }

// validNIF completes an 8 digit prefix with its mod-11 check digit.
func validNIF(prefix8 string) string {
	sum := 0
	for i := 0; i < 8; i++ {
		sum += int(prefix8[i]-'0') * (9 - i)
	}
	check := 11 - sum%11
	if check >= 10 {
		check = 0
	}
	return prefix8 + strconv.Itoa(check)
}

func randomKey() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// supplierForm are the fields of the supplier pages.
func supplierForm(name, nif string) url.Values {
	return url.Values{
		"name": {name}, "nif": {nif}, "country": {"PT"},
		"address": {"Rua Direita 1"}, "postal_code": {"1000-001"}, "locality": {"Lisboa"},
		"email": {"geral@example.com"}, "phone": {"210000000"},
	}
}

// newSupplier creates a supplier through POST /suppliers and returns its id.
func (e *invEnv) newSupplier(t *testing.T, c *http.Client, condo, name, nif string) string {
	t.Helper()
	r := e.post(t, c, "/suppliers", supplierForm(name, nif))
	if r.code != http.StatusSeeOther {
		t.Fatalf("create supplier %q = %d, want 303\n%s", name, r.code, r.body)
	}
	var id string
	if err := e.db.QueryRow(`SELECT id FROM suppliers WHERE condominium_id=$1 AND name=$2`, condo, name).Scan(&id); err != nil {
		t.Fatalf("find supplier %q: %v", name, err)
	}
	return id
}

// invOpts tunes invoiceForm.
type invOpts struct {
	docType     string
	number      string
	issue       string
	baseCents   string // line base, free text ("1000.00")
	treatment   string
	rate        string
	withholding string
}

// invoiceForm is a complete, valid form of a non-product invoice with one line.
func invoiceForm(supplierID, number string, mod ...func(*invOpts)) url.Values {
	o := invOpts{docType: "fatura", number: number, issue: "2026-03-10", baseCents: "1000.00", treatment: "reverse_charge"}
	for _, m := range mod {
		m(&o)
	}
	v := url.Values{
		"action":          {"register"},
		"idempotency_key": {randomKey()},
		"document_type":   {o.docType},
		"number":          {o.number},
		"issue_date":      {o.issue},
		"supplier_id":     {supplierID},
		"expense_type":    {"manutencao"},
		"category":        {"manutencao"},
		"description":     {"Reparacao do elevador"},
		"fiscal_region":   {"continental"},
		"withholding":     {o.withholding},

		"lines[0].description": {"Mao de obra"},
		"lines[0].base":        {o.baseCents},
		"lines[0].treatment":   {o.treatment},
		"lines[0].rate":        {o.rate},
	}
	return v
}

var invoiceIDRe = regexp.MustCompile(`^/invoices/([0-9a-f-]{36})`)

// idFromLocation extracts the invoice id of a /invoices/<id> redirect.
func idFromLocation(t *testing.T, r resp) string {
	t.Helper()
	m := invoiceIDRe.FindStringSubmatch(r.location())
	if m == nil {
		t.Fatalf("response %d has no invoice Location (%q)\n%s", r.code, r.location(), r.body)
	}
	return m[1]
}

// createInvoice posts v to /invoices and returns the new id, failing unless the
// response is a 303 to the invoice.
func (e *invEnv) createInvoice(t *testing.T, c *http.Client, v url.Values) string {
	t.Helper()
	r := e.post(t, c, "/invoices", v)
	if r.code != http.StatusSeeOther {
		t.Fatalf("create invoice = %d, want 303\n%s", r.code, r.body)
	}
	return idFromLocation(t, r)
}

// payment posts a payment of amount (free text) on invoice id.
func (e *invEnv) payment(t *testing.T, c *http.Client, id, amount, justification string) resp {
	t.Helper()
	v := url.Values{
		"payment_date":          {"2026-04-01"},
		"payment_amount":        {amount},
		"payment_method":        {"transferencia_bancaria"},
		"payment_reference":     {"REF-1"},
		"payment_justification": {justification},
	}
	return e.post(t, c, "/invoices/"+id+"/payments", v)
}

func (e *invEnv) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func (e *invEnv) invoiceCount(t *testing.T, condo string) int {
	t.Helper()
	return e.count(t, `SELECT count(*) FROM invoices WHERE condominium_id=$1`, condo)
}

// auditRows counts audit rows of an event type written by user for condo.
func (e *invEnv) auditRows(t *testing.T, event, user, condo string) int {
	t.Helper()
	return e.count(t,
		`SELECT count(*) FROM audit_events WHERE event_type=$1 AND user_id=$2 AND details->>'condominium_id'=$3`,
		event, user, condo)
}

// attachmentIDs lists the attachment ids of an invoice, oldest first.
func (e *invEnv) attachmentIDs(t *testing.T, invoiceID string) []string {
	t.Helper()
	rows, err := e.db.Query(`SELECT id FROM invoice_attachments WHERE invoice_id=$1 ORDER BY created_at, id`, invoiceID)
	if err != nil {
		t.Fatalf("attachments: %v", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan attachment: %v", err)
		}
		ids = append(ids, id)
	}
	return ids
}

// section returns the part of body that starts at marker and ends at end.
func section(t *testing.T, body, marker, end string) string {
	t.Helper()
	i := strings.Index(body, marker)
	if i < 0 {
		t.Fatalf("marker %q not found in page", marker)
	}
	rest := body[i:]
	if j := strings.Index(rest, end); j >= 0 {
		return rest[:j]
	}
	return rest
}

func mustContain(t *testing.T, what, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("%s: missing %q", what, w)
		}
	}
}

func mustNotContain(t *testing.T, what, body string, nots ...string) {
	t.Helper()
	for _, n := range nots {
		if strings.Contains(body, n) {
			t.Errorf("%s: unexpected %q", what, n)
		}
	}
}
