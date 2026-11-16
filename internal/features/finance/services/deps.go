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
	// ListPendingForUnit returns non-settled, non-canceled receivables of a
	// unit (resident pendências, FR-016).
	ListPendingForUnit(ctx context.Context, condominiumID, unitID string) ([]domain.FinancialAccount, error)
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

// UnitStore finds a user's active unit and lists a condominium's units.
// The concrete *store.UnitStore satisfies this interface.
type UnitStore interface {
	ListUnitsForCondominium(ctx context.Context, condominiumID string) ([]model.Unit, error)
	GetActiveUnitForUser(ctx context.Context, userID, condominiumID string) (model.Unit, error)
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
	// unitIDs is not empty) and the approved budget of today's year.
	CardData(ctx context.Context, condoID string, unitIDs []string, today time.Time) (CardRows, error)
	// UnitShares lists the condominium's units with their current shares,
	// ordered by code (read outside a lock: previews and unit selectors).
	UnitShares(ctx context.Context, condoID string) ([]domain.ShareInput, error)
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
