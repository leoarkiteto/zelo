// Package handler contains HTTP handlers and route wiring.
package handler

import (
	"log/slog"

	"github.com/leoarkiteto/zelo/internal/auth"
	"github.com/leoarkiteto/zelo/internal/service"
	"github.com/leoarkiteto/zelo/internal/store"
)

// Dependencies are the concrete collaborators used by handlers.
type Dependencies struct {
	Logger        *slog.Logger
	Sessions      *auth.SessionManager
	Passwords     auth.PasswordHasher
	Tokens        TokenHasher
	Users         *store.UserStore
	Roles         *store.RoleStore
	Units         *store.UnitStore
	Invitations   *store.InvitationStore
	Audit         *store.AuditStore
	Registration  *service.RegistrationService
	AuthService   *service.AuthService
	PasswordReset *service.PasswordResetService
	RoleService   *service.RoleService
}

// TokenHasher adapts auth.HashToken to the service TokenHasher port.
type TokenHasher struct{}

// HashToken hashes an opaque token with SHA-256.
func (TokenHasher) HashToken(token string) string { return auth.HashToken(token) }

// Handler bundles dependencies for handler methods.
type Handler struct {
	deps Dependencies
}
