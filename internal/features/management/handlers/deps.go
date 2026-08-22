// Package handlers contains management HTTP handlers and route registration.
package handlers

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/management/core/ports"
	"github.com/leoarkiteto/zelo/internal/features/management/core/services"
	"github.com/leoarkiteto/zelo/internal/shared/model"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

// Roles returns role data used by management handlers.
type Roles interface {
	ActiveRolesForUser(ctx context.Context, userID, condominiumID string) ([]model.Role, error)
	ListUsersWithRoles(ctx context.Context, condominiumID string) ([]store.UserRolesRow, error)
}

// Deps are the collaborators used by management handlers.
type Deps struct {
	Roles       Roles
	Audit       ports.AuditRecorder
	Tokens      ports.TokenHasher
	Invitations ports.InvitationStore
	Units       ports.UnitReader
	RoleService *services.RoleService
}

// Handler bundles dependencies for management handler methods.
type Handler struct {
	deps Deps
}
