// Package handlers contains management HTTP handlers and route registration.
package handlers

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/management/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/money"
	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

// CurrencyReader reads the condominium currency. *store.CondominiumStore
// satisfies it.
type CurrencyReader interface {
	GetCurrency(ctx context.Context, condoID string) (money.Currency, error)
}

// Deps are the collaborators used by management handlers.
type Deps struct {
	Roles       *store.RoleStore
	Audit       middleware.AuditRecorder
	Tokens      security.TokenHasher
	Invitations *store.InvitationStore
	Units       *store.UnitStore
	RoleService *services.RoleService

	// Condominiums and Settings back the currency settings page.
	Condominiums    CurrencyReader
	SettingsService *services.SettingsService
}

// Handler bundles dependencies for management handler methods.
type Handler struct {
	deps Deps
}
