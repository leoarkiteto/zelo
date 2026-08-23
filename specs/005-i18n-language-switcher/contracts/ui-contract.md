# UI Contract: Language Toggle

**Feature**: `005-i18n-language-switcher` | **Contract type**: user interface

## Location

- The toggle lives on the profile page (`GET /profile`), inside the authenticated app shell.
- It is not shown on pages rendered before login; those pages render in the default language, English.

## Anatomy

The toggle presents exactly two options:

| Option | Label | Flag | Value |
|---|---|---|---|
| English | `English` | 🇺🇸 (United States) | `en` |
| Português (BR) | `Português (BR)` | 🇧🇷 (Brazil) | `pt-br` |

- Each option shows its flag and its label.
- The flag emoji is marked `aria-hidden="true"`; the label carries the accessible name, so the flag is never the only way to identify an option (FR-012).
- The currently active language is clearly indicated as selected/checked.

## Behavior

- The toggle is an HTMX form:
  - `hx-post="/profile/language"`
  - `hx-target="#app-shell"`
  - `hx-swap="outerHTML"`
  - includes hidden `csrf_token` and the selected `language` value.
- On success the server returns the full authenticated shell as `<div id="app-shell" lang="{new-locale}">…</div>`; HTMX swaps it in place, so the entire visible interface (topbar, sidebar, page content) updates in the selected language in one round-trip.
- The URL remains `/profile`; the user is not navigated away.
- Any form inputs on the profile page that must survive the toggle live outside the `#app-shell` swap target (or are otherwise preserved), so unsaved input is never lost (FR-003).
- Plain-form (non-HTMX) fallback still works: the server redirects back to `/profile` after saving.

## Accessibility

- The toggle is keyboard-operable and announced by assistive technology.
- The change of language is announced; focus is not lost during the HTMX swap.
- The `lang` attribute on `#app-shell` (and on `<html>` for initial loads) reflects the active locale so screen readers pronounce content correctly.
