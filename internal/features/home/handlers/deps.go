// Package handlers contains home HTTP handlers and route registration.
package handlers

import (
	"context"

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

// Deps are the collaborators used by home handlers.
type Deps struct {
	Roles Roles
	Audit AuditRecorder
}

// Handler bundles dependencies for home handler methods.
type Handler struct {
	deps Deps
}
