// Package handlers contains auth HTTP handlers and route registration.
package handlers

import (
	"log/slog"

	"github.com/leoarkiteto/zelo/internal/features/auth/core/ports"
	"github.com/leoarkiteto/zelo/internal/features/auth/core/services"
	"github.com/leoarkiteto/zelo/internal/shared/security"
)

// Deps are the collaborators used by auth handlers.
type Deps struct {
	Logger        *slog.Logger
	Sessions      *security.SessionManager
	Invitations   ports.InvitationStore
	Tokens        ports.TokenHasher
	Audit         ports.AuditRecorder
	Registration  *services.RegistrationService
	AuthService   *services.AuthService
	PasswordReset *services.PasswordResetService
}

// Handler bundles dependencies for auth handler methods.
type Handler struct {
	deps Deps
}
