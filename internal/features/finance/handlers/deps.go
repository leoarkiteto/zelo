// Package handlers contains finance HTTP handlers and route registration.
package handlers

import (
	"context"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/pagination"
)

// Finance is the use-case surface the finance handlers depend on.
type Finance interface {
	CreateAccount(ctx context.Context, actorID, condominiumID string, in services.AccountInput) (domain.FinancialAccount, error)
	EditAccount(ctx context.Context, actorID, accountID, condominiumID string, in services.AccountInput) error
	SettleAccount(ctx context.Context, actorID, accountID, condominiumID, settlementDate string) error
	CancelAccount(ctx context.Context, actorID, accountID, condominiumID string) error
	IsQuotaAccount(ctx context.Context, accountID string) (bool, error)
	QuotaPeriod(ctx context.Context, accountID string) (domain.QuotaPeriod, bool, error)
	// IsLateFeeAccount and LateFeeOrigin identify a late fee receivable (R2).
	IsLateFeeAccount(ctx context.Context, accountID string) (bool, error)
	LateFeeOrigin(ctx context.Context, accountID string) (domain.LateFeeOrigin, bool, error)
	Summary(ctx context.Context, condominiumID string, month time.Time) (services.MonthSummary, error)
	ListAccountsPageWithFlags(ctx context.Context, condominiumID string, f services.AccountFilter, requested int) ([]services.AccountRow, pagination.Page, error)
	Overview(ctx context.Context, condominiumID string, p domain.Period) (services.Overview, error)
	ResolvePeriod(raw string) domain.Period
}

// QuotaAdmin is the budget and issuance use-case surface of the syndic screens.
type QuotaAdmin interface {
	CreateBudget(ctx context.Context, actorID, condoID string, in services.BudgetInput) (services.Budget, error)
	UpdateBudget(ctx context.Context, actorID, condoID, budgetID string, p services.BudgetParams) error
	AddLine(ctx context.Context, actorID, condoID, budgetID string, in services.LineInput) (string, error)
	UpdateLine(ctx context.Context, actorID, condoID, budgetID, lineID string, in services.LineInput) error
	RemoveLine(ctx context.Context, actorID, condoID, budgetID, lineID string) error
	ApproveBudget(ctx context.Context, actorID, condoID, budgetID string) error
	ReopenBudget(ctx context.Context, actorID, condoID, budgetID string) error
	ListBudgets(ctx context.Context, condoID string) ([]services.BudgetSummary, error)
	GetBudget(ctx context.Context, condoID, id string) (services.BudgetDetail, error)
	PreviewIssuance(ctx context.Context, condoID, budgetID string, periodIndex int) (services.IssuancePreview, error)
	Issue(ctx context.Context, actorID, condoID, budgetID string, periodIndex int) (string, bool, error)
	CancelIssuance(ctx context.Context, actorID, condoID, issuanceID string) error
	ListIssuances(ctx context.Context, condoID, budgetID string) ([]services.IssuanceSummary, error)
	Statement(ctx context.Context, viewer services.Viewer, condoID, unitID string) (services.Statement, error)
	Receipt(ctx context.Context, viewer services.Viewer, condoID, accountID string) (services.ReceiptData, error)
	Card(ctx context.Context, viewer services.Viewer, condoID string, today time.Time) (services.CardData, error)
	// SettleQuota is the settle path of quota charges (late fee decision "", "apply" or "waive").
	SettleQuota(ctx context.Context, actorID, condoID, accountID string, settledOn time.Time, decision string) error
	// LateFeePreview is the late fee a quota charge owes if settled on the date (zero when none).
	LateFeePreview(ctx context.Context, condoID, accountID string, settledOn time.Time) (services.LateFeeQuote, error)
	// Debtors and ReserveBalance feed the debtors page and the reserve fund card (syndic only).
	Debtors(ctx context.Context, condoID string, today time.Time) ([]services.Debtor, error)
	ReserveBalance(ctx context.Context, condoID string, today time.Time) (services.ReserveBalance, error)
	// Extraordinary quotas (syndic only).
	PreviewExtraordinary(ctx context.Context, condoID string, in services.ExtraordinaryInput) (services.ExtraordinaryPreview, error)
	CreateExtraordinary(ctx context.Context, actorID, condoID string, in services.ExtraordinaryInput) (string, error)
	ListExtraordinaries(ctx context.Context, condoID string) ([]services.ExtraordinarySummary, error)
	GetExtraordinary(ctx context.Context, condoID, id string) (services.ExtraordinaryDetail, error)
	CancelExtraordinary(ctx context.Context, actorID, condoID, id string) error
}

// AccountReader loads a single account by id.
type AccountReader interface {
	GetByID(ctx context.Context, id string) (domain.FinancialAccount, error)
}

// UnitLister lists a condominium's units for form selects.
type UnitLister interface {
	ListUnitsForCondominium(ctx context.Context, condominiumID string) ([]model.Unit, error)
}

// Deps are the collaborators used by finance handlers.
type Deps struct {
	Roles     middleware.RoleChecker
	Audit     middleware.AuditRecorder
	Accounts  AccountReader
	Units     UnitLister
	Finance   Finance
	Quotas    QuotaAdmin
	UploadDir string
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// Handler bundles dependencies for finance handler methods.
type Handler struct {
	deps Deps
}

// now is the current time from the injected clock (time.Now when unset).
func (h *Handler) now() time.Time {
	if h.deps.Now == nil {
		return time.Now()
	}
	return h.deps.Now()
}

// today is the UTC calendar day of now, the same day the services derive with
// utcDate (the instant is converted to UTC before taking year, month and day).
func (h *Handler) today() time.Time {
	n := h.now().UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}
