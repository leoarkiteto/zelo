package templates

import (
	"fmt"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/percent"
	organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
)

// SettleLateFeeData backs the late fee fragment of the settle form: the
// breakdown for the chosen date and the buttons that fit it.
type SettleLateFeeData struct {
	Locale    i18n.Language
	AccountID string
	// HasLateFee is true when the quota owes a late fee (> 0) on the chosen date.
	HasLateFee bool
	Fine       string
	Interest   string
	Total      string
	DaysLate   int
	// Error is a translated message shown instead of the breakdown.
	Error string
}

// LateFeeReceiptView is the late fee breakdown shown on the Recibo (FR-16).
type LateFeeReceiptView struct {
	Origin       string
	Fine         string
	Interest     string
	DaysLate     int
	FeeRate      string
	InterestRate string
	GraceDays    int
}

// DebtorRowView is one unit of the debtors list (FR-1).
type DebtorRowView struct {
	UnitCode string
	// StatementHref opens the Conta corrente of the unit.
	StatementHref string
	// Occupants are "e-mail (role)" texts; empty means no active occupants.
	Occupants    []string
	OverdueCount int
	Total        string
	OldestDue    string
	DaysLate     int
	Estimated    string
}

// DebtorsPageData backs the debtors page.
type DebtorsPageData struct {
	Shell  *organisms.ShellData
	Locale i18n.Language
	Rows   []DebtorRowView
}

// ReserveView carries the reserve fund balance figures (FR-17), already formatted.
type ReserveView struct {
	Realized     string
	Forecast     string
	RealizedYear string
}

// RateText renders basis points as a percentage ("2", "0,5", "12.25").
func RateText(locale i18n.Language, bp int) string {
	return percent.FormatBP(bp, locale) + "%"
}

// DebtorsLine is the syndic card line "N units overdue — total" (FR-18).
func DebtorsLine(locale i18n.Language, units int, total string) string {
	if units == 1 {
		return fmt.Sprintf(i18n.T(locale, "quotas.card.debtors_one"), total)
	}
	return fmt.Sprintf(i18n.T(locale, "quotas.card.debtors"), units, total)
}
