# Quickstart: i18n Language Switcher

**Feature**: `005-i18n-language-switcher` | **Phase 1 output** | **Date**: 2026-08-23

Validation guide for proving the language toggle works end to end. Contract details live in [contracts/](./contracts/) and entity/storage details in [data-model.md](./data-model.md).

## Prerequisites

- Go 1.27
- A running PostgreSQL database
- `templ` CLI on `PATH`
- Node.js + npm (Tailwind CSS build only)

## Setup

```bash
cp .env.example .env      # fill DATABASE_URL, SESSION_SECRET, PASSWORD_PEPPER
make migrate              # apply migrations, including 0008_add_user_language_preference.sql
go run ./cmd/web -seed    # idempotent dev seed (syndic@example.com / syndic-password-123)
make templ                # generate *_templ.go
make tailwind             # build CSS
```

## Run

```bash
make run                  # http://localhost:8080
```

## Validation scenarios

### 1. Default language is English

- Open `http://localhost:8080/login` while logged out.
- **Expected**: every visible string is English (`Sign in`, navigation labels, buttons).

### 2. Profile page shows the toggle with flags

- Sign in as `syndic@example.com` / `syndic-password-123`.
- Open `http://localhost:8080/profile`.
- **Expected**: the page shows two options — `🇺🇸 English` and `🇧🇷 Português (BR)` — with English marked selected.

### 3. Toggle switches the whole interface to Portuguese

- On `/profile`, select `🇧🇷 Português (BR)`.
- **Expected**: topbar, sidebar, and page content all change to Brazilian Portuguese; the URL stays `/profile`; the shell carries `lang="pt-br"`; the Português option becomes selected.

### 4. Preference is saved and applied at login

- Sign out, then sign in again.
- **Expected**: the dashboard appears in Brazilian Portuguese immediately after login.
- Optional DB check:
  ```sql
  SELECT language_preference FROM users WHERE email = 'syndic@example.com';
  ```
  **Expected**: `pt-br`.

### 5. Toggle back to English on another device

- Sign in from a different browser/incognito window and toggle to `🇺🇸 English`.
- **Expected**: English is applied immediately; signing in on the first browser also shows English (preference is per account, not per device).

### 6. Missing-translation fallback (defensive)

- Temporarily remove one `pt-br` catalog entry and render a page that uses it.
- **Expected**: the English text is shown; no technical code, placeholder identifier, or blank text appears (FR-009).
- Restore the entry before finishing.

## Automated checks

```bash
go test ./...                       # unit + handler + smoke tests (TDD gate)
scripts/check-feature-boundaries.sh # vertical-slice isolation gate
```

**Expected**: all tests pass and the boundary check reports no cross-slice violations.
