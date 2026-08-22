# Data Model: RBAC Authentication & Authorization

Feature: `specs/001-rbac-auth/spec.md` — persistence model and validation rules for the
auth/RBAC slice. Implementation details (migration SQL, repository code) belong to
`tasks.md` and the implementation phase. Access is via `database/sql` + the `pgx`
stdlib driver with plain SQL.

## Entities

### User

Represents a person with an account.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK, default `gen_random_uuid()` | Immutable |
| `email` | text, unique (via unique index on `lower(email)`) | Required, normalized to lowercase, valid email shape |
| `password_hash` | text | Required; Argon2id PHC string, never plaintext |
| `status` | enum `user_status` (`active`, `disabled`) | Default `active`; `disabled` blocks sign-in |
| `failed_sign_in_count` | integer | Default `0`; increments on failed sign-in, resets on success or lock expiry |
| `locked_until` | timestamptz, nullable | When set in the future, sign-in is rejected (FR-016) |
| `created_at` | timestamptz | Default `now()` |
| `updated_at` | timestamptz | Updated on changes |

Validation:
- Email is unique case-insensitively.
- Password is between 12 and 256 characters at registration (validated before hashing).
- After 5 consecutive failed sign-in attempts, `locked_until` is set to
  `now() + 15 minutes` and `failed_sign_in_count` resets when the lock expires.

### Condominium

Scope within which roles apply.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK | Immutable |
| `name` | text | Required |
| `created_at` | timestamptz | Default `now()` |

### Unit

A property within a condominium.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK | Immutable |
| `condominium_id` | UUID, FK → `condominiums.id` | Required |
| `code` | text | Required; unique within a condominium via `UNIQUE(condominium_id, code)` |

### UnitOccupancy

Ties a user to a unit as either its owner or its tenant. Kept separate from roles so
the "syndic must be an owner" rule can be checked against a factual relationship.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK | Immutable |
| `user_id` | UUID, FK → `users.id` | Required |
| `unit_id` | UUID, FK → `units.id` | Required |
| `occupancy_type` | enum `occupancy_type` (`owner`, `tenant`) | Required |
| `started_at` | timestamptz | Default `now()` |
| `ended_at` | timestamptz, nullable | When set, the occupancy is historical |

Validation:
- `UNIQUE(user_id, unit_id, occupancy_type)` prevents duplicates.
- A user cannot hold active `owner` and active `tenant` occupancies for the same unit
  at the same time (service rule).

### UserRole

RBAC role assignment scoped to a condominium.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK | Immutable |
| `user_id` | UUID, FK → `users.id` | Required |
| `condominium_id` | UUID, FK → `condominiums.id` | Required |
| `role` | enum `user_role` (`syndic`, `owner`, `tenant`) | Required |
| `granted_at` | timestamptz | Default `now()` |
| `revoked_at` | timestamptz, nullable | When set, the assignment is no longer active |

Validation:
- Active assignments have `revoked_at IS NULL`.
- `UNIQUE(user_id, condominium_id, role)` prevents duplicate assignments.
- Partial unique index on `(user_id, condominium_id) WHERE role IN ('owner','tenant')`
  enforces that a user cannot hold both owner and tenant in the same condominium.
- `syndic` requires an active `owner` role in the same condominium (enforced by the
  service layer with an integration test; a DB trigger is optional and may be added).
- `syndic` must never be assigned to a user with an active `tenant` role (service rule).
- Only the current syndic of the condominium may grant/revoke roles (FR-015).

State transitions:

- `owner` can be granted when a user has an active `owner` `UnitOccupancy`.
- `tenant` can be granted when a user has an active `tenant` `UnitOccupancy`.
- `syndic` can be granted only while the user has an active `owner` role.
- When the `owner` role is revoked, the `syndic` role is revoked automatically
  (spec FR-010).
- A user with no active roles is denied all protected access.

### Invitation

Syndic-issued link/code for registration.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK | Immutable |
| `token` | text, unique (hashed) | Random value; raw token only shown once to the syndic |
| `condominium_id` | UUID, FK → `condominiums.id` | Required |
| `unit_id` | UUID, FK → `units.id` | Required |
| `invited_role` | enum `user_role` restricted to `owner`, `tenant` | Required; `syndic` not allowed |
| `invited_email` | text, nullable | Optional pre-fill; registration email must match when set |
| `status` | enum `invitation_status` (`pending`, `accepted`, `expired`, `revoked`) | Default `pending` |
| `expires_at` | timestamptz | Default `now() + 7 days` |
| `created_by` | UUID, FK → `users.id` | The syndic who issued it |
| `created_at` | timestamptz | Default `now()` |
| `accepted_at` | timestamptz, nullable | Set on successful registration |

Validation:
- Single-use: `accepted_at` is set at most once.
- Expired/rejected: registration fails when `status != 'pending'` or
  `expires_at < now()` (spec edge case).

### Session

Server-side session record referenced by the session ID stored in the browser cookie.

| Field | Type | Rules |
|-------|------|-------|
| `token_hash` | text, PK (SHA-256 of raw session ID) | Raw session ID is never stored |
| `user_id` | UUID, FK → `users.id` | Set on sign-in |
| `created_at` | timestamptz | Default `now()` |
| `expires_at` | timestamptz | Required; 12 h idle / 7 d absolute, whichever comes first |
| `csrf_token_hash` | text | Required; synchronizer token for CSRF protection |

Validation:
- Multiple concurrent sessions per user are allowed (FR-017).
- Sign-out deletes the session row; an expired or missing row is treated as
  unauthenticated.

### AuditEvent

Security-relevant event record required by spec FR-013.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK | Immutable |
| `user_id` | UUID, FK → `users.id`, nullable | Actor when known |
| `event_type` | text | One of: `sign_in`, `failed_sign_in`, `sign_out`, `account_locked`, `role_granted`, `role_revoked`, `access_denied` |
| `details` | jsonb | Context: role, route, condominium id |
| `created_at` | timestamptz | Default `now()` |

Validation:
- Every event has an `event_type` from the allowed set.
- Events are append-only; updates/deletes are not permitted by application code.

## Relationships

```text
User 1 ──── N UnitOccupancy N ──── 1 Unit N ──── 1 Condominium
User 1 ──── N UserRole       N ──── 1 Condominium
User 1 ──── N Invitation (created_by) N ──── 1 Condominium
User 1 ──── N Session
Invitation N ──── 1 Unit
```

- A `User` may have zero or more `UnitOccupancy` rows.
- A `User` may have zero or more `UserRole` rows, at most one active `owner`/`tenant`
  row per condominium, and optionally one active `syndic` row.
- A `Session` belongs to exactly one `User`.

## Planned migrations

1. `0001_create_users.sql` — `users`, `user_status` enum, lockout columns.
2. `0002_create_condominiums_units.sql` — `condominiums`, `units`.
3. `0003_create_occupancies_roles.sql` — `occupancy_type` enum, `user_role` enum,
   `unit_occupancies`, `user_roles`, indexes.
4. `0004_create_invitations.sql` — `invitation_status` enum, `invitations`.
5. `0005_create_sessions.sql` — `sessions` table.
6. `0006_create_audit_events.sql` — `audit_events` table.
