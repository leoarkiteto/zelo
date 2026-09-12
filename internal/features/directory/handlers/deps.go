// Package handlers contains directory HTTP handlers and route registration.
package handlers

import (
	"github.com/leoarkiteto/zelo/internal/features/directory/repositories"
	"github.com/leoarkiteto/zelo/internal/features/directory/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
)

// Deps are the collaborators used by directory handlers.
type Deps struct {
	Roles      middleware.RoleChecker
	Audit      middleware.AuditRecorder
	Listings   *repositories.ListingStore
	Categories *repositories.CategoryStore
	Directory  *services.DirectoryService
}

// Handler bundles dependencies for directory handler methods.
type Handler struct {
	deps Deps
}
