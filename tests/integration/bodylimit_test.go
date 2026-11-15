package integration

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NFR-8: the body limits of cmd/web/main.go are part of the composed harness. A
// 2 MB finance receipt (above the 1 MB default) still uploads end to end, and
// the limits still bite where they should.
func TestFinanceReceiptUploadWithinBodyLimit(t *testing.T) {
	e := newInvEnv(t)
	fields := map[string]string{
		"type": "payable", "title": "Elevator repair", "category": "maintenance",
		"amount": "120.00", "due_date": "2026-12-31", "supplier_payee": "Acme Lda",
	}
	receipt := pdfBytes(2 << 20)

	r := e.multipart(t, e.syndic, "/finance", fields, "receipt", []upFile{{"receipt.pdf", receipt}})
	if r.code != http.StatusSeeOther || !strings.HasPrefix(r.location(), "/finance?flash=created") {
		t.Fatalf("2 MB receipt upload = %d %q, want 303 to /finance?flash=created\n%.400s", r.code, r.location(), r.body)
	}
	var id, path, name string
	if err := e.db.QueryRow(`SELECT id, receipt_path, receipt_name FROM finance_accounts WHERE title='Elevator repair'`).Scan(&id, &path, &name); err != nil {
		t.Fatalf("stored account: %v", err)
	}
	if name != "receipt.pdf" {
		t.Errorf("receipt name = %q, want receipt.pdf", name)
	}
	if st, err := os.Stat(filepath.Join(e.uploadDir, path)); err != nil || st.Size() != 2<<20 {
		t.Errorf("stored receipt = %v (%v), want 2 MB on disk", st, err)
	}
	d := e.get(t, e.syndic, "/finance/"+id+"/receipt")
	if d.code != http.StatusOK || len(d.body) != 2<<20 {
		t.Errorf("receipt download = %d with %d bytes, want 200 with 2 MB", d.code, len(d.body))
	}

	// Above the finance limit (6 MB) the request is refused.
	big := map[string]string{
		"type": "payable", "title": "Too big", "category": "maintenance",
		"amount": "1.00", "due_date": "2026-12-31", "supplier_payee": "Acme Lda",
	}
	r = e.multipart(t, e.syndic, "/finance", big, "receipt", []upFile{{"big.pdf", pdfBytes(7 << 20)}})
	if r.code != http.StatusRequestEntityTooLarge {
		t.Errorf("7 MB receipt = %d, want 413", r.code)
	}
	if n := e.count(t, `SELECT count(*) FROM finance_accounts WHERE title='Too big'`); n != 0 {
		t.Errorf("oversized receipt saved an account")
	}

	// A plain form is capped at 1 MB (including the invoice form).
	sup := e.newSupplier(t, e.syndic, e.condoA, "Limit Lda", validNIF("50000023"))
	v := invoiceForm(sup, "LIM 1")
	v.Set("notes", strings.Repeat("x", 2<<20))
	if r := e.post(t, e.syndic, "/invoices", v); r.code != http.StatusRequestEntityTooLarge {
		t.Errorf("2 MB invoice form = %d, want 413", r.code)
	}
}
