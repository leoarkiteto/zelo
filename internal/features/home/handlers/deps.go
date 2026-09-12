// Package handlers contains home HTTP handlers and route registration.
package handlers

import (
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
)

// Deps are the collaborators used by home handlers.
type Deps struct {
	Roles middleware.RoleChecker
	Audit middleware.AuditRecorder
}

// Handler bundles dependencies for home handler methods.
type Handler struct {
	deps Deps
}
