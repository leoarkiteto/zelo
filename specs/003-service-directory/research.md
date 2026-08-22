# Phase 0 Research: Service Provider Directory

Feature: `specs/003-service-directory/spec.md`

Constraint input: GOTTH stack, standard library first (no frameworks/ORM), CSS-first
over JavaScript, vertical slice isolation, TDD. All Technical Context unknowns resolved
below.

## R1 — Condominium membership gate for directory access

- **Decision**: Reuse the existing `middleware.RequireAuth` + `middleware.RequireRole`
  stack. Directory routes allow any of the three active roles (`owner`, `tenant`,
  `syndic`) for the session's condominium; moderation and category-management routes
  allow `syndic` only. A session with no active role in its condominium is denied.
- **Rationale**: `RequireRole` already enforces "authenticated + active role in the
  session condominium", which is exactly the spec's "authenticated users belonging to
  the condominium" rule. No new middleware is needed, and the existing audit event
  (`access_denied`) covers denials.
- **Alternatives considered**:
  - New `RequireMember` middleware — duplicates `RequireRole` behavior for no benefit
    (rejected).
  - Checking `unit_occupancies` instead of roles — a resident could have an occupancy
    but no active role yet; role membership is the existing source of truth for access
    (rejected).

## R2 — Category management data shape

- **Decision**: A `service_categories` table scoped by `condominium_id` with `name`
  and `active` columns. The syndic can add, rename, and deactivate categories; a
  partial unique index enforces unique active names per condominium. Deactivating a
  category sets `active = false`; the row is retained so existing listings keep their
  category. The resident submission form and the category filter only offer active
  categories.
- **Rationale**: Matches the clarification ("syndic can add, rename, and deactivate")
  and keeps category identity stable for existing listings. A partial unique index
  allows a deactivated name to be reused later while preventing two active duplicates.
- **Alternatives considered**:
  - Free-text categories — makes filtering inconsistent and was explicitly rejected in
    the clarification (rejected).
  - Hard-deleting categories — would orphan or distort existing listings; deactivation
    preserves history (rejected).
  - A global (non-condominium-scoped) category list — breaks the condominium scoping
    already used by every other entity (rejected).

## R3 — Capturing the recommending resident's unit

- **Decision**: On listing submission, look up the submitting user's active
  `unit_occupancies` in the session condominium and store two values on the listing:
  `recommended_by_unit_id` (nullable FK to `units`, `ON DELETE SET NULL`) and
  `recommended_by_unit_code` (a required text snapshot of `units.code`). The directory
  displays the snapshot.
- **Rationale**: The spec requires the recommending resident's unit to be recorded
  automatically and displayed. A snapshot keeps the listing correct even if the
  occupancy changes or the unit row is later removed; the nullable FK preserves
  referential context when available.
- **Alternatives considered**:
  - FK-only join to `units` — if a unit is deleted the listing breaks, and historical
    listings would silently change if a unit code were edited (rejected).
  - Storing the resident's full name — the spec/clarification says unit only, for
    privacy (rejected).

## R4 — Search and filter implementation

- **Decision**: Plain parameterized SQL on `service_provider_listings`. Keyword search
  uses `ILIKE '%<escaped>%'` across `name`, `category name` (via join), and `notes`;
  filter uses `category_id = $n` with active categories only. `%`, `_`, and `\` in user
  input are escaped before matching. No full-text search engine.
- **Rationale**: At the stated scale (hundreds of listings), `ILIKE` on indexed text is
  fast and requires no additional dependency, honoring the standard-library-first
  constraint. Escaping prevents wildcard-injection surprises.
- **Alternatives considered**:
  - PostgreSQL full-text search (`tsvector`) — more moving parts (generated columns,
    triggers) with no user-facing benefit at this scale (rejected).
  - Client-side filtering — would require JavaScript beyond the CSS-first boundary
    (rejected).

## R5 — Duplicate phone handling

- **Decision**: Store the phone exactly as entered plus a normalized `phone_digits`
  column (digits only). On submission, query for an existing listing in the same
  condominium with the same `phone_digits`; if found, re-render the form with a
  warning. The resident can save anyway by resubmitting with `confirm_duplicate=1`.
  No unique constraint on phone.
- **Rationale**: Matches the clarification ("warn but still allow saving") and gives
  the resident a chance to back out, while the syndic can still clean up duplicates.
  Normalizing to digits makes the check tolerant of formatting differences.
- **Alternatives considered**:
  - Unique constraint on `phone_digits` — would block legitimate re-recommendations,
    contrary to the clarification (rejected).
  - Silent duplicates — loses the warning the user explicitly chose (rejected).
  - Fuzzy matching (name + phone) — unnecessary complexity; phone equality is enough
    (rejected).

## R6 — Moderation model

- **Decision**: Listings are published immediately. The syndic can edit any listing's
  name, category, phone, and notes, and delete any listing. Deletion is a hard delete
  plus an `audit_events` record; edit is an update plus an `audit_events` record.
  Residents have no edit/delete affordances and the service rejects such attempts.
- **Rationale**: Matches the clarification (immediate visibility + post-hoc
  moderation, syndic-only edits). Hard delete keeps the table and queries simple;
  audit events preserve the moderation trail required by the existing security model.
- **Alternatives considered**:
  - Soft delete (`deleted_at`) — adds filtering complexity without a restore
    requirement (rejected).
  - Pre-publication approval queue — explicitly rejected in the clarification.

## R7 — Routes and UI contract

- **Decision**: Server-rendered pages under `/directory`. The list/search/filter page
  is a single `GET /directory` with `q` and `category` query parameters rendered by the
  same Templ component. Submission, edit, delete, and category management are standard
  POST forms protected by the existing CSRF middleware. HTMX is used only as a
  progressive enhancement where it reduces full reloads (e.g., the duplicate-warning
  confirmation form); CSS-first styling covers the rest.
- **Rationale**: Fits the GOTTH/CSS-first constitution and the existing route patterns
  (`/roles`, `/invitations` are all server-rendered POST forms).
- **Alternatives considered**:
  - JSON API + client rendering — violates the server-rendered GOTTH approach
    (rejected).
  - Separate search endpoint — unnecessary; query parameters on the directory page are
    sufficient (rejected).

## R8 — Validation rules

- **Decision**: Name required (1–120 chars), category required and must be an active
  category in the same condominium, phone required and validated as 7–15 digits with
  optional `+`, spaces, dashes, and parentheses, notes optional (max 500 chars).
  Duplicate warning compares `phone_digits`. Category name required (1–80 chars) and
  unique among active categories per condominium (case-insensitive).
- **Rationale**: Keeps data clean and testable while staying permissive about phone
  formatting, matching the spec's "basic acceptable format" assumption.
- **Alternatives considered**:
  - Strict E.164 phone validation — too strict for a community directory and not
    required by the spec (rejected).
  - No length limits — risks unbounded input; low-cost limits were chosen (rejected).
