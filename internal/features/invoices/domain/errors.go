package domain

import (
	"errors"
	"sort"
	"strings"
)

// i18n keys of every validation message the domain can emit. The catalog task
// must provide each of them (see MessageKeys).
const (
	KeyRequired               = "invoices.error.required"
	KeyInvalid                = "invoices.error.invalid"
	KeyInvalidNIF             = "invoices.error.invalid_nif"
	KeyInvalidQuantity        = "invoices.error.invalid_quantity"
	KeyInvalidAmount          = "invoices.error.invalid_amount"
	KeyAmountTooLarge         = "invoices.error.amount_too_large"
	KeyNegativeBase           = "invoices.error.negative_base"
	KeyRateNotAllowed         = "invoices.error.rate_not_allowed"
	KeyUnknownTreatment       = "invoices.error.unknown_treatment"
	KeyExemptionCodeRequired  = "invoices.error.exemption_code_required"
	KeyExemptionCodeInvalid   = "invoices.error.exemption_code_invalid"
	KeyExemptionReasonMissing = "invoices.error.exemption_reason_required"
	KeyWithholdingRange       = "invoices.error.withholding_out_of_range"
	KeyNoLines                = "invoices.error.no_lines"
	KeyTooManyLines           = "invoices.error.too_many_lines"
	KeyTotalNotPositive       = "invoices.error.total_not_positive"
	KeyDueBeforeIssue         = "invoices.error.due_before_issue"
	KeyOverpayment            = "invoices.error.overpayment"
	KeyPayableBelowPaid       = "invoices.error.payable_below_paid"
	KeyAnnulled               = "invoices.error.annulled"
	KeyDraftNoPayments        = "invoices.error.draft_no_payments"
	KeyAnnulReasonRequired    = "invoices.error.annul_reason_required"
	KeyNotRegistered          = "invoices.error.not_registered"
	KeyTooManyAttachments     = "invoices.error.too_many_attachments"
	KeyInvalidPeriod          = "invoices.error.invalid_period"
	KeyDocumentTypeNotAllowed = "invoices.error.document_type_not_allowed"
)

// keyError is an error whose message is an i18n key.
type keyError struct{ key string }

func (e *keyError) Error() string { return e.key }

func newKeyError(key string) error { return &keyError{key: key} }

// Typed rule errors. Compare with errors.Is; get the i18n key with ErrorKey.
var (
	// ErrOverpayment: the payment would make the paid amount exceed the
	// valor a pagar and no justification was given (FR-17).
	ErrOverpayment = newKeyError(KeyOverpayment)
	// ErrDueBeforeIssue: due date before issue date without justification (FR-18).
	ErrDueBeforeIssue = newKeyError(KeyDueBeforeIssue)
	// ErrPayableBelowPaid: an edit would drop the valor a pagar below the
	// amount already paid (FR-19).
	ErrPayableBelowPaid = newKeyError(KeyPayableBelowPaid)
	// ErrAnnulled: the fatura is annulled; edits, payments and attachment
	// changes are blocked (FR-22).
	ErrAnnulled = newKeyError(KeyAnnulled)
	// ErrDraftNoPayments: a Rascunho accepts no payments (FR-20).
	ErrDraftNoPayments = newKeyError(KeyDraftNoPayments)
	// ErrInvalidAmount: a payment amount must be > 0.
	ErrInvalidAmount = newKeyError(KeyInvalidAmount)
	// ErrInvalidNIF: a PT NIF is not 9 digits with a valid mod-11 check digit.
	ErrInvalidNIF = newKeyError(KeyInvalidNIF)
	// ErrInvalidQuantity: a quantity could not be parsed.
	ErrInvalidQuantity = newKeyError(KeyInvalidQuantity)
	// ErrAnnulReasonRequired: annulment needs a reason (FR-22).
	ErrAnnulReasonRequired = newKeyError(KeyAnnulReasonRequired)
	// ErrNotRegistered: only a registered fatura can be annulled.
	ErrNotRegistered = newKeyError(KeyNotRegistered)
	// ErrTooManyAttachments: the 10-file limit would be exceeded (FR-25).
	ErrTooManyAttachments = newKeyError(KeyTooManyAttachments)
)

// ErrorKey returns the i18n key carried by err, or "" when err is not a
// domain error. A FieldErrors value has no single key and returns "".
func ErrorKey(err error) string {
	var ke *keyError
	if errors.As(err, &ke) {
		return ke.key
	}
	return ""
}

// FieldErrors maps a field name to an i18n key. Line fields use the form
// "lines[2].rate". It implements error; an empty map is never returned as an
// error by this package.
type FieldErrors map[string]string

func (f FieldErrors) Error() string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+f[k])
	}
	return strings.Join(parts, "; ")
}

// Add records the first error for a field.
func (f FieldErrors) Add(field, key string) {
	if _, ok := f[field]; !ok {
		f[field] = key
	}
}

// OrNil returns nil when there are no errors, so callers can return it as an
// error without the typed-nil pitfall.
func (f FieldErrors) OrNil() error {
	if len(f) == 0 {
		return nil
	}
	return f
}

// MessageKeys lists every i18n key the domain can emit or expose: validation
// errors, enum labels and exemption descriptions. The catalog completeness
// test of the i18n task uses it.
func MessageKeys() []string {
	keys := []string{
		KeyRequired, KeyInvalid, KeyInvalidNIF, KeyInvalidQuantity, KeyInvalidAmount,
		KeyAmountTooLarge, KeyNegativeBase, KeyRateNotAllowed, KeyUnknownTreatment,
		KeyExemptionCodeRequired, KeyExemptionCodeInvalid, KeyExemptionReasonMissing,
		KeyWithholdingRange, KeyNoLines, KeyTooManyLines, KeyTotalNotPositive,
		KeyDueBeforeIssue, KeyOverpayment, KeyPayableBelowPaid, KeyAnnulled,
		KeyDraftNoPayments, KeyAnnulReasonRequired, KeyNotRegistered,
		KeyTooManyAttachments, KeyInvalidPeriod, KeyDocumentTypeNotAllowed,
	}
	for _, v := range AllDocumentTypes() {
		keys = append(keys, v.LabelKey())
	}
	for _, v := range AllExpenseTypes() {
		keys = append(keys, v.LabelKey())
	}
	for _, v := range AllCategories() {
		keys = append(keys, v.LabelKey())
	}
	for _, v := range AllPaymentMethods() {
		keys = append(keys, v.LabelKey())
	}
	for _, v := range AllFiscalRegions() {
		keys = append(keys, v.LabelKey())
	}
	for _, v := range AllVATTreatments() {
		keys = append(keys, v.LabelKey())
	}
	for _, v := range AllPaymentStatuses() {
		keys = append(keys, v.LabelKey())
	}
	for _, c := range ExemptionCodes() {
		keys = append(keys, c.DescriptionKey)
	}
	return keys
}
