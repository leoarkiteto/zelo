// Package templates contains the finance page views (Templ) and their data
// view types.
package templates

import (
	"context"
	"fmt"
	"strconv"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	atoms "github.com/leoarkiteto/zelo/internal/shared/templates/atoms"
	molecules "github.com/leoarkiteto/zelo/internal/shared/templates/molecules"
	organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
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
	// IsQuota hides edit/cancel: a quota is changed only through its issuance (FR-16).
	IsQuota bool
	// IsLateFee hides edit (a late fee is not editable, FR-8) but keeps cancel,
	// which is a later waiver.
	IsLateFee bool
	// CanReceipt is true for settled receivables (link to the Recibo, FR-26).
	CanReceipt bool
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
	Shell      *organisms.ShellData
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
	Pagination molecules.PaginationProps
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
	Shell       *organisms.ShellData
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
	Shell   *organisms.ShellData
	Locale  i18n.Language
	CSRF    string
	Account AccountView
	Error   string
	// Date is the settlement date shown in the input (YYYY-MM-DD).
	Date string
	// IsQuota enables the late fee fragment, refreshed when the date changes.
	IsQuota bool
	LateFee SettleLateFeeData
}

// CancelData backs the cancel confirmation page.
type CancelData struct {
	Shell   *organisms.ShellData
	Locale  i18n.Language
	CSRF    string
	Account AccountView
	Error   string
}

// Money renders integer cents as a localized amount in the currency carried
// by ctx (BRL when unset).
func Money(ctx context.Context, locale i18n.Language, cents int64) string {
	return money.Format(locale, money.From(ctx), cents)
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

// PeriodText renders a quota period in the viewer's language: a month name and
// year (monthly) or an ordinal quarter and year (quarterly). The machine label
// (YYYY-MM / YYYY-Tn) is never shown.
func PeriodText(locale i18n.Language, year int, p domain.Periodicity, index int) string {
	if p == domain.PeriodicityQuarterly {
		return fmt.Sprintf(i18n.T(locale, "quotas.period.quarterly"), index, year)
	}
	month := i18n.T(locale, i18n.MessageKey("quotas.month."+strconv.Itoa(index)))
	return fmt.Sprintf(i18n.T(locale, "quotas.period.monthly"), month, year)
}

// QuotaPeriodText is PeriodText for a QuotaPeriod.
// For an extraordinary installment it is "installment n/N".
func QuotaPeriodText(locale i18n.Language, q domain.QuotaPeriod) string {
	if q.Kind == domain.QuotaKindExtraordinary {
		return fmt.Sprintf(i18n.T(locale, "quotas.installment_label"), q.Installment, q.Installments)
	}
	return PeriodText(locale, q.Year, q.Periodicity, q.Index)
}

// PeriodState joins a localised period and its state ("dezembro de 2026 — não emitido").
func PeriodState(locale i18n.Language, period, state string) string {
	return fmt.Sprintf(i18n.T(locale, "quotas.card.period_state"), period, state)
}

// QuotaTitle is the display title of a quota receivable ("Taxa condominial
// dezembro de 2026 — Unidade A"), rendered per locale at display time. The
// stored account title is only a neutral fallback. An extraordinary installment
// renders "Quota extraordinária — <descrição> — parcela n/N" (FR-14).
func QuotaTitle(locale i18n.Language, q domain.QuotaPeriod) string {
	if q.Kind == domain.QuotaKindExtraordinary {
		return fmt.Sprintf(i18n.T(locale, "quotas.title_extraordinary"), q.Description, q.Installment, q.Installments)
	}
	return fmt.Sprintf(i18n.T(locale, "quotas.title"), QuotaPeriodText(locale, q), q.UnitCode)
}

// LateFeeTitle is the display title of a late fee receivable: "Multa e juros —
// <title of the origin quota>" (FR-14).
func LateFeeTitle(locale i18n.Language, origin domain.QuotaPeriod) string {
	return fmt.Sprintf(i18n.T(locale, "quotas.title_late_fee"), QuotaTitle(locale, origin))
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

// statusBadgeVariant maps an account status to the shared badge variant.
func statusBadgeVariant(s domain.AccountStatus) atoms.BadgeVariant {
	switch s {
	case domain.StatusOverdue:
		return atoms.BadgeVariantDanger
	case domain.StatusSettled:
		return atoms.BadgeVariantSuccess
	case domain.StatusPending:
		return atoms.BadgeVariantInfo
	default:
		return atoms.BadgeVariantNeutral
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
