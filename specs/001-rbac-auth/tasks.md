# Tasks: RBAC Authentication & Authorization

**Input**: Design documents from `/specs/001-rbac-auth/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Included — the project constitution mandates test-first development for handlers, services, and store adapters.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

Single server-rendered GOTTH web application per `specs/001-rbac-auth/plan.md`:
`cmd/web/`, `internal/{auth,config,handler,middleware,model,service,store}/`,
`migrations/`, `web/templates/`, `web/static/`, repo-root tooling files.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and local development tooling

- [X] T001 Create `docker-compose.yml` at repo root with a `postgres:16-alpine` service (named volume, port `5432`, `POSTGRES_DB/POSTGRES_USER/POSTGRES_PASSWORD`, healthcheck)
- [X] T002 [P] Create `.env.example` at repo root documenting `DATABASE_URL`, `SESSION_SECRET`, `APP_ENV`, `HTTP_ADDR`
- [X] T003 [P] Create `tailwind.config.js` at repo root with content globs for `web/templates/**/*.templ` and `assets/css/**/*.css`
- [X] T004 [P] Create `Makefile` at repo root with targets `run`, `build`, `dev`, `migrate`, `templ`, `tailwind`, and create `.air.toml` at repo root for hot-reload dev server
- [X] T005 [P] Create `tools/tools.go` in `tools/` pinning `github.com/a-h/templ`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Implement environment configuration loading and validation in `internal/config/config.go` using `os.Getenv` (`DATABASE_URL`, `SESSION_SECRET`, `APP_ENV`, `HTTP_ADDR`)
- [X] T007 [P] Create domain types in `internal/model/user.go` (`User`, `UserStatus`, `Role`, `RoleName`)
- [X] T008 [P] Create domain types in `internal/model/condominium.go` (`Condominium`, `Unit`, `OccupancyType`)
- [X] T009 [P] Create domain types in `internal/model/invitation.go` (`Invitation`, `InvitationStatus`)
- [X] T010 [P] Create domain types in `internal/model/session.go` (`Session`)
- [X] T011 Create forward-only SQL migrations in `migrations/`: `0001_create_users.sql`, `0002_create_condominiums_units.sql`, `0003_create_occupancies_roles.sql`, `0004_create_invitations.sql`, `0005_create_sessions.sql`, `0006_create_audit_events.sql` per `specs/001-rbac-auth/data-model.md`
- [X] T012 Implement minimal forward-only migration runner in `internal/store/migrate.go` (`schema_migrations` table, run pending files in order)
- [X] T013 Implement database adapter in `internal/store/db.go` using `database/sql` + `github.com/jackc/pgx/v5/stdlib` driver and connection pool setup
- [X] T014 [P] Write failing tests for password hashing/verification in `internal/auth/password_test.go`
- [X] T015 Implement Argon2id password hashing in `internal/auth/password.go` (PHC-format strings) until T014 passes
- [X] T016 [P] Write failing tests for session lifecycle in `internal/auth/session_test.go`
- [X] T017 Implement stdlib session manager in `internal/auth/session.go` (`crypto/rand` session ID, SHA-256 token hash, `zelo_session` cookie with `HttpOnly`/`Secure`/`SameSite=Lax`) until T016 passes
- [X] T018 [P] Write failing tests for CSRF token generation/validation in `internal/auth/csrf_test.go`
- [X] T019 Implement per-session CSRF helpers in `internal/auth/csrf.go` and CSRF enforcement middleware in `internal/middleware/csrf.go` (validate `csrf_token` on all state-changing routes) until T018 passes
- [X] T020 [P] Implement plain-SQL user repository in `internal/store/users.go` (`CreateUser`, `GetUserByEmail`, `UpdatePassword`, `RecordFailedSignIn`, `ApplyLock`, `ClearLock`)
- [X] T021 [P] Implement plain-SQL role repository in `internal/store/roles.go` (`GrantRole`, `RevokeRole`, `ActiveRolesForUser`, `ActiveSyndicForCondominium`)
- [X] T022 [P] Implement plain-SQL invitation repository in `internal/store/invitations.go` (`CreateInvitation`, `GetInvitationByToken`, `MarkInvitationAccepted`, `RevokeInvitation`)
- [X] T023 [P] Implement plain-SQL session repository in `internal/store/sessions.go` (`CreateSession`, `GetSessionByTokenHash`, `DeleteSession`, `DeleteExpiredSessions`)
- [X] T024 [P] Implement request logging middleware in `internal/middleware/logging.go` and audit event store in `internal/store/audit.go` (`RecordEvent` for sign-in, failed sign-in, sign-out, account lockout, role changes, denied access per FR-013)
- [X] T025 [P] Implement panic recovery middleware in `internal/middleware/recover.go`
- [X] T026 [P] Implement security headers middleware in `internal/middleware/security.go`
- [X] T027 Create `cmd/web/main.go` skeleton: load config, open DB, run migrations, add a dev-only seed command that creates the first condominium and syndic, construct server (routes wired in later phases)
- [X] T028 [P] Create base layout in `web/templates/layout.templ`

**Checkpoint**: Foundation ready — user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Register with a role (Priority: P1) 🎯 MVP

**Goal**: A person registers only via a syndic-issued invitation that pre-fills condominium, unit, and invited role (owner or tenant).

**Independent Test**: Seed a pending invitation directly in the database, then complete `POST /register` with that token and assert the user exists with the invited role and the invitation is marked accepted.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T029 [P] [US1] Write failing service test for invitation registration (valid, invalid, expired, already-used) in `internal/service/registration_test.go`
- [X] T030 [P] [US1] Write failing handler test for `GET/POST /register` in `internal/handler/register_test.go`

### Implementation for User Story 1

- [X] T031 [US1] Implement `RegisterUser` use case in `internal/service/registration.go` (validate invitation status/expiry, create user, grant invited role, mark invitation accepted) until T029 passes
- [X] T032 [US1] Implement `GET/POST /register` handlers in `internal/handler/register.go` (pre-fill from token, no role selector, generic errors) until T030 passes
- [X] T033 [US1] Create registration form in `web/templates/register.templ` (token, email, password, password_confirm, `csrf_token`; invitation pre-fill and validation errors)
- [X] T034 [US1] Wire `/register` routes in `cmd/web/main.go`
- [X] T035 [US1] Run `go test ./internal/service/... ./internal/handler/...` and validate registration against `specs/001-rbac-auth/quickstart.md` scenario 1

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Sign in and stay signed in appropriately (Priority: P1)

**Goal**: A registered user signs in with email + password, gets a server-side session, and signing out ends only that session. Lockout after 5 failed attempts.

**Independent Test**: Create a user directly in the database, sign in via `POST /login`, assert a `zelo_session` cookie is set and `/` is reachable; sign out and assert the session row is deleted.

### Tests for User Story 2 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T036 [P] [US2] Write failing service test for authenticate/lockout/sign-out in `internal/service/auth_test.go`
- [X] T037 [P] [US2] Write failing handler test for `GET/POST /login` and `POST /logout` in `internal/handler/login_test.go`

### Implementation for User Story 2

- [X] T038 [US2] Implement `Authenticate`, `SignOut`, and lockout logic in `internal/service/auth.go` (5 failures → 15-minute lock, generic invalid-credentials error; record sign-in, failed sign-in, and account lockout events via `internal/store/audit.go`) until T036 passes
- [X] T039 [US2] Implement `GET/POST /login` handlers in `internal/handler/login.go` (issue session cookie, `423` when locked) until T037 passes
- [X] T040 [US2] Implement `POST /logout` handler in `internal/handler/logout.go` (delete current session only and record sign-out event via `internal/store/audit.go`)
- [X] T041 [US2] Create sign-in form in `web/templates/login.templ` (email, password, `csrf_token`, generic error message)
- [X] T042 [US2] Implement password reset use case in `internal/service/password_reset.go` (request token, reset password, 1-hour expiry)
- [X] T043 [US2] Implement `GET/POST /password/forgot` and `GET/POST /password/reset` handlers in `internal/handler/password.go`
- [X] T044 [US2] Create `web/templates/forgot_password.templ`
- [X] T045 [US2] Create `web/templates/reset_password.templ`
- [X] T046 [US2] Wire `/login`, `/logout`, `/password/*` routes in `cmd/web/main.go`

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Access restricted by role (Priority: P2)

**Goal**: Signed-in users can only reach pages their role permits: syndic manages the condominium, owners manage their own unit, tenants see only their own tenancy.

**Independent Test**: Sign in as each of the three roles and request `/condominium`, `/unit`, and `/tenancy`; assert each role gets exactly the allowed routes and `403` for the others.

### Tests for User Story 3 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T047 [P] [US3] Write failing middleware test for role-based access guard in `internal/middleware/auth_test.go`
- [X] T048 [P] [US3] Write failing handler test for role-restricted routes in `internal/handler/home_test.go`

### Implementation for User Story 3

- [X] T049 [US3] Implement role-required middleware in `internal/middleware/auth.go` (session load, user status check, `RequireRole`) until T047 passes
- [X] T050 [US3] Implement `GET /` handler in `internal/handler/home.go` (redirect to role-appropriate area) until T048 passes
- [X] T051 [US3] Create role-aware dashboard in `web/templates/dashboard.templ`
- [X] T052 [US3] Implement `GET /condominium`, `GET /unit`, `GET /tenancy` handlers in `internal/handler/areas.go` with role checks, friendly `403`, and record denied-access events via `internal/store/audit.go`
- [X] T053 [US3] Wire protected routes in `cmd/web/main.go` using `RequireRole`
- [X] T054 [US3] Validate against `specs/001-rbac-auth/quickstart.md` scenario 2 and `contracts/http-routes.md`

**Checkpoint**: At this point, User Stories 1, 2, and 3 should all work independently

---

## Phase 6: User Story 4 - Syndic eligibility (Priority: P3)

**Goal**: Only the current syndic can grant/revoke roles; the syndic role can only be granted to an active owner; tenants are never eligible; revoking owner auto-revokes syndic.

**Independent Test**: As syndic, grant `syndic` to an owner (succeeds), attempt to grant `syndic` to a tenant (rejected), then revoke the owner role and assert the syndic role is auto-revoked.

### Tests for User Story 4 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T055 [P] [US4] Write failing service test for grant/revoke role eligibility rules in `internal/service/roles_test.go`
- [X] T056 [P] [US4] Write failing handler test for `GET/POST /invitations` and `/roles` routes in `internal/handler/invitations_test.go`

### Implementation for User Story 4

- [X] T057 [US4] Implement `GrantRole`/`RevokeRole` use cases in `internal/service/roles.go` (syndic-only caller, owner prerequisite, tenant exclusion, auto-revoke syndic; record role-change events via `internal/store/audit.go`) until T055 passes
- [X] T058 [US4] Implement `GET/POST /invitations` and `POST /invitations/{id}/revoke` handlers in `internal/handler/invitations.go` (syndic only) until T056 passes
- [X] T059 [US4] Implement `GET /roles`, `POST /roles/assign`, `POST /roles/revoke` handlers in `internal/handler/roles.go` (syndic only)
- [X] T060 [US4] Create `web/templates/invitations.templ` (list/create invitations with unit and role)
- [X] T061 [US4] Create `web/templates/roles.templ` (list users/roles, assign/revoke actions)
- [X] T062 [US4] Wire `/invitations` and `/roles` routes in `cmd/web/main.go`

**Checkpoint**: All user stories should now be independently functional

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [X] T063 [P] Create `web/templates/error.templ` for friendly `403`, `404`, `410` pages and use it in middleware/handlers
- [X] T064 [P] Create `assets/css/input.css` Tailwind input and build scripts in `scripts/` so compiled output lands in `web/static/css`
- [X] T065 [P] Ensure `templ generate` and Tailwind build are scripted (Makefile/`scripts/`) and generated files are committed per constitution
- [X] T066 Run all `specs/001-rbac-auth/quickstart.md` scenarios end-to-end and fix any issues
- [X] T067 Run `go test ./...`, `templ generate`, and Tailwind build; commit generated `*_templ.go` and compiled CSS
- [X] T068 [P] Security hardening pass: verify cookie flags, generic error messages, session cleanup job, and audit event logging per `specs/001-rbac-auth/spec.md` FR-013

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 and US2 are independent of each other after Foundational
  - US3 depends on US2 (needs sessions and sign-in)
  - US4 depends on US3 (needs role-restricted areas) and US1 (needs invitations)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — no dependencies on other stories
- **User Story 2 (P1)**: Can start after Foundational (Phase 2) — no dependencies on other stories
- **User Story 3 (P2)**: Depends on US2 for authenticated sessions
- **User Story 4 (P3)**: Depends on US1 (invitations) and US3 (syndic-only areas)

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Services before handlers
- Handlers before template wiring in `cmd/web/main.go`
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- Foundational tasks marked [P] can run in parallel (within Phase 2)
- US1 and US2 can be implemented in parallel after Foundational
- Tests for a user story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members once their dependencies are met

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "T029 Write failing service test in internal/service/registration_test.go"
Task: "T030 Write failing handler test in internal/handler/register_test.go"

# After tests fail, implement the story sequentially:
Task: "T031 Implement internal/service/registration.go"
Task: "T032 Implement internal/handler/register.go"
```

## Parallel Example: Foundational

```bash
# Independent foundational components can be built together:
Task: "T014-T015 password hashing in internal/auth/"
Task: "T016-T017 session manager in internal/auth/"
Task: "T018-T019 CSRF helpers in internal/auth/"
Task: "T020-T023 repositories in internal/store/"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (invitation-based registration)
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo (now users can sign in)
4. Add User Story 3 → Test independently → Deploy/Demo (role-restricted areas)
5. Add User Story 4 → Test independently → Deploy/Demo (syndic role management)
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (registration)
   - Developer B: User Story 2 (sign-in/session)
3. After US2: Developer B starts User Story 3 (access control)
4. After US1+US3: Developer A or B starts User Story 4 (syndic role management)
5. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing (constitution TDD requirement)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
