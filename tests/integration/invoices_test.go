package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func en(key string) string { return i18n.T(i18n.LanguageEN, i18n.MessageKey(key)) }

// tableRows returns the invoice rows of the list page.
func tableRows(body string) []string {
	re := regexp.MustCompile(`(?s)<tr>\s*<td class="font-medium text-ink-800">.*?</tr>`)
	return re.FindAllString(body, -1)
}

// summaryHas asserts that the summary shows value next to the label of key.
func summaryHas(t *testing.T, what, body, labelKey, value string) {
	t.Helper()
	re := regexp.MustCompile(`(?s)<dt class="text-ink-600">` + regexp.QuoteMeta(en(labelKey)) + `</dt>\s*<dd class="[^"]*">` + regexp.QuoteMeta(value) + `</dd>`)
	if !re.MatchString(body) {
		t.Errorf("%s: summary row %q = %q not found", what, en(labelKey), value)
	}
}

// statusBadge returns the page-subtitle block of the view page, which holds the
// derived payment status badge.
func statusBadge(t *testing.T, body string) string {
	t.Helper()
	return section(t, body, `page-subtitle`, `</p>`)
}

// AC-7: a registration with nothing filled in is rejected with an error next to
// every missing field and nothing is saved.
func TestInvoiceRegisterEmptyFormRejected(t *testing.T) {
	e := newInvEnv(t)
	r := e.post(t, e.syndic, "/invoices", url.Values{"action": {"register"}, "idempotency_key": {randomKey()}})
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("empty registration = %d, want 422", r.code)
	}
	for _, id := range []string{"number", "issue_date", "description", "category"} {
		mustContain(t, "field "+id, r.body, `id="`+id+`-error"`)
	}
	mustContain(t, "required message", r.body, en(domain.KeyRequired))
	mustContain(t, "lines message", r.body, en(domain.KeyNoLines))
	// number, issue date, supplier, description, category, lines.
	if n := strings.Count(r.body, `role="alert"`); n < 6 {
		t.Errorf("inline errors = %d, want at least 6 (one per missing field)", n)
	}
	if n := e.invoiceCount(t, e.condoA); n != 0 {
		t.Fatalf("invoices after rejected registration = %d, want 0", n)
	}
}

// AC-8: the expense type selects the conditional section and line columns, and
// values of hidden sections are not saved.
func TestInvoiceExpenseTypeSections(t *testing.T) {
	e := newInvEnv(t)

	monthly := e.get(t, e.syndic, "/invoices/form/section?expense_type=despesa_mensal", hx)
	if monthly.code != http.StatusOK {
		t.Fatalf("monthly section = %d", monthly.code)
	}
	mustContain(t, "despesa_mensal section", monthly.body, `name="meter_id"`, `name="billing_period"`, `name="cpe"`)
	mustNotContain(t, "despesa_mensal section", monthly.body, `name="service_description"`, `name="service_contract"`)

	service := e.get(t, e.syndic, "/invoices/form/section?expense_type=prestacao_servicos", hx)
	mustContain(t, "prestacao section", service.body, `name="service_description"`, `name="service_contract"`)
	mustNotContain(t, "prestacao section", service.body, `name="meter_id"`, `name="billing_period"`)

	linesFor := func(expense string) string {
		r := e.post(t, e.syndic, "/invoices/form/lines", url.Values{
			"expense_type": {expense}, "lines[0].treatment": {"rate"},
		}, hx)
		if r.code != http.StatusOK {
			t.Fatalf("lines fragment (%s) = %d", expense, r.code)
		}
		return r.body
	}
	product := linesFor("compra_produtos")
	mustContain(t, "product lines", product, `lines[0].qty`, `lines[0].unit_price`, `lines[0].discount`)
	mustNotContain(t, "product lines", product, `lines[0].base`)
	other := linesFor("manutencao")
	mustContain(t, "other lines", other, `lines[0].base`)
	mustNotContain(t, "other lines", other, `lines[0].qty`, `lines[0].unit_price`)

	sup := e.newSupplier(t, e.syndic, e.condoA, "Sections Lda", validNIF("50000001"))

	// Despesa mensal: the posted service fields belong to a hidden section.
	v := invoiceForm(sup, "SEC 1")
	v.Set("expense_type", "despesa_mensal")
	v.Set("service_description", "must not persist")
	v.Set("service_contract", "SVC-1")
	v.Set("contract_number", "C-99")
	v.Set("meter_id", "M-1")
	v.Set("recurring", "1")
	id := e.createInvoice(t, e.syndic, v)
	var svcDesc, svcContract, monthlyContract, meter string
	var recurring bool
	if err := e.db.QueryRow(`SELECT service_description, service_contract, monthly_contract_number, monthly_meter_id, monthly_recurring FROM invoices WHERE id=$1`, id).
		Scan(&svcDesc, &svcContract, &monthlyContract, &meter, &recurring); err != nil {
		t.Fatalf("read invoice: %v", err)
	}
	if svcDesc != "" || svcContract != "" {
		t.Errorf("hidden service fields saved: %q / %q", svcDesc, svcContract)
	}
	if monthlyContract != "C-99" || meter != "M-1" || !recurring {
		t.Errorf("monthly fields = %q %q %v, want C-99 M-1 true", monthlyContract, meter, recurring)
	}

	// Prestacao de servicos: the posted monthly fields are hidden.
	v = invoiceForm(sup, "SEC 2")
	v.Set("expense_type", "prestacao_servicos")
	v.Set("service_description", "Gardening")
	v.Set("contract_number", "must-not-persist")
	v.Set("meter_id", "must-not-persist")
	v.Set("recurring", "1")
	id = e.createInvoice(t, e.syndic, v)
	var monthlyContract2, meter2 string
	var recurring2 bool
	if err := e.db.QueryRow(`SELECT service_description, monthly_contract_number, monthly_meter_id, monthly_recurring FROM invoices WHERE id=$1`, id).
		Scan(&svcDesc, &monthlyContract2, &meter2, &recurring2); err != nil {
		t.Fatalf("read invoice: %v", err)
	}
	if svcDesc != "Gardening" {
		t.Errorf("service description = %q, want Gardening", svcDesc)
	}
	if monthlyContract2 != "" || meter2 != "" || recurring2 {
		t.Errorf("hidden monthly fields saved: %q %q %v", monthlyContract2, meter2, recurring2)
	}
}

// AC-13 (and AC-14 for over-payments): payments move the derived status from
// pendente to parcialmente pago to pago, and are listed.
func TestInvoicePaymentsAndStatus(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Pay Lda", validNIF("50000002"))
	// 1000.00 with 250.00 withheld: valor a pagar 750.00.
	id := e.createInvoice(t, e.syndic, invoiceForm(sup, "PAY 1", func(o *invOpts) { o.withholding = "250.00" }))

	view := e.get(t, e.syndic, "/invoices/"+id)
	if view.code != http.StatusOK {
		t.Fatalf("view = %d", view.code)
	}
	mustContain(t, "pending badge", statusBadge(t, view.body), en("invoices.status.pendente"))
	summaryHas(t, "pending", view.body, "invoices.summary.payable", fmtMoney(75000))
	summaryHas(t, "pending", view.body, "invoices.summary.debt", fmtMoney(75000))

	r := e.payment(t, e.syndic, id, "300.00", "")
	if r.code != http.StatusSeeOther {
		t.Fatalf("first payment = %d, want 303\n%s", r.code, r.body)
	}
	view = e.get(t, e.syndic, "/invoices/"+id)
	mustContain(t, "partial badge", statusBadge(t, view.body), en("invoices.status.parcialmente_pago"))
	summaryHas(t, "partial", view.body, "invoices.summary.paid", fmtMoney(30000))
	summaryHas(t, "partial", view.body, "invoices.summary.debt", fmtMoney(45000))

	r = e.payment(t, e.syndic, id, "450.00", "")
	if r.code != http.StatusSeeOther {
		t.Fatalf("second payment = %d, want 303\n%s", r.code, r.body)
	}
	view = e.get(t, e.syndic, "/invoices/"+id)
	mustContain(t, "paid badge", statusBadge(t, view.body), en("invoices.status.pago"))
	summaryHas(t, "paid", view.body, "invoices.summary.paid", fmtMoney(75000))
	summaryHas(t, "paid", view.body, "invoices.summary.debt", fmtMoney(0))
	payments := section(t, view.body, `id="payments"`, `id="attachments"`)
	mustContain(t, "payments list", payments, fmtMoney(30000), fmtMoney(45000))
	if n := e.count(t, `SELECT count(*) FROM invoice_payments WHERE invoice_id=$1`, id); n != 2 {
		t.Errorf("stored payments = %d, want 2", n)
	}

	// An over-payment needs a justification (AC-14).
	r = e.payment(t, e.syndic, id, "10.00", "")
	if r.code != http.StatusUnprocessableEntity {
		t.Errorf("over-payment without justification = %d, want 422", r.code)
	}
	r = e.payment(t, e.syndic, id, "10.00", "bank fee")
	if r.code != http.StatusSeeOther {
		t.Errorf("over-payment with justification = %d, want 303", r.code)
	}
	view = e.get(t, e.syndic, "/invoices/"+id)
	summaryHas(t, "credit", view.body, "invoices.summary.credit", fmtMoney(1000))
}

// AC-17: drafts need only a document type, stay out of the list totals, take no
// payments, free their duplicate key when deleted and are fully validated when
// registered.
func TestInvoiceDraftLifecycle(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Draft Lda", validNIF("50000003"))

	// Only the document type.
	r := e.post(t, e.syndic, "/invoices", url.Values{
		"action": {"draft"}, "idempotency_key": {randomKey()}, "document_type": {"fatura"},
	})
	if r.code != http.StatusSeeOther {
		t.Fatalf("minimal draft = %d, want 303\n%s", r.code, r.body)
	}
	minimalID := idFromLocation(t, r)
	var state string
	if err := e.db.QueryRow(`SELECT state FROM invoices WHERE id=$1`, minimalID).Scan(&state); err != nil || state != "draft" {
		t.Fatalf("minimal draft state = %q (%v), want draft", state, err)
	}
	view := e.get(t, e.syndic, "/invoices/"+minimalID)
	mustContain(t, "draft badge", statusBadge(t, view.body), en("invoices.status.rascunho"))

	// A payment on a draft is rejected.
	r = e.payment(t, e.syndic, minimalID, "10.00", "")
	if r.code != http.StatusUnprocessableEntity {
		t.Errorf("payment on draft = %d, want 422", r.code)
	}
	if n := e.count(t, `SELECT count(*) FROM invoice_payments WHERE invoice_id=$1`, minimalID); n != 0 {
		t.Errorf("payments on draft = %d, want 0", n)
	}

	// Registering the minimal draft applies the full validation.
	r = e.post(t, e.syndic, "/invoices/"+minimalID+"/edit", url.Values{"action": {"register"}, "document_type": {"fatura"}})
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("registering incomplete draft = %d, want 422", r.code)
	}
	mustContain(t, "incomplete registration", r.body, `id="number-error"`, `id="description-error"`)
	if err := e.db.QueryRow(`SELECT state FROM invoices WHERE id=$1`, minimalID).Scan(&state); err != nil || state != "draft" {
		t.Fatalf("state after rejected registration = %q (%v), want draft", state, err)
	}

	// Totals exclude drafts: a registered 1000.00 and a draft of 500.00.
	e.createInvoice(t, e.syndic, invoiceForm(sup, "REG 1"))
	draft := invoiceForm(sup, "DRAFT 1", func(o *invOpts) { o.baseCents = "500.00" })
	draft.Set("action", "draft")
	draftID := e.createInvoice(t, e.syndic, draft)
	list := e.get(t, e.syndic, "/invoices")
	footer := section(t, list.body, `data-totals-footer`, `</section>`)
	mustContain(t, "totals footer", footer, fmtMoney(100000))
	mustNotContain(t, "totals footer", footer, fmtMoney(150000), fmtMoney(50000))
	mustContain(t, "list shows the draft", list.body, ">DRAFT 1</a>")

	// The draft holds the duplicate key.
	r = e.post(t, e.syndic, "/invoices", invoiceForm(sup, "DRAFT 1"))
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("registering over a draft's key = %d, want 422", r.code)
	}

	// Deleting hides it from the list and frees the key.
	r = e.post(t, e.syndic, "/invoices/"+draftID+"/delete", nil)
	if r.code != http.StatusSeeOther || !strings.HasPrefix(r.location(), "/invoices?flash=draft_deleted") {
		t.Fatalf("delete draft = %d %q, want 303 to the list", r.code, r.location())
	}
	list = e.get(t, e.syndic, "/invoices")
	mustNotContain(t, "list after delete", list.body, ">DRAFT 1</a>")
	if got := e.get(t, e.syndic, "/invoices/"+draftID); got.code != http.StatusNotFound {
		t.Errorf("deleted draft view = %d, want 404", got.code)
	}
	r = e.post(t, e.syndic, "/invoices", invoiceForm(sup, "DRAFT 1"))
	if r.code != http.StatusSeeOther {
		t.Fatalf("registering after delete = %d, want 303 (key freed)\n%s", r.code, r.body)
	}
	registeredID := idFromLocation(t, r)

	// Only drafts can be deleted.
	r = e.post(t, e.syndic, "/invoices/"+registeredID+"/delete", nil)
	if r.code != http.StatusConflict {
		t.Errorf("delete registered invoice = %d, want 409", r.code)
	}

	// A complete form registers the minimal draft.
	full := invoiceForm(sup, "REG 2")
	r = e.post(t, e.syndic, "/invoices/"+minimalID+"/edit", full)
	if r.code != http.StatusSeeOther || !strings.Contains(r.location(), "flash=registered") {
		t.Fatalf("registering completed draft = %d %q, want 303 flash=registered\n%s", r.code, r.location(), r.body)
	}
	if err := e.db.QueryRow(`SELECT state FROM invoices WHERE id=$1`, minimalID).Scan(&state); err != nil || state != "registered" {
		t.Fatalf("state after registration = %q (%v), want registered", state, err)
	}
}

// AC-1: a draft whose lines or withholding are invalid still saves as a
// rascunho (with zero totals); registering it is rejected with the errors
// shown under the form input names.
func TestInvoiceDraftWithInvalidLinesSavesAndRegisterRejected(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Invalid Draft Lda", validNIF("50000010"))

	cases := []struct {
		name   string
		mod    func(*invOpts)
		wantKy string
	}{
		{"exempt line without code", func(o *invOpts) { o.treatment = "exempt" }, domain.KeyExemptionCodeRequired},
		{"withholding above total", func(o *invOpts) { o.withholding = "999999.00" }, domain.KeyWithholdingRange},
	}
	for i, c := range cases {
		number := "INV DRAFT " + string(rune('A'+i))
		draft := invoiceForm(sup, number, c.mod)
		draft.Set("action", "draft")
		r := e.post(t, e.syndic, "/invoices", draft)
		if r.code != http.StatusSeeOther {
			t.Fatalf("%s: draft = %d, want 303\n%s", c.name, r.code, r.body)
		}
		id := idFromLocation(t, r)
		var state string
		var total int64
		if err := e.db.QueryRow(`SELECT state, total_cents FROM invoices WHERE id=$1`, id).Scan(&state, &total); err != nil {
			t.Fatalf("%s: load draft: %v", c.name, err)
		}
		if state != "draft" || total != 0 {
			t.Errorf("%s: state=%q total=%d, want draft with zero total", c.name, state, total)
		}

		reg := invoiceForm(sup, number, c.mod)
		r = e.post(t, e.syndic, "/invoices/"+id+"/edit", reg)
		if r.code != http.StatusUnprocessableEntity {
			t.Fatalf("%s: register = %d, want 422\n%s", c.name, r.code, r.body)
		}
		mustContain(t, c.name, r.body, en(c.wantKy))
		if err := e.db.QueryRow(`SELECT state FROM invoices WHERE id=$1`, id).Scan(&state); err != nil || state != "draft" {
			t.Errorf("%s: state after rejected registration = %q (%v), want draft", c.name, state, err)
		}
	}

	// Errors sit under the form input names: a missing supplier shows under
	// supplier_id (not "supplier") and a product line's quantity under lines[i].qty.
	r := e.post(t, e.syndic, "/invoices", url.Values{"action": {"register"}, "idempotency_key": {randomKey()}})
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("empty registration = %d, want 422", r.code)
	}
	mustContain(t, "supplier_id error", r.body, en(domain.KeyRequired))
	v := invoiceForm(sup, "INV QTY")
	v.Set("expense_type", "compra_produtos")
	v.Set("lines[0].qty", "0")
	v.Set("lines[0].unit_price", "10.00")
	v.Del("lines[0].base")
	r = e.post(t, e.syndic, "/invoices", v)
	if r.code != http.StatusUnprocessableEntity {
		t.Fatalf("zero quantity = %d, want 422\n%s", r.code, r.body)
	}
	mustContain(t, "qty error", r.body, en(domain.KeyInvalidQuantity))
}

// AC-19: a duplicate (supplier, document type, number) is rejected with a link
// to the existing invoice; another supplier or document type is accepted.
func TestInvoiceDuplicateDetection(t *testing.T) {
	e := newInvEnv(t)
	x := e.newSupplier(t, e.syndic, e.condoA, "Supplier X", validNIF("50000004"))
	y := e.newSupplier(t, e.syndic, e.condoA, "Supplier Y", validNIF("50000005"))
	first := e.createInvoice(t, e.syndic, invoiceForm(x, "FT 2026/15"))

	for _, number := range []string{"FT 2026/15", "  FT   2026/15 "} {
		r := e.post(t, e.syndic, "/invoices", invoiceForm(x, number))
		if r.code != http.StatusUnprocessableEntity {
			t.Fatalf("duplicate %q = %d, want 422", number, r.code)
		}
		// AC-6: the link to the existing invoice is rendered by the form template.
		mustContain(t, "duplicate response", r.body, "data-duplicate-link", `href="/invoices/`+first+`"`, en("invoices.flash.duplicate"), en("invoices.flash.duplicate_link"))
		alert := section(t, r.body, `data-duplicate-link`, `</p>`)
		mustContain(t, "duplicate alert", alert, `href="/invoices/`+first+`"`)
	}
	if n := e.invoiceCount(t, e.condoA); n != 1 {
		t.Fatalf("invoices after duplicates = %d, want 1", n)
	}

	e.createInvoice(t, e.syndic, invoiceForm(y, "FT 2026/15"))
	e.createInvoice(t, e.syndic, invoiceForm(x, "FT 2026/15", func(o *invOpts) { o.docType = "fatura_recibo" }))
	if n := e.invoiceCount(t, e.condoA); n != 3 {
		t.Fatalf("invoices = %d, want 3", n)
	}
}

var idemRe = regexp.MustCompile(`name="idempotency_key" value="([^"]+)"`)

// AC-20: submitting twice with the same form token creates one invoice, also
// when the submits race.
func TestInvoiceIdempotentSubmit(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Idem Lda", validNIF("50000006"))

	page := e.get(t, e.syndic, "/invoices/new")
	m := idemRe.FindStringSubmatch(page.body)
	if m == nil {
		t.Fatal("no idempotency_key on the new invoice form")
	}
	v := invoiceForm(sup, "IDEM 1")
	v.Set("idempotency_key", m[1])
	first := e.post(t, e.syndic, "/invoices", v)
	second := e.post(t, e.syndic, "/invoices", v)
	if first.code != http.StatusSeeOther || second.code != http.StatusSeeOther {
		t.Fatalf("submits = %d and %d, want 303 and 303\n%s\n%s", first.code, second.code, first.body, second.body)
	}
	if idFromLocation(t, first) != idFromLocation(t, second) {
		t.Errorf("replay redirected to %q, first to %q", second.location(), first.location())
	}
	if n := e.invoiceCount(t, e.condoA); n != 1 {
		t.Fatalf("invoices after double submit = %d, want 1", n)
	}

	// Concurrent submits of one token.
	race := invoiceForm(sup, "IDEM 2")
	race.Set("csrf_token", e.csrf(t, e.syndic))
	body := race.Encode()
	const workers = 6
	codes := make([]int, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPost, e.base+"/invoices", strings.NewReader(body))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			res, err := e.syndic.Do(req)
			if err != nil {
				return
			}
			res.Body.Close()
			codes[i] = res.StatusCode
		}(i)
	}
	wg.Wait()
	for i, c := range codes {
		if c != http.StatusSeeOther {
			t.Errorf("concurrent submit %d = %d, want 303", i, c)
		}
	}
	if n := e.count(t, `SELECT count(*) FROM invoices WHERE number='IDEM 2'`); n != 1 {
		t.Fatalf("invoices after concurrent submits = %d, want 1", n)
	}
}

// AC-24 (with AC-9 and AC-11): the pre-save summary and the view show the VAT
// breakdown per rate and every total.
func TestInvoiceSummaryAndViewBreakdown(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Vat Lda", validNIF("50000007"))

	v := invoiceForm(sup, "VAT 1", func(o *invOpts) { o.withholding = "190.00" })
	v.Set("lines[0].treatment", "rate")
	v.Set("lines[0].rate", "2300")
	v.Set("lines[0].base", "600.00")
	v.Set("lines[1].description", "Materiais")
	v.Set("lines[1].treatment", "rate")
	v.Set("lines[1].rate", "1300")
	v.Set("lines[1].base", "400.00")

	// Pre-save summary: 600.00 at 23 % (138.00 VAT) and 400.00 at 13 % (52.00).
	prev := e.post(t, e.syndic, "/invoices/form/preview", v, hx)
	if prev.code != http.StatusOK {
		t.Fatalf("preview = %d", prev.code)
	}
	check := func(what, body string, paid bool) {
		t.Helper()
		mustContain(t, what+" breakdown", body, en("invoices.summary.vat_breakdown"), "23 %", "13 %",
			fmtMoney(60000), fmtMoney(13800), fmtMoney(40000), fmtMoney(5200))
		summaryHas(t, what, body, "invoices.summary.gross", fmtMoney(100000))
		summaryHas(t, what, body, "invoices.summary.discounts", fmtMoney(0))
		summaryHas(t, what, body, "invoices.summary.base", fmtMoney(100000))
		summaryHas(t, what, body, "invoices.summary.vat", fmtMoney(19000))
		summaryHas(t, what, body, "invoices.summary.total", fmtMoney(119000))
		summaryHas(t, what, body, "invoices.summary.withholding", fmtMoney(19000))
		summaryHas(t, what, body, "invoices.summary.payable", fmtMoney(100000))
		if paid {
			summaryHas(t, what, body, "invoices.summary.paid", fmtMoney(30000))
			summaryHas(t, what, body, "invoices.summary.debt", fmtMoney(70000))
		}
	}
	check("preview", prev.body, false)

	id := e.createInvoice(t, e.syndic, v)
	if r := e.payment(t, e.syndic, id, "300.00", ""); r.code != http.StatusSeeOther {
		t.Fatalf("payment = %d\n%s", r.code, r.body)
	}
	view := e.get(t, e.syndic, "/invoices/"+id)
	check("view", view.body, true)

	// Product line: 3 x 9.99 - 1.00 = 28.97 (AC-11).
	p := e.post(t, e.syndic, "/invoices/form/preview", url.Values{
		"expense_type": {"compra_produtos"}, "fiscal_region": {"continental"},
		"lines[0].qty": {"3"}, "lines[0].unit_price": {"9.99"}, "lines[0].discount": {"1.00"},
		"lines[0].treatment": {"rate"}, "lines[0].rate": {"2300"},
	}, hx)
	summaryHas(t, "product preview", p.body, "invoices.summary.gross", fmtMoney(2997))
	summaryHas(t, "product preview", p.body, "invoices.summary.discounts", fmtMoney(100))
	summaryHas(t, "product preview", p.body, "invoices.summary.base", fmtMoney(2897))
}

// AC-5: supplier search by name or NIF fragment is scoped to the condominium,
// selecting fills in the supplier and an inline create selects the new supplier.
func TestSupplierSearchSelectAndInlineCreate(t *testing.T) {
	e := newInvEnv(t)
	alphaNIF := validNIF("50000008")
	alpha := e.newSupplier(t, e.syndic, e.condoA, "Alpha Servicos Lda", alphaNIF)
	e.newSupplier(t, e.syndic, e.condoA, "Beta Limpezas SA", validNIF("60000001"))
	// Same name and a similar NIF in the other condominium must never be offered.
	otherNIF := validNIF("50000009")
	e.newSupplier(t, e.syndicB, e.condoB, "Alpha Concorrente Lda", otherNIF)

	byName := e.get(t, e.syndic, "/suppliers/search?q=alpha", hx)
	if byName.code != http.StatusOK {
		t.Fatalf("search by name = %d", byName.code)
	}
	mustContain(t, "name search", byName.body, "Alpha Servicos Lda", alpha)
	mustNotContain(t, "name search", byName.body, "Beta Limpezas", "Alpha Concorrente", otherNIF)

	byNIF := e.get(t, e.syndic, "/suppliers/search?q="+alphaNIF[:6], hx)
	mustContain(t, "NIF search", byNIF.body, "Alpha Servicos Lda")
	mustNotContain(t, "NIF search", byNIF.body, "Beta Limpezas", "Alpha Concorrente")

	none := e.get(t, e.syndic, "/suppliers/search?q=zzzz", hx)
	mustContain(t, "empty search", none.body, en("invoices.supplier.no_results"))

	// AC-11: selection goes through the dedicated route.
	sel := e.get(t, e.syndic, "/suppliers/"+alpha+"/selected", hx)
	if sel.code != http.StatusOK {
		t.Fatalf("select own supplier = %d, want 200", sel.code)
	}
	mustContain(t, "selection", sel.body, `name="supplier_id" value="`+alpha+`"`, "Alpha Servicos Lda", alphaNIF)

	// The other condominium's supplier id is not selectable: 404, nothing leaked.
	var foreign string
	if err := e.db.QueryRow(`SELECT id FROM suppliers WHERE condominium_id=$1`, e.condoB).Scan(&foreign); err != nil {
		t.Fatalf("foreign supplier: %v", err)
	}
	fsel := e.get(t, e.syndic, "/suppliers/"+foreign+"/selected", hx)
	if fsel.code != http.StatusNotFound {
		t.Fatalf("select foreign supplier = %d, want 404", fsel.code)
	}
	mustNotContain(t, "foreign selection", fsel.body, "Alpha Concorrente", otherNIF)

	// The removed select= mode of the search no longer selects anything.
	old := e.get(t, e.syndic, "/suppliers/search?select="+alpha, hx)
	mustNotContain(t, "removed select mode", old.body, `name="supplier_id"`)

	// Inline create: the supplier is stored and returned selected.
	inlineNIF := validNIF("50000010")
	r := e.post(t, e.syndic, "/suppliers/inline", url.Values{
		"supplier_name": {"Inline Lda"}, "supplier_nif": {inlineNIF}, "supplier_country": {"PT"},
		"supplier_locality": {"Porto"},
	}, hx)
	if r.code != http.StatusOK {
		t.Fatalf("inline create = %d\n%s", r.code, r.body)
	}
	var inlineID string
	if err := e.db.QueryRow(`SELECT id FROM suppliers WHERE condominium_id=$1 AND name='Inline Lda'`, e.condoA).Scan(&inlineID); err != nil {
		t.Fatalf("inline supplier not stored: %v", err)
	}
	mustContain(t, "inline selection", r.body, `name="supplier_id" value="`+inlineID+`"`, "data-supplier-selected", "Inline Lda")
	// ... and usable on an invoice.
	e.createInvoice(t, e.syndic, invoiceForm(inlineID, "INL 1"))

	before := e.count(t, `SELECT count(*) FROM suppliers`)
	bad := e.post(t, e.syndic, "/suppliers/inline", url.Values{
		"supplier_name": {"Bad NIF Lda"}, "supplier_nif": {"123456780"}, "supplier_country": {"PT"},
	}, hx)
	if bad.code != http.StatusUnprocessableEntity {
		t.Errorf("inline create with invalid NIF = %d, want 422", bad.code)
	}
	dup := e.post(t, e.syndic, "/suppliers/inline", url.Values{
		"supplier_name": {"Dup NIF Lda"}, "supplier_nif": {inlineNIF}, "supplier_country": {"PT"},
	}, hx)
	if dup.code != http.StatusUnprocessableEntity {
		t.Errorf("inline create with duplicate NIF = %d, want 422", dup.code)
	}
	if after := e.count(t, `SELECT count(*) FROM suppliers`); after != before {
		t.Errorf("suppliers after rejected inline creates = %d, want %d", after, before)
	}

	// The supplier pages: edit and the list.
	r = e.post(t, e.syndic, "/suppliers/"+inlineID+"/edit", supplierForm("Inline Renamed Lda", inlineNIF))
	if r.code != http.StatusSeeOther {
		t.Fatalf("edit supplier = %d\n%s", r.code, r.body)
	}
	list := e.get(t, e.syndic, "/suppliers")
	mustContain(t, "supplier list", list.body, "Inline Renamed Lda", "Alpha Servicos Lda")
	mustNotContain(t, "supplier list", list.body, "Alpha Concorrente")
}

// AC-23: the list filters combine, show the required columns and paginate.
func TestInvoiceListFilters(t *testing.T) {
	e := newInvEnv(t)
	nif1, nif2 := validNIF("50000011"), validNIF("50000012")
	s1 := e.newSupplier(t, e.syndic, e.condoA, "Filter One Lda", nif1)
	s2 := e.newSupplier(t, e.syndic, e.condoA, "Filter Two SA", nif2)

	mk := func(sup, number, issue, base, pay string) {
		t.Helper()
		id := e.createInvoice(t, e.syndic, invoiceForm(sup, number, func(o *invOpts) { o.issue = issue; o.baseCents = base }))
		if pay != "" {
			if r := e.payment(t, e.syndic, id, pay, ""); r.code != http.StatusSeeOther {
				t.Fatalf("payment on %s = %d\n%s", number, r.code, r.body)
			}
		}
	}
	mk(s1, "A-1", "2026-01-10", "100.00", "")       // pendente
	mk(s1, "A-2", "2026-02-10", "200.00", "50.00")  // parcialmente pago, in range
	mk(s1, "A-3", "2026-03-10", "300.00", "100.00") // parcialmente pago, out of range
	mk(s1, "A-4", "2026-02-20", "400.00", "400.00") // pago
	mk(s2, "B-1", "2026-02-15", "500.00", "10.00")  // parcialmente pago, other supplier

	q := url.Values{"nif": {nif1}, "status": {"parcialmente_pago"}, "issue_from": {"2026-02-01"}, "issue_to": {"2026-02-28"}}
	r := e.get(t, e.syndic, "/invoices?"+q.Encode())
	if r.code != http.StatusOK {
		t.Fatalf("filtered list = %d", r.code)
	}
	rows := tableRows(r.body)
	if len(rows) != 1 {
		t.Fatalf("filtered rows = %d, want 1\n%s", len(rows), r.body)
	}
	row := rows[0]
	mustContain(t, "matching row", row, ">A-2</a>", "Filter One Lda", nif1,
		fmtMoney(20000), fmtMoney(5000), fmtMoney(15000), en("invoices.status.parcialmente_pago"), en("invoices.expense_type.manutencao"))
	for _, other := range []string{"A-1", "A-3", "A-4", "B-1"} {
		mustNotContain(t, "filtered list", r.body, ">"+other+"</a>")
	}
	head := section(t, r.body, `<thead>`, `</thead>`)
	for _, col := range []string{"number", "supplier", "nif", "expense_type", "issue_date", "due_date", "total", "paid", "debt", "status"} {
		mustContain(t, "column "+col, head, ">"+en("invoices.col."+col)+"<")
	}
	footer := section(t, r.body, `data-totals-footer`, `</section>`)
	mustContain(t, "filtered totals", footer, fmtMoney(20000), fmtMoney(5000), fmtMoney(15000))

	// Each filter on its own.
	count := func(query string) int {
		t.Helper()
		return len(tableRows(e.get(t, e.syndic, "/invoices?"+query).body))
	}
	if n := count("nif=" + nif2); n != 1 {
		t.Errorf("nif filter rows = %d, want 1", n)
	}
	if n := count("status=pago"); n != 1 {
		t.Errorf("status=pago rows = %d, want 1", n)
	}
	if n := count("status=pendente"); n != 1 {
		t.Errorf("status=pendente rows = %d, want 1", n)
	}
	if n := count("supplier=two"); n != 1 {
		t.Errorf("supplier name filter rows = %d, want 1", n)
	}
	if n := count("number=A-"); n != 4 {
		t.Errorf("number filter rows = %d, want 4", n)
	}
	if n := count(""); n != 5 {
		t.Errorf("unfiltered rows = %d, want 5", n)
	}
	if n := count("issue_from=2026-02-16&issue_to=2026-03-31"); n != 2 {
		t.Errorf("issue range rows = %d, want 2 (A-4 and A-3)", n)
	}
}

func TestInvoiceListPagination(t *testing.T) {
	e := newInvEnv(t)
	sup := e.newSupplier(t, e.syndic, e.condoA, "Many Lda", validNIF("50000013"))
	if _, err := e.db.Exec(`
		INSERT INTO invoices (condominium_id, state, document_type, expense_type, number, issue_date,
			supplier_id, category, description, total_cents, payable_cents, created_by, updated_by)
		SELECT $1, 'registered', 'fatura', 'manutencao', 'P-' || lpad(g::text, 3, '0'), DATE '2026-01-01' + g,
			$2, 'manutencao', 'bulk', 10000, 10000, $3, $3
		FROM generate_series(1, 30) g`, e.condoA, sup, e.syndicAID); err != nil {
		t.Fatalf("bulk insert: %v", err)
	}

	p1 := e.get(t, e.syndic, "/invoices")
	if n := len(tableRows(p1.body)); n != domain.DefaultPageSize {
		t.Fatalf("page 1 rows = %d, want %d", n, domain.DefaultPageSize)
	}
	mustContain(t, "page 1", p1.body, fmt.Sprintf(en("pagination.page_of"), 1, 2), en("pagination.next"))
	mustNotContain(t, "page 1", p1.body, ">"+en("pagination.previous")+"<")
	// Totals cover the whole filtered set, not the page: 30 x 100.00.
	mustContain(t, "totals", section(t, p1.body, `data-totals-footer`, `</section>`), fmtMoney(300000))

	p2 := e.get(t, e.syndic, "/invoices?page=2")
	if n := len(tableRows(p2.body)); n != 30-domain.DefaultPageSize {
		t.Fatalf("page 2 rows = %d, want %d", n, 30-domain.DefaultPageSize)
	}
	mustContain(t, "page 2", p2.body, fmt.Sprintf(en("pagination.page_of"), 2, 2), en("pagination.previous"))
	mustNotContain(t, "page 2", p2.body, ">"+en("pagination.next")+"<")

	// A page past the last resolves to the last page (FR-4).
	p9 := e.get(t, e.syndic, "/invoices?page=9")
	if n := len(tableRows(p9.body)); n != 30-domain.DefaultPageSize {
		t.Fatalf("page 9 rows = %d, want %d", n, 30-domain.DefaultPageSize)
	}
	mustContain(t, "page 9", p9.body, fmt.Sprintf(en("pagination.page_of"), 2, 2))

	seen := map[string]bool{}
	for _, body := range []string{p1.body, p2.body} {
		for _, row := range tableRows(body) {
			m := regexp.MustCompile(`>(P-\d+)</a>`).FindStringSubmatch(row)
			if m == nil {
				t.Fatalf("row without number: %s", row)
			}
			if seen[m[1]] {
				t.Errorf("invoice %s listed on both pages", m[1])
			}
			seen[m[1]] = true
		}
	}
	if len(seen) != 30 {
		t.Errorf("distinct invoices over the pages = %d, want 30", len(seen))
	}
}
