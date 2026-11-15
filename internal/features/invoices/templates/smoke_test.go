package templates

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/a-h/templ"
	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
	"github.com/leoarkiteto/zelo/internal/shared/templates/testutil"
)

const ptBR = i18n.LanguagePTBR

func shellFixture() *organisms.ShellData {
	return &organisms.ShellData{
		User:   &organisms.UserView{Email: "syndic@example.com", Roles: []string{"syndic"}, Initial: "S"},
		Nav:    []organisms.NavItem{{Label: "Faturas", Path: "/invoices", Active: true}},
		CSRF:   "tok",
		Locale: ptBR,
	}
}

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func lineFixture() LineForm {
	return LineForm{Product: "Lâmpada", Qty: "2", UnitPrice: "10,00", Treatment: "rate", Rate: "2300", TotalCents: 2460, ShowTotal: true}
}

func formFixture(et domain.ExpenseType) FormData {
	return FormData{
		Shell:  shellFixture(),
		Locale: ptBR,
		CSRF:   "tok",
		Values: FormValues{IdempotencyKey: "key-123", ExpenseType: string(et)},
		Lines:  []LineForm{lineFixture()},
		Summary: SummaryData{Locale: ptBR, Totals: domain.Totals{
			GrossCents: 2000, BaseCents: 2000, VATCents: 460, TotalCents: 2460, PayableCents: 2460,
			Taxes: []domain.TaxBucket{{Treatment: domain.TreatmentRate, RateBP: 2300, BaseCents: 2000, VATCents: 460}},
		}},
	}
}

func invoiceFixture(state domain.State) domain.Invoice {
	return domain.Invoice{
		ID: "inv1", State: state, DocumentType: domain.DocumentFatura, ExpenseType: domain.ExpenseCompraProdutos,
		Number: "FT 2026/1", IssueDate: date(2026, 3, 1), DueDate: date(2026, 3, 31),
		Supplier: &domain.Supplier{ID: "s1", Name: "Eletro Lda", NIF: "501964843", Country: "PT"},
		Category: domain.CategoryEletricidade, Description: "Material elétrico", FiscalRegion: domain.RegionContinental,
		Lines: []domain.Line{{Product: "Lâmpada", QuantityMilli: 2500, UnitPriceCents: 1000, BaseCents: 2500, Treatment: domain.TreatmentRate, RateBP: 2300}},
		Totals: domain.Totals{BaseCents: 2500, VATCents: 575, TotalCents: 3075, PayableCents: 3075,
			Taxes: []domain.TaxBucket{{Treatment: domain.TreatmentRate, RateBP: 2300, BaseCents: 2500, VATCents: 575}}},
		Payments:    []domain.Payment{{ID: "p1", Date: *date(2026, 3, 10), AmountCents: 1000, Method: domain.MethodMBWay}},
		Attachments: []domain.Attachment{{ID: "a1", InvoiceID: "inv1", OriginalName: "fatura.pdf", SizeBytes: 2048}},
	}
}

func listFixture() ListPageData {
	return ListPageData{
		ListResult: domain.ListResult{
			Rows: []domain.ListRow{{
				ID: "inv1", Number: "FT 2026/1", SupplierName: "Eletro Lda", SupplierNIF: "501964843",
				ExpenseType: domain.ExpenseCompraProdutos, IssueDate: date(2026, 3, 1), DueDate: date(2026, 3, 31),
				TotalCents: 123456, PayableCents: 123456, PaidCents: 1000, DebtCents: 122456,
				State: domain.StateRegistered, Status: domain.StatusParcialmentePago,
			}, {
				ID: "inv2", Number: "FT 2026/2", SupplierName: "Anulada SA", State: domain.StateAnnulled, Status: domain.StatusAnulado,
			}},
			RowCount: 60, Page: 2, PageSize: 25,
			Totals: domain.ListTotals{TotalCents: 123456, PayableCents: 123456, PaidCents: 1000, DebtCents: 122456, Count: 1},
		},
		Shell: shellFixture(), Locale: ptBR, CSRF: "tok",
		Filter: domain.ListFilter{SupplierName: "Eletro"},
	}
}

func allPages() map[string]templ.Component {
	view := ViewPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Invoice: invoiceFixture(domain.StateRegistered)}
	return map[string]templ.Component{
		"list":          ListPage(listFixture()),
		"form-new":      FormPage(formFixture(domain.ExpenseCompraProdutos)),
		"form-edit":     FormPage(FormData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Edit: true, InvoiceID: "inv1", State: domain.StateRegistered, Values: FormValues{ExpenseType: "manutencao"}, Payments: invoiceFixture(domain.StateRegistered).Payments, Attachments: invoiceFixture(domain.StateRegistered).Attachments}),
		"view":          ViewPage(view),
		"suppliers":     SupplierListPage(SupplierListData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Suppliers: []domain.Supplier{{ID: "s1", Name: "Eletro Lda", NIF: "501964843", Country: "PT"}}}),
		"supplier-form": SupplierFormPage(SupplierPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Values: SupplierValues{Country: "PT"}}),
	}
}

func TestPagesRenderInEveryLanguage(t *testing.T) {
	for _, lang := range allLanguages {
		d := formFixture(domain.ExpenseDespesaMensal)
		d.Locale = lang
		d.Summary.Locale = lang
		out := testutil.RenderToString(t, FormPage(d))
		if !strings.Contains(out, "invoice-form") {
			t.Errorf("%s: form did not render", lang)
		}
		for _, key := range []string{"invoices.section.supplier", "invoices.help.not_certified", "invoices.expense_section.monthly"} {
			if !strings.Contains(out, templ.EscapeString(i18n.T(lang, i18n.MessageKey(key)))) {
				t.Errorf("%s: %s not rendered", lang, key)
			}
		}
	}
}

func TestNoScriptNoAlpineOnAnyPage(t *testing.T) {
	for name, page := range allPages() {
		out := testutil.RenderToString(t, page)
		// The layout loads vendored scripts with <script src>; the views
		// themselves must add none and no inline script.
		if strings.Contains(out, "<script>") || strings.Contains(out, "<script type") {
			t.Errorf("%s: inline script found", name)
		}
		if strings.Contains(out, "x-data") || strings.Contains(out, "x-on:") || strings.Contains(out, "@click") {
			t.Errorf("%s: Alpine directive found", name)
		}
	}
	for name, frag := range map[string]templ.Component{
		"lines":    LinesFragment(LinesFragmentData{Locale: ptBR, ExpenseType: domain.ExpenseManutencao, Lines: []LineForm{{}}}),
		"section":  ExpenseSection(ExpenseSectionData{Locale: ptBR, ExpenseType: domain.ExpenseDespesaMensal}),
		"summary":  SummaryFragment(SummaryData{Locale: ptBR}),
		"search":   SupplierSearchResults(SupplierSearchData{Locale: ptBR}),
		"selected": SupplierSelected(SupplierSelectedData{Locale: ptBR}),
	} {
		if out := testutil.RenderToString(t, frag); strings.Contains(out, "<script") {
			t.Errorf("fragment %s: script tag found", name)
		}
	}
}

func TestConditionalSectionSwitchesByExpenseType(t *testing.T) {
	render := func(et domain.ExpenseType) string {
		return testutil.RenderToString(t, ExpenseSection(ExpenseSectionData{Locale: ptBR, ExpenseType: et}))
	}
	svc := render(domain.ExpensePrestacaoServicos)
	for _, want := range []string{`data-section="prestacao_servicos"`, `name="service_description"`, `name="service_start"`, `name="service_end"`, `name="service_contract"`} {
		if !strings.Contains(svc, want) {
			t.Errorf("prestacao_servicos section missing %q", want)
		}
	}
	if strings.Contains(svc, "meter_id") {
		t.Error("prestacao_servicos section shows monthly fields")
	}
	mon := render(domain.ExpenseDespesaMensal)
	for _, want := range []string{`data-section="despesa_mensal"`, `name="monthly_service_type"`, `name="billing_period"`, `name="contract_number"`, `name="meter_id"`, `name="cpe"`, `name="cui"`, `name="consumption_place"`, `name="recurring"`} {
		if !strings.Contains(mon, want) {
			t.Errorf("despesa_mensal section missing %q", want)
		}
	}
	if strings.Contains(mon, "service_description") {
		t.Error("despesa_mensal section shows service fields")
	}
	for _, et := range []domain.ExpenseType{domain.ExpenseCompraProdutos, domain.ExpenseManutencao, domain.ExpenseOutraDespesa, domain.ExpenseDespesaAdministrativa} {
		if out := render(et); strings.Contains(out, "data-section") {
			t.Errorf("%s renders a conditional section", et)
		}
	}
	// The expense type select drives the swap with HTMX, no page reload.
	page := testutil.RenderToString(t, FormPage(formFixture(domain.ExpensePrestacaoServicos)))
	for _, want := range []string{`hx-get="/invoices/form/section"`, `hx-target="#expense-section"`, `data-section="prestacao_servicos"`} {
		if !strings.Contains(page, want) {
			t.Errorf("form page missing %q", want)
		}
	}
}

func TestProductColumnsOnlyForCompraProdutos(t *testing.T) {
	render := func(et domain.ExpenseType) string {
		return testutil.RenderToString(t, LinesFragment(LinesFragmentData{
			Locale: ptBR, ExpenseType: et, Region: domain.RegionContinental, Lines: []LineForm{lineFixture(), {}},
		}))
	}
	product := render(domain.ExpenseCompraProdutos)
	for _, f := range []string{"product", "reference", "qty", "unit", "unit_price", "discount", "treatment", "rate"} {
		if !strings.Contains(product, `name="lines[0].`+f+`"`) || !strings.Contains(product, `name="lines[1].`+f+`"`) {
			t.Errorf("compra_produtos lines missing indexed field %q", f)
		}
	}
	if strings.Contains(product, `lines[0].base`) || strings.Contains(product, `lines[0].description`) {
		t.Error("compra_produtos shows the generic columns")
	}
	other := render(domain.ExpenseManutencao)
	for _, f := range []string{"description", "base", "treatment", "rate"} {
		if !strings.Contains(other, `name="lines[0].`+f+`"`) {
			t.Errorf("manutencao lines missing %q", f)
		}
	}
	for _, f := range []string{"product", "reference", "qty", "unit_price", "discount"} {
		if strings.Contains(other, `lines[0].`+f+`"`) {
			t.Errorf("manutencao lines show product column %q", f)
		}
	}
	// Add and remove are HTMX posts of the whole form.
	for _, want := range []string{`hx-post="/invoices/form/lines"`, `&#34;op&#34;:&#34;add&#34;`, `&#34;op&#34;:&#34;remove&#34;`} {
		if !strings.Contains(product, want) && !strings.Contains(product, strings.ReplaceAll(want, "&#34;", `"`)) {
			t.Errorf("lines fragment missing %q", want)
		}
	}
	if !strings.Contains(product, `hx-include="#invoice-form"`) {
		t.Error("lines fragment does not include the form")
	}
}

func TestExemptTreatmentShowsCodeAndReason(t *testing.T) {
	ln := LineForm{Description: "x", Base: "10,00", Treatment: "exempt", ExemptionCode: "M07"}
	out := testutil.RenderToString(t, LinesFragment(LinesFragmentData{Locale: ptBR, ExpenseType: domain.ExpenseManutencao, Lines: []LineForm{ln}}))
	for _, want := range []string{`lines[0].exemption_code`, `lines[0].exemption_reason`, "M07 — "} {
		if !strings.Contains(out, want) {
			t.Errorf("exempt line missing %q", want)
		}
	}
	if strings.Contains(out, `name="lines[0].rate"`) {
		t.Error("exempt line still offers a VAT rate")
	}
}

func TestNoNotaCreditoOption(t *testing.T) {
	for name, out := range map[string]string{
		"form": testutil.RenderToString(t, FormPage(formFixture(domain.ExpenseCompraProdutos))),
		"list": testutil.RenderToString(t, ListPage(listFixture())),
	} {
		if strings.Contains(out, `value="nota_credito"`) {
			t.Errorf("%s offers nota_credito", name)
		}
		if !strings.Contains(out, `value="fatura_recibo"`) || !strings.Contains(out, `value="nota_debito"`) {
			t.Errorf("%s misses a registrable document type", name)
		}
	}
}

func TestFormRequiredMarkersAndHiddenFields(t *testing.T) {
	out := testutil.RenderToString(t, FormPage(formFixture(domain.ExpenseCompraProdutos)))
	if n := strings.Count(out, `aria-hidden="true">*</span>`); n < 6 {
		t.Errorf("only %d required markers", n)
	}
	for _, id := range []string{"document_type", "number", "issue_date", "expense_type", "category", "description"} {
		if !strings.Contains(out, `id="`+id+`" name="`+id+`"`) {
			t.Errorf("field %s missing", id)
		}
	}
	if !strings.Contains(out, " required") {
		t.Error("no input carries the required attribute")
	}
	for _, want := range []string{
		`name="idempotency_key" value="key-123"`,
		`name="csrf_token" value="tok"`,
		`name="due_justification"`,
		`name="action" value="draft" formnovalidate`,
		`name="action" value="register"`,
		`hx-post="/invoices/form/preview"`,
		`hx-get="/suppliers/search"`,
		`id="summary"`,
		`name="withholding"`,
		`name="fiscal_region"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("form missing %q", want)
		}
	}
	// Edit form carries no idempotency key.
	edit := testutil.RenderToString(t, FormPage(FormData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Edit: true, InvoiceID: "i", State: domain.StateRegistered}))
	if strings.Contains(edit, "idempotency_key") {
		t.Error("edit form has an idempotency key")
	}
}

func TestFormSectionsAndLegalHelp(t *testing.T) {
	out := testutil.RenderToString(t, FormPage(formFixture(domain.ExpenseCompraProdutos)))
	for _, want := range []string{"Identificação", "Fornecedor", "Despesa", "Valores e impostos", "Pagamentos", "Anexos", "artigo 36.º", "artigo 40.º", "certificado", "No documento original", "Gestão interna"} {
		if !strings.Contains(out, want) {
			t.Errorf("form missing %q", want)
		}
	}
}

func TestFieldErrorsRenderInline(t *testing.T) {
	d := formFixture(domain.ExpenseCompraProdutos)
	d.Errors = domain.FieldErrors{"number": domain.KeyRequired, "lines[0].qty": domain.KeyInvalidQuantity, "supplier_id": domain.KeyRequired}
	out := testutil.RenderToString(t, FormPage(d))
	for _, want := range []string{i18n.T(ptBR, domain.KeyRequired), i18n.T(ptBR, domain.KeyInvalidQuantity), `id="number-error"`} {
		if !strings.Contains(out, want) {
			t.Errorf("inline error missing %q", want)
		}
	}
}

func TestSummaryShowsVATBreakdownAndTotals(t *testing.T) {
	out := testutil.RenderToString(t, SummaryFragment(formFixture(domain.ExpenseCompraProdutos).Summary))
	for _, want := range []string{"Discriminação do IVA", "23 %", "R$ 20,00", "R$ 4,60", "R$ 24,60", "Valor a pagar", "Retenção na fonte"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q", want)
		}
	}
	if strings.Contains(out, "Valor em dívida") {
		t.Error("form summary shows paid and debt")
	}
}

func TestListShowsColumnsFiltersAndTotalsFooter(t *testing.T) {
	out := testutil.RenderToString(t, ListPage(listFixture()))
	for _, want := range []string{
		"Número", "Fornecedor", "NIF", "Tipo de despesa", "Data de emissão", "Vencimento", "Total", "Pago", "Em dívida", "Situação",
		"FT 2026/1", "501964843", "R$ 1.234,56", "Parcialmente pago", "Anulado",
		`data-totals-footer`, "Totais das faturas filtradas", "excluem rascunhos",
		`name="supplier" value="Eletro"`, `name="status"`, `name="issue_from"`, `name="due_to"`,
		"Página 2 de 3", "Anterior", "Próxima",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("list missing %q", want)
		}
	}
	if !strings.Contains(out, `href="/invoices?page=1&amp;supplier=Eletro"`) && !strings.Contains(out, `supplier=Eletro&amp;page=1`) && !strings.Contains(out, "page=3") {
		t.Error("pagination links do not keep the filters")
	}
	empty := listFixture()
	empty.Rows = nil
	if out := testutil.RenderToString(t, ListPage(empty)); !strings.Contains(out, "Nenhuma fatura encontrada") || !strings.Contains(out, "data-totals-footer") {
		t.Error("empty list should show the empty state and the totals footer")
	}
}

func TestViewPageSections(t *testing.T) {
	d := ViewPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Invoice: invoiceFixture(domain.StateRegistered)}
	out := testutil.RenderToString(t, ViewPage(d))
	for _, want := range []string{
		"FT 2026/1", "Eletro Lda", "501964843", "Discriminação do IVA", "R$ 30,75", "2.5",
		"Pagamentos", "MB Way", "R$ 10,00", "Valor em dívida", "R$ 20,75",
		"fatura.pdf", `/invoices/inv1/attachments/a1`, `/invoices/inv1/attachments/a1/delete`,
		`name="files"`, `enctype="multipart/form-data"`,
		`action="/invoices/inv1/payments"`, `name="payment_justification"`, `name="payment_amount"`,
		`action="/invoices/inv1/annul"`, `name="annul_reason"`, "Parcialmente pago",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("view missing %q", want)
		}
	}

	draft := ViewPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Invoice: invoiceFixture(domain.StateDraft)}
	out = testutil.RenderToString(t, ViewPage(draft))
	if strings.Contains(out, "/payments") || strings.Contains(out, "/annul") {
		t.Error("a draft offers payments or annulment")
	}
	if !strings.Contains(out, "/invoices/inv1/delete") || !strings.Contains(out, "Rascunho") {
		t.Error("a draft should offer deletion and show its status")
	}

	annulled := invoiceFixture(domain.StateAnnulled)
	annulled.AnnulReason = "Emitida por engano"
	out = testutil.RenderToString(t, ViewPage(ViewPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Invoice: annulled}))
	for _, want := range []string{"data-annulled", "Emitida por engano", "Anulado"} {
		if !strings.Contains(out, want) {
			t.Errorf("annulled view missing %q", want)
		}
	}
	for _, bad := range []string{"/payments", "/annul\"", "/edit", `name="files"`} {
		if strings.Contains(out, bad) {
			t.Errorf("annulled view offers %q", bad)
		}
	}
}

func TestViewShowsCreditWhenOverpaid(t *testing.T) {
	inv := invoiceFixture(domain.StateRegistered)
	inv.Payments = []domain.Payment{{ID: "p", Date: *date(2026, 3, 10), AmountCents: 4000, Method: domain.MethodOutro, Justification: "Adiantamento"}}
	out := testutil.RenderToString(t, ViewPage(ViewPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Invoice: inv}))
	if !strings.Contains(out, "Crédito a favor") || !strings.Contains(out, "Adiantamento") {
		t.Error("overpaid invoice should show the credit and the justification")
	}
}

func TestViewPageConditionalDetails(t *testing.T) {
	inv := invoiceFixture(domain.StateRegistered)
	inv.ExpenseType = domain.ExpenseDespesaMensal
	inv.Monthly = domain.MonthlyDetails{MeterID: "MTR-9", CPE: "PT0002000123456789XX"}
	inv.Lines = []domain.Line{{Description: "Consumo", BaseCents: 1000, Treatment: domain.TreatmentRate, RateBP: 600}}
	out := testutil.RenderToString(t, ViewPage(ViewPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Invoice: inv}))
	if !strings.Contains(out, "MTR-9") || !strings.Contains(out, "PT0002000123456789XX") {
		t.Error("monthly details not shown")
	}
	if strings.Contains(out, ">Quantidade<") {
		t.Error("non-product invoice shows the quantity column")
	}
}

func TestSupplierFragments(t *testing.T) {
	results := testutil.RenderToString(t, SupplierSearchResults(SupplierSearchData{Locale: ptBR, Query: "ele", Results: []domain.Supplier{{ID: "s1", Name: "Eletro Lda", NIF: "501964843"}}}))
	for _, want := range []string{"Eletro Lda", "501964843", `hx-get="/suppliers/search"`, `hx-target="#supplier-selected"`, "s1"} {
		if !strings.Contains(results, want) {
			t.Errorf("search results missing %q", want)
		}
	}
	if none := testutil.RenderToString(t, SupplierSearchResults(SupplierSearchData{Locale: ptBR})); !strings.Contains(none, "Nenhum fornecedor encontrado") {
		t.Error("no-results text missing")
	}
	selected := testutil.RenderToString(t, SupplierSelected(SupplierSelectedData{Locale: ptBR, Supplier: SupplierValues{ID: "s1", Name: "Eletro Lda", NIF: "501964843", Country: "PT"}}))
	if !strings.Contains(selected, `name="supplier_id" value="s1"`) || !strings.Contains(selected, "Eletro Lda") {
		t.Error("selected supplier fragment incomplete")
	}
	unselected := testutil.RenderToString(t, SupplierSelected(SupplierSelectedData{Locale: ptBR, Errors: domain.FieldErrors{"supplier_id": domain.KeyRequired}}))
	if !strings.Contains(unselected, `name="supplier_id" value=""`) || !strings.Contains(unselected, i18n.T(ptBR, domain.KeyRequired)) {
		t.Error("unselected supplier fragment should carry an empty id and the error")
	}
}

func TestSupplierPages(t *testing.T) {
	list := testutil.RenderToString(t, SupplierListPage(SupplierListData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Suppliers: []domain.Supplier{{ID: "s1", Name: "Eletro Lda", NIF: "501964843"}}}))
	for _, want := range []string{"Eletro Lda", "/suppliers/new", "/suppliers/s1/edit"} {
		if !strings.Contains(list, want) {
			t.Errorf("supplier list missing %q", want)
		}
	}
	form := testutil.RenderToString(t, SupplierFormPage(SupplierPageData{Shell: shellFixture(), Locale: ptBR, CSRF: "tok", Edit: true,
		Values: SupplierValues{ID: "s1", Name: "Eletro", NIF: "1"}, Errors: domain.FieldErrors{"nif": domain.KeyInvalidNIF}}))
	for _, want := range []string{`action="/suppliers/s1/edit"`, `name="name"`, `name="nif"`, `name="country"`, "required", i18n.T(ptBR, domain.KeyInvalidNIF)} {
		if !strings.Contains(form, want) {
			t.Errorf("supplier form missing %q", want)
		}
	}
}

func renderCtx(t *testing.T, ctx context.Context, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(ctx, &buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

// TestMoneyUsesCondominiumCurrency checks the amounts follow the currency of
// the request context, not a hard-coded symbol.
func TestMoneyUsesCondominiumCurrency(t *testing.T) {
	d := listFixture()
	ctx := money.WithCurrency(context.Background(), money.EUR)
	d.Locale = i18n.LanguagePTPT
	out := renderCtx(t, ctx, ListPage(d))
	if !strings.Contains(out, money.Format(i18n.LanguagePTPT, money.EUR, 123456)) {
		t.Error("EUR amount in pt-pt not rendered with the money formatter")
	}
	if strings.Contains(out, "R$") {
		t.Error("BRL symbol rendered for an EUR condominium")
	}
	en := listFixture()
	en.Locale = i18n.LanguageEN
	out = renderCtx(t, ctx, ListPage(en))
	if !strings.Contains(out, "€1,234.56") {
		t.Error("EUR amount in en not rendered")
	}
}
