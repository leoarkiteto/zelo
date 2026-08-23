# Data Model: i18n Language Switcher

**Feature**: `005-i18n-language-switcher` | **Phase 1 output** | **Date**: 2026-08-23

## Entities

### User (existing, extended)

The account that owns a language preference.

| Field | Type | Rules |
|---|---|---|
| `id` | UUID | existing, primary key |
| `email` | TEXT | existing |
| `password_hash` | TEXT | existing |
| `status` | user_status | existing (`active` / `disabled`) |
| `language_preference` | TEXT, nullable | NEW — must be `'en'` or `'pt-br'` when set; `NULL` means "use default (`en`)" |

- A `User` has exactly zero or one `language_preference`.
- Validation: the value is constrained by `CHECK (language_preference IN ('en','pt-br'))` in the migration and by the `Language` value object in code.

### Language (domain value object, new)

The two supported interface languages.

| Value | Display label | Flag | Default |
|---|---|---|---|
| `en` | English | 🇺🇸 (United States) | yes |
| `pt-br` | Português (BR) | 🇧🇷 (Brazil) | no |

- `Language.Valid()` rejects anything other than `en` and `pt-br`.
- `Language.Default()` returns `en`.
- Used in the request context, the message catalog, and the persisted preference.

### Message Catalog (in-memory, new)

Static UI text for every user-facing string, keyed by language.

- Structure: `map[Language]map[MessageKey]string`, e.g. `en["nav.dashboard"] = "Dashboard"`, `pt-br["nav.dashboard"] = "Painel"`.
- Every key MUST exist in both languages (enforced by a completeness test).
- Lookup `T(lang, key)` returns the requested language's value; on a missing key it returns the English value (FR-009).

## Relationships

- `User 1 — 0..1 Language` (via `users.language_preference`).

## State Transitions

Language preference lifecycle:

```text
(unset / NULL)
      │
      │ first login or anonymous request → resolved to en (default)
      ▼
    en ◄────────── toggle to en ──────────┐
      │                                    │
      │ toggle to pt-br                    │
      ▼                                    │
   pt-br ──────────────────────────────────┘
```

- `NULL` is never exposed to templates: the request context always carries a resolved `en` or `pt-br`.
- Toggle endpoints only accept a valid value; invalid input is rejected with a `400` and no state change.

## Storage

- Migration `0008_add_user_language_preference.sql`:
  - `ALTER TABLE users ADD COLUMN language_preference TEXT;`
  - `ALTER TABLE users ADD CONSTRAINT users_language_preference_check CHECK (language_preference IN ('en','pt-br'));`
- `UserStore` gains:
  - `UpdateLanguagePreference(ctx, userID, language string) error`
  - `GetUserByID`/`GetUserByEmail` extended to scan `language_preference` into `model.User.LanguagePreference`.
- No new tables, no new entities persisted beyond `users.language_preference`.

## Validation Rules

- `language` form input: `"en"` or `"pt-br"` exactly; otherwise `400`.
- Missing/`NULL` stored preference: resolve to `en`.
- Stored value that is somehow invalid (defensive): treat as `en`, matching FR-007/FR-009 fallback.
- User-generated content is never passed through the catalog or machine-translated (FR-011).
