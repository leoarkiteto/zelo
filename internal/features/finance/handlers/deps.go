// Package handlers contains finance HTTP handlers and route registration.
package handlers

import (
	"context"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// Finance is the use-case surface the finance handlers depend on.
type Finance interface {
	CreateAccount(ctx context.Context, actorID, condominiumID string, in services.AccountInput) (domain.FinancialAccount, error)
	EditAccount(ctx context.Context, actorID, accountID, condominiumID string, in services.AccountInput) error
	SettleAccount(ctx context.Context, actorID, accountID, condominiumID, settlementDate string) error
	CancelAccount(ctx context.Context, actorID, accountID, condominiumID string) error
	IsQuotaAccount(ctx context.Context, accountID string) (bool, error)
	Summary(ctx context.Context, condominiumID string, month time.Time) (services.MonthSummary, error)
	ListAccountsWithFlags(ctx context.Context, condominiumID string, f services.AccountFilter) ([]services.AccountRow, error)
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
}

// Handler bundles dependencies for finance handler methods.
type Handler struct {
	deps Deps
}
