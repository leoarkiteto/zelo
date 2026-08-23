// Package templates contains the finance page views (Templ) and their data
// view types.
package templates

import (
	"fmt"
	"strings"

	"github.com/leoarkiteto/zelo/internal/features/finance/core/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	sharedtemplates "github.com/leoarkiteto/zelo/internal/shared/templates"
)

// CategoryOption is a selectable category key with its localized label.
type CategoryOption struct {
	Key   string
	Label string
}

// StatusOption is a selectable status key with its localized label.
type StatusOption struct {
	Key   string
	Label string
}

// TypeOption is a selectable account type with its localized label.
type TypeOption struct {
	Key   string
	Label string
}

// UnitOption is a selectable unit.
type UnitOption struct {
	ID   string
	Code string
}

// AccountView is the display shape of a financial account.
type AccountView struct {
	ID              string
	Type            domain.AccountType
	Title           string
	CategoryLabel   string
	Amount          string
	DueDate         string
	EffectiveStatus domain.AccountStatus
	StatusLabel     string
	SupplierPayee   string
	HasReceipt      bool
	SettlementDate  string
}

// SummaryView is the dashboard projection.
type SummaryView struct {
	TotalReceivable  string
	TotalPayable     string
	ProjectedBalance string
	RealizedBalance  string
}

// FinancePageData backs the dashboard + list page.
type FinancePageData struct {
	Shell      *sharedtemplates.ShellData
	Locale     i18n.Language
	CSRF       string
	Accounts   []AccountView
	Categories []CategoryOption
	Statuses   []StatusOption
	Types      []TypeOption
	Query      string
	Type       string
	Category   string
	Status     string
	Period     string
	Summary    SummaryView
	Flash      string
}

// FormValues carries the raw form input for re-rendering on validation errors.
type FormValues struct {
	Type           string
	Title          string
	Category       string
	Amount         string
	DueDate        string
	Status         string
	SettlementDate string
	SupplierPayee  string
	PaymentCode    string
	UnitID         string
}

// FinanceFormData backs the create/edit form page.
type FinanceFormData struct {
	Shell       *sharedtemplates.ShellData
	Locale      i18n.Language
	CSRF        string
	IsEdit      bool
	AccountID   string
	AccountType domain.AccountType
	Values      FormValues
	Categories  []CategoryOption
	Units       []UnitOption
	Error       string
}

// SettleData backs the settlement form page.
type SettleData struct {
	Shell   *sharedtemplates.ShellData
	Locale  i18n.Language
	CSRF    string
	Account AccountView
	Error   string
}

// CancelData backs the cancel confirmation page.
type CancelData struct {
	Shell   *sharedtemplates.ShellData
	Locale  i18n.Language
	CSRF    string
	Account AccountView
	Error   string
}

// ChargesPageData backs the resident pendências page.
type ChargesPageData struct {
	Shell   *sharedtemplates.ShellData
	Locale  i18n.Language
	CSRF    string
	Charges []AccountView
	NoUnit  bool
	Empty   bool
}

// HealthPageData backs the resident financial-health page.
type HealthPageData struct {
	Shell         *sharedtemplates.ShellData
	Locale        i18n.Language
	CSRF          string
	Period        string
	TotalSpent    string
	TotalCollected string
	Empty         bool
}

// Money renders integer cents as a localized BRL amount.
func Money(locale i18n.Language, cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole := cents / 100
	frac := cents % 100
	if locale == i18n.LanguagePTBR {
		return sign + "R$ " + groupThousands(whole, ".") + "," + fmt.Sprintf("%02d", frac)
	}
	return sign + "R$ " + groupThousands(whole, ",") + "." + fmt.Sprintf("%02d", frac)
}

func groupThousands(n int64, sep string) string {
	s := fmt.Sprintf("%d", n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteString(sep)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// StatusLabel returns the localized label for a status.
func StatusLabel(locale i18n.Language, s domain.AccountStatus) string {
	switch s {
	case domain.StatusPending:
		return i18n.T(locale, "finance.status.pending")
	case domain.StatusSettled:
		return i18n.T(locale, "finance.status.settled")
	case domain.StatusOverdue:
		return i18n.T(locale, "finance.status.overdue")
	case domain.StatusCanceled:
		return i18n.T(locale, "finance.status.canceled")
	default:
		return string(s)
	}
}

// statusBadge returns the badge variant for a status.
func statusBadge(s domain.AccountStatus) string {
	switch s {
	case domain.StatusOverdue:
		return "badge-danger"
	case domain.StatusSettled:
		return "badge-success"
	case domain.StatusPending:
		return "badge-info"
	default:
		return "badge-neutral"
	}
}

// CategoryLabel returns the localized label for a category key.
func CategoryLabel(locale i18n.Language, t domain.AccountType, c domain.Category) string {
	return i18n.T(locale, i18n.MessageKey("finance.cat."+string(t)+"."+string(c)))
}

// CategoryOptions builds the selectable categories for a type. An empty type
// yields the union of both category lists for the filter form.
func CategoryOptions(locale i18n.Language, cats []domain.Category) []CategoryOption {
	if len(cats) == 0 {
		cats = append(domain.PayableCategories(), domain.ReceivableCategories()...)
	}
	out := make([]CategoryOption, 0, len(cats))
	for _, c := range cats {
		out = append(out, CategoryOption{Key: string(c), Label: CategoryLabel(locale, categoryTypeOf(c), c)})
	}
	return out
}

func categoryTypeOf(c domain.Category) domain.AccountType {
	if c.ValidFor(domain.AccountTypePayable) {
		return domain.AccountTypePayable
	}
	return domain.AccountTypeReceivable
}

// StatusOptions returns the four filterable statuses plus the dedicated
// delinquents (inadimplentes) quick filter (FR-015).
func StatusOptions(locale i18n.Language) []StatusOption {
	keys := []domain.AccountStatus{domain.StatusPending, domain.StatusSettled, domain.StatusOverdue, domain.StatusCanceled}
	out := make([]StatusOption, 0, len(keys)+1)
	out = append(out, StatusOption{Key: "inadimplentes", Label: i18n.T(locale, "finance.filter.inadimplentes")})
	for _, k := range keys {
		out = append(out, StatusOption{Key: string(k), Label: StatusLabel(locale, k)})
	}
	return out
}

// TypeOptions returns the two account types.
func TypeOptions(locale i18n.Language) []TypeOption {
	return []TypeOption{
		{Key: string(domain.AccountTypePayable), Label: i18n.T(locale, "finance.type_payable")},
		{Key: string(domain.AccountTypeReceivable), Label: i18n.T(locale, "finance.type_receivable")},
	}
}

// UnitOptions maps units to select options.
func UnitOptions(units []model.Unit) []UnitOption {
	out := make([]UnitOption, 0, len(units))
	for _, u := range units {
		out = append(out, UnitOption{ID: u.ID, Code: u.Code})
	}
	return out
}
