package domain

import (
	"strings"
	"time"
)

// Attachment limits (FR-25).
const (
	MaxAttachments     = 10
	MaxAttachmentBytes = 5 << 20
)

// CheckPayment validates a new Pagamento da fatura against the invoice as
// committed (FR-15, FR-17, FR-20, FR-22). A draft returns ErrDraftNoPayments,
// an annulled invoice ErrAnnulled, an amount <= 0 ErrInvalidAmount, and an
// amount that makes paid+amount exceed payable returns ErrOverpayment unless
// the justification is non-blank.
func CheckPayment(state State, payableCents, paidCents, amountCents int64, justification string) error {
	switch state {
	case StateDraft:
		return ErrDraftNoPayments
	case StateAnnulled:
		return ErrAnnulled
	case StateRegistered:
	default:
		return ErrNotRegistered
	}
	if amountCents <= 0 {
		return ErrInvalidAmount
	}
	if paidCents+amountCents > payableCents && strings.TrimSpace(justification) == "" {
		return ErrOverpayment
	}
	return nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// CheckDueDate rejects a due date before the issue date unless a
// justification is given (FR-18). A zero time means the date is not set and
// is never an error. Only the calendar date is compared.
func CheckDueDate(issue, due time.Time, justification string) error {
	if issue.IsZero() || due.IsZero() {
		return nil
	}
	if dateOnly(due).Before(dateOnly(issue)) && strings.TrimSpace(justification) == "" {
		return ErrDueBeforeIssue
	}
	return nil
}

// CheckEdit rejects an edit whose new valor a pagar falls below the amount
// already paid (FR-19).
func CheckEdit(newPayableCents, paidCents int64) error {
	if newPayableCents < paidCents {
		return ErrPayableBelowPaid
	}
	return nil
}

// CheckEditable rejects any edit of an annulled invoice (FR-22).
func CheckEditable(state State) error {
	if state == StateAnnulled {
		return ErrAnnulled
	}
	return nil
}

// CheckAnnul validates an Anulação: only a registered invoice can be annulled
// (annulled returns ErrAnnulled, draft ErrNotRegistered) and a non-blank
// reason is required (FR-22).
func CheckAnnul(state State, reason string) error {
	switch state {
	case StateAnnulled:
		return ErrAnnulled
	case StateRegistered:
	default:
		return ErrNotRegistered
	}
	if strings.TrimSpace(reason) == "" {
		return ErrAnnulReasonRequired
	}
	return nil
}

// CheckAttachmentChange validates adding or removing an attachment: annulled
// invoices return ErrAnnulled; drafts and registered invoices are allowed.
func CheckAttachmentChange(state State) error {
	if state == StateAnnulled {
		return ErrAnnulled
	}
	return nil
}

// CheckAttachmentCount rejects adding `adding` files when `existing` are
// already stored and the total would exceed MaxAttachments (FR-25).
func CheckAttachmentCount(existing, adding int) error {
	if existing+adding > MaxAttachments {
		return ErrTooManyAttachments
	}
	return nil
}

// ValidateSupplier validates a Fornecedor: the name is required and the NIF
// must be valid for the country (FR-5, FR-6). For PT the NIF is required.
func ValidateSupplier(s Supplier) FieldErrors {
	errs := FieldErrors{}
	if strings.TrimSpace(s.Name) == "" {
		errs.Add("name", KeyRequired)
	}
	if err := ValidateNIF(s.Country, s.NIF); err != nil {
		errs.Add("nif", ErrorKey(err))
	}
	return errs
}

func validPeriod(p string) bool {
	if len(p) != 7 || p[4] != '-' {
		return false
	}
	_, err := time.Parse("2006-01", p)
	return err == nil
}

// validateCommon checks the fields shared by drafts and registrations: enum
// membership of the values that are set, the reference period and the due
// date rule.
func validateCommon(inv Invoice, errs FieldErrors) {
	if !inv.DocumentType.Valid() {
		errs.Add("document_type", KeyRequired)
	} else if !inv.DocumentType.Registrable() {
		errs.Add("document_type", KeyDocumentTypeNotAllowed)
	}
	if inv.ExpenseType != "" && !inv.ExpenseType.Valid() {
		errs.Add("expense_type", KeyInvalid)
	}
	if inv.Category != "" && !inv.Category.Valid() {
		errs.Add("category", KeyInvalid)
	}
	if inv.FiscalRegion != "" && !inv.FiscalRegion.Valid() {
		errs.Add("fiscal_region", KeyInvalid)
	}
	if inv.ReferencePeriod != "" && !validPeriod(inv.ReferencePeriod) {
		errs.Add("reference_period", KeyInvalidPeriod)
	}
	if inv.IssueDate != nil && inv.DueDate != nil {
		if err := CheckDueDate(*inv.IssueDate, *inv.DueDate, inv.DueJustification); err != nil {
			errs.Add("due_date", ErrorKey(err))
		}
	}
}

func regionOrDefault(r FiscalRegion) FiscalRegion {
	if r == "" {
		return RegionContinental
	}
	return r
}

// ValidateDraft validates a Rascunho (FR-20): only the document type is
// required. Values that are set must still be well formed (enums, reference
// period, due date rule). Lines are not validated: a draft may hold incomplete
// lines.
func ValidateDraft(inv Invoice) FieldErrors {
	errs := FieldErrors{}
	validateCommon(inv, errs)
	return errs
}

// ValidateRegistration applies the full validation of a registered invoice
// (FR-21, FR-12, FR-14, FR-18): number, issue date, supplier, description,
// category, at least one line, a computed total > 0, valid lines and
// withholding. The totals are recomputed from Lines, FiscalRegion (continental
// when empty) and Totals.WithholdingCents; Invoice.Totals is not trusted. The
// result is empty when the invoice may be registered.
func ValidateRegistration(inv Invoice) FieldErrors {
	errs := FieldErrors{}
	validateCommon(inv, errs)
	if strings.TrimSpace(inv.Number) == "" {
		errs.Add("number", KeyRequired)
	}
	if inv.IssueDate == nil {
		errs.Add("issue_date", KeyRequired)
	}
	if strings.TrimSpace(inv.SupplierID) == "" {
		errs.Add("supplier", KeyRequired)
	}
	if strings.TrimSpace(inv.Description) == "" {
		errs.Add("description", KeyRequired)
	}
	if inv.Category == "" {
		errs.Add("category", KeyRequired)
	}
	if len(inv.Lines) == 0 {
		errs.Add("lines", KeyNoLines)
		return errs
	}
	totals, err := ComputeTotals(regionOrDefault(inv.FiscalRegion), inv.Lines, inv.Totals.WithholdingCents)
	if err != nil {
		if fe, ok := err.(FieldErrors); ok {
			for k, v := range fe {
				errs.Add(k, v)
			}
		}
		return errs
	}
	if totals.TotalCents <= 0 {
		errs.Add("lines", KeyTotalNotPositive)
	}
	return errs
}
