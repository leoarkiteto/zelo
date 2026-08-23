# HTTP Endpoints: i18n Language Switcher

**Feature**: `005-i18n-language-switcher` | **Contract type**: server-rendered web endpoints

Both routes are registered by `internal/features/profile/handlers` and are protected by the existing `RequireAuth` middleware, so anonymous requests redirect to `/login` (`302 See Other`).

## GET /profile

- **Purpose**: render the profile page containing the language toggle.
- **Auth**: required (`RequireAuth`).
- **Success**: `200 OK`, HTML page rendered with the authenticated shell.
  - The page includes the toggle with the currently resolved locale marked as selected.
  - The root shell element is `<div id="app-shell" lang="{locale}">` where `{locale}` is `en` or `pt-br`.
- **Anonymous**: `302` to `/login`.

## POST /profile/language

- **Purpose**: persist the user's chosen language and return the updated interface.
- **Auth**: required (`RequireAuth`).
- **CSRF**: required — the form must include `csrf_token` (enforced by the existing CSRF middleware).
- **Request body**: `application/x-www-form-urlencoded`
  - `language`: one of `en` | `pt-br` (required)
- **Valid request — HTMX** (`HX-Request: true` header present):
  - Persists the preference via `UserStore.UpdateLanguagePreference`.
  - Returns `200 OK` with the re-rendered authenticated shell:
    `<div id="app-shell" lang="{new-locale}">…</div>`
  - No redirect; the browser URL stays `/profile`.
- **Valid request — plain form fallback** (no HTMX header):
  - Persists the preference, then returns `302 See Other` to `/profile`.
- **Invalid language** (missing, or not `en`/`pt-br`):
  - Returns `400 Bad Request` with the shared error page in the user's current locale.
  - No state change.
- **Persistence failure**:
  - Returns `500 Internal Server Error` with the shared error page in the user's current locale.
  - No language change is applied to the response.

## Status summary

| Route | Auth | Success | Anonymous | Invalid input |
|---|---|---|---|---|
| `GET /profile` | required | `200` page | `302 /login` | n/a |
| `POST /profile/language` | required | `200` (HTMX) or `302 /profile` | `302 /login` | `400` |
