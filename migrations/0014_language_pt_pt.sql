-- 0014: add European Portuguese (pt-pt) as a supported interface language.
-- Replaces the check from 0008; see internal/shared/i18n/language.go.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_language_preference_check;

ALTER TABLE users ADD CONSTRAINT users_language_preference_check
    CHECK (language_preference IN ('en', 'pt-br', 'pt-pt'));
