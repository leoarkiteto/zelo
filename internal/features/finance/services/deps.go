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

// AuditRecorder records security-relevant events.
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}
