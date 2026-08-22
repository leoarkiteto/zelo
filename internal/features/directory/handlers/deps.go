// Package handlers contains directory HTTP handlers and route registration.
package handlers

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/directory/core/services"
	"github.com/leoarkiteto/zelo/internal/features/directory/repositories"
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

// Deps are the collaborators used by directory handlers.
type Deps struct {
	Roles      Roles
	Audit      AuditRecorder
	Listings   *repositories.ListingStore
	Categories *repositories.CategoryStore
	Directory  *services.DirectoryService
}

// Handler bundles dependencies for directory handler methods.
type Handler struct {
	deps Deps
}
