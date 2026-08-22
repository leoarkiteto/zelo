# Quickstart Validation Guide: Service Provider Directory

Feature: `specs/003-service-directory/spec.md`

Runnable validation scenarios that prove the feature end-to-end after implementation.
Details live in [data-model.md](./data-model.md) and
[contracts/http-routes.md](./contracts/http-routes.md).

## Prerequisites

- Go 1.27+
- Docker with Compose
- Existing auth/RBAC slice working (users, roles, sessions, CSRF)
- `templ` and Tailwind tooling available (pinned via `tools/` during implementation)

## Setup

```bash
# 1. Start PostgreSQL
docker compose up -d

# 2. Configure environment
cp .env.example .env
#    DATABASE_URL=postgres://zelo:zelo@localhost:5432/zelo?sslmode=disable
#    SESSION_SECRET=<32+ random bytes>
#    APP_ENV=development

# 3. Apply migrations (includes 0007_create_service_directory.sql)
make migrate

# 4. Generate templates and run the server
make templ
make run
```

Seed or create, via the existing invitation flow: one syndic, one owner, and one tenant
in the same condominium (see the auth feature quickstart for bootstrap steps).

## Validation scenarios

### 1. Access control

1. Open `/directory` signed out → redirected to `/login`.
2. Sign in as the owner → `/directory` renders.
3. Sign in as the tenant → `/directory` renders.
4. Sign in as a user with no active role in their session condominium (if available) →
   `403`.

**Expected**: only authenticated condominium members reach the directory, per
[contracts/http-routes.md](./contracts/http-routes.md).

### 2. Resident submits a listing

1. Sign in as the owner and open `/directory/new`.
2. Submit a listing: name `Ana's Plumbing`, category `Plumber`, phone
   `+55 11 91234-5678`, notes `Fast and reliable`.
3. Confirm the listing appears in `/directory` showing name, category, phone, notes,
   and the owner's unit code.
4. Submit another listing with missing phone → form re-renders with a validation error
   and nothing is saved.

**Expected**: the listing is visible immediately and the recommending unit is recorded
automatically (spec FR-002, FR-003, FR-007).

### 3. Search and filter

1. Add a second listing in a different category (e.g., `Electrician`).
2. On `/directory`, search `plumbing` → only the plumbing listing appears.
3. Clear the search and filter by `Electrician` → only the electrician appears.
4. Search `zzz-no-match` → friendly empty state appears.

**Expected**: keyword matches name/category/notes and category filtering works
(spec FR-008, FR-009, FR-010).

### 4. Duplicate phone warning

1. Sign in as the tenant and open `/directory/new`.
2. Submit a listing with the same phone as `Ana's Plumbing` (any formatting, e.g.,
   `55 11 91234 5678`).
3. The form re-renders with a duplicate warning and the listing is NOT saved yet.
4. Resubmit with the confirmation (`confirm_duplicate=1`) → the listing is saved.

**Expected**: warning on first submit, saving allowed on confirmation (spec FR-021).

### 5. Syndic moderation

1. Sign in as the syndic and open `/directory`.
2. Edit a listing's phone number → the change is visible to residents immediately.
3. Delete another listing from its page with confirmation → the listing disappears
   from the directory and from search results.
4. Sign in as the owner and try to open `/directory/{id}/edit` for an existing listing
   → `403`; there are no edit/delete affordances for residents.

**Expected**: only the syndic can edit or delete listings (spec FR-013, FR-014, FR-020).

### 6. Category management

1. Sign in as the syndic and open `/directory/categories`.
2. Add category `Carpenter` → it appears and can be selected on `/directory/new`.
3. Rename `Carpenter` → existing listings in that category show the new name.
4. Deactivate `Carpenter` → it no longer appears in submission or filter lists, but
   existing `Carpenter` listings remain visible and keyword-searchable.
5. Add a duplicate active name → validation error is shown.

**Expected**: category lifecycle matches spec FR-011, FR-012 and
[contracts/http-routes.md](./contracts/http-routes.md).

### 7. Automated tests

```bash
go test ./...
```

**Expected**: stdlib-based unit and integration tests pass, covering the directory
service, store adapters, handlers, access control, duplicate warning, category
lifecycle, and the `0007_create_service_directory.sql` migration.
