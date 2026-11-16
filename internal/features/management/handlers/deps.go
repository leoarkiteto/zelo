// Package handlers contains management HTTP handlers and route registration.
package handlers

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/management/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

// CurrencyReader reads the condominium currency and late-fee settings.
// *store.CondominiumStore satisfies it.
type CurrencyReader interface {
	GetCurrency(ctx context.Context, condoID string) (money.Currency, error)
	GetLateFeeSettings(ctx context.Context, condoID string) (model.LateFeeSettings, error)
}

// RoleStore is the role lookup the handlers need. *store.RoleStore satisfies it.
type RoleStore interface {
	ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error)
	ListUsersWithRoles(ctx context.Context, condominiumID string) ([]store.UserRolesRow, error)
}

// UnitManager is the Unidades use-case surface. *services.UnitService
// satisfies it.
type UnitManager interface {
	List(ctx context.Context, condoID string) (services.UnitList, error)
	Get(ctx context.Context, condoID, unitID string) (model.Unit, error)
	Create(ctx context.Context, actorID, condoID, code, share string) (model.Unit, error)
	Update(ctx context.Context, actorID, condoID, unitID, code, share string) error
}

// Deps are the collaborators used by management handlers.
type Deps struct {
	Roles       RoleStore
	Audit       middleware.AuditRecorder
	Tokens      security.TokenHasher
	Invitations *store.InvitationStore
	Units       *store.UnitStore
	RoleService *services.RoleService
	UnitService UnitManager

	// Condominiums and Settings back the currency settings page.
	Condominiums    CurrencyReader
	SettingsService *services.SettingsService
}

// Handler bundles dependencies for management handler methods.
type Handler struct {
	deps Deps
}
