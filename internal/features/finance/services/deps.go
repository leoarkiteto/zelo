// Package services defines the finance use cases together with the narrow
// persistence and collaborator interfaces they consume (vertical slice; the
// former hexagonal core/ports layer was removed).
package services

import (
	"context"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// AccountStore is the persistence port for financial accounts.
type AccountStore interface {
	// Create inserts an account and returns its id.
	Create(ctx context.Context, a domain.FinancialAccount) (string, error)
	// GetByID returns an account by id.
	GetByID(ctx context.Context, id string) (domain.FinancialAccount, error)
	// Update replaces the editable fields of an account.
	Update(ctx context.Context, a domain.FinancialAccount) error
	// SetStatus transitions an account's status, optionally recording the
	// settlement date.
	SetStatus(ctx context.Context, id, condominiumID string, status domain.AccountStatus, settlementDate *time.Time) error
	// List returns accounts of a condominium matching the filter.
	List(ctx context.Context, condominiumID string, f AccountFilter) ([]domain.FinancialAccount, error)
}

// AccountFilter carries the list filters (contracts/http-routes.md).
type AccountFilter struct {
	Query    string
	Type     domain.AccountType
	Category domain.Category
	Status   domain.AccountStatus
	// Month is the first day of the period month; zero time means no period filter.
	Month time.Time
}

// MonthSummary is the dashboard projection for one month (research.md R5).
type MonthSummary struct {
	TotalReceivableCents    int64
	TotalPayableCents       int64
	ProjectedBalanceCents   int64
	RealizedReceivableCents int64
	RealizedPayableCents    int64
	RealizedBalanceCents    int64
}

// SummaryStore computes monthly aggregates.
type SummaryStore interface {
	// SummaryForMonth computes the summary for the month containing month.
	SummaryForMonth(ctx context.Context, condominiumID string, month time.Time) (MonthSummary, error)
}

// OverviewStore is the read-only persistence port for the financial overview.
// Month keys: stores return MonthFlow.Month as time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).
// Consumers match months by (Year(), Month()), never by time.Time equality.
// Boundaries passed in are UTC first-of-month values compared against DATE columns.
type OverviewStore interface {
	// MonthlyRealized returns settled flows grouped by settlement month (sparse).
	MonthlyRealized(ctx context.Context, condominiumID string, start, end time.Time) ([]MonthFlow, error)
	// RealizedByCategory returns settled totals grouped by type and category.
	RealizedByCategory(ctx context.Context, condominiumID string, start, end time.Time) ([]CategoryTotal, error)
	// CurrentPosition returns the pending receivable/payable of the month
	// [monthStart, monthEnd) and the overdue receivables before today.
	CurrentPosition(ctx context.Context, condominiumID string, monthStart, monthEnd, today time.Time) (CurrentPosition, error)
	// SettlementYears returns distinct settlement years, descending.
	SettlementYears(ctx context.Context, condominiumID string) ([]int, error)
}

// UnitStore lists a condominium's units.
// The concrete *store.UnitStore satisfies this interface.
type UnitStore interface {
	ListUnitsForCondominium(ctx context.Context, condominiumID string) ([]model.Unit, error)
}

// QuotaStore is the persistence seam of the quota use cases. In finance,
// repositories imports services (never the reverse), so every type used here is
// declared in services or domain. *repositories.QuotaStore satisfies it.
//
// Lock order (deadlock-free by construction): issue and budget operations lock
// only the quota_budgets row; cancel locks only the quota_issuances row and
// never the budget.
type QuotaStore interface {
	// WithLockedBudget opens a transaction, locks the budget row FOR UPDATE
	// (scoped by condominium), runs fn and commits when fn returns nil;
	// otherwise it rolls back. ErrNotFound when the budget is missing.
	WithLockedBudget(ctx context.Context, condoID, budgetID string, fn func(ctx context.Context, b LockedBudget) error) error
	// WithLockedIssue is WithLockedBudget for issuing: the callback gets
	// LockedIssue, the only interface that can insert issuances and charges.
	WithLockedIssue(ctx context.Context, condoID, budgetID string, fn func(ctx context.Context, b LockedIssue) error) error
	// WithLockedIssuance opens a transaction and locks the issuance row FOR
	// UPDATE; it never locks the budget.
	WithLockedIssuance(ctx context.Context, condoID, issuanceID string, fn func(ctx context.Context, i LockedIssuance) error) error
	// CreateBudget inserts a draft budget; domain.ErrBudgetExists when the
	// condominium already has a budget for the year.
	CreateBudget(ctx context.Context, b Budget) (string, error)
	ListBudgets(ctx context.Context, condoID string) ([]BudgetSummary, error)
	// GetBudget returns the budget with its lines (computed fields are filled by the service).
	GetBudget(ctx context.Context, condoID, budgetID string) (BudgetDetail, error)
	ListIssuances(ctx context.Context, condoID, budgetID string) ([]IssuanceSummary, error)
	// ActiveIssuanceFor returns the active (issued) issuance of a period, if any.
	ActiveIssuanceFor(ctx context.Context, condoID, budgetID string, periodIndex int) (IssuanceSummary, bool, error)
	// UnitReceivables returns all receivables of a unit with quota period info.
	UnitReceivables(ctx context.Context, condoID, unitID string) ([]StatementRow, error)
	// ReceiptData returns the receivable joined with condominium, unit and
	// quota data; ErrNotFound when it is missing or not a receivable of the condominium.
	ReceiptData(ctx context.Context, condoID, accountID string) (ReceiptRow, error)
	// CardData returns the dashboard rows: the unit-scoped quota figures (when
	// unitIDs is not empty) and the approved budget of today's year. The
	// condominium-wide debtors aggregate is computed only when includeDebtors is
	// true (syndic viewer); otherwise the debtor fields are zero.
	CardData(ctx context.Context, condoID string, unitIDs []string, today time.Time, includeDebtors bool) (CardRows, error)
	// CreateExtraordinary inserts the extraordinary quota, its installment
	// issuances and every charge (receivable plus quota_charges row) in ONE
	// transaction and returns the quota id.
	CreateExtraordinary(ctx context.Context, e NewExtraordinary, installments []NewInstallment) (string, error)
	// WithLockedExtraordinary opens a transaction, locks the extraordinary
	// quota row FOR UPDATE (scoped by condominium), runs fn and commits when fn
	// returns nil. ErrNotFound when it is missing. It locks nothing else.
	WithLockedExtraordinary(ctx context.Context, condoID, id string, fn func(ctx context.Context, e LockedExtraordinary) error) error
	// ListExtraordinaries lists the condominium's extraordinary quotas, newest first.
	ListExtraordinaries(ctx context.Context, condoID string) ([]ExtraordinarySummary, error)
	// GetExtraordinary returns one extraordinary quota with its installments;
	// ErrNotFound when it is missing.
	GetExtraordinary(ctx context.Context, condoID, id string) (ExtraordinaryDetail, error)
	// QuotaAccount reads (without locking) the receivable and whether it is a
	// quota charge; ErrNotFound when it is not a receivable of the condominium.
	QuotaAccount(ctx context.Context, condoID, accountID string) (QuotaAccount, error)
	// WithLockedQuotaAccount opens a transaction, locks the receivable row FOR
	// UPDATE (scoped by condominium), runs fn and commits when fn returns nil.
	// It locks nothing else, so it cannot invert the order used by the issuance
	// and extraordinary cancels (which only lock their own row and then update
	// account rows). ErrNotFound when the receivable is missing.
	WithLockedQuotaAccount(ctx context.Context, condoID, accountID string, fn func(ctx context.Context, a LockedQuotaAccount) error) error
	// DebtorRows returns, in one set-based query, one row per unit with at least
	// one pending receivable due before today, with its active occupants, sorted
	// by total descending.
	DebtorRows(ctx context.Context, condoID string, today time.Time) ([]DebtorRow, error)
	// OverdueQuotaRows lists the pending quota charges due before today (the
	// input of the late fee estimate), in one query.
	OverdueQuotaRows(ctx context.Context, condoID string, today time.Time) ([]OverdueQuota, error)
	// ReserveTotals sums quota_charges.reserve_cents by account status; the
	// settled-in-year part covers settlements in the calendar year.
	ReserveTotals(ctx context.Context, condoID string, year int) (ReserveTotals, error)
}

// QuotaAccount is a receivable seen by the settle path.
type QuotaAccount struct {
	ID            string
	CondominiumID string
	UnitID        string
	Title         string
	AmountCents   int64
	DueDate       time.Time
	Status        domain.AccountStatus
	// IsQuota is true when the receivable is a quota charge (ordinary or
	// installment); a late fee receivable is never a quota charge.
	IsQuota bool
}

// NewLateFee is the late fee receivable inserted when the late fee is applied.
type NewLateFee struct {
	// Title is the neutral stored title; views render the localised one.
	Title     string
	DueDate   time.Time
	CreatedBy string
	// Detail carries multa, juros, days late and the effective rates; the
	// receivable amount is FineCents + InterestCents.
	Detail domain.LateFeeDetail
}

// LockedQuotaAccount is the receivable row locked inside WithLockedQuotaAccount.
type LockedQuotaAccount interface {
	// Account is read after the lock was acquired, so it reflects what the
	// previous holder committed.
	Account() QuotaAccount
	// Settle marks the pending receivable settled on the given date.
	Settle(on time.Time) error
	// InsertLateFee creates the pending late fee receivable (category fine and
	// interest, same unit) and its quota_late_fees row, returning the new
	// account id. A second late fee for the same quota yields domain.ErrNotEditable.
	InsertLateFee(in NewLateFee) (accountID string, err error)
}

// LateFeeLinks tells which accounts are late fees and where they come from
// (R2): one batch query, an entry per late fee account id.
type LateFeeLinks interface {
	LateFeeOrigins(ctx context.Context, ids []string) (map[string]domain.LateFeeOrigin, error)
}

// LockedExtraordinary is the extraordinary quota row locked inside
// WithLockedExtraordinary.
type LockedExtraordinary interface {
	Extraordinary() ExtraordinarySummary
	// CancelPendingCharges cancels the receivables of every installment
	// WHERE status='pending' and returns how many were updated and how many
	// charges the quota has.
	CancelPendingCharges() (affected int, total int, err error)
	// MarkCanceled marks the quota and all its installment issuances canceled (R9).
	MarkCanceled(at time.Time) error
}

// LockedBudget is the budget row locked FOR UPDATE inside WithLockedBudget.
type LockedBudget interface {
	Budget() Budget
	Lines() []BudgetLine
	UpdateParams(p BudgetParams) error
	AddLine(l BudgetLine) (string, error)
	UpdateLine(l BudgetLine) error
	RemoveLine(lineID string) error
	SetStatus(status domain.BudgetStatus) error
	HasActiveIssuance() (bool, error)
}

// LockedIssue is the budget row locked inside WithLockedIssue.
type LockedIssue interface {
	Budget() Budget
	Lines() []BudgetLine
	// Shares reads the unit shares inside the transaction.
	Shares() ([]domain.ShareInput, error)
	// InsertIssuance inserts the issuance ON CONFLICT DO NOTHING; when an
	// active issuance of the period already exists it returns that id and created=false.
	InsertIssuance(in NewIssuance) (id string, created bool, err error)
	// InsertCharges creates one pending receivable plus quota_charges row per charge.
	InsertCharges(issuanceID string, charges []NewCharge) error
}

// LockedIssuance is the issuance row locked inside WithLockedIssuance.
type LockedIssuance interface {
	Issuance() IssuanceSummary
	// CancelPendingCharges cancels the issuance's receivables WHERE status='pending'
	// and returns how many were updated and how many charges the issuance has.
	CancelPendingCharges() (affected int, total int, err error)
	MarkCanceled(at time.Time) error
}

// QuotaStore (extraordinary part): installments are issuances too, but they are
// reachable only through the methods above; the ordinary issuance paths
// (WithLockedIssuance, ListIssuances, ActiveIssuanceFor) never see them.

// QuotaLinks tells which accounts are managed by a quota (FR-16): the result
// has an entry, with the period and unit code for display, per quota account
// id, in one batch query.
type QuotaLinks interface {
	QuotaPeriods(ctx context.Context, ids []string) (map[string]domain.QuotaPeriod, error)
}

// OccupancyLister lists the units a user currently occupies; satisfied by
// *store.UnitStore.
type OccupancyLister interface {
	ListActiveOccupanciesForUser(ctx context.Context, userID, condominiumID string) ([]model.ActiveUnit, error)
}

// AuditRecorder records security-relevant events.
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}
