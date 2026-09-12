// Package services implements profile application use cases.
package services

import (
	"context"

	"github.com/leoarkiteto/zelo/internal/features/profile/domain"
	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// ProfileService implements the profile use cases: reading the profile and
// changing the interface language (FR-004/FR-005).
type ProfileService struct {
	Users       LanguagePreferenceReader
	Preferences LanguagePreferenceUpdater
}

// GetProfile returns the signed-in user's profile with the resolved language.
func (s *ProfileService) GetProfile(ctx context.Context, userID string) (domain.Profile, error) {
	u, err := s.Users.GetUserByID(ctx, userID)
	if err != nil {
		return domain.Profile{}, err
	}
	return domain.Profile{
		UserID:   u.ID,
		Language: i18n.Resolve(u.LanguagePreference),
	}, nil
}

// ChangeLanguage validates and persists the user's chosen language. Invalid
// codes are rejected with domain.ErrInvalidLanguage and nothing is saved.
func (s *ProfileService) ChangeLanguage(ctx context.Context, userID string, lang i18n.Language) error {
	if err := domain.ValidateLanguage(lang); err != nil {
		return err
	}
	return s.Preferences.UpdateLanguagePreference(ctx, userID, lang.String())
}
