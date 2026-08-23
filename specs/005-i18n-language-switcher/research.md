# Research: i18n Language Switcher

**Feature**: `005-i18n-language-switcher` | **Phase 0 output** | **Date**: 2026-08-23

All unknowns from the Technical Context are resolved below. Each entry records the decision, the rationale, and the alternatives that were considered.

## 1. Where to store the language preference

- **Decision**: Add a single nullable `language_preference TEXT` column to the existing `users` table with a `CHECK (language_preference IN ('en','pt-br'))` constraint.
- **Rationale**: The preference is a 1:1 attribute of the account, is read on every authenticated request, and must follow the user across sessions and devices. A column on `users` is the simplest storage that satisfies FR-005/FR-006 and is loaded for free by the existing `GetUserByID` call in `WithUser`.
- **Alternatives considered**:
  - Separate `user_preferences` key/value table — more flexible, but overkill for one value and adds a join to every request.
  - Session-only storage — does not survive across sessions/devices; violates the spec.
  - Browser cookie — tied to the device, not the account; does not apply at login on other devices.

## 2. How to represent the two locales without new dependencies

- **Decision**: A small `Language` type in `internal/shared/i18n` with constants `LanguageEN = "en"` and `LanguagePTBR = "pt-br"`, a `Valid()` method, and `Default()` returning `en`. The message catalog is a plain Go structure keyed by language and message key.
- **Rationale**: The constitution mandates the Go standard library first and forbids ORMs/frameworks. Two locales do not justify an i18n framework. A typed value object prevents invalid strings from reaching storage and gives compile-time safety in templates.
- **Alternatives considered**:
  - `golang.org/x/text` — present transitively, but a full message-printer setup adds complexity for two static locales.
  - A third-party i18n library — violates the standard-library-first principle without material benefit.
  - Database-driven translations — adds runtime queries for static UI text; unnecessary.

## 3. How to apply the locale to each request

- **Decision**: Add a `WithLocale` middleware that runs after `WithUser`. It reads the authenticated user's `LanguagePreference` (default `en` when empty or invalid) and stores the resolved language in the request context. Handlers and templates read it via helpers in `internal/shared/i18n`.
- **Rationale**: This mirrors the existing `WithUser` pattern, keeps every handler free of locale-resolution code, and makes the default-English rule (FR-007) apply uniformly — including before login and after login with no saved preference.
- **Alternatives considered**:
  - Resolve the locale inside each handler — repetitive and error-prone.
  - URL prefix (`/pt-br/...`) — changes every route and complicates redirects for two locales.
  - Cookie-first resolution — conflicts with account-level persistence and adds cookie handling.

## 4. How to render country flags in the UI

- **Decision**: Use Unicode regional-indicator emoji — 🇺🇸 for English and 🇧🇷 for Portuguese — rendered `aria-hidden="true"` and always paired with the visible, accessible label ("English" / "Português (BR)").
- **Rationale**: Zero assets, zero JavaScript, works in the CSS-first Tailwind UI, and satisfies FR-002/FR-012 (flags plus accessible text labels). Emoji flags are the lightest way to show a country flag in a server-rendered HTML page.
- **Alternatives considered**:
  - Inline SVG flags — sharper across platforms but more markup and maintenance.
  - Static flag image assets — extra asset pipeline work for two flags.
  - Flag-only labels — fails accessibility and fails FR-012.

## 5. How the toggle updates the whole UI without losing state

- **Decision**: The profile page renders the authenticated shell inside a single `<div id="app-shell" lang="{locale}">`. The language toggle is an HTMX form: `hx-post="/profile/language"`, `hx-target="#app-shell"`, `hx-swap="outerHTML"`. The server responds with the re-rendered shell carrying the new `lang`. Any unrelated form fields are kept outside the swapped region, so their input is preserved.
- **Rationale**: This is the constitution-approved progressive-enhancement path: one HTMX request, no custom JavaScript, server-rendered output, and immediate whole-UI update in a single round-trip. The `lang` attribute moves with the swapped shell so assistive technology announces content in the new language.
- **Alternatives considered**:
  - Plain form POST + redirect back — causes a full page reload and loses unsaved input; fails FR-003.
  - Custom fetch/JS state swap — violates the CSS-first, HTMX-only directive.
  - Full-page `hx-boost` navigation — still a navigation, not a targeted swap.

## 6. Date, time, and number formatting per locale

- **Decision**: A small set of locale-aware formatting helpers in `internal/shared/i18n` using the Go standard library: `time.Format` with locale-specific layouts (month/day names in English and Portuguese) and simple number formatting helpers for the two locales.
- **Rationale**: The feature only needs `en` and `pt-br` conventions (FR-010). Hand-written helpers keep the standard-library-first constraint intact.
- **Alternatives considered**:
  - `golang.org/x/text/message` — supports locale-aware printing but introduces a direct dependency for marginal benefit over two hardcoded locale profiles.
  - Ignoring date/number formatting in v1 — would leave FR-010 unmet.

## 7. Where the profile page lives

- **Decision**: Create a new vertical slice `internal/features/profile/` via `scripts/new-feature.sh profile`, exposing `GET /profile` (page with the toggle) and `POST /profile/language` (save + re-render). Wire it in `cmd/web/main.go`.
- **Rationale**: No profile page exists today (`grep profile` finds nothing). The user explicitly placed the toggle on the profile page, so the slice is new. A dedicated slice satisfies vertical-slice isolation.
- **Alternatives considered**:
  - Put the toggle in the topbar or dashboard — contradicts the user's explicit instruction.
  - Add the profile page inside `internal/features/auth` — mixes concerns and violates slice isolation.

## 8. How translations are organized and missing-message fallback handled

- **Decision**: The catalog lives in `internal/shared/i18n/catalog.go` as static Go data (`map[Language]map[MessageKey]string`). A `T(lang, key)` lookup returns the message for the requested language; if a key is missing for that language, it returns the English value. A test enumerates keys to prove both languages are complete before implementation is accepted.
- **Rationale**: Static Go data is fast, type-safe, testable, and keeps templates simple (`i18n.T(locale, "nav.dashboard")`). English fallback implements FR-009 without runtime lookups.
- **Alternatives considered**:
  - JSON/YAML message files with a loader — adds file I/O and parsing for no runtime benefit.
  - Templ-level translation components — scatters translation state across templates and makes completeness testing harder.
  - Silent empty fallback — violates FR-009.
