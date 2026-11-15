// Package templates holds the Templ views of the invoices feature: the invoice
// list, form, view, the supplier pages and the HTMX fragments they swap.
//
// # Form contract for the handlers (T9)
//
// Everything is posted as an ordinary url-encoded form (the invoice form is
// url-encoded; only the attachment upload form is multipart). Money and
// quantity inputs are free text and are parsed on the server with
// money.ParseCents and domain.ParseQuantityMilli. Date inputs are YYYY-MM-DD.
//
// Header fields of the invoice form (names are the field keys used in
// domain.FieldErrors):
//
//	csrf_token, idempotency_key (create only), action (draft | register),
//	document_type, number, issue_date, due_date, operation_date,
//	due_justification, supplier_id, expense_type, category, description,
//	project, cost_center, reference_period (YYYY-MM), notes, fiscal_region,
//	withholding
//
// Conditional fields of an expense type (all optional, FR-10):
//
//	prestacao_servicos: service_description, service_start, service_end,
//	                    service_contract
//	despesa_mensal:     monthly_service_type, billing_period, monthly_start,
//	                    monthly_end, contract_number, meter_id, cpe, cui,
//	                    consumption_place, recurring ("1" when checked)
//
// Lines are indexed, starting at 0 and without gaps:
//
//	lines[0].description  lines[0].product    lines[0].reference
//	lines[0].unit         lines[0].qty        lines[0].unit_price
//	lines[0].discount     lines[0].base       lines[0].treatment
//	lines[0].rate         lines[0].exemption_code lines[0].exemption_reason
//
// "product", "reference", "qty", "unit", "unit_price" and "discount" are
// rendered only when the expense type is compra_produtos; "description" and
// "base" are rendered for every other type. "treatment" is a
// domain.VATTreatment, "rate" is the rate in basis points (2300 = 23 %) and
// is only meaningful for the "rate" treatment. FieldErrors for a line field
// use the key "lines[2].rate".
//
// HTMX routes (see plan.md):
//
//	GET  /invoices/form/section?expense_type=...  -> ExpenseSection fragment
//	POST /invoices/form/lines   (whole form + op=add|remove|"" + index=N)
//	                             -> LinesFragment (op "" just re-renders, used
//	                             when the expense type or a treatment changes)
//	POST /invoices/form/preview (whole form) -> SummaryFragment
//	GET  /suppliers/search?q=... -> SupplierSearchResults fragment
//	GET  /suppliers/search?select=<id> -> SupplierSelected fragment
//	POST /suppliers/inline (supplier_* fields) -> SupplierSelected fragment
//
// Supplier fields used by the inline create: supplier_name, supplier_nif,
// supplier_country, supplier_address, supplier_postal_code,
// supplier_locality, supplier_email, supplier_phone. Their FieldErrors keys
// are the same names.
//
// Payment form on the view page (POST /invoices/{id}/payments):
// payment_date, payment_amount, payment_method, payment_reference,
// payment_bank_account, payment_justification. Annul form (POST
// /invoices/{id}/annul): annul_reason. Attachment upload (multipart, POST
// /invoices/{id}/attachments): the "files" field, multiple.
package templates

import (
	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
)

// FormValues are the raw, string-typed header values of the invoice form, so a
// failed submit re-renders exactly what the user typed.
type FormValues struct {
	IdempotencyKey string

	DocumentType     string
	Number           string
	IssueDate        string
	DueDate          string
	OperationDate    string
	DueJustification string

	SupplierID string

	ExpenseType     string
	Category        string
	Description     string
	Project         string
	CostCenter      string
	ReferencePeriod string
	Notes           string

	FiscalRegion string
	Withholding  string

	// Prestação de serviços.
	ServiceDescription string
	ServiceStart       string
	ServiceEnd         string
	ServiceContract    string

	// Despesa mensal.
	MonthlyServiceType string
	BillingPeriod      string
	MonthlyStart       string
	MonthlyEnd         string
	ContractNumber     string
	MeterID            string
	CPE                string
	CUI                string
	ConsumptionPlace   string
	Recurring          bool
}

// LineForm is one raw line of the form. Rate is in basis points as text.
type LineForm struct {
	Description, Product, Reference, Unit string
	Qty, UnitPrice, Discount, Base        string
	Treatment                             string
	Rate                                  string
	ExemptionCode, ExemptionReason        string
	// TotalCents is the server-computed line total (base + VAT); it is shown
	// only when ShowTotal is true.
	TotalCents int64
	ShowTotal  bool
}

// SummaryData backs the financial summary (FR-29, FR-30). On the form it is
// computed by the server from the current lines; on the view page ShowPaid is
// set and PaidCents/DebtCents are filled in.
type SummaryData struct {
	Locale i18n.Language
	Totals domain.Totals
	// Error is a message shown instead of the figures when the lines cannot
	// be computed yet (already translated).
	Error string

	ShowPaid  bool
	PaidCents int64
	DebtCents int64 // negative = credit in favour (shown as such)
}

// LinesFragmentData backs the line table fragment.
type LinesFragmentData struct {
	Locale      i18n.Language
	ExpenseType domain.ExpenseType
	Region      domain.FiscalRegion
	Lines       []LineForm
	Errors      domain.FieldErrors
}

// ExpenseSectionData backs the conditional section of an expense type.
type ExpenseSectionData struct {
	Locale      i18n.Language
	ExpenseType domain.ExpenseType
	Values      FormValues
	Errors      domain.FieldErrors
}

// SupplierValues are the raw values of a supplier.
type SupplierValues struct {
	ID         string
	Name       string
	NIF        string
	Country    string
	Address    string
	PostalCode string
	Locality   string
	Email      string
	Phone      string
}

// SupplierSelectedData backs the selected-supplier fragment of the invoice
// form: a hidden supplier_id plus a read-only summary of the supplier.
type SupplierSelectedData struct {
	Locale   i18n.Language
	Supplier SupplierValues // empty ID = no supplier selected
	Errors   domain.FieldErrors
}

// SupplierSearchData backs the supplier search results fragment.
type SupplierSearchData struct {
	Locale  i18n.Language
	Query   string
	Results []domain.Supplier
}

// FormData backs the invoice create and edit page.
type FormData struct {
	Shell  *organisms.ShellData
	Locale i18n.Language
	CSRF   string

	Edit      bool
	InvoiceID string
	State     domain.State // edit only

	Values   FormValues
	Supplier SupplierValues
	Lines    []LineForm
	Summary  SummaryData

	// Errors holds the inline field errors (i18n keys); Error a page-level
	// message (already translated).
	Errors domain.FieldErrors
	Error  string

	// Edit only: shown read-only in the Pagamentos and Anexos sections.
	Payments    []domain.Payment
	Attachments []domain.Attachment
}

// ListPageData backs the invoice list. It embeds domain.ListResult; its
// Totals are shown in the footer.
type ListPageData struct {
	domain.ListResult
	Shell  *organisms.ShellData
	Locale i18n.Language
	CSRF   string
	Filter domain.ListFilter
	Flash  string
	Error  string
}

// PaymentValues are the raw values of the payment form.
type PaymentValues struct {
	Date          string
	Amount        string
	Method        string
	Reference     string
	BankAccount   string
	Justification string
}

// ViewPageData backs the invoice view page.
type ViewPageData struct {
	Shell   *organisms.ShellData
	Locale  i18n.Language
	CSRF    string
	Invoice domain.Invoice
	Flash   string
	Error   string

	Payment       PaymentValues
	PaymentErrors domain.FieldErrors
	AnnulReason   string
	AnnulError    string // i18n key or ""
	AttachError   string // already translated
}

// SupplierListData backs the supplier list page.
type SupplierListData struct {
	Shell     *organisms.ShellData
	Locale    i18n.Language
	CSRF      string
	Query     string
	Suppliers []domain.Supplier
	Flash     string
}

// SupplierPageData backs the supplier create and edit form.
type SupplierPageData struct {
	Shell  *organisms.ShellData
	Locale i18n.Language
	CSRF   string
	Edit   bool
	Values SupplierValues
	Errors domain.FieldErrors
	Error  string
}
