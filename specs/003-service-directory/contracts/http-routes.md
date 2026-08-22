# HTTP Routes Contract: Service Provider Directory

Feature: `specs/003-service-directory/spec.md`

All routes are server-rendered HTML. State-changing routes (`POST`) require a valid
CSRF token submitted as a form field (`csrf_token`); an invalid or missing token
returns `403 Forbidden`.

Access model:
- Unauthenticated visitors are redirected to `/login` (existing `RequireAuth`).
- Authenticated users with no active role in their session condominium receive `403`
  with a friendly explanation (existing `RequireRole` deny path).
- Member routes allow `owner`, `tenant`, and `syndic` roles.
- Syndic routes allow `syndic` only.

## Member routes (any authenticated condominium member)

| Method | Path | Purpose | Success | Errors |
|--------|------|---------|---------|--------|
| GET | `/directory` | Show directory with search form; `q` (keyword) and `category` (category id) query params filter results | `200` | — |
| GET | `/directory/new` | Show listing submission form with active categories | `200` | — |
| POST | `/directory` | Create a listing | `303` → `/directory` | `400` validation errors re-rendered; duplicate phone without `confirm_duplicate=1` re-renders with warning |

## Syndic routes (moderation and category management)

| Method | Path | Purpose | Success | Errors |
|--------|------|---------|---------|--------|
| GET | `/directory/{id}/edit` | Show edit form for a listing | `200` | `404` listing not found |
| POST | `/directory/{id}/edit` | Update name, category, phone, notes | `303` → `/directory` | `400` validation errors re-rendered; `404` listing not found |
| GET | `/directory/{id}/delete` | Show delete confirmation page | `200` | `404` listing not found |
| POST | `/directory/{id}/delete` | Delete a listing after page confirmation | `303` → `/directory` | `404` listing not found |
| GET | `/directory/categories` | List active and deactivated categories with management forms | `200` | — |
| POST | `/directory/categories` | Add a category | `303` → `/directory/categories` | `400` validation error (including duplicate active name) re-rendered |
| POST | `/directory/categories/{id}/rename` | Rename a category | `303` → `/directory/categories` | `400` validation error; `404` category not found |
| POST | `/directory/categories/{id}/deactivate` | Deactivate a category | `303` → `/directory/categories` | `404` category not found |

Deleting a listing while another user is viewing it: the next action on that listing
(`edit`/`delete`) returns `404` with a "no longer available" message (spec edge case).

## Listing form contract

Used by `GET/POST /directory/new` (resident submission) and `GET/POST
/directory/{id}/edit` (syndic edit).

| Field | Submission | Edit | Validation |
|-------|------------|------|------------|
| `name` | required | required | 1–120 characters |
| `category_id` | required | required | Must be an active category in the session condominium |
| `phone` | required | required | 7–15 digits; optional `+`, spaces, dashes, parentheses |
| `notes` | optional | optional | Max 500 characters |
| `confirm_duplicate` | optional | not used | For submission only; presence (`1`) allows saving despite a duplicate phone warning |
| `csrf_token` | required | required | Valid CSRF token |

Submission duplicate flow:
1. First `POST /directory` without `confirm_duplicate` and with a `phone_digits` value
   already present in the condominium → `200` re-render of the form with a warning.
2. Resident resubmits with `confirm_duplicate=1` → listing is saved; `303` to
   `/directory`.
3. No duplicate → saved directly on the first `POST`.

The resident never supplies `recommended_by_unit_*` or `submitted_by`; these come from
the session user and their active unit occupancy. Editing as syndic applies changes
directly (no duplicate warning) because the syndic is the moderator.

## Category form contracts

| Field | Add | Rename | Validation |
|-------|-----|--------|------------|
| `name` | required | required | 1–80 characters; case-insensitive unique among active categories in the condominium |
| `csrf_token` | required | required | Valid CSRF token |

Deactivation form sends only `csrf_token`; there is no delete for categories. Renaming
a category immediately changes the name shown on its existing listings (via the FK
join). Deactivating a category removes it from submission and filter lists; existing
listings keep their category and remain searchable by keyword.
