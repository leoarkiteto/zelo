# Data Model: Service Provider Directory

Feature: `specs/003-service-directory/spec.md` — persistence model and validation rules
for the directory slice. Implementation details (migration SQL, repository code) belong
to `tasks.md` and the implementation phase. Access is via `database/sql` + the `pgx`
stdlib driver with plain SQL.

## Entities

### ServiceCategory

A speciality/category that listings can be filed under, scoped to a condominium and
maintained by the syndic.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK, default `gen_random_uuid()` | Immutable |
| `condominium_id` | UUID, FK → `condominiums.id` ON DELETE CASCADE | Required |
| `name` | text | Required, 1–80 chars |
| `active` | boolean | Default `true`; `false` means deactivated |
| `created_at` | timestamptz | Default `now()` |
| `updated_at` | timestamptz | Updated on rename/deactivation |

Validation:
- Case-insensitive unique active name per condominium via partial unique index:
  `UNIQUE (condominium_id, lower(name)) WHERE active = true`.
- Deactivation sets `active = false`; the row is retained so existing listings keep
  their category. A deactivated category is not offered in submission or filter forms.

State transitions:
- `active` → `inactive` (syndic deactivates).
- No re-activation in v1; the syndic adds a new category if needed.

### ServiceProviderListing

A recommended service professional submitted by a resident.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK, default `gen_random_uuid()` | Immutable |
| `condominium_id` | UUID, FK → `condominiums.id` ON DELETE CASCADE | Required; set from the submitter's session condominium |
| `category_id` | UUID, FK → `service_categories.id` ON DELETE RESTRICT | Required; must be an active category in the same condominium at submission time |
| `name` | text | Required, 1–120 chars |
| `phone` | text | Required; as entered, 7–15 digits with optional `+`, spaces, dashes, parentheses |
| `phone_digits` | text | Required; normalized digits-only copy used for duplicate detection |
| `notes` | text, nullable | Optional, max 500 chars |
| `recommended_by_unit_id` | UUID, FK → `units.id` ON DELETE SET NULL, nullable | The resident's unit at submission time |
| `recommended_by_unit_code` | text | Required; snapshot of `units.code` shown in the directory |
| `submitted_by` | UUID, FK → `users.id` ON DELETE CASCADE | Required; the resident who submitted |
| `created_at` | timestamptz | Default `now()` |
| `updated_at` | timestamptz | Updated on syndic edit |

Validation:
- `phone_digits` is derived from `phone` (digits only) and never user-supplied.
- Duplicate warning (not a block): on submission, if another listing in the same
  condominium has the same `phone_digits`, the form is re-rendered with a warning;
  saving requires `confirm_duplicate=1`.
- No unique constraint on `phone_digits`; duplicates are allowed.
- Residents cannot edit or delete listings (service + route level, spec FR-020).

State transitions:
- No approval lifecycle: a listing becomes visible as soon as it is created.
- Deletion is a hard delete (plus an audit event); there is no draft or archived state.

## Relationships

```text
ServiceCategory N ──── 1 Condominium
ServiceProviderListing N ──── 1 ServiceCategory
ServiceProviderListing N ──── 1 Condominium
ServiceProviderListing N ──── 1 User (submitted_by)
ServiceProviderListing N ──── 0..1 Unit (recommended_by_unit_id)
```

- A `ServiceCategory` belongs to exactly one `Condominium`.
- A `ServiceProviderListing` belongs to exactly one `Condominium`, one active
  `ServiceCategory`, and one submitting `User`.
- A `ServiceProviderListing` may reference at most one `Unit`; the unit reference is
  nullable so historical listings survive unit deletion.

## Planned migrations

1. `0007_create_service_directory.sql` — `service_categories` (with partial unique
   index), `service_provider_listings`, indexes for search and duplicate detection:
   - `idx_service_categories_condominium` on `(condominium_id)`.
   - `idx_listings_condominium_phone` on `(condominium_id, phone_digits)`.
   - `idx_listings_condominium_category` on `(condominium_id, category_id)`.
