// Package handlers contains profile HTTP handlers and route registration.
package handlers

import (
	"context"
	"log/slog"

	"github.com/leoarkiteto/zelo/internal/features/profile/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
	"github.com/leoarkiteto/zelo/internal/shared/middleware"
)

// Profile is the use-case surface the profile handlers depend on.
type Profile interface {
	GetProfile(ctx context.Context, userID string) (domain.Profile, error)
	ChangeLanguage(ctx context.Context, userID string, lang i18n.Language) error
}

// Deps are the collaborators used by profile handlers.
type Deps struct {
	Logger  *slog.Logger
	Roles   middleware.RoleChecker
	Profile Profile
}

// Handler bundles dependencies for profile handler methods.
type Handler struct {
	deps Deps
}
