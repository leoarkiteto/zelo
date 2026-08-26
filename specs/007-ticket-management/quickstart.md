# Quickstart Validation Guide: Ticket Management

Feature: `specs/007-ticket-management/spec.md`

Runnable validation scenarios that prove the feature end-to-end after implementation.
Details live in [data-model.md](./data-model.md) and
[contracts/http-routes.md](./contracts/http-routes.md).

## Prerequisites

- Go 1.27+
- Docker with Compose
- Existing auth/RBAC slice working (users, roles, sessions, CSRF)
- Existing i18n slice working (`en` / `pt-br`)
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

# 3. Apply migrations (includes 0010_create_tickets.sql)
make migrate

# 4. Generate templates and run the server
make templ
make run
```

Seed or create, via the existing invitation flow: one syndic and two owners in the
same condominium, each owner occupying a unit.

## Validation scenarios

### 1. Access control

1. Open `/tickets` signed out → redirected to `/login`.
2. Sign in as an owner → `/tickets` renders their own (empty) ticket list.
3. Owner opens `/tickets/inbox` → `403` (syndic-only).
4. Sign in as the syndic → `/tickets/inbox` renders the inbox.
5. Syndic opens `/tickets` → renders the syndic's own ticket list (member route).

**Expected**: only the syndic reaches inbox/reply/close; residents reach only the
member routes, per [contracts/http-routes.md](./contracts/http-routes.md).

### 2. Resident creates a ticket

1. As owner A, open `/tickets/new`.
2. Submit title `Kitchen leak`, category `repair`, and description `Leak under
   the kitchen sink` → redirected to `/tickets/{id}` showing status `Open`.
3. Open `/tickets` → the new ticket appears with status `Open`.
4. Try to submit without a description → form re-renders with a validation error and
   nothing is saved.
5. Try to submit without a title → form re-renders with a validation error and
   nothing is saved.
6. Try to submit with an invalid category (tampered form) → form re-renders with a
   validation error.
7. Create a ticket with category `other` → the category is stored and displayed.

**Expected**: valid tickets are created in under 2 minutes; empty/invalid submissions
are blocked (spec FR-001, FR-002, SC-001, SC-002).

### 3. Syndic receives, replies, and closes

1. As the syndic, open `/tickets/inbox` → owner A's ticket appears at the top (oldest
   open first).
2. Open the ticket and submit a reply `A maintenance visit is scheduled for Friday` →
   redirects to the detail page and the reply appears with the syndic's name and date.
3. Owner A opens the same ticket → sees the reply.
4. As the syndic, submit the close form → status becomes `Closed`, the ticket leaves
   the default inbox, and `/tickets/inbox?status=closed` shows it.
5. As owner A, open the closed ticket → read-only: no reply form, status `Closed`.

**Expected**: reply-then-close flow works end to end; closed tickets are read-only for
both parties (spec FR-005, FR-006, FR-007, FR-008, SC-004, SC-005).

### 4. Closed tickets reject new messages

1. As the syndic, try to POST a reply to the closed ticket (direct form replay) →
   `400` error page, no new reply row.
2. As owner A, try to POST a reply to the closed ticket (direct form replay) → `404`
   or `403` (route is syndic-only) and no new reply row.

**Expected**: 100% of closed tickets reject new messages from either party (spec
FR-008, SC-005).

### 5. Resident privacy

1. As owner B (same condominium), open `/tickets/{ownerA-ticket-id}` → `404`.
2. As owner B, open `/tickets` → only owner B's own tickets appear (none of owner A's).

**Expected**: residents see only their own tickets; foreign ticket ids do not leak
existence (spec FR-009, SC-006).

### 6. Empty states and i18n

1. As owner B, open `/tickets` with no tickets → friendly empty state with a shortcut
   to `/tickets/new`.
2. Switch the interface language to `pt-br` → category labels show `Solicitação de
   reparo`, `Reclamação de barulho`, `Sugestão de pauta para a assembleia`; switch
   back to `en` → English labels.

**Expected**: friendly empty states and translated labels in both languages (spec
FR-010, SC-007).
