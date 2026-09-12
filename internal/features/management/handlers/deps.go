// Package handlers contains management HTTP handlers and route registration.
package handlers

import (
	"github.com/leoarkiteto/zelo/internal/features/management/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

// Deps are the collaborators used by management handlers.
type Deps struct {
	Roles       *store.RoleStore
	Audit       middleware.AuditRecorder
	Tokens      security.TokenHasher
	Invitations *store.InvitationStore
	Units       *store.UnitStore
	RoleService *services.RoleService
}

// Handler bundles dependencies for management handler methods.
type Handler struct {
	deps Deps
}
