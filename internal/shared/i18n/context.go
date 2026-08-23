package i18n

import "context"

type ctxKey int

const languageKey ctxKey = iota

// WithLanguage stores the resolved interface language in ctx.
func WithLanguage(ctx context.Context, lang Language) context.Context {
	return context.WithValue(ctx, languageKey, lang)
}

// LanguageFrom returns the resolved interface language from ctx, defaulting
// to English when unset or invalid (FR-007).
func LanguageFrom(ctx context.Context) Language {
	if l, ok := ctx.Value(languageKey).(Language); ok && l.Valid() {
		return l
	}
	return Default()
}

// Resolve maps a stored preference string to a Language. Empty or invalid
// values resolve to the default language, English (FR-007/FR-009).
func Resolve(pref string) Language {
	l := Language(pref)
	if l.Valid() {
		return l
	}
	return Default()
}
