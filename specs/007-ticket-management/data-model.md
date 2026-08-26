# Data Model: Ticket Management

Feature: `specs/007-ticket-management/spec.md` — persistence model and validation
rules for the `tickets` slice. Access is via `database/sql` + the `pgx` stdlib driver
with plain SQL. Implementation details (migration SQL, repository code) belong to
`tasks.md` and the implementation phase.

## Entities

### Ticket

A single request, complaint, or suggestion submitted by a resident.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK, default `gen_random_uuid()` | Immutable |
| `condominium_id` | UUID, FK → `condominiums.id` ON DELETE CASCADE | Required; set from the session condominium |
| `author_id` | UUID, FK → `users.id` ON DELETE RESTRICT | Required; the resident who created the ticket |
| `unit_id` | UUID, nullable, FK → `units.id` ON DELETE SET NULL | Captured from the author's active unit at creation; becomes `NULL` if the unit is later deleted (FR-003) |
| `title` | text | Required, 1–120 chars |
| `category` | text, CHECK `IN ('repair','noise_complaint','assembly_topic','other')` | Required; fixed list (FR-001) |
| `description` | text | Required, 1–2000 chars (FR-002) |
| `status` | text, CHECK `IN ('open','closed')` | Required; new tickets are `open` (FR-004) |
| `closed_at` | timestamptz, nullable | Required when `status = 'closed'`; must be `NULL` otherwise (FR-007) |
| `closed_by` | UUID, nullable, FK → `users.id` ON DELETE SET NULL | The syndic who closed the ticket; required when `closed`, `NULL` otherwise |
| `created_at` | timestamptz | Default `now()` |
| `updated_at` | timestamptz | Updated on close (status change) |

Integrity checks (in the migration):

```text
CHECK (category IN ('repair', 'noise_complaint', 'assembly_topic', 'other'))
CHECK (status IN ('open', 'closed'))
CHECK (char_length(title) BETWEEN 1 AND 120)
CHECK (char_length(description) BETWEEN 1 AND 2000)
CHECK (
    (status = 'closed' AND closed_at IS NOT NULL AND closed_by IS NOT NULL)
    OR
    (status = 'open'   AND closed_at IS NULL     AND closed_by IS NULL)
)
```

### TicketReply

A message from the syndic attached to a ticket.

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK, default `gen_random_uuid()` | Immutable |
| `ticket_id` | UUID, FK → `tickets.id` ON DELETE CASCADE | Required; the ticket this reply belongs to |
| `author_id` | UUID, FK → `users.id` ON DELETE RESTRICT | Required; the syndic who replied |
| `content` | text | Required, 1–2000 chars |
| `created_at` | timestamptz | Default `now()` |

Integrity checks (in the migration):

```text
CHECK (char_length(content) BETWEEN 1 AND 2000)
```

Replies are append-only: a reply row is created only while its ticket is `open`, and
no update/delete operations exist in v1. The closed-ticket read-only rule (FR-008) is
enforced by the service layer, not by a database trigger.

### TicketCategory (value object, not a table)

Stable keys persisted in `tickets.category`; display labels come from the i18n catalog.

| key | pt-br label | en label |
|---|---|---|
| `repair` | Solicitação de reparo | Repair request |
| `noise_complaint` | Reclamação de barulho | Noise complaint |
| `assembly_topic` | Sugestão de pauta para a assembleia | Suggestion for the next assembly agenda |
| `other` | Outro | Other |

### TicketStatus (value object)

| Stored value | Meaning | Reached by |
|---|---|---|
| `open` | Ticket awaiting the syndic's reply/close | Ticket creation |
| `closed` | Ticket handled; read-only | Syndic close action |

There are no derived statuses and no `reopen` transition in v1.

## Relationships

```text
Ticket      N ──── 1 Condominium  (condominium_id)
Ticket      N ──── 1 User         (author_id)
Ticket      N ──── 0..1 Unit      (unit_id, copied at creation)
Ticket      1 ──── N TicketReply  (ticket_id)
TicketReply N ──── 1 User         (author_id)
```

- A `Ticket` belongs to exactly one `Condominium`.
- A `Ticket` is created by exactly one resident `User`.
- A `Ticket` records the author's `Unit` at creation; the reference is nullable so
  historical tickets survive unit deletion.
- A `Ticket` has zero or more `TicketReply` rows (replies from the syndic).
- A `TicketReply` belongs to exactly one `Ticket` and exactly one syndic `User`.

## State Transitions

```text
creation ──▶ open ── reply (adds a TicketReply row; status unchanged) ──▶ open
              │
              └────────────── close (closed_at + closed_by) ────────────▶ closed (terminal, read-only)
```

- Creation always produces `open` (FR-004).
- `reply` is allowed only on `open` tickets (FR-006).
- `close` is allowed only on `open` tickets; a ticket MAY be closed with zero replies
  (FR-007).
- `closed` is terminal and read-only: no new replies from either party and no reopen
  in v1 (FR-008, spec Assumptions).

## Planned migration

1. `0010_create_tickets.sql`:
   - `tickets` table with the columns and CHECK constraints above (as of 0010).
   - `ticket_replies` table with the columns and CHECK constraint above.
   - Indexes:
     - `idx_tickets_inbox` on `(condominium_id, status, created_at)` for the syndic
       inbox (open tickets oldest-first).
     - `idx_tickets_author` on `(author_id, created_at)` for the resident's own
       ticket list.
     - `idx_ticket_replies_ticket` on `(ticket_id, created_at)` for rendering a
       ticket's replies in order.
   - Extend `audit_events.event_type` CHECK allow-list with `ticket_created`,
     `ticket_replied`, `ticket_closed` (drop/re-add constraint, same pattern as
     migration `0009`).
2. `0011_ticket_title_and_other_category.sql` (forward-only follow-up):
   - Adds the required `title` column (backfilling existing rows, then enforcing
     `CHECK (char_length(title) BETWEEN 1 AND 120)`).
   - Extends the category CHECK to include `other`.

## Validation Rules

- `title`: required, trimmed non-empty, 1–120 characters.
- `category`: required; must be one of `repair`, `noise_complaint`,
  `assembly_topic`, `other`.
- `description`: required, trimmed non-empty, 1–2000 characters.
- `content` (reply): required, trimmed non-empty, 1–2000 characters.
- `status`: never user-submitted; derived from the action (`open` on create, `closed`
  on close).
- `unit_id`: never user-submitted; captured from the session user's active unit at
  creation. If the session user has no active unit, ticket creation is refused with a
  friendly message.
- `closed_at` / `closed_by`: never user-submitted; set together by the close action.
- `condominium_id` / `author_id`: never user-submitted; set from the session.
- `csrf_token`: required on every POST (create, reply, close).
