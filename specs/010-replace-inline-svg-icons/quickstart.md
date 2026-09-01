# Quickstart Validation Guide: Replace Inline SVG Icons

**Feature**: 010-replace-inline-svg-icons
**Purpose**: Runnable validation scenarios that prove the icon migration works
end-to-end before the implementation is accepted.

## Prerequisites

- Go 1.27 installed (`go version`)
- `make` available
- Node/npm available for `make tailwind` (or prebuilt `web/static/css/output.css` committed)
- Docker services only if running the full integration suite
  (`docker compose up -d` for PostgreSQL 16 + Redis 7)

## 1. Build and unit-test the shared component

```bash
make templ      # regenerate *_templ.go after .templ edits
make tailwind   # rebuild web/static/css/output.css from assets/css/input.css
go test ./internal/shared/templates/...
```

**Expected**: all shared-template tests pass, including the updated `Icon`
render tests that assert the Material Symbols span, class list, `data-icon`
hook, and fallback for unknown kinds.

## 2. Full test suite and boundary checks

```bash
make check
```

**Expected**: `go test ./...`, the feature-boundary script, and the
template-atomic-boundary script all pass.

## 3. No hand-authored SVG icons remain

```bash
rg '<svg' internal --glob '*.templ'
```

**Expected**: no matches in production templates. (The generated
`*_templ.go` files are excluded from this check by design.)

## 4. Password visibility toggle (manual or integration)

Start the server and open `/login`:

```bash
make run
# open http://localhost:<HTTP_ADDR>/login
```

1. Click the show/hide password control.
2. Confirm the password field switches between masked and visible.
3. Confirm the icon switches between the eye and eye-off glyphs.
4. Confirm the button's accessible label and pressed state change with each
   toggle.
5. Tab to the control and repeat with the keyboard.

**Expected**: identical behavior to the current page, now rendered with
Material Symbols icons.

## 5. Icon spot checks on authenticated pages

With a seeded dev environment (`go run ./cmd/web -seed`), open:

- A dashboard/area page — breadcrumb separator and link arrows render in the
  same positions and sizes as before.
- The mobile layout (narrow viewport) — the menu toggle renders the Material
  Symbols menu glyph.

**Expected**: icons appear in the same position and size; no layout shifts.

## 6. Full integration suite (optional, requires Docker)

```bash
docker compose up -d
TEST_DATABASE_URL=... TEST_REDIS_URL=... go test ./tests/...
```

**Expected**: full-app flows pass, including login and any page flows that
render the migrated icons.

## Definition of Done for Validation

- Commands in sections 1–3 pass locally.
- Sections 4–5 verified manually, or covered by the integration suite in
  section 6.
- `make check` is green with the regenerated Templ and Tailwind artifacts
  committed.
