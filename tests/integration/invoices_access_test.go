package integration

import (
	"bytes"
	"context"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"log/slog"

	invoiceservices "github.com/leoarkiteto/zelo/internal/features/invoices/services"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

// AC-21: PDF and PNG attachments are stored, listed and downloadable; invalid,
// oversized and surplus files are rejected.
func TestInvoiceAttachments(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Attach Lda", validNIF("50000014"))
	id := e.createInvoice(t, e.syndic, invoiceForm(sup, "ATT 1"))
	pdf, png := pdfBytes(3000), pngBytes(2000)

	r := e.upload(t, e.syndic, "/invoices/"+id+"/attachments", []upFile{{"invoice.pdf", pdf}, {"scan.png", png}})
	if r.code != http.StatusSeeOther {
		t.Fatalf("upload pdf + png = %d, want 303\n%s", r.code, r.body)
	}
	ids := e.attachmentIDs(t, id)
	if len(ids) != 2 {
		t.Fatalf("stored attachments = %d, want 2", len(ids))
	}
	view := e.get(t, e.syndic, "/invoices/"+id)
	attSection := section(t, view.body, `id="attachments"`, `</section>`)
	mustContain(t, "attachment list", attSection, "invoice.pdf", "scan.png")
	for _, a := range ids {
		mustContain(t, "attachment link", attSection, "/invoices/"+id+"/attachments/"+a)
	}

	for _, tc := range []struct {
		name, ctype string
		want        []byte
	}{{"invoice.pdf", "application/pdf", pdf}, {"scan.png", "image/png", png}} {
		var aid string
		if err := e.db.QueryRow(`SELECT id FROM invoice_attachments WHERE invoice_id=$1 AND original_name=$2`, id, tc.name).Scan(&aid); err != nil {
			t.Fatalf("attachment %s: %v", tc.name, err)
		}
		d := e.get(t, e.syndic, "/invoices/"+id+"/attachments/"+aid)
		if d.code != http.StatusOK || !bytes.Equal([]byte(d.body), tc.want) {
			t.Errorf("download %s = %d, %d bytes, want 200 and the uploaded %d bytes", tc.name, d.code, len(d.body), len(tc.want))
		}
		if ct := d.header.Get("Content-Type"); ct != tc.ctype {
			t.Errorf("download %s Content-Type = %q, want %q", tc.name, ct, tc.ctype)
		}
		if cd := d.header.Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment") || !strings.Contains(cd, tc.name) {
			t.Errorf("download %s Content-Disposition = %q", tc.name, cd)
		}
		if d.header.Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("download %s lacks nosniff", tc.name)
		}
	}

	reject := func(what string, files []upFile, wantStored int) {
		t.Helper()
		got := e.upload(t, e.syndic, "/invoices/"+id+"/attachments", files)
		if got.code != http.StatusUnprocessableEntity {
			t.Errorf("%s = %d, want 422", what, got.code)
		}
		if n := len(e.attachmentIDs(t, id)); n != wantStored {
			t.Errorf("%s: stored attachments = %d, want %d", what, n, wantStored)
		}
	}
	reject("exe renamed .pdf", []upFile{{"totally-a.pdf", exeBytes()}}, 2)
	reject("6 MB file", []upFile{{"big.pdf", pdfBytes(6 << 20)}}, 2)
	reject("valid file with an invalid companion", []upFile{{"ok.pdf", pdf}, {"bad.pdf", exeBytes()}}, 2)

	// Exactly 5 MB is allowed.
	if r := e.upload(t, e.syndic, "/invoices/"+id+"/attachments", []upFile{{"limit.pdf", pdfBytes(5 << 20)}}); r.code != http.StatusSeeOther {
		t.Fatalf("5 MB upload = %d, want 303", r.code)
	}

	// Fill up to 10 files, then reject the 11th.
	for i := len(e.attachmentIDs(t, id)); i < 9; i++ {
		if r := e.upload(t, e.syndic, "/invoices/"+id+"/attachments", []upFile{{"extra.pdf", pdf}}); r.code != http.StatusSeeOther {
			t.Fatalf("filler upload = %d", r.code)
		}
	}
	reject("two files over the limit of 10", []upFile{{"a.pdf", pdf}, {"b.pdf", pdf}}, 9)
	if r := e.upload(t, e.syndic, "/invoices/"+id+"/attachments", []upFile{{"tenth.pdf", pdf}}); r.code != http.StatusSeeOther {
		t.Fatalf("10th upload = %d, want 303", r.code)
	}
	reject("11th file", []upFile{{"eleventh.pdf", pdf}}, 10)

	// A client file name with path segments is never used as a path.
	removed := e.attachmentIDs(t, id)[0]
	var storedPath string
	if err := e.db.QueryRow(`SELECT stored_path FROM invoice_attachments WHERE id=$1`, removed).Scan(&storedPath); err != nil {
		t.Fatalf("stored path: %v", err)
	}
	invoicesDir := filepath.Join(e.uploadDir, "invoices")
	if _, err := os.Stat(filepath.Join(invoicesDir, storedPath)); err != nil {
		t.Fatalf("stored file missing: %v", err)
	}

	// Removing an attachment deletes the row and the file.
	r = e.post(t, e.syndic, "/invoices/"+id+"/attachments/"+removed+"/delete", nil)
	if r.code != http.StatusSeeOther {
		t.Fatalf("remove attachment = %d, want 303\n%s", r.code, r.body)
	}
	if n := len(e.attachmentIDs(t, id)); n != 9 {
		t.Errorf("attachments after removal = %d, want 9", n)
	}
	if _, err := os.Stat(filepath.Join(invoicesDir, storedPath)); !os.IsNotExist(err) {
		t.Errorf("removed attachment file still on disk (err=%v)", err)
	}
}

// AC-22: an attachment is served only to the syndic of its own condominium and
// never through a path traversal.
func TestInvoiceAttachmentAccessControl(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Secret Lda", validNIF("50000015"))
	id := e.createInvoice(t, e.syndic, invoiceForm(sup, "SEC 1"))
	// A traversal client name is neutralised on upload.
	if r := e.upload(t, e.syndic, "/invoices/"+id+"/attachments", []upFile{{"../../evil.pdf", pdfBytes(100)}}); r.code != http.StatusSeeOther {
		t.Fatalf("upload = %d\n%s", r.code, r.body)
	}
	aid := e.attachmentIDs(t, id)[0]
	for _, p := range []string{filepath.Join(e.uploadDir, "evil.pdf"), filepath.Join(filepath.Dir(e.uploadDir), "evil.pdf"), filepath.Join(e.uploadDir, "invoices", "evil.pdf")} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("traversal upload wrote %s", p)
		}
	}
	path := "/invoices/" + id + "/attachments/" + aid

	if r := e.get(t, e.syndic, path); r.code != http.StatusOK {
		t.Fatalf("owner syndic download = %d, want 200", r.code)
	}
	if r := e.get(t, e.owner, path); r.code != http.StatusForbidden {
		t.Errorf("owner download = %d, want 403", r.code)
	}
	if r := e.get(t, e.tenant, path); r.code != http.StatusForbidden {
		t.Errorf("tenant download = %d, want 403", r.code)
	}
	if r := e.get(t, e.syndicB, path); r.code != http.StatusNotFound {
		t.Errorf("syndic of another condominium = %d, want 404", r.code)
	}
	if r := e.get(t, newClient(t), path); r.code == http.StatusOK {
		t.Errorf("anonymous download = 200, want a redirect to login")
	}

	// Traversal in the attachment segment of the URL.
	const secret = "TOP-SECRET-FILE-CONTENT"
	if err := os.WriteFile(filepath.Join(e.uploadDir, "secret.txt"), []byte(secret), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	for _, seg := range []string{"..%2F..%2Fsecret.txt", "..%2Fsecret.txt", "%2E%2E%2F%2E%2E%2Fsecret.txt", "../../secret.txt", "..%5C..%5Csecret.txt"} {
		r := e.get(t, e.syndic, "/invoices/"+id+"/attachments/"+seg)
		if r.code == http.StatusOK || strings.Contains(r.body, secret) {
			t.Errorf("traversal segment %q served content (code %d)", seg, r.code)
		}
		if r.code >= 500 {
			t.Errorf("traversal segment %q = %d, want a client error", seg, r.code)
		}
	}

	// A row whose stored path escapes the attachments folder is refused.
	var planted string
	if err := e.db.QueryRow(`
		INSERT INTO invoice_attachments (condominium_id, invoice_id, stored_path, original_name, content_type, size_bytes, created_by, updated_by)
		VALUES ($1, $2, '../secret.txt', 'secret.pdf', 'application/pdf', 10, $3, $3) RETURNING id`,
		e.condoA, id, e.syndicAID).Scan(&planted); err != nil {
		t.Fatalf("plant attachment: %v", err)
	}
	r := e.get(t, e.syndic, "/invoices/"+id+"/attachments/"+planted)
	if r.code != http.StatusNotFound || strings.Contains(r.body, secret) {
		t.Errorf("planted traversal path = %d (secret leaked: %v), want 404", r.code, strings.Contains(r.body, secret))
	}
}

type route struct {
	method, path string
	upload       bool
	fragment     bool
}

// AC-25: every /invoices and /suppliers route is syndic-only.
func TestInvoiceAndSupplierRoutesForbiddenToMembers(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Guarded Lda", validNIF("50000016"))
	id := e.createInvoice(t, e.syndic, invoiceForm(sup, "GRD 1"))
	if r := e.upload(t, e.syndic, "/invoices/"+id+"/attachments", []upFile{{"a.pdf", pdfBytes(100)}}); r.code != http.StatusSeeOther {
		t.Fatalf("upload = %d", r.code)
	}
	aid := e.attachmentIDs(t, id)[0]
	inv, att := "/invoices/"+id, "/invoices/"+id+"/attachments/"+aid

	routes := []route{
		{"GET", "/invoices", false, false},
		{"GET", "/invoices/new", false, false},
		{"POST", "/invoices", false, false},
		{"GET", inv, false, false},
		{"GET", inv + "/edit", false, false},
		{"POST", inv + "/edit", false, false},
		{"POST", inv + "/delete", false, false},
		{"POST", inv + "/annul", false, false},
		{"POST", inv + "/payments", false, false},
		{"POST", inv + "/attachments", true, false},
		{"GET", att, false, false},
		{"POST", att + "/delete", false, false},
		{"GET", "/invoices/form/section?expense_type=despesa_mensal", false, true},
		{"POST", "/invoices/form/lines", false, true},
		{"POST", "/invoices/form/preview", false, true},
		{"GET", "/suppliers", false, false},
		{"GET", "/suppliers/new", false, false},
		{"POST", "/suppliers", false, false},
		{"GET", "/suppliers/" + sup + "/edit", false, false},
		{"POST", "/suppliers/" + sup + "/edit", false, false},
		{"GET", "/suppliers/search?q=guard", false, true},
		{"POST", "/suppliers/inline", false, true},
	}
	members := map[string]*http.Client{"owner": e.owner, "tenant": e.tenant}
	for who, c := range members {
		for _, rt := range routes {
			var r resp
			var hdrs []map[string]string
			if rt.fragment {
				hdrs = append(hdrs, hx)
			}
			switch {
			case rt.method == "GET":
				r = e.get(t, c, rt.path, hdrs...)
			case rt.upload:
				r = e.upload(t, c, rt.path, []upFile{{"x.pdf", pdfBytes(50)}})
			default:
				r = e.post(t, c, rt.path, url.Values{"supplier_name": {"x"}, "name": {"x"}}, hdrs...)
			}
			if r.code != http.StatusForbidden {
				t.Errorf("%s %s %s = %d, want 403", who, rt.method, rt.path, r.code)
			}
			if strings.Contains(r.body, "Invalid CSRF") {
				t.Errorf("%s %s %s was rejected by CSRF, not by the role guard", who, rt.method, rt.path)
			}
			if strings.Contains(r.body, "Guarded Lda") || strings.Contains(r.body, "GRD 1") {
				t.Errorf("%s %s %s leaked invoice data", who, rt.method, rt.path)
			}
		}
	}
	// The same fragment routes without htmx headers are also refused.
	if r := e.get(t, e.owner, "/suppliers/search?q=guard"); r.code != http.StatusForbidden {
		t.Errorf("owner plain GET /suppliers/search = %d, want 403", r.code)
	}
	if n := e.invoiceCount(t, e.condoA); n != 1 {
		t.Errorf("invoices after forbidden writes = %d, want 1", n)
	}
	if n := e.count(t, `SELECT count(*) FROM suppliers`); n != 1 {
		t.Errorf("suppliers after forbidden writes = %d, want 1", n)
	}
}

// AC-25: a syndic gets 404 for records of another condominium, which stay
// untouched, and cannot attach a foreign supplier to an invoice.
func TestInvoiceCrossCondominiumIsolation(t *testing.T) {
	e := newInvEnv(t)
	supA := e.newSupplier(t, e.syndic, e.condoA, "Tenant A Supplier", validNIF("50000017"))
	supB := e.newSupplier(t, e.syndicB, e.condoB, "Tenant B Supplier", validNIF("50000018"))
	id := e.createInvoice(t, e.syndic, invoiceForm(supA, "ISO 1"))
	e.upload(t, e.syndic, "/invoices/"+id+"/attachments", []upFile{{"a.pdf", pdfBytes(100)}})
	aid := e.attachmentIDs(t, id)[0]
	inv := "/invoices/" + id

	for _, p := range []string{inv, inv + "/edit", inv + "/attachments/" + aid, "/suppliers/" + supA + "/edit"} {
		if r := e.get(t, e.syndicB, p); r.code != http.StatusNotFound {
			t.Errorf("syndic B GET %s = %d, want 404", p, r.code)
		} else if strings.Contains(r.body, "ISO 1") || strings.Contains(r.body, "Tenant A Supplier") {
			t.Errorf("syndic B GET %s leaked data", p)
		}
	}
	writes := []struct {
		path string
		v    url.Values
	}{
		{inv + "/edit", invoiceForm(supB, "HACK 1")},
		{inv + "/delete", nil},
		{inv + "/annul", url.Values{"annul_reason": {"hijack"}}},
		{inv + "/payments", url.Values{"payment_date": {"2026-04-01"}, "payment_amount": {"10.00"}, "payment_method": {"outro"}}},
		{inv + "/attachments/" + aid + "/delete", nil},
		{"/suppliers/" + supA + "/edit", supplierForm("Hijacked", validNIF("50000019"))},
	}
	for _, w := range writes {
		if r := e.post(t, e.syndicB, w.path, w.v); r.code != http.StatusNotFound {
			t.Errorf("syndic B POST %s = %d, want 404", w.path, r.code)
		}
	}
	if r := e.upload(t, e.syndicB, inv+"/attachments", []upFile{{"b.pdf", pdfBytes(100)}}); r.code != http.StatusNotFound {
		t.Errorf("syndic B upload = %d, want 404", r.code)
	}
	var state, number string
	if err := e.db.QueryRow(`SELECT state, number FROM invoices WHERE id=$1`, id).Scan(&state, &number); err != nil || state != "registered" || number != "ISO 1" {
		t.Errorf("invoice after cross-tenant writes = %q %q (%v), want registered ISO 1", state, number, err)
	}
	if n := e.count(t, `SELECT count(*) FROM invoice_payments`); n != 0 {
		t.Errorf("payments = %d, want 0", n)
	}
	if n := len(e.attachmentIDs(t, id)); n != 1 {
		t.Errorf("attachments = %d, want 1", n)
	}
	var name string
	if err := e.db.QueryRow(`SELECT name FROM suppliers WHERE id=$1`, supA).Scan(&name); err != nil || name != "Tenant A Supplier" {
		t.Errorf("supplier A name = %q (%v)", name, err)
	}

	// Lists and the supplier search show only the session's condominium.
	if r := e.get(t, e.syndicB, "/invoices"); strings.Contains(r.body, "ISO 1") {
		t.Errorf("syndic B list shows condominium A's invoice")
	}
	if r := e.get(t, e.syndicB, "/suppliers"); strings.Contains(r.body, "Tenant A Supplier") || !strings.Contains(r.body, "Tenant B Supplier") {
		t.Errorf("syndic B supplier list is not scoped to condominium B")
	}
	if r := e.get(t, e.syndicB, "/suppliers/search?q=Tenant", hx); strings.Contains(r.body, "Tenant A Supplier") {
		t.Errorf("syndic B search offered condominium A's supplier")
	}

	// A supplier_id of condominium B on a form of condominium A.
	for _, action := range []string{"register", "draft"} {
		v := invoiceForm(supB, "CROSS "+action)
		v.Set("action", action)
		r := e.post(t, e.syndic, "/invoices", v)
		if r.code != http.StatusUnprocessableEntity {
			t.Errorf("%s with a foreign supplier = %d, want 422", action, r.code)
		}
		mustContain(t, action+" foreign supplier", r.body, en(invoiceservices.KeyUnknownSupplier))
		mustNotContain(t, action+" foreign supplier", r.body, "Tenant B Supplier", validNIF("50000018"))
	}
	if n := e.invoiceCount(t, e.condoA); n != 1 {
		t.Errorf("invoices of A = %d, want 1 (foreign supplier must not save)", n)
	}
	if n := e.invoiceCount(t, e.condoB); n != 0 {
		t.Errorf("invoices of B = %d, want 0", n)
	}
	// Editing an own invoice to point at the foreign supplier is also refused.
	r := e.post(t, e.syndic, inv+"/edit", invoiceForm(supB, "ISO 1"))
	if r.code != http.StatusUnprocessableEntity {
		t.Errorf("edit with a foreign supplier = %d, want 422", r.code)
	}
	var supplierID string
	if err := e.db.QueryRow(`SELECT supplier_id FROM invoices WHERE id=$1`, id).Scan(&supplierID); err != nil || supplierID != supA {
		t.Errorf("invoice supplier after rejected edit = %q (%v), want %q", supplierID, err, supA)
	}
}

// AC-29: the invoice menu entries are only rendered for the syndic.
func TestInvoiceNavigationOnlyForSyndic(t *testing.T) {
	e := newInvEnv(t)
	links := []string{`href="/invoices"`, `href="/suppliers"`, `href="/condominium/settings"`}
	for who, c := range map[string]*http.Client{"owner": e.owner, "tenant": e.tenant} {
		r := e.get(t, c, "/")
		if r.code != http.StatusOK {
			t.Fatalf("%s dashboard = %d", who, r.code)
		}
		mustNotContain(t, who+" navigation", r.body, links...)
		mustNotContain(t, who+" navigation", r.body, en("nav.invoices"), en("nav.suppliers"))
	}
	r := e.get(t, e.syndic, "/")
	mustContain(t, "syndic navigation", r.body, links...)
	mustContain(t, "syndic navigation", r.body, en("nav.invoices"), en("nav.suppliers"))
	list := e.get(t, e.syndic, "/invoices")
	mustContain(t, "register action", list.body, `href="/invoices/new"`, en("invoices.new"))
}

// AC-2: the syndic sets the currency, which is audited; members get 403.
func TestCondominiumSettingsCurrency(t *testing.T) {
	e := newInvEnv(t)
	currency := func(condo string) string {
		t.Helper()
		var c string
		if err := e.db.QueryRow(`SELECT currency FROM condominiums WHERE id=$1`, condo).Scan(&c); err != nil {
			t.Fatalf("currency: %v", err)
		}
		return c
	}
	if currency(e.condoA) != "BRL" || currency(e.condoB) != "BRL" {
		t.Fatalf("default currency = %s / %s, want BRL / BRL", currency(e.condoA), currency(e.condoB))
	}

	page := e.get(t, e.syndic, "/condominium/settings")
	if page.code != http.StatusOK {
		t.Fatalf("settings page = %d", page.code)
	}
	mustContain(t, "settings page", page.body, en("settings.title"), `name="currency"`)

	for who, c := range map[string]*http.Client{"owner": e.owner, "tenant": e.tenant} {
		if r := e.get(t, c, "/condominium/settings"); r.code != http.StatusForbidden {
			t.Errorf("%s GET settings = %d, want 403", who, r.code)
		}
		r := e.post(t, c, "/condominium/settings", url.Values{"currency": {"EUR"}})
		if r.code != http.StatusForbidden || strings.Contains(r.body, "Invalid CSRF") {
			t.Errorf("%s POST settings = %d, want 403 from the role guard", who, r.code)
		}
	}
	if currency(e.condoA) != "BRL" {
		t.Fatalf("a member changed the currency")
	}

	r := e.post(t, e.syndic, "/condominium/settings", url.Values{"currency": {"USD"}})
	if r.code != http.StatusUnprocessableEntity || currency(e.condoA) != "BRL" {
		t.Errorf("invalid currency = %d (stored %s), want 422 and BRL", r.code, currency(e.condoA))
	}

	r = e.post(t, e.syndic, "/condominium/settings", url.Values{"currency": {"EUR"}})
	if r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=currency_changed") {
		t.Fatalf("set EUR = %d %q, want 303 flash=currency_changed", r.code, r.location())
	}
	if currency(e.condoA) != "EUR" {
		t.Fatalf("stored currency = %s, want EUR", currency(e.condoA))
	}
	if currency(e.condoB) != "BRL" {
		t.Errorf("condominium B currency = %s, want BRL (settings are per condominium)", currency(e.condoB))
	}
	if n := e.auditRows(t, "condominium_currency_changed", e.syndicAID, e.condoA); n != 1 {
		t.Errorf("currency audit rows = %d, want 1", n)
	}
	flash := e.get(t, e.syndic, r.location())
	mustContain(t, "flash", flash.body, en("settings.flash.currency_changed"))

	// Amounts follow the condominium currency; the other condominium is unaffected.
	sup := e.newSupplier(t, e.syndic, e.condoA, "Euro Lda", validNIF("50000020"))
	id := e.createInvoice(t, e.syndic, invoiceForm(sup, "EUR 1"))
	eur := money.Format(i18n.LanguageEN, money.EUR, 100000)
	mustContain(t, "EUR view", e.get(t, e.syndic, "/invoices/"+id).body, eur)
	mustContain(t, "EUR list", e.get(t, e.syndic, "/invoices").body, eur)
	mustNotContain(t, "EUR view", e.get(t, e.syndic, "/invoices/"+id).body, "R$")
}

// AC-1: existing condominiums are BRL and the finance pages keep their "R$"
// amounts; AC-2: the finance pages follow a currency change.
func TestFinanceCurrencyDefaultBRLAndChange(t *testing.T) {
	f := newFinanceEnv(t)
	var cur string
	if err := f.db.QueryRow(`SELECT currency FROM condominiums WHERE id=$1`, f.condoID).Scan(&cur); err != nil || cur != "BRL" {
		t.Fatalf("currency = %q (%v), want BRL", cur, err)
	}
	c := loginAs(t, f.base, "syndic@example.com", "syndic-pass-123")
	code, _, body := get(t, c, f.base+"/finance/overview", nil)
	if code != http.StatusOK {
		t.Fatalf("overview = %d", code)
	}
	assertExpectedTotals(t, "BRL overview", body)
	if code, _, list := get(t, c, f.base+"/finance", nil); code != http.StatusOK || !strings.Contains(list, "R$") {
		t.Errorf("finance list = %d, want 200 with R$ amounts", code)
	}

	tok := csrfFrom(t, c, f.base, "/condominium/settings")
	if code, _ := postForm(t, c, f.base, "/condominium/settings", map[string]string{"currency": "EUR", "csrf_token": tok}); code != http.StatusSeeOther {
		t.Fatalf("set EUR = %d", code)
	}
	_, _, body = get(t, c, f.base+"/finance/overview", nil)
	if got := tile(body, "Income"); got != "€1,320.50" {
		t.Errorf("EUR income tile = %q, want €1,320.50", got)
	}
	if strings.Contains(body, "R$") {
		t.Errorf("overview still shows R$ after switching to EUR")
	}
}

// AC-26: every audited action writes an audit row with event type, user and
// condominium.
func TestInvoiceAuditEvents(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Audit Lda", validNIF("50000021"))
	if r := e.post(t, e.syndic, "/suppliers/"+sup+"/edit", supplierForm("Audit Renamed Lda", validNIF("50000021"))); r.code != http.StatusSeeOther {
		t.Fatalf("edit supplier = %d", r.code)
	}

	draft := invoiceForm(sup, "AUD DRAFT")
	draft.Set("action", "draft")
	draftID := e.createInvoice(t, e.syndic, draft)
	if r := e.post(t, e.syndic, "/invoices/"+draftID+"/delete", nil); r.code != http.StatusSeeOther {
		t.Fatalf("delete draft = %d", r.code)
	}

	form := invoiceForm(sup, "AUD 1")
	id := e.createInvoice(t, e.syndic, form)
	edit := invoiceForm(sup, "AUD 1")
	edit.Set("description", "Reparacao do elevador (revista)")
	if r := e.post(t, e.syndic, "/invoices/"+id+"/edit", edit); r.code != http.StatusSeeOther {
		t.Fatalf("edit invoice = %d\n%s", r.code, r.body)
	}
	if r := e.payment(t, e.syndic, id, "100.00", ""); r.code != http.StatusSeeOther {
		t.Fatalf("payment = %d", r.code)
	}
	if r := e.upload(t, e.syndic, "/invoices/"+id+"/attachments", []upFile{{"a.pdf", pdfBytes(100)}}); r.code != http.StatusSeeOther {
		t.Fatalf("upload = %d", r.code)
	}
	aid := e.attachmentIDs(t, id)[0]
	if r := e.post(t, e.syndic, "/invoices/"+id+"/attachments/"+aid+"/delete", nil); r.code != http.StatusSeeOther {
		t.Fatalf("remove attachment = %d", r.code)
	}
	if r := e.post(t, e.syndic, "/invoices/"+id+"/annul", url.Values{"annul_reason": {"issued in error"}}); r.code != http.StatusSeeOther {
		t.Fatalf("annul = %d", r.code)
	}
	if r := e.post(t, e.syndic, "/condominium/settings", url.Values{"currency": {"EUR"}}); r.code != http.StatusSeeOther {
		t.Fatalf("currency = %d", r.code)
	}

	want := map[string]int{
		"supplier_created":             1,
		"supplier_updated":             1,
		"invoice_created":              1, // the draft
		"invoice_registered":           1,
		"invoice_updated":              1,
		"invoice_draft_deleted":        1,
		"invoice_annulled":             1,
		"invoice_payment_added":        1,
		"invoice_attachment_added":     1,
		"invoice_attachment_removed":   1,
		"condominium_currency_changed": 1,
	}
	for event, n := range want {
		if got := e.auditRows(t, event, e.syndicAID, e.condoA); got != n {
			t.Errorf("audit %s rows (user, condominium) = %d, want %d", event, got, n)
		}
		if got := e.auditRows(t, event, e.syndicBID, e.condoB); got != 0 {
			t.Errorf("audit %s rows for the other condominium = %d, want 0", event, got)
		}
	}
}

// missingTranslations records the slog warnings of the i18n fallback.
type missingTranslations struct {
	mu   *sync.Mutex
	keys *[]string
}

func (m missingTranslations) Enabled(context.Context, slog.Level) bool { return true }
func (m missingTranslations) Handle(_ context.Context, r slog.Record) error {
	if !strings.Contains(r.Message, "missing translation") {
		return nil
	}
	key := ""
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "key" {
			key = a.Value.String()
		}
		return true
	})
	m.mu.Lock()
	*m.keys = append(*m.keys, key)
	m.mu.Unlock()
	return nil
}
func (m missingTranslations) WithAttrs([]slog.Attr) slog.Handler { return m }
func (m missingTranslations) WithGroup(string) slog.Handler      { return m }

// AC-4: with pt-PT selected, every page of the module renders from the pt-PT
// catalog (no fallback to English, no raw keys), amounts use pt-PT formatting
// and pt-PT decimal input is accepted.
func TestInvoicePagesInPortuguesePT(t *testing.T) {
	e := newInvEnv(t)
	var mu sync.Mutex
	var missing []string
	prev := slog.Default()
	slog.SetDefault(slog.New(missingTranslations{mu: &mu, keys: &missing}))
	t.Cleanup(func() { slog.SetDefault(prev) })

	if r := e.post(t, e.syndic, "/profile/language", url.Values{"language": {"pt-pt"}}); r.code != http.StatusSeeOther {
		t.Fatalf("select pt-pt = %d", r.code)
	}
	pt := func(key string) string { return i18n.T(i18n.LanguagePTPT, i18n.MessageKey(key)) }
	ptMoney := func(c money.Currency, cents int64) string { return money.Format(i18n.LanguagePTPT, c, cents) }

	// Create data with pt-PT decimal input.
	if r := e.post(t, e.syndic, "/suppliers", supplierForm("Fornecedor PT Lda", validNIF("50000022"))); r.code != http.StatusSeeOther {
		t.Fatalf("create supplier = %d", r.code)
	}
	var sup string
	if err := e.db.QueryRow(`SELECT id FROM suppliers WHERE name='Fornecedor PT Lda'`).Scan(&sup); err != nil {
		t.Fatal(err)
	}
	v := invoiceForm(sup, "PT 1", func(o *invOpts) { o.baseCents = "1000,00"; o.withholding = "250,00" })
	id := e.createInvoice(t, e.syndic, v)
	var payable int64
	if err := e.db.QueryRow(`SELECT payable_cents FROM invoices WHERE id=$1`, id).Scan(&payable); err != nil || payable != 75000 {
		t.Fatalf("payable from pt-PT input = %d (%v), want 75000", payable, err)
	}

	type page struct {
		path string
		keys []string
		body string
	}
	empty := e.post(t, e.syndic, "/invoices", url.Values{"action": {"register"}, "idempotency_key": {randomKey()}})
	if empty.code != http.StatusUnprocessableEntity {
		t.Fatalf("empty registration = %d", empty.code)
	}
	pages := []page{
		{path: "/", keys: []string{"nav.invoices", "nav.suppliers", "nav.condominium_settings"}},
		{path: "/invoices", keys: []string{"invoices.title", "invoices.new", "invoices.col.number", "invoices.col.debt", "invoices.totals.title", "invoices.summary.payable"}},
		{path: "/invoices/new", keys: []string{"invoices.form.title_new", "invoices.form.register", "invoices.summary.title", "invoices.section.supplier", "invoices.help.title", "invoices.badge.legal", "invoices.field.number"}},
		{path: "/invoices/" + id, keys: []string{"invoices.view.title", "invoices.section.payments", "invoices.payments.none", "invoices.summary.vat_breakdown", "invoices.summary.paid", "invoices.status.pendente", "invoices.attachments.none", "invoices.annul.reason"}},
		{path: "/invoices/" + id + "/edit", keys: []string{"invoices.form.title_edit", "invoices.field.document_type"}},
		{path: "/suppliers", keys: []string{"suppliers.title", "nav.suppliers"}},
		{path: "/suppliers/new", keys: []string{"invoices.supplier.name", "invoices.supplier.nif"}},
		{path: "/condominium/settings", keys: []string{"settings.title", "settings.currency", "settings.currency_warning"}},
		{path: "/invoices (rejected form)", body: empty.body, keys: []string{"invoices.form.title_new", "invoices.error.no_lines"}},
	}
	for _, p := range pages {
		body := p.body
		if body == "" {
			r := e.get(t, e.syndic, p.path)
			if r.code != http.StatusOK {
				t.Errorf("%s = %d, want 200", p.path, r.code)
				continue
			}
			body = r.body
		}
		mustContain(t, p.path, body, `lang="pt-pt"`)
		for _, k := range p.keys {
			want, english := pt(k), en(k)
			if want == k {
				t.Errorf("%s: key %s has no pt-PT message", p.path, k)
				continue
			}
			if !strings.Contains(body, html.EscapeString(want)) && !strings.Contains(body, want) {
				t.Errorf("%s: pt-PT text of %s (%q) not rendered", p.path, k, want)
			}
			if len(english) >= 8 && english != want && !strings.Contains(want, english) &&
				(strings.Contains(body, html.EscapeString(english)) || strings.Contains(body, english)) {
				t.Errorf("%s: English text of %s (%q) rendered in pt-PT", p.path, k, english)
			}
		}
	}

	// pt-PT amounts: decimal comma, "R$" for BRL, then euro with NBSP.
	view := e.get(t, e.syndic, "/invoices/"+id)
	mustContain(t, "pt-PT BRL amounts", view.body, ptMoney(money.BRL, 75000), ptMoney(money.BRL, 100000))
	if r := e.post(t, e.syndic, "/condominium/settings", url.Values{"currency": {"EUR"}}); r.code != http.StatusSeeOther {
		t.Fatalf("set EUR = %d", r.code)
	}
	flash := e.get(t, e.syndic, "/condominium/settings?flash=currency_changed")
	mustContain(t, "pt-PT flash", flash.body, pt("settings.flash.currency_changed"))
	view = e.get(t, e.syndic, "/invoices/"+id)
	mustContain(t, "pt-PT EUR amounts", view.body, ptMoney(money.EUR, 75000), ptMoney(money.EUR, 100000))
	mustNotContain(t, "pt-PT EUR amounts", view.body, "R$")

	// Validation messages come from the pt-PT catalog too.
	empty = e.post(t, e.syndic, "/invoices", url.Values{"action": {"register"}, "idempotency_key": {randomKey()}})
	mustContain(t, "pt-PT validation", empty.body, html.EscapeString(pt("invoices.error.no_lines")))

	mu.Lock()
	defer mu.Unlock()
	if len(missing) > 0 {
		t.Errorf("pt-PT pages fell back to English for keys: %v", missing)
	}
}
