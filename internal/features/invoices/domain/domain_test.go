package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

func rateLine(base int64, bp int) Line {
	return Line{BaseCents: base, Treatment: TreatmentRate, RateBP: bp}
}

func d(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 0, 0, 0, 0, time.UTC) }

func TestAC9Buckets(t *testing.T) {
	lines := []Line{
		rateLine(10000, 2300),
		rateLine(3333, 2300),
		rateLine(1000, 600),
		{BaseCents: 5000, Treatment: TreatmentExempt, ExemptionCode: "M07", ExemptionReason: "isento"},
	}
	tot, err := ComputeTotals(RegionContinental, lines, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []TaxBucket{
		{TreatmentRate, 2300, 13333, 3067},
		{TreatmentRate, 600, 1000, 60},
		{TreatmentExempt, 0, 5000, 0},
	}
	if len(tot.Taxes) != len(want) {
		t.Fatalf("buckets = %+v", tot.Taxes)
	}
	for i, w := range want {
		if tot.Taxes[i] != w {
			t.Errorf("bucket %d = %+v, want %+v", i, tot.Taxes[i], w)
		}
	}
	if tot.TotalCents != 22460 || tot.BaseCents != 19333 || tot.VATCents != 3127 || tot.PayableCents != 22460 {
		t.Errorf("totals = %+v", tot)
	}
}

func TestAC10Region(t *testing.T) {
	_, err := ComputeTotals(RegionAcores, []Line{rateLine(1000, 2300)}, 0)
	var fe FieldErrors
	if !errors.As(err, &fe) || fe["lines[0].rate"] != KeyRateNotAllowed {
		t.Fatalf("err = %v", err)
	}
	if _, err := ComputeTotals(RegionAcores, []Line{rateLine(1000, 1600)}, 0); err != nil {
		t.Fatalf("16%% in Acores: %v", err)
	}
	_, err = ComputeTotals(RegionContinental, []Line{{BaseCents: 100, Treatment: TreatmentExempt, ExemptionReason: "x"}}, 0)
	if !errors.As(err, &fe) || fe["lines[0].exemption_code"] != KeyExemptionCodeRequired {
		t.Fatalf("err = %v", err)
	}
	_, err = ComputeTotals(RegionContinental, []Line{{BaseCents: 100, Treatment: TreatmentExempt, ExemptionCode: "M17", ExemptionReason: "x"}}, 0)
	if !errors.As(err, &fe) || fe["lines[0].exemption_code"] != KeyExemptionCodeInvalid {
		t.Fatalf("M17 should be invalid: %v", err)
	}
	_, err = ComputeTotals(RegionContinental, []Line{{BaseCents: 100, Treatment: TreatmentExempt, ExemptionCode: "M07"}}, 0)
	if !errors.As(err, &fe) || fe["lines[0].exemption_reason"] != KeyExemptionReasonMissing {
		t.Fatalf("missing reason: %v", err)
	}
}

func TestAllowedRates(t *testing.T) {
	cases := map[FiscalRegion][]int{
		RegionContinental: {600, 1300, 2300},
		RegionMadeira:     {400, 1200, 2200},
		RegionAcores:      {400, 900, 1600},
	}
	for r, want := range cases {
		got := AllowedRates(r)
		if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
			t.Errorf("%s = %v", r, got)
		}
	}
	if AllowedRates("x") != nil {
		t.Error("unknown region must have no rates")
	}
}

func TestAC11ProductLine(t *testing.T) {
	l := Line{QuantityMilli: 3000, UnitPriceCents: 999, DiscountCents: 100, Treatment: TreatmentRate, RateBP: 2300}
	base, err := LineBase(l)
	if err != nil || base != 2897 {
		t.Fatalf("base = %d, %v", base, err)
	}
	tot, err := ComputeTotals(RegionContinental, []Line{l}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if tot.GrossCents != 2997 || tot.DiscountCents != 100 || tot.BaseCents != 2897 {
		t.Errorf("totals = %+v", tot)
	}
}

func TestDecimalQuantityAndHalfUp(t *testing.T) {
	// 2.5 x 0.99 = 2.475 -> 2.48 (half up).
	l := Line{QuantityMilli: 2500, UnitPriceCents: 99, Treatment: TreatmentRate, RateBP: 2300}
	if base, _ := LineBase(l); base != 248 {
		t.Errorf("base = %d", base)
	}
	// 2.5 x 10.00 = 25.00 exactly.
	l = Line{QuantityMilli: 2500, UnitPriceCents: 1000, Treatment: TreatmentRate, RateBP: 600}
	if base, _ := LineBase(l); base != 2500 {
		t.Errorf("base = %d", base)
	}
	// VAT half-up: 0.50 at 1% not allowed; use 6%: 0.25 * 6% = 0.015 -> 0.02
	tot, _ := ComputeTotals(RegionContinental, []Line{rateLine(25, 600)}, 0)
	if tot.VATCents != 2 {
		t.Errorf("vat = %d", tot.VATCents)
	}
}

func TestNegativeBaseRejected(t *testing.T) {
	_, err := ComputeTotals(RegionContinental, []Line{{BaseCents: -1, Treatment: TreatmentRate, RateBP: 2300}}, 0)
	var fe FieldErrors
	if !errors.As(err, &fe) || fe["lines[0].base"] != KeyNegativeBase {
		t.Fatalf("err = %v", err)
	}
	// Discount larger than qty x price.
	l := Line{QuantityMilli: 1000, UnitPriceCents: 100, DiscountCents: 101, Treatment: TreatmentRate, RateBP: 2300}
	_, err = ComputeTotals(RegionContinental, []Line{l}, 0)
	if !errors.As(err, &fe) || fe["lines[0].discount"] != KeyNegativeBase {
		t.Fatalf("err = %v", err)
	}
}

func TestAllExemptAndReverseCharge(t *testing.T) {
	lines := []Line{
		{BaseCents: 1000, Treatment: TreatmentExempt, ExemptionCode: "M01", ExemptionReason: "r"},
		{BaseCents: 500, Treatment: TreatmentExempt, ExemptionCode: "M99", ExemptionReason: "r"},
	}
	tot, err := ComputeTotals(RegionMadeira, lines, 0)
	if err != nil || tot.VATCents != 0 || tot.TotalCents != 1500 || len(tot.Taxes) != 1 {
		t.Fatalf("%+v %v", tot, err)
	}
	rc := []Line{{BaseCents: 4000, Treatment: TreatmentReverseCharge, RateBP: 2300}}
	tot, err = ComputeTotals(RegionContinental, rc, 0)
	if err != nil || tot.VATCents != 0 || tot.TotalCents != 4000 {
		t.Fatalf("%+v %v", tot, err)
	}
	if len(tot.Taxes) != 1 || tot.Taxes[0].Treatment != TreatmentReverseCharge || tot.Taxes[0].RateBP != 0 || tot.Taxes[0].BaseCents != 4000 {
		t.Errorf("breakdown = %+v", tot.Taxes)
	}
}

func TestAC12Withholding(t *testing.T) {
	tot, err := ComputeTotals(RegionContinental, []Line{{BaseCents: 100000, Treatment: TreatmentReverseCharge}}, 25000)
	if err != nil || tot.TotalCents != 100000 || tot.PayableCents != 75000 || tot.WithholdingCents != 25000 {
		t.Fatalf("%+v %v", tot, err)
	}
	for _, w := range []int64{100001, -1} {
		_, err = ComputeTotals(RegionContinental, []Line{{BaseCents: 100000, Treatment: TreatmentReverseCharge}}, w)
		var fe FieldErrors
		if !errors.As(err, &fe) || fe["withholding"] != KeyWithholdingRange {
			t.Fatalf("w=%d err = %v", w, err)
		}
	}
	// Withholding equal to the total: payable 0 and pago with no payments.
	tot, err = ComputeTotals(RegionContinental, []Line{{BaseCents: 100000, Treatment: TreatmentReverseCharge}}, 100000)
	if err != nil || tot.PayableCents != 0 {
		t.Fatalf("%+v %v", tot, err)
	}
	if Status(StateRegistered, tot.PayableCents, 0) != StatusPago {
		t.Error("payable 0 must be pago")
	}
}

func TestAC13Status(t *testing.T) {
	cases := []struct {
		paid int64
		want PaymentStatus
		debt int64
	}{{0, StatusPendente, 75000}, {30000, StatusParcialmentePago, 45000}, {75000, StatusPago, 0}}
	for _, c := range cases {
		if got := Status(StateRegistered, 75000, c.paid); got != c.want {
			t.Errorf("paid %d: %s", c.paid, got)
		}
		if DebtCents(75000, c.paid) != c.debt {
			t.Errorf("paid %d: debt", c.paid)
		}
	}
	if Status(StateDraft, 100, 0) != StatusRascunho || Status(StateAnnulled, 100, 100) != StatusAnulado {
		t.Error("draft/annulled status")
	}
}

func TestAC14Overpayment(t *testing.T) {
	if err := CheckPayment(StateRegistered, 10000, 0, 15000, ""); !errors.Is(err, ErrOverpayment) {
		t.Fatalf("err = %v", err)
	}
	if err := CheckPayment(StateRegistered, 10000, 0, 15000, "  "); !errors.Is(err, ErrOverpayment) {
		t.Fatalf("blank justification: %v", err)
	}
	if err := CheckPayment(StateRegistered, 10000, 0, 15000, "adiantamento"); err != nil {
		t.Fatal(err)
	}
	if Status(StateRegistered, 10000, 15000) != StatusPago || DebtCents(10000, 15000) != -5000 || CreditCents(10000, 15000) != 5000 {
		t.Error("credit a favor")
	}
	if err := CheckPayment(StateRegistered, 10000, 0, 0, ""); !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("zero amount: %v", err)
	}
	if err := CheckPayment(StateDraft, 10000, 0, 100, ""); !errors.Is(err, ErrDraftNoPayments) {
		t.Errorf("draft: %v", err)
	}
}

func TestAC15DueBeforeIssue(t *testing.T) {
	issue, due := d(2026, 3, 10), d(2026, 3, 1)
	if err := CheckDueDate(issue, due, ""); !errors.Is(err, ErrDueBeforeIssue) {
		t.Fatalf("err = %v", err)
	}
	if err := CheckDueDate(issue, due, "acordo"); err != nil {
		t.Fatal(err)
	}
	if err := CheckDueDate(issue, issue, ""); err != nil {
		t.Errorf("same day: %v", err)
	}
	if err := CheckDueDate(issue, time.Time{}, ""); err != nil {
		t.Errorf("unset due: %v", err)
	}
}

func TestAC16PayableBelowPaid(t *testing.T) {
	if err := CheckEdit(40000, 50000); !errors.Is(err, ErrPayableBelowPaid) {
		t.Fatalf("err = %v", err)
	}
	if err := CheckEdit(50000, 50000); err != nil {
		t.Fatal(err)
	}
}

func TestAC18Annulled(t *testing.T) {
	if !errors.Is(CheckEditable(StateAnnulled), ErrAnnulled) {
		t.Error("edit")
	}
	if !errors.Is(CheckPayment(StateAnnulled, 100, 0, 10, ""), ErrAnnulled) {
		t.Error("payment")
	}
	if !errors.Is(CheckAttachmentChange(StateAnnulled), ErrAnnulled) {
		t.Error("attachment")
	}
	if CheckAttachmentChange(StateDraft) != nil || CheckAttachmentChange(StateRegistered) != nil {
		t.Error("attachments allowed on draft and registered")
	}
	if !errors.Is(CheckAnnul(StateAnnulled, "x"), ErrAnnulled) ||
		!errors.Is(CheckAnnul(StateDraft, "x"), ErrNotRegistered) ||
		!errors.Is(CheckAnnul(StateRegistered, " "), ErrAnnulReasonRequired) ||
		CheckAnnul(StateRegistered, "erro") != nil {
		t.Error("CheckAnnul")
	}
	if !errors.Is(CheckAttachmentCount(10, 1), ErrTooManyAttachments) || CheckAttachmentCount(9, 1) != nil {
		t.Error("attachment count")
	}
}

func TestAC6NIF(t *testing.T) {
	if err := ValidateNIF("PT", "123456789"); err != nil {
		t.Errorf("valid NIF: %v", err)
	}
	for _, bad := range []string{"123456780", "12345678", "1234567890", "12345678a", ""} {
		if err := ValidateNIF("PT", bad); !errors.Is(err, ErrInvalidNIF) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	got := NormalizeNIF("PT", "PT 123 456 789")
	if got != "123456789" || ValidateNIF("PT", got) != nil {
		t.Errorf("normalized = %q", got)
	}
	if NormalizeNIF("pt", " pt123456789 ") != "123456789" {
		t.Error("lowercase prefix")
	}
	if got := NormalizeNIF("ES", "  ES-B 123  "); got != "ES-B 123" {
		t.Errorf("non-PT must only trim: %q", got)
	}
	if ValidateNIF("ES", "not numeric!") != nil || ValidateNIF("ES", "") != nil {
		t.Error("non-PT is free text")
	}
}

func TestNormalizeNumber(t *testing.T) {
	if got := NormalizeNumber("  FT   2026/15 "); got != "FT 2026/15" {
		t.Errorf("got %q", got)
	}
}

func TestParseQuantityMilli(t *testing.T) {
	ok := []struct {
		lang i18n.Language
		in   string
		want int64
	}{
		{i18n.LanguageEN, "2.5", 2500},
		{i18n.LanguageEN, "1,234.567", 1234567},
		{i18n.LanguageEN, "3", 3000},
		{i18n.LanguagePTBR, "2,5", 2500},
		{i18n.LanguagePTBR, "1.234,5", 1234500},
		{i18n.LanguagePTPT, "0,001", 1},
		{i18n.LanguagePTPT, "1 234,50", 1234500},
		{i18n.LanguagePTPT, "2.5", 2500},
		{i18n.LanguagePTPT, "1.234", 1234000},
		{i18n.LanguageEN, " 7 ", 7000},
	}
	for _, c := range ok {
		got, err := ParseQuantityMilli(c.lang, c.in)
		if err != nil || got != c.want {
			t.Errorf("%s %q = %d, %v; want %d", c.lang, c.in, got, err, c.want)
		}
	}
	for _, in := range []string{"", "abc", "1.2345", "-1", "1..2", "1.2.3", "1,23,4", "12,34.5", "1,2345"} {
		if _, err := ParseQuantityMilli(i18n.LanguageEN, in); !errors.Is(err, ErrInvalidQuantity) {
			t.Errorf("en %q accepted (err %v)", in, err)
		}
	}
	if _, err := ParseQuantityMilli(i18n.LanguagePTBR, "1,2345"); err == nil {
		t.Error("4 decimals must be rejected")
	}
	if _, err := ParseQuantityMilli(i18n.LanguagePTBR, "1.23.456,0"); err == nil {
		t.Error("bad thousands grouping must be rejected")
	}
}

func TestProductQuantityMustBePositive(t *testing.T) {
	l := Line{UnitPriceCents: 100, Treatment: TreatmentRate, RateBP: 2300}
	_, err := ComputeTotals(RegionContinental, []Line{l}, 0)
	var fe FieldErrors
	if !errors.As(err, &fe) || fe["lines[0].qty"] != KeyInvalidQuantity {
		t.Fatalf("err = %v", err)
	}
}

func TestDocumentTypes(t *testing.T) {
	if !DocumentNotaCredito.Valid() || DocumentNotaCredito.Registrable() {
		t.Error("nota_credito must be valid but not registrable")
	}
	if string(DocumentFaturaRecibo) != "fatura_recibo" || string(StatusParcialmentePago) != "parcialmente_pago" ||
		string(MethodMBWay) != "mb_way" || string(MethodTransferenciaBancaria) != "transferencia_bancaria" ||
		string(ExpenseCompraProdutos) != "compra_produtos" {
		t.Error("stored values")
	}
	if len(AllCategories()) != 18 || len(AllExpenseTypes()) != 6 || len(AllPaymentMethods()) != 7 || len(AllDocumentTypes()) != 6 {
		t.Error("enum sizes")
	}
	if len(RegistrableDocumentTypes()) != 5 {
		t.Error("registrable types")
	}
	if DocumentType("x").Valid() || Category("").Valid() {
		t.Error("unknown values must be invalid")
	}
}

func TestExemptionCodes(t *testing.T) {
	codes := ExemptionCodes()
	if len(codes) != 42 {
		t.Fatalf("len = %d", len(codes))
	}
	for _, c := range []string{"M01", "M16", "M19", "M26", "M30", "M46", "M99"} {
		if !ValidExemptionCode(c) {
			t.Errorf("%s should be valid", c)
		}
	}
	for _, c := range []string{"M00", "M17", "M18", "M27", "M29", "M47", "M98", "m07", ""} {
		if ValidExemptionCode(c) {
			t.Errorf("%s should be invalid", c)
		}
	}
}

func TestValidateDraftAndRegistration(t *testing.T) {
	if errs := ValidateDraft(Invoice{DocumentType: DocumentFatura}); len(errs) != 0 {
		t.Errorf("draft with only document type: %v", errs)
	}
	if errs := ValidateDraft(Invoice{}); errs["document_type"] != KeyRequired {
		t.Errorf("draft needs document type: %v", errs)
	}
	if errs := ValidateDraft(Invoice{DocumentType: DocumentNotaCredito}); errs["document_type"] != KeyDocumentTypeNotAllowed {
		t.Errorf("credit note: %v", errs)
	}

	errs := ValidateRegistration(Invoice{DocumentType: DocumentFatura})
	for _, f := range []string{"number", "issue_date", "supplier_id", "description", "category", "lines"} {
		if errs[f] == "" {
			t.Errorf("missing error for %s: %v", f, errs)
		}
	}

	issue := d(2026, 3, 10)
	inv := Invoice{
		DocumentType: DocumentFatura, ExpenseType: ExpenseManutencao, Number: "FT 1", IssueDate: &issue,
		SupplierID: "s1", Description: "x", Category: CategoryManutencao,
		Lines: []Line{rateLine(1000, 2300)},
	}
	if errs := ValidateRegistration(inv); len(errs) != 0 {
		t.Fatalf("valid invoice: %v", errs)
	}
	inv.Lines = []Line{{BaseCents: 0, Treatment: TreatmentReverseCharge}}
	if errs := ValidateRegistration(inv); errs["lines"] != KeyTotalNotPositive {
		t.Errorf("zero total: %v", errs)
	}
	inv.Lines = []Line{rateLine(1000, 2300)}
	due := d(2026, 3, 1)
	inv.DueDate = &due
	if errs := ValidateRegistration(inv); errs["due_date"] != KeyDueBeforeIssue || errs["due_justification"] != KeyDueBeforeIssue {
		t.Errorf("due before issue: %v", errs)
	}
	inv.DueJustification = "acordo"
	if errs := ValidateRegistration(inv); len(errs) != 0 {
		t.Errorf("justified: %v", errs)
	}
	inv.ReferencePeriod = "2026-13"
	if errs := ValidateRegistration(inv); errs["reference_period"] != KeyInvalidPeriod {
		t.Errorf("period: %v", errs)
	}
	inv.ReferencePeriod = "2026-03"
	inv.FiscalRegion = RegionAcores
	if errs := ValidateRegistration(inv); errs["lines[0].rate"] != KeyRateNotAllowed {
		t.Errorf("region/rate: %v", errs)
	}
	inv.FiscalRegion = ""
	inv.Totals.WithholdingCents = 5000
	if errs := ValidateRegistration(inv); errs["withholding"] != KeyWithholdingRange {
		t.Errorf("withholding: %v", errs)
	}
}

func TestValidateSupplier(t *testing.T) {
	if errs := ValidateSupplier(Supplier{Name: "A", Country: "PT", NIF: "123456780"}); errs["nif"] != KeyInvalidNIF {
		t.Errorf("%v", errs)
	}
	if errs := ValidateSupplier(Supplier{Name: " ", Country: "ES", NIF: "x y"}); errs["name"] != KeyRequired || len(errs) != 1 {
		t.Errorf("%v", errs)
	}
}

func TestFieldErrorsAsError(t *testing.T) {
	if (FieldErrors{}).OrNil() != nil {
		t.Error("empty must be nil")
	}
	if err := (FieldErrors{"a": "k"}).OrNil(); err == nil || err.Error() != "a: k" {
		t.Errorf("err = %v", err)
	}
}

func TestMessageKeys(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range MessageKeys() {
		if seen[k] {
			t.Errorf("duplicate key %s", k)
		}
		seen[k] = true
	}
	// Every key that ComputeTotals / rules can emit must be listed.
	emitted := []string{
		KeyRequired, KeyInvalid, KeyInvalidNIF, KeyInvalidQuantity, KeyInvalidAmount, KeyAmountTooLarge,
		KeyNegativeBase, KeyRateNotAllowed, KeyUnknownTreatment, KeyExemptionCodeRequired,
		KeyExemptionCodeInvalid, KeyExemptionReasonMissing, KeyWithholdingRange, KeyNoLines,
		KeyTooManyLines, KeyTotalNotPositive, KeyDueBeforeIssue, KeyOverpayment, KeyPayableBelowPaid,
		KeyAnnulled, KeyDraftNoPayments, KeyAnnulReasonRequired, KeyNotRegistered,
		KeyTooManyAttachments, KeyInvalidPeriod, KeyDocumentTypeNotAllowed,
	}
	for _, k := range emitted {
		if !seen[k] {
			t.Errorf("%s missing from MessageKeys", k)
		}
	}
	for _, e := range []error{ErrOverpayment, ErrDueBeforeIssue, ErrPayableBelowPaid, ErrAnnulled, ErrDraftNoPayments, ErrInvalidAmount, ErrInvalidNIF, ErrInvalidQuantity, ErrAnnulReasonRequired, ErrNotRegistered, ErrTooManyAttachments} {
		if !seen[ErrorKey(e)] {
			t.Errorf("%v missing", e)
		}
	}
	for _, p := range AllPaymentStatuses() {
		if !seen[p.LabelKey()] {
			t.Errorf("%s missing", p)
		}
	}
	if !seen["invoices.exemption.M07"] || !seen["invoices.category.agua"] {
		t.Error("enum keys")
	}
}

func TestLimits(t *testing.T) {
	l := Line{QuantityMilli: 1 << 40, UnitPriceCents: 1 << 40, Treatment: TreatmentRate, RateBP: 2300}
	_, err := ComputeTotals(RegionContinental, []Line{l}, 0)
	var fe FieldErrors
	if !errors.As(err, &fe) || fe["lines[0].discount"] != KeyAmountTooLarge {
		t.Fatalf("err = %v", err)
	}
	if _, err := ComputeTotals(RegionContinental, make([]Line, MaxLines+1), 0); err == nil {
		t.Error("too many lines")
	}
}

func TestListFilterNormalized(t *testing.T) {
	f := ListFilter{}.Normalized()
	if f.Page != 1 || f.PageSize != DefaultPageSize {
		t.Errorf("%+v", f)
	}
	if (ListFilter{Page: 3, PageSize: 1000}).Normalized().PageSize != MaxPageSize {
		t.Error("max page size")
	}
}
