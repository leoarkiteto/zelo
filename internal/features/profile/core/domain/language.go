// Package domain contains profile domain rules.
package domain

import (
	"errors"

	"github.com/leoarkiteto/zelo/internal/shared/i18n"
)

// ErrInvalidLanguage is returned when a language code is not supported.
var ErrInvalidLanguage = errors.New("invalid language")

// Profile is the signed-in user's profile view shown on the profile page.
type Profile struct {
	UserID   string
	Language i18n.Language
}

// LanguagePreference is the account's chosen interface language.
type LanguagePreference struct {
	Language i18n.Language
}

// DefaultLanguagePreference returns the default (English) preference.
func DefaultLanguagePreference() LanguagePreference {
	return LanguagePreference{Language: i18n.Default()}
}

// ResolveLanguagePreference builds a preference from a stored string, falling
// back to the default language for empty or invalid values (FR-007/FR-009).
func ResolveLanguagePreference(pref string) LanguagePreference {
	return LanguagePreference{Language: i18n.Resolve(pref)}
}

// ValidateLanguage rejects unsupported codes so the toggle never persists an
// invalid value (data-model.md validation rules).
func ValidateLanguage(l i18n.Language) error {
	if !l.Valid() {
		return ErrInvalidLanguage
	}
	return nil
}
