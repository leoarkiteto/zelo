// Package handlers contains finance HTTP handlers and route registration.
package handlers

import (
	"context"
	"time"

	"github.com/leoarkiteto/zelo/internal/features/finance/core/domain"
	"github.com/leoarkiteto/zelo/internal/features/finance/core/ports"
	"github.com/leoarkiteto/zelo/internal/features/finance/core/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// Roles returns the active roles for a user in a condominium.
type Roles interface {
	ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error)
}

// AuditRecorder records security-relevant events.
type AuditRecorder interface {
	RecordEvent(ctx context.Context, userID *string, eventType model.AuditEventType, details map[string]any) error
}

// Finance is the use-case surface the finance handlers depend on.
type Finance interface {
	CreateAccount(ctx context.Context, actorID, condominiumID string, in services.AccountInput) (domain.FinancialAccount, error)
	EditAccount(ctx context.Context, actorID, accountID, condominiumID string, in services.AccountInput) error
	SettleAccount(ctx context.Context, actorID, accountID, condominiumID, settlementDate string) error
	CancelAccount(ctx context.Context, actorID, accountID, condominiumID string) error
	ListAccounts(ctx context.Context, condominiumID string, f ports.AccountFilter) ([]domain.FinancialAccount, error)
	Summary(ctx context.Context, condominiumID string, month time.Time) (ports.MonthSummary, error)
	ResidentCharges(ctx context.Context, userID, condominiumID string) ([]domain.FinancialAccount, error)
	Health(ctx context.Context, condominiumID string, month time.Time) (ports.MonthSummary, error)
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
	Roles     Roles
	Audit     AuditRecorder
	Accounts  AccountReader
	Units     UnitLister
	Finance   Finance
	UploadDir string
}

// Handler bundles dependencies for finance handler methods.
type Handler struct {
	deps Deps
}
