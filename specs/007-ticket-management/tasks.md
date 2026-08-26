---
description: "Task list for feature implementation"
---

# Tasks: Ticket Management

**Input**: Design documents from `/specs/007-ticket-management/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: INCLUDED — the zelo constitution mandates TDD for all feature work (red-green-refactor). Every story phase starts with failing tests before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Go module root: `github.com/leoarkiteto/zelo`; app code under `internal/`, entrypoint `cmd/web/`
- Vertical slice: `internal/features/tickets/` with `core/{domain,ports,services}`, `handlers/`, `repositories/`, `templates/`
- Go tests live next to the code they test (`*_test.go`), matching the existing repo
- Templates: `internal/features/tickets/templates/*.templ` (+ generated `*_templ.go` via `scripts/templ-generate.sh`)
- Migrations: `migrations/NNNN_description.sql` (forward-only, applied by `make migrate`)
- i18n: `internal/shared/i18n/catalog.go` (both `enMessages` and `ptbrMessages`)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Slice skeleton and shared i18n touched by the tickets feature

- [X] T001 Create the tickets vertical slice directories under `internal/features/tickets/`: `core/domain`, `core/ports`, `core/services`, `handlers`, `repositories`, `templates`
- [X] T002 [P] Add tickets i18n keys to `internal/shared/i18n/catalog.go` in both `enMessages` and `ptbrMessages`: navigation (`nav.tickets`), page titles/subtitles, category labels (`tickets.category.repair`, `tickets.category.noise_complaint`, `tickets.category.assembly_topic`), status labels (`open`/`closed`), form labels, validation messages, empty states, and flash messages per `data-model.md`

**Checkpoint**: `go build ./...` still compiles; `go test ./internal/shared/i18n` passes.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Migration and core domain/persistence types every user story depends on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 Write migration `migrations/0010_create_tickets.sql` per `specs/007-ticket-management/data-model.md`: `tickets` table (category CHECK `repair|noise_complaint|assembly_topic`, status CHECK `open|closed`, description length 1–2000, closed_at/closed_by alignment CHECK), `ticket_replies` table (content length 1–2000, FK cascade), indexes (`idx_tickets_inbox`, `idx_tickets_author`, `idx_ticket_replies_ticket`), and drop/re-add of the `audit_events.event_type` CHECK to add `ticket_created`, `ticket_replied`, `ticket_closed`
- [X] T004 [P] Write failing domain tests in `internal/features/tickets/core/domain/tickets_test.go` covering category validity, description/content validation (trimmed non-empty, 1–2000 chars), and status transition rules (`CanReply`, `CanClose`, closed is read-only)
- [X] T005 Implement `internal/features/tickets/core/domain/tickets.go` per `data-model.md`: `TicketCategory`, `TicketStatus`, `Ticket`, `TicketReply`, validation helpers, and transition helpers until T004 is green (depends on T004)
- [X] T006 [P] Create `internal/features/tickets/core/ports/repositories.go` with `TicketStore` (Create/Get/ListMine/ListInbox/AddReply/Close), `UnitResolver` (GetActiveUnitForUser), and `AuditRecorder` ports per `research.md` R5/R7
- [X] T007 Implement `internal/features/tickets/repositories/tickets.go` with `database/sql` adapters for `TicketStore` per `data-model.md` (parameterized SQL, FIFO for open inbox, newest-first for resident/closed lists, `status` filter expansion for `open|closed|all`) (depends on T003, T005, T006)
- [X] T008 [P] Write failing repository integration tests in `internal/features/tickets/repositories/tickets_test.go` (create/get/listMine/listInbox with status filter, addReply, close, against dockerized PostgreSQL) and make them green

**Checkpoint**: `make migrate` applies `0010` cleanly; `go test ./internal/features/tickets/...` passes; foundation ready for story work.

---

## Phase 3: User Story 1 - Resident creates a ticket (Priority: P1) 🎯 MVP

**Goal**: A resident creates a ticket from their own space choosing one of the three categories (repair request, noise complaint, assembly topic suggestion) with a description; the ticket is saved as `Open` with author/unit/category/description/date.

**Independent Test**: Sign in as an owner with an active unit, open `/tickets/new`, submit category `repair` and a description, and confirm the redirect to `/tickets/{id}` shows status `Open`; the ticket also appears in the ticket store's inbox query.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T009 [P] [US1] Write failing service test in `internal/features/tickets/core/services/tickets_test.go` for `CreateTicket` (required category, description trimmed 1–2000, active-unit capture, author/condominium from session, audit event `ticket_created`)
- [X] T010 [P] [US1] Write failing handler test in `internal/features/tickets/handlers/tickets_test.go` for `GET /tickets/new` and `POST /tickets` (member access, CSRF, validation errors, no-active-unit `403`, redirect to `/tickets/{id}`)

### Implementation for User Story 1

- [X] T011 [US1] Create `TicketService` in `internal/features/tickets/core/services/tickets.go` with `CreateTicket` (category/description validation, resolves the session user's active unit via `UnitResolver`, records audit `ticket_created`) until T009 is green (depends on T009)
- [X] T012 [US1] Create `internal/features/tickets/handlers/deps.go` (`Deps` struct) and `internal/features/tickets/handlers/routes.go` registering member routes with `middleware.RequireAuth` + `middleware.RequireRole(model.RoleOwner, model.RoleTenant, model.RoleSyndic)` per `contracts/http-routes.md`
- [X] T013 [US1] Implement `GET /tickets/new` and `POST /tickets` handlers in `internal/features/tickets/handlers/tickets.go` (render form, parse `category`/`description`, map service errors to i18n messages, redirect to `/tickets/{id}`) until T010 is green (depends on T010, T011, T012)
- [X] T014 [US1] Create `internal/features/tickets/templates/tickets.templ` with the ticket creation form (category select, description textarea, Templ + Tailwind + i18n) and run `scripts/templ-generate.sh`
- [X] T015 [US1] Wire tickets deps and `RegisterRoutes` into `cmd/web/main.go` (construct ticket repository, unit resolver, audit recorder, service, and handler deps; register routes)
- [X] T016 [US1] Add template smoke test `internal/features/tickets/templates/smoke_test.go` and run `scripts/tailwind-build.sh`

**Checkpoint**: User Story 1 fully functional — a resident can create a ticket and it is stored as `Open` with the correct metadata.

---

## Phase 4: User Story 2 - Syndic views tickets and replies (Priority: P1)

**Goal**: The syndic sees open tickets in an inbox (with category/author/unit/date), opens a ticket, and writes a reply that the resident can see.

**Independent Test**: Sign in as the syndic, open `/tickets/inbox`, open a ticket created in US1, submit a reply, and confirm the reply appears on `/tickets/{id}` for the resident.

### Tests for User Story 2 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T017 [P] [US2] Write failing service test in `internal/features/tickets/core/services/tickets_test.go` for `ReplyToTicket` (open-only, content trimmed 1–2000, audit event `ticket_replied`)
- [X] T018 [P] [US2] Write failing handler tests in `internal/features/tickets/handlers/tickets_test.go` for `GET /tickets/inbox`, `GET /tickets/{id}` (syndic view), and `POST /tickets/{id}/reply` (syndic-only access, CSRF, `404` for foreign/missing, open-only `400`)

### Implementation for User Story 2

- [X] T019 [US2] Implement `ReplyToTicket` in `internal/features/tickets/core/services/tickets.go` (open-only guard, audit `ticket_replied`) until T017 is green (depends on T017)
- [X] T020 [US2] Implement `GET /tickets/inbox` handler in `internal/features/tickets/handlers/tickets.go` (`status=open|closed|all` filter, `open` default oldest-first, `closed`/`all` newest-first) until T018 is green
- [X] T021 [US2] Implement `GET /tickets/{id}` handler in `internal/features/tickets/handlers/tickets.go` (syndic sees any ticket in the session condominium; member sees only their own; foreign/missing returns `404`)
- [X] T022 [US2] Implement `POST /tickets/{id}/reply` handler in `internal/features/tickets/handlers/tickets.go` (syndic-only, open-only, validation errors re-rendered) until T018 is green
- [X] T023 [US2] Add inbox list and ticket detail templates (detail includes the syndic reply form) to `internal/features/tickets/templates/tickets.templ`; regenerate with `scripts/templ-generate.sh`

**Checkpoint**: User Stories 1 AND 2 both work — residents create tickets and the syndic can answer them.

---

## Phase 5: User Story 3 - Syndic closes the ticket (Priority: P2)

**Goal**: After replying, the syndic closes the ticket; the status becomes `Closed` with a closing date, the ticket leaves the open inbox, and no further messages can be added by either party.

**Independent Test**: As the syndic, close an open ticket and confirm its status becomes `Closed`, it leaves the default inbox, and any further reply/close attempt is rejected.

### Tests for User Story 3 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T024 [P] [US3] Write failing service test in `internal/features/tickets/core/services/tickets_test.go` for `CloseTicket` (open-only, sets `closed_at`/`closed_by`, close without a prior reply allowed, audit event `ticket_closed`)
- [X] T025 [P] [US3] Write failing handler test in `internal/features/tickets/handlers/tickets_test.go` for `POST /tickets/{id}/close` (syndic-only access, CSRF, `404`, open-only `400`, redirect to `/tickets/inbox`)

### Implementation for User Story 3

- [X] T026 [US3] Implement `CloseTicket` in `internal/features/tickets/core/services/tickets.go` (open-only transition, audit `ticket_closed`) until T024 is green (depends on T024)
- [X] T027 [US3] Implement `POST /tickets/{id}/close` handler in `internal/features/tickets/handlers/tickets.go` (syndic-only, open-only, redirect to `/tickets/inbox`) until T025 is green
- [X] T028 [US3] Add the close confirmation form and closed read-only rendering (status badge, no reply/close forms) to `internal/features/tickets/templates/tickets.templ`; regenerate with `scripts/templ-generate.sh`
- [X] T029 [US3] Enforce the transition contract in `internal/features/tickets/repositories/tickets.go` (`AddReply` and `Close` update/insert guarded by `WHERE status = 'open'`) so closed tickets reject writes at the data layer

**Checkpoint**: The full ticket lifecycle works — create → reply → close — and closed tickets are read-only everywhere.

---

## Phase 6: User Story 4 - Resident tracks their tickets (Priority: P2)

**Goal**: A resident sees only their own tickets, each with status (`Open`/`Closed`) and the syndic's replies and dates; tickets from other residents are never shown.

**Independent Test**: Sign in as an owner with tickets, open `/tickets`, confirm only their own tickets appear with status and reply info; opening another owner's ticket id returns `404`.

### Tests for User Story 4 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T030 [P] [US4] Write failing service test in `internal/features/tickets/core/services/tickets_test.go` for `ListMine` (only the current user's tickets, newest-first)
- [X] T031 [P] [US4] Write failing handler test in `internal/features/tickets/handlers/tickets_test.go` for `GET /tickets` (member access, only the current user's tickets, newest-first, friendly empty state)

### Implementation for User Story 4

- [X] T032 [US4] Implement `ListMine` in `internal/features/tickets/core/services/tickets.go` (current user's tickets only, newest-first) until T030 is green
- [X] T033 [US4] Implement `GET /tickets` handler in `internal/features/tickets/handlers/tickets.go` (member list of the current user's own tickets, newest-first) until T031 is green
- [X] T034 [US4] Add the resident ticket list template (own tickets with status/reply count, empty state with a shortcut to `/tickets/new`) to `internal/features/tickets/templates/tickets.templ`; regenerate with `scripts/templ-generate.sh`

**Checkpoint**: All four user stories work independently — create, reply, close, and resident tracking.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Navigation, full-suite verification, and the quickstart validation pass

- [X] T035 [P] Add role-aware tickets navigation entries to the shared navigation in `internal/shared/templates/` (residents see `/tickets`; syndic sees `/tickets/inbox`)
- [X] T036 [P] Ensure all new i18n keys exist in both languages by running the existing catalog completeness test and fixing any gaps in `internal/shared/i18n/catalog.go`
- [X] T037 Run `scripts/templ-generate.sh`, `scripts/tailwind-build.sh`, `make migrate`, `go test ./...`, and `scripts/check-feature-boundaries.sh`; fix all failures
- [X] T038 Run every scenario in `specs/007-ticket-management/quickstart.md` end-to-end and confirm expected outcomes
- [X] T039 Review the final diff for accidental leftovers, generated files committed, and no cross-feature imports

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 (create) and US2 (inbox/reply) can be developed back-to-back; US2 reuses US1's routes/service wiring and needs tickets to exist
  - US3 (close) builds on US2 (reply flow + detail page)
  - US4 (resident tracking) builds on US2's detail handler
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — no dependencies on other stories
- **User Story 2 (P1)**: Depends on US1 (routes/service wiring; open tickets to reply to)
- **User Story 3 (P2)**: Depends on US1/US2 (open tickets + detail page with reply flow)
- **User Story 4 (P2)**: Depends on US1/US2 (own tickets exist; reuses `GET /tickets/{id}` ownership rules)

### Within Each User Story

- Tests (included — constitution TDD) MUST be written and FAIL before implementation
- Domain/ports before services/repositories
- Services before handlers
- Handlers before templates render correctly
- Core implementation before integration/wiring
- Story complete before moving to next priority

### Parallel Opportunities

- Phase 1: T001 and T002 can run in parallel
- Phase 2: T004 and T006 can run in parallel; T008 can start once T007 is underway
- Within each story: the failing test tasks marked [P] can run in parallel (e.g., T009+T010, T017+T018, T024+T025, T030+T031)
- Once Foundational completes, US1 and US2 can be picked up back-to-back; with a second developer, US2's failing tests (T017/T018) can be written while US1 finishes its implementation
- Polish tasks T035/T036 are independent and can run in parallel

### Parallel Example: User Story 1

```bash
# Launch both failing test tasks for User Story 1 together:
Task: "Write failing service test ... in internal/features/tickets/core/services/tickets_test.go"
Task: "Write failing handler test ... in internal/features/tickets/handlers/tickets_test.go"
```

### Parallel Example: Foundational

```bash
# Independent foundational pieces in parallel:
Task: "Write failing domain tests in internal/features/tickets/core/domain/tickets_test.go"
Task: "Create ports in internal/features/tickets/core/ports/repositories.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (resident creates a ticket)
4. **STOP and VALIDATE**: Test User Story 1 independently (create a ticket, confirm `Open` + metadata)
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP: residents can create tickets)
3. Add User Story 2 → Test independently → Deploy/Demo (syndic inbox + replies)
4. Add User Story 3 → Test independently → Deploy/Demo (reply-then-close lifecycle complete)
5. Add User Story 4 → Test independently → Deploy/Demo (residents track their tickets)
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (create)
   - Developer B: prepares User Story 2 failing tests while A finishes (then implements inbox/reply)
3. After US1/US2: Developer A takes US3 (close), Developer B takes US4 (resident tracking)
4. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing (constitution TDD)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
