// Package handlers contains profile HTTP handlers and route registration.
package handlers

import (
	"log/slog"

	"github.com/leoarkiteto/zelo/internal/features/profile/core/ports"
	"github.com/leoarkiteto/zelo/internal/shared/httpx"
)

// Deps are the collaborators used by profile handlers.
type Deps struct {
	Logger  *slog.Logger
	Roles   httpx.RoleReader
	Profile ports.ProfileService
}

// Handler bundles dependencies for profile handler methods.
type Handler struct {
	deps Deps
}
