package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/invoices/domain"
	"github.com/leoarkiteto/zelo/internal/features/invoices/templates"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/money"
)

const dateLayout = "2006-01-02"

var lineKeyRE = regexp.MustCompile(`^lines\[(\d+)\]\.([a-z_]+)$`)

// parsedForm is the invoice form read from a request: the raw values for
// re-rendering, the typed invoice for the services, and the parse errors
// (unparsable money, quantity or date inputs).
type parsedForm struct {
	Values  templates.FormValues
	Lines   []templates.LineForm
	Invoice domain.Invoice
	Errors  domain.FieldErrors

	Locale      i18n.Language
	ExpenseType domain.ExpenseType // effective: compra_produtos when unset
	Region      domain.FiscalRegion
	Withholding int64
}

func formString(r *http.Request, key string) string { return r.FormValue(key) }

func parseDate(r *http.Request, errs domain.FieldErrors, field string) *time.Time {
	raw := strings.TrimSpace(formString(r, field))
	if raw == "" {
		return nil
	}
	t, err := time.Parse(dateLayout, raw)
	if err != nil {
		errs.Add(field, domain.KeyInvalid)
		return nil
	}
	return &t
}

// parseInvoiceForm reads the invoice form. Money and quantity inputs go through
// money.ParseCents and domain.ParseQuantityMilli; an empty input is zero and an
// unparsable one is a field error.
func parseInvoiceForm(r *http.Request) *parsedForm {
	_ = r.ParseForm()
	locale := i18n.LanguageFrom(r.Context())
	s := func(k string) string { return formString(r, k) }
	errs := domain.FieldErrors{}

	v := templates.FormValues{
		IdempotencyKey:     strings.TrimSpace(s("idempotency_key")),
		DocumentType:       s("document_type"),
		Number:             s("number"),
		IssueDate:          s("issue_date"),
		DueDate:            s("due_date"),
		OperationDate:      s("operation_date"),
		DueJustification:   s("due_justification"),
		SupplierID:         strings.TrimSpace(s("supplier_id")),
		ExpenseType:        s("expense_type"),
		Category:           s("category"),
		Description:        s("description"),
		Project:            s("project"),
		CostCenter:         s("cost_center"),
		ReferencePeriod:    s("reference_period"),
		Notes:              s("notes"),
		FiscalRegion:       s("fiscal_region"),
		Withholding:        s("withholding"),
		ServiceDescription: s("service_description"),
		ServiceStart:       s("service_start"),
		ServiceEnd:         s("service_end"),
		ServiceContract:    s("service_contract"),
		MonthlyServiceType: s("monthly_service_type"),
		BillingPeriod:      s("billing_period"),
		MonthlyStart:       s("monthly_start"),
		MonthlyEnd:         s("monthly_end"),
		ContractNumber:     s("contract_number"),
		MeterID:            s("meter_id"),
		CPE:                s("cpe"),
		CUI:                s("cui"),
		ConsumptionPlace:   s("consumption_place"),
		Recurring:          s("recurring") == "1",
	}

	pf := &parsedForm{Values: v, Errors: errs, Locale: locale}
	pf.ExpenseType = domain.ExpenseType(orDefault(strings.TrimSpace(v.ExpenseType), string(domain.ExpenseCompraProdutos)))
	pf.Region = domain.FiscalRegion(orDefault(strings.TrimSpace(v.FiscalRegion), string(domain.RegionContinental)))

	if strings.TrimSpace(v.Withholding) != "" {
		c, err := money.ParseCents(locale, v.Withholding)
		if err != nil {
			errs.Add("withholding", domain.KeyInvalidAmount)
		} else {
			pf.Withholding = c
		}
	}

	lineForms, lines := parseLines(r, locale, pf.ExpenseType, pf.Region, errs)
	pf.Lines = lineForms

	inv := domain.Invoice{
		IdempotencyKey:   v.IdempotencyKey,
		DocumentType:     domain.DocumentType(orDefault(strings.TrimSpace(v.DocumentType), string(domain.DocumentFatura))),
		ExpenseType:      pf.ExpenseType,
		Number:           v.Number,
		IssueDate:        parseDate(r, errs, "issue_date"),
		DueDate:          parseDate(r, errs, "due_date"),
		OperationDate:    parseDate(r, errs, "operation_date"),
		SupplierID:       v.SupplierID,
		Description:      strings.TrimSpace(v.Description),
		DueJustification: strings.TrimSpace(v.DueJustification),
		Category:         domain.Category(strings.TrimSpace(v.Category)),
		Project:          strings.TrimSpace(v.Project),
		CostCenter:       strings.TrimSpace(v.CostCenter),
		ReferencePeriod:  strings.TrimSpace(v.ReferencePeriod),
		Notes:            strings.TrimSpace(v.Notes),
		FiscalRegion:     pf.Region,
		Lines:            lines,
		Totals:           domain.Totals{WithholdingCents: pf.Withholding},
	}
	switch pf.ExpenseType {
	case domain.ExpensePrestacaoServicos:
		inv.Service = domain.ServiceDetails{
			Description: strings.TrimSpace(v.ServiceDescription),
			StartDate:   parseDate(r, errs, "service_start"),
			EndDate:     parseDate(r, errs, "service_end"),
			Contract:    strings.TrimSpace(v.ServiceContract),
		}
	case domain.ExpenseDespesaMensal:
		inv.Monthly = domain.MonthlyDetails{
			ServiceType:      strings.TrimSpace(v.MonthlyServiceType),
			BillingPeriod:    strings.TrimSpace(v.BillingPeriod),
			StartDate:        parseDate(r, errs, "monthly_start"),
			EndDate:          parseDate(r, errs, "monthly_end"),
			ContractNumber:   strings.TrimSpace(v.ContractNumber),
			MeterID:          strings.TrimSpace(v.MeterID),
			CPE:              strings.TrimSpace(v.CPE),
			CUI:              strings.TrimSpace(v.CUI),
			ConsumptionPlace: strings.TrimSpace(v.ConsumptionPlace),
			Recurring:        v.Recurring,
		}
	}
	pf.Invoice = inv
	fillLineTotals(pf)
	return pf
}

// lineIndexes returns the sorted indexes of the posted lines[i].* fields.
func lineIndexes(r *http.Request) []int {
	seen := map[int]bool{}
	for key := range r.Form {
		m := lineKeyRE.FindStringSubmatch(key)
		if m == nil {
			continue
		}
		if i, err := strconv.Atoi(m[1]); err == nil && i >= 0 && i < domain.MaxLines+1 {
			seen[i] = true
		}
	}
	out := make([]int, 0, len(seen))
	for i := range seen {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

// parseLines reads the indexed lines[i].* fields. The returned slices are
// renumbered 0..n-1 in posted order, so a gap left by a removed line is closed.
func parseLines(r *http.Request, locale i18n.Language, et domain.ExpenseType, region domain.FiscalRegion, errs domain.FieldErrors) ([]templates.LineForm, []domain.Line) {
	product := et == domain.ExpenseCompraProdutos
	var forms []templates.LineForm
	var lines []domain.Line

	for n, idx := range lineIndexes(r) {
		get := func(f string) string { return r.FormValue(fmt.Sprintf("lines[%d].%s", idx, f)) }
		fail := func(f, key string) { errs.Add(fmt.Sprintf("lines[%d].%s", n, f), key) }

		lf := templates.LineForm{
			Description: get("description"), Product: get("product"), Reference: get("reference"),
			Unit: get("unit"), Qty: get("qty"), UnitPrice: get("unit_price"),
			Discount: get("discount"), Base: get("base"),
			Treatment: get("treatment"), Rate: get("rate"),
			ExemptionCode: get("exemption_code"), ExemptionReason: get("exemption_reason"),
		}
		ln := domain.Line{
			Treatment:       domain.VATTreatment(orDefault(strings.TrimSpace(lf.Treatment), string(domain.TreatmentRate))),
			ExemptionCode:   strings.TrimSpace(lf.ExemptionCode),
			ExemptionReason: strings.TrimSpace(lf.ExemptionReason),
		}

		if ln.Treatment == domain.TreatmentRate {
			if raw := strings.TrimSpace(lf.Rate); raw == "" {
				if allowed := domain.AllowedRates(region); len(allowed) > 0 {
					ln.RateBP = allowed[0]
				}
			} else if bp, err := strconv.Atoi(raw); err != nil || bp < 0 {
				fail("rate", domain.KeyInvalid)
			} else {
				ln.RateBP = bp
			}
		}

		cents := func(field, raw string) int64 {
			if strings.TrimSpace(raw) == "" {
				return 0
			}
			c, err := money.ParseCents(locale, raw)
			if err != nil {
				fail(field, domain.KeyInvalidAmount)
			}
			return c
		}
		if product {
			ln.Product = strings.TrimSpace(lf.Product)
			ln.Reference = strings.TrimSpace(lf.Reference)
			ln.Unit = strings.TrimSpace(lf.Unit)
			if strings.TrimSpace(lf.Qty) != "" {
				q, err := domain.ParseQuantityMilli(locale, lf.Qty)
				if err != nil {
					fail("qty", domain.KeyInvalidQuantity)
				}
				ln.QuantityMilli = q
			}
			ln.UnitPriceCents = cents("unit_price", lf.UnitPrice)
			ln.DiscountCents = cents("discount", lf.Discount)
		} else {
			ln.Description = strings.TrimSpace(lf.Description)
			ln.BaseCents = cents("base", lf.Base)
		}
		forms = append(forms, lf)
		lines = append(lines, ln)
	}
	return forms, lines
}

// fillLineTotals sets the per-line total (base + VAT) shown in the table, for
// the lines that compute on their own.
func fillLineTotals(pf *parsedForm) {
	for i := range pf.Lines {
		pf.Lines[i].ShowTotal = false
		t, err := domain.ComputeTotals(pf.Region, []domain.Line{pf.Invoice.Lines[i]}, 0)
		if err != nil {
			continue
		}
		pf.Lines[i].TotalCents = t.TotalCents
		pf.Lines[i].ShowTotal = true
	}
}

// blankLine is a new empty line: taxed at the first rate of the region.
func blankLine() templates.LineForm {
	return templates.LineForm{Treatment: string(domain.TreatmentRate)}
}

// applyLineOp applies op=add|remove (index=N) to the posted lines. The
// parsedForm keeps lines and invoice lines in step.
func applyLineOp(pf *parsedForm, op, rawIndex string) {
	switch op {
	case "add":
		if len(pf.Lines) >= domain.MaxLines {
			return
		}
		pf.Lines = append(pf.Lines, blankLine())
		ln := domain.Line{Treatment: domain.TreatmentRate}
		if allowed := domain.AllowedRates(pf.Region); len(allowed) > 0 {
			ln.RateBP = allowed[0]
		}
		pf.Invoice.Lines = append(pf.Invoice.Lines, ln)
	case "remove":
		i, err := strconv.Atoi(strings.TrimSpace(rawIndex))
		if err != nil || i < 0 || i >= len(pf.Lines) {
			return
		}
		pf.Lines = append(pf.Lines[:i], pf.Lines[i+1:]...)
		pf.Invoice.Lines = append(pf.Invoice.Lines[:i], pf.Invoice.Lines[i+1:]...)
		// Errors are keyed by line number: drop them, the lines are re-rendered
		// without stale messages.
		for k := range pf.Errors {
			if strings.HasPrefix(k, "lines[") {
				delete(pf.Errors, k)
			}
		}
	}
	fillLineTotals(pf)
}

// resetDisallowedRates clears a rate that the fiscal region does not allow, so
// the select falls back to its first option when the region changed.
func resetDisallowedRates(pf *parsedForm) {
	for i := range pf.Lines {
		raw := strings.TrimSpace(pf.Lines[i].Rate)
		if raw == "" {
			continue
		}
		bp, err := strconv.Atoi(raw)
		if err != nil || !domain.RateAllowed(pf.Region, bp) {
			pf.Lines[i].Rate = ""
			if i < len(pf.Invoice.Lines) && pf.Invoice.Lines[i].Treatment == domain.TreatmentRate {
				if allowed := domain.AllowedRates(pf.Region); len(allowed) > 0 {
					pf.Invoice.Lines[i].RateBP = allowed[0]
				}
				delete(pf.Errors, fmt.Sprintf("lines[%d].rate", i))
			}
		}
	}
	fillLineTotals(pf)
}

// firstMessage returns the translated message of the first error, in key order.
func firstMessage(locale i18n.Language, errs domain.FieldErrors) string {
	keys := make([]string, 0, len(errs))
	for k := range errs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		return i18n.T(locale, i18n.MessageKey(errs[k]))
	}
	return ""
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// --- reverse mapping: invoice to form values (edit form) ---

func dateString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(dateLayout)
}

func commaDecimal(l i18n.Language) bool { return l == i18n.LanguagePTBR || l == i18n.LanguagePTPT }

// centsText renders cents as a plain decimal the form parsers read back
// (no thousands separator), "" for zero when blankZero is set.
func centsText(l i18n.Language, cents int64, blankZero bool) string {
	if cents == 0 && blankZero {
		return ""
	}
	sep := "."
	if commaDecimal(l) {
		sep = ","
	}
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d%s%02d", sign, cents/100, sep, cents%100)
}

// milliText renders milli-units without trailing zeros: 2500 -> "2.5" ("2,5" in pt).
func milliText(l i18n.Language, m int64) string {
	if m == 0 {
		return ""
	}
	whole, frac := m/1000, m%1000
	if frac == 0 {
		return strconv.FormatInt(whole, 10)
	}
	sep := "."
	if commaDecimal(l) {
		sep = ","
	}
	return fmt.Sprintf("%d%s%s", whole, sep, strings.TrimRight(fmt.Sprintf("%03d", frac), "0"))
}

func valuesFromInvoice(l i18n.Language, inv domain.Invoice) templates.FormValues {
	return templates.FormValues{
		DocumentType:       string(inv.DocumentType),
		Number:             inv.Number,
		IssueDate:          dateString(inv.IssueDate),
		DueDate:            dateString(inv.DueDate),
		OperationDate:      dateString(inv.OperationDate),
		DueJustification:   inv.DueJustification,
		SupplierID:         inv.SupplierID,
		ExpenseType:        string(inv.ExpenseType),
		Category:           string(inv.Category),
		Description:        inv.Description,
		Project:            inv.Project,
		CostCenter:         inv.CostCenter,
		ReferencePeriod:    inv.ReferencePeriod,
		Notes:              inv.Notes,
		FiscalRegion:       string(inv.FiscalRegion),
		Withholding:        centsText(l, inv.Totals.WithholdingCents, true),
		ServiceDescription: inv.Service.Description,
		ServiceStart:       dateString(inv.Service.StartDate),
		ServiceEnd:         dateString(inv.Service.EndDate),
		ServiceContract:    inv.Service.Contract,
		MonthlyServiceType: inv.Monthly.ServiceType,
		BillingPeriod:      inv.Monthly.BillingPeriod,
		MonthlyStart:       dateString(inv.Monthly.StartDate),
		MonthlyEnd:         dateString(inv.Monthly.EndDate),
		ContractNumber:     inv.Monthly.ContractNumber,
		MeterID:            inv.Monthly.MeterID,
		CPE:                inv.Monthly.CPE,
		CUI:                inv.Monthly.CUI,
		ConsumptionPlace:   inv.Monthly.ConsumptionPlace,
		Recurring:          inv.Monthly.Recurring,
	}
}

func lineFormsFromInvoice(l i18n.Language, inv domain.Invoice) []templates.LineForm {
	out := make([]templates.LineForm, 0, len(inv.Lines))
	for _, ln := range inv.Lines {
		lf := templates.LineForm{
			Description: ln.Description, Product: ln.Product, Reference: ln.Reference, Unit: ln.Unit,
			Qty:           milliText(l, ln.QuantityMilli),
			UnitPrice:     centsText(l, ln.UnitPriceCents, ln.UnitPriceCents == 0),
			Discount:      centsText(l, ln.DiscountCents, true),
			Base:          centsText(l, ln.BaseCents, false),
			Treatment:     string(ln.Treatment),
			ExemptionCode: ln.ExemptionCode, ExemptionReason: ln.ExemptionReason,
		}
		if ln.Treatment == domain.TreatmentRate {
			lf.Rate = strconv.Itoa(ln.RateBP)
		}
		if t, err := domain.ComputeTotals(inv.FiscalRegion, []domain.Line{ln}, 0); err == nil {
			lf.TotalCents = t.TotalCents
			lf.ShowTotal = true
		}
		out = append(out, lf)
	}
	return out
}
