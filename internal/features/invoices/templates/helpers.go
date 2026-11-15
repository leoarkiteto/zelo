package templates

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	atoms "github.com/leoarkiteto/zelo/internal/shared/templates/atoms"
)

// Option is one <option> of a select.
type Option struct {
	Value string
	Label string
}

func tr(l i18n.Language, key string) string { return i18n.T(l, i18n.MessageKey(key)) }

// fieldError returns the translated inline error of a field, or "".
func fieldError(errs domain.FieldErrors, l i18n.Language, field string) string {
	if key, ok := errs[field]; ok {
		return tr(l, key)
	}
	return ""
}

func lineName(i int, field string) string { return fmt.Sprintf("lines[%d].%s", i, field) }

func lineID(i int, field string) string { return fmt.Sprintf("line-%d-%s", i, field) }

func isProduct(et domain.ExpenseType) bool { return et == domain.ExpenseCompraProdutos }

func dateValue(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func dateText(l i18n.Language, t *time.Time) string {
	if t == nil || t.IsZero() {
		return "—"
	}
	return i18n.FormatDate(l, *t)
}

func timeText(l i18n.Language, t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return i18n.FormatDate(l, t)
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// pctLabel renders basis points as a percentage: 2300 -> "23 %", 650 -> "6.5 %".
func pctLabel(bp int) string {
	whole, frac := bp/100, bp%100
	if frac == 0 {
		return strconv.Itoa(whole) + " %"
	}
	s := fmt.Sprintf("%d.%02d", whole, frac)
	for s[len(s)-1] == '0' {
		s = s[:len(s)-1]
	}
	return s + " %"
}

func documentTypeOptions(l i18n.Language) []Option {
	var out []Option
	for _, d := range domain.RegistrableDocumentTypes() {
		out = append(out, Option{string(d), tr(l, d.LabelKey())})
	}
	return out
}

func expenseTypeOptions(l i18n.Language) []Option {
	var out []Option
	for _, e := range domain.AllExpenseTypes() {
		out = append(out, Option{string(e), tr(l, e.LabelKey())})
	}
	return out
}

func categoryOptions(l i18n.Language) []Option {
	var out []Option
	for _, c := range domain.AllCategories() {
		out = append(out, Option{string(c), tr(l, c.LabelKey())})
	}
	return out
}

func regionOptions(l i18n.Language) []Option {
	var out []Option
	for _, r := range domain.AllFiscalRegions() {
		out = append(out, Option{string(r), tr(l, r.LabelKey())})
	}
	return out
}

func methodOptions(l i18n.Language) []Option {
	var out []Option
	for _, m := range domain.AllPaymentMethods() {
		out = append(out, Option{string(m), tr(l, m.LabelKey())})
	}
	return out
}

func treatmentOptions(l i18n.Language) []Option {
	var out []Option
	for _, t := range domain.AllVATTreatments() {
		out = append(out, Option{string(t), tr(l, t.LabelKey())})
	}
	return out
}

func statusOptions(l i18n.Language) []Option {
	var out []Option
	for _, s := range domain.AllPaymentStatuses() {
		out = append(out, Option{string(s), tr(l, s.LabelKey())})
	}
	return out
}

// withAll prepends the "all" option used by the list filters.
func withAll(l i18n.Language, opts []Option) []Option {
	return append([]Option{{"", tr(l, "invoices.filter.all")}}, opts...)
}

func rateOptions(region domain.FiscalRegion) []Option {
	if !region.Valid() {
		region = domain.RegionContinental
	}
	var out []Option
	for _, bp := range domain.AllowedRates(region) {
		out = append(out, Option{strconv.Itoa(bp), pctLabel(bp)})
	}
	return out
}

func exemptionOptions(l i18n.Language) []Option {
	out := []Option{{"", "—"}}
	for _, c := range domain.ExemptionCodes() {
		out = append(out, Option{c.Code, c.Code + " — " + tr(l, c.DescriptionKey)})
	}
	return out
}

// countryOptions is the short list of countries offered for a supplier; the
// default is PT. The value is an ISO code.
func countryOptions() []Option {
	return []Option{
		{"PT", "PT"}, {"ES", "ES"}, {"FR", "FR"}, {"DE", "DE"}, {"IT", "IT"},
		{"GB", "GB"}, {"BR", "BR"}, {"US", "US"}, {"OTHER", "—"},
	}
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func statusVariant(s domain.PaymentStatus) atoms.BadgeVariant {
	switch s {
	case domain.StatusPago:
		return atoms.BadgeVariantSuccess
	case domain.StatusParcialmentePago:
		return atoms.BadgeVariantWarning
	case domain.StatusPendente:
		return atoms.BadgeVariantInfo
	case domain.StatusAnulado:
		return atoms.BadgeVariantDanger
	default:
		return atoms.BadgeVariantNeutral
	}
}

// pageURL builds the list URL of a page keeping the active filters.
func pageURL(f domain.ListFilter, page int) string {
	v := url.Values{}
	set := func(k, val string) {
		if val != "" {
			v.Set(k, val)
		}
	}
	set("supplier", f.SupplierName)
	set("nif", f.NIF)
	set("number", f.Number)
	set("document_type", f.DocumentType)
	set("expense_type", f.ExpenseType)
	set("category", f.Category)
	set("status", f.Status)
	set("issue_from", f.IssueFrom)
	set("issue_to", f.IssueTo)
	set("due_from", f.DueFrom)
	set("due_to", f.DueTo)
	set("period", f.ReferencePeriod)
	if page > 1 {
		v.Set("page", strconv.Itoa(page))
	}
	if len(v) == 0 {
		return "/invoices"
	}
	return "/invoices?" + v.Encode()
}

func totalPages(r domain.ListResult) int {
	size := r.PageSize
	if size < 1 {
		size = domain.DefaultPageSize
	}
	n := (r.RowCount + size - 1) / size
	if n < 1 {
		n = 1
	}
	return n
}

func currentPage(r domain.ListResult) int {
	if r.Page < 1 {
		return 1
	}
	return r.Page
}

func pageIndicator(l i18n.Language, r domain.ListResult) string {
	return fmt.Sprintf(tr(l, "invoices.list.page_of"), currentPage(r), totalPages(r))
}

func invoiceHref(id string) string { return "/invoices/" + id }

func attachmentHref(invoiceID, attID string) string {
	return "/invoices/" + invoiceID + "/attachments/" + attID
}

func formAction(d FormData) string {
	if d.Edit {
		return "/invoices/" + d.InvoiceID + "/edit"
	}
	return "/invoices"
}

func supplierFormAction(d SupplierPageData) string {
	if d.Edit {
		return "/suppliers/" + d.Values.ID + "/edit"
	}
	return "/suppliers"
}

func removeVals(i int) string { return fmt.Sprintf(`{"op":"remove","index":"%d"}`, i) }

func selectVals(id string) string { return fmt.Sprintf(`{"select":%q}`, id) }

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

func itoa(i int) string { return strconv.Itoa(i) }

// canPay reports whether the invoice accepts payments (registered only).
func canPay(inv domain.Invoice) bool { return inv.State == domain.StateRegistered }
