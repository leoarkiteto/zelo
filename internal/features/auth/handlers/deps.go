// Package handlers contains auth HTTP handlers and route registration.
package handlers

import (
	"log/slog"

	"github.com/leoarkiteto/zelo/internal/features/auth/services"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
	"github.com/leoarkiteto/zelo/internal/shared/security"
	"github.com/leoarkiteto/zelo/internal/shared/store"
)

// Deps are the collaborators used by auth handlers.
type Deps struct {
	Logger        *slog.Logger
	Sessions      *security.SessionManager
	Invitations   *store.InvitationStore
	Tokens        security.TokenHasher
	Audit         middleware.AuditRecorder
	Registration  *services.RegistrationService
	AuthService   *services.AuthService
	PasswordReset *services.PasswordResetService
}

// Handler bundles dependencies for auth handler methods.
type Handler struct {
	deps Deps
}
