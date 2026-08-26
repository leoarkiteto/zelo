# Phase 0 Research: Ticket Management

Feature: `specs/007-ticket-management/spec.md`

Constraint input: GOTTH stack, standard library first (no frameworks/ORM), CSS-first
over JavaScript, vertical slice isolation, TDD, i18n (`en` + `pt-br`). All Technical
Context unknowns resolved below.

## R1 — Feature slice placement and shape

- **Decision**: One new self-contained vertical slice at `internal/features/tickets/`
  with `core/{domain,ports,services}`, `handlers/`, `repositories/`, and `templates/`.
  Domain types (`Ticket`, `TicketReply`, `TicketCategory`, `TicketStatus`) live in
  `core/domain`; repositories and handlers import only that same-feature package or
  `internal/shared/*`.
- **Rationale**: Matches the constitution's vertical slice layout and the existing
  `internal/features/*` convention (`auth`, `directory`, `finance`, `home`,
  `management`, `profile`). Keeping the persisted model inside the slice makes it
  fully self-contained, as required by Principle III.
- **Alternatives considered**:
  - Placing `Ticket` in `internal/shared/model` — pollutes shared code with a
    feature-specific model and weakens slice isolation (rejected).
  - A separate resident slice + syndic slice — both share the same ticket lifecycle
    and data; one slice avoids duplicated code and keeps the feature cohesive
    (rejected).

## R2 — Persistence model: two tables

- **Decision**: Two new tables: `tickets` (one row per ticket) and `ticket_replies`
  (zero or more syndic messages per ticket). Reply rows are created only while the
  ticket is `open` and are immutable afterwards. Status is persisted only as `open` /
  `closed`.
- **Rationale**: Mirrors the spec's Key Entities (`Ticket`, `Ticket Reply`) and keeps
  inbox/list queries simple plain SQL. There are no derived statuses in this feature,
  so nothing extra needs to be computed at read time.
- **Alternatives considered**:
  - One table with JSON-encoded replies — breaks plain-SQL inspection and constraints
    (rejected).
  - A shared `messages` table reused across future features — leaks feature
    boundaries and forces cross-slice coupling (rejected).

## R3 — Status model and transitions

- **Decision**: Persist `open` and `closed` only. Creation → `open`. The `close`
  operation sets `status = 'closed'`, `closed_at`, and `closed_by` in one transition.
  `closed` is terminal and read-only; reopening is not supported in v1.
- **Rationale**: The spec defines exactly `Open` and `Closed` (FR-004) and makes
  closed tickets read-only (FR-008). A CHECK constraint keeps `closed_at`/`closed_by`
  aligned with the status so invalid rows are impossible.
- **Alternatives considered**:
  - Adding `answered`/`in_progress` states — not requested and adds scope (rejected).
  - Allowing reopen — explicitly out of scope in the spec's Assumptions (rejected).

## R4 — Category representation

- **Decision**: Persist stable category keys in `tickets.category`:
  `repair`, `noise_complaint`, `assembly_topic`. Display labels come from the i18n
  catalog (`tickets.category.repair`, etc.) in `en` and `pt-br`.
- **Rationale**: Same pattern as the finance categories and the service directory:
  stable keys keep stored data language-independent and CHECK-able in the migration.
- **Alternatives considered**:
  - Free-text category — the spec fixes exactly three categories (rejected).
  - Storing localized labels — data becomes language-dependent and hard to validate
    (rejected).

## R5 — Authorization model

- **Decision**: Reuse `RequireAuth` + `RequireRole` middleware. Resident routes allow
  `owner`, `tenant`, and `syndic` (the existing "member" set). Syndic-only routes:
  inbox, reply, close. Handler-level ownership check on `GET /tickets/{id}`: members
  see only their own ticket; the syndic sees any ticket of the session condominium;
  foreign or missing tickets return `404` so ticket existence is not leaked.
- **Rationale**: The existing roles (`RoleOwner`, `RoleTenant`, `RoleSyndic`) already
  model the spec's actors and match the finance slice's member/syndic split.
- **Alternatives considered**:
  - A new ticket-specific permission table — overkill for two roles (rejected).
  - Letting members query any ticket by id — violates spec FR-009 (rejected).

## R6 — Text limits and validation

- **Decision**: `description` and reply `content` are required, trimmed non-empty, and
  limited to 2000 characters. `category` must be one of the three stable keys. Server
  enforces everything; forms re-render with inline field errors; all POSTs require a
  valid CSRF token.
- **Rationale**: Spec FR-002 requires a non-empty description with a maximum length
  and clear validation UX. 2000 characters is a sensible default for repair
  descriptions and replies.
- **Alternatives considered**:
  - No maximum length — unbounded user input (rejected).
  - A small limit (e.g., 255) — too restrictive for describing a repair or a noise
    complaint (rejected).

## R7 — Inbox and list ordering

- **Decision**: `GET /tickets/inbox` accepts `status=open|closed|all` (default
  `open`); `open` is ordered oldest-first (FIFO so the syndic handles the oldest
  request first), `closed` and `all` are ordered newest-first. The resident list
  `GET /tickets` shows only the current user's tickets, newest-first.
- **Rationale**: The syndic must view all tickets, including closed ones (spec
  FR-005), so a status filter is the smallest mechanism. FIFO for open tickets is the
  standard helpdesk default (spec assumes "handled in creation order").
- **Alternatives considered**:
  - A separate archive page — extra page for a single filter (rejected).
  - Pagination in v1 — scale is hundreds of tickets per year; pagination can be added
    later (rejected).

## R8 — i18n catalog keys

- **Decision**: Add `tickets.*` keys to `internal/shared/i18n/catalog.go` in both
  languages: nav label, page titles, category labels, status labels, form labels,
  empty states, and validation messages. The existing `TestCatalogComplete` guard
  keeps both languages in sync.
- **Rationale**: The app ships `en` + `pt-br` and every feature slice follows this
  pattern.
- **Alternatives considered**: hard-coding English strings in templates — breaks the
  i18n requirement (rejected).

## R9 — Route and CSRF contracts

- **Decision**: Follow the finance slice's route pattern: GET pages for viewing/forms,
  POST routes for every state change, CSRF token as a required form field on all
  POSTs. Full contract in `contracts/http-routes.md`.
- **Rationale**: Keeps the feature consistent with the rest of the app and reuses the
  existing CSRF middleware unchanged.
- **Alternatives considered**: HTMX JSON endpoints — the app is form-driven
  server-rendered HTML, so plain form POSTs are the established pattern (rejected).
