package services

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// LanguagePreferenceReader loads the user whose preference is being read.
type LanguagePreferenceReader interface {
	GetUserByID(ctx context.Context, id string) (model.User, error)
}

// LanguagePreferenceUpdater persists a user's language preference.
type LanguagePreferenceUpdater interface {
	UpdateLanguagePreference(ctx context.Context, userID, language string) error
}
