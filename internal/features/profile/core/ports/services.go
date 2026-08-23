package ports

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/profile/core/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// ProfileService is the application use-case port consumed by profile handlers.
type ProfileService interface {
	GetProfile(ctx context.Context, userID string) (domain.Profile, error)
	ChangeLanguage(ctx context.Context, userID string, lang i18n.Language) error
}
