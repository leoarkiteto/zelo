// Package i18n provides the shared interface-language support: the two
// supported locales, the message catalog, and per-request locale context.
package i18n

// Language is one of the supported interface languages.
type Language string

const (
	// LanguageEN is English, the default interface language.
	LanguageEN Language = "en"
	// LanguagePTBR is Brazilian Portuguese.
	LanguagePTBR Language = "pt-br"
)

// Valid reports whether l is a supported interface language.
func (l Language) Valid() bool {
	return l == LanguageEN || l == LanguagePTBR
}

// Default returns the default interface language, English (FR-007).
func Default() Language {
	return LanguageEN
}

// String returns the language code (e.g. "en", "pt-br").
func (l Language) String() string {
	return string(l)
}
