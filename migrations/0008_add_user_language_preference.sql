-- 0008: i18n language switcher - per-user interface language preference.
-- NULL means "use the default language (en)". Values are constrained to the
-- two supported locales; see internal/shared/i18n/language.go.
ALTER TABLE users ADD COLUMN language_preference TEXT;

ALTER TABLE users ADD CONSTRAINT users_language_preference_check
    CHECK (language_preference IN ('en', 'pt-br'));
