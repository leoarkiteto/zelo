---
description: "Task list for feature implementation: In-Memory Session Storage (Redis)"
---

# Tasks: In-Memory Session Storage (Redis)

**Input**: Design documents from `/specs/008-in-memory-session-storage/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Included — the project constitution mandates TDD for all feature implementations.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add Redis to the development environment and dependencies

- [x] T001 [P] Add a `redis` service to `docker-compose.yml` — image `redis:7-alpine`, container name `zelo-redis`, port `6379:6379`, command `["redis-server", "--save", "", "--appendonly", "no"]`, no volume, healthcheck `redis-cli ping`
- [x] T002 [P] Add `REDIS_URL=redis://localhost:6379/0` to `.env.example`
- [x] T003 [P] Export `REDIS_URL` in `Makefile` alongside the existing `DATABASE_URL SESSION_SECRET PASSWORD_PEPPER APP_ENV HTTP_ADDR` exports
- [x] T004 [P] Add `github.com/redis/go-redis/v9` and `github.com/alicebob/miniredis/v2` to `go.mod` via `go get`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Configuration, migration, and helpers that all user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T005 Add `RedisURL` to `internal/shared/config/config.go` (default `redis://localhost:6379/0`, validate parseable, fail fast on invalid) and add `internal/shared/config/config_test.go` covering default and explicit values
- [x] T006 [P] Create `migrations/0012_drop_sessions.sql` containing `DROP TABLE IF EXISTS sessions;`
- [x] T007 [P] Remove `sessions` from the `TRUNCATE` list in `internal/shared/store/users_test.go`
- [x] T008 [P] Remove `sessions` from the `TRUNCATE` list in `tests/integration/integration_test.go`
- [x] T009 [P] Add `OpenRedis` helper in `internal/shared/store/redis.go` — parse URL with `redis.ParseURL`, ping, return `*redis.Client`, fail fast on error
- [x] T010 [P] Add `TestRedisURL` helper in `internal/shared/testutil/testredis.go` — return `TEST_REDIS_URL` with a distinct Redis DB number per suffix (e.g., a checksum of the suffix modulo 16) or empty when unset (skip convention)

**Checkpoint**: Foundation ready — Redis is configured and the PostgreSQL `sessions` table is gone after `make migrate`

---

## Phase 3: User Story 1 - User logs in and stays authenticated during active use (Priority: P1) 🎯 MVP

**Goal**: Login creates a session in Redis; every authenticated request validates the cookie against Redis and refreshes the idle timeout; PostgreSQL stores no session data.

**Independent Test**: Log in via the integration test suite, confirm a `zelo:session:{hash}` key exists in Redis with a positive TTL, load protected pages while authenticated, and confirm no session rows can exist in PostgreSQL.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [x] T011 [P] [US1] Write unit tests in `internal/shared/store/redis_sessions_test.go` (miniredis) for `RedisSessionStore`: `CreateSession` stores JSON at `zelo:session:{TokenHash}` with TTL; `GetSessionByTokenHash` returns the stored session including the exact `CSRFToken`; `GetSessionByTokenHash` on a missing key returns `ErrNotFound`; `UpdateSessionExpiry` updates `ExpiresAt` and TTL; concurrent reads/refreshes on the same key do not corrupt the session; two sessions for the same user with different token hashes are independent. Must fail to compile until T013 exists.
- [x] T012 [P] [US1] Update `newApp` in `tests/integration/integration_test.go` to open Redis via `store.OpenRedis(testutil.TestRedisURL("integration"))` and pass `store.NewRedisSessionStore(redisClient)` to `security.NewSessionManager`; skip when `TEST_DATABASE_URL` or `TEST_REDIS_URL` is unset. Must fail to compile until T013 exists.

### Implementation for User Story 1

- [x] T013 [US1] Implement `RedisSessionStore` in `internal/shared/store/redis_sessions.go` per `contracts/session-store.md`: JSON value with fields from `data-model.md`, key `zelo:session:{TokenHash}`, `SET ... EX <remaining-ttl>`, map `redis.Nil` to `store.ErrNotFound`, wrap other errors with context (depends on T011)
- [x] T014 [US1] Wire Redis into `cmd/web/main.go`: call `store.OpenRedis(cfg.RedisURL)` after the Postgres open, and replace `store.NewSessionStore(db)` with `store.NewRedisSessionStore(redisClient)` for `security.NewSessionManager` (depends on T013)
- [x] T015 [US1] Delete `internal/shared/store/sessions.go` (the SQL `SessionStore`) after T012 and T014 remove its last references
- [x] T016 [US1] Add integration test in `tests/integration/session_redis_test.go`: login creates a `zelo:session:*` key with positive TTL, a protected page loads while the session is active, and idle refresh extends the TTL

**Checkpoint**: User Story 1 fully functional — login and authenticated browsing run entirely against Redis

---

## Phase 4: User Story 2 - User logs out and the session ends immediately (Priority: P2)

**Goal**: Logout removes the session from Redis and clears the cookie, so the browser can no longer access the authenticated area.

**Independent Test**: Log in, capture the Redis key, log out, confirm the key is gone and the next request with the old cookie is anonymous.

### Tests for User Story 2

- [x] T017 [P] [US2] Add unit tests in `internal/shared/store/redis_sessions_test.go`: `DeleteSession` removes the key; deleting a missing key is not an error (idempotent)
- [x] T018 [P] [US2] Add integration test in `tests/integration/session_redis_test.go`: logout deletes the Redis key and the next request with the old cookie is redirected to login; logout without a valid session cookie completes without error

### Implementation for User Story 2

- [x] T019 [US2] Verify `security.SessionManager.Invalidate` in `internal/shared/security/session.go` still deletes via the repository and clears the cookie; fix only if T017/T018 fail

**Checkpoint**: User Stories 1 AND 2 both work independently — logout is immediate and idempotent

---

## Phase 5: User Story 3 - Sessions are ephemeral: expiry and Redis restart discard them (Priority: P2)

**Goal**: Sessions survive application restarts but are discarded on expiry or Redis restart/loss; the Redis store stays bounded via TTL.

**Independent Test**: Rebuild the app-level `SessionManager` against the same Redis client and confirm the cookie still works (app restart); flush Redis or fast-forward TTL and confirm the cookie is rejected.

### Tests for User Story 3

- [x] T020 [P] [US3] Add unit tests in `internal/shared/store/redis_sessions_test.go`: miniredis fast-forward past TTL makes the key disappear automatically; read after expiry returns `ErrNotFound`
- [x] T021 [P] [US3] Add integration test in `tests/integration/session_redis_test.go`: rebuilding the `SessionManager` against the same Redis client still authenticates the cookie (app restart), while `FLUSHDB` on the test Redis DB rejects the cookie (Redis restart)

### Implementation for User Story 3

- [x] T022 [US3] Ensure `RedisSessionStore` always sets TTL to the remaining lifetime on create and idle refresh; adjust `internal/shared/store/redis_sessions.go` if T020/T021 expose gaps

**Checkpoint**: All user stories independently functional — expiry and Redis restart behavior verified

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup affecting all stories

- [x] T023 [P] Update `README.md` development setup section (the part that documents `docker compose` and `.env` setup) to mention the Redis service and `REDIS_URL`
- [x] T024 [P] Run `scripts/check-feature-boundaries.sh` and fix any violations
- [x] T025 Run `go test ./...` and fix all failures
- [x] T026 Run `templ generate` and confirm no generated diffs (no template changes expected)
- [x] T027 Run the `quickstart.md` validation steps 1–10 and record the results in the PR description

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 (P1) is the MVP and must be completed first
  - US2 and US3 can then proceed in either order (both P2)
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — no dependencies on other stories; delivers the Redis session store used by all stories
- **User Story 2 (P2)**: Depends on US1 (the repository and wiring exist); independently testable via logout tests
- **User Story 3 (P2)**: Depends on US1 (the repository and wiring exist); independently testable via TTL/restart tests

### Within Each User Story

- Tests MUST be written and FAIL before implementation (T011/T012 before T013/T014)
- Adapter implementation before composition-root wiring
- Wiring before deleting the old SQL store
- Story checkpoint verified before moving to the next priority

### Parallel Opportunities

- Phase 1: T001–T004 all touch different files and can run in parallel
- Phase 2: T006–T010 touch different files and can run in parallel after T005
- Phase 3: T011 and T012 (test files) can run in parallel; T013–T015 are sequential
- Phase 4: T017 and T018 can run in parallel
- Phase 5: T020 and T021 can run in parallel
- Phase 6: T023 and T024 can run in parallel; T025–T027 sequential validation

---

## Parallel Example: User Story 1

```bash
# Launch the two failing tests together:
Task: "Write unit tests in internal/shared/store/redis_sessions_test.go"
Task: "Update newApp in tests/integration/integration_test.go to use RedisSessionStore"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: run T011–T016 tests and the quickstart steps 1–6
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Redis wired into the environment
2. Add User Story 1 → login/authenticated browsing on Redis → Test independently → Demo (MVP!)
3. Add User Story 2 → logout verified → Test independently → Demo
4. Add User Story 3 → expiry/restart verified → Test independently → Demo
5. Each story adds verified behavior without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (adapter + wiring)
3. After US1 merges:
   - Developer A: User Story 2 (logout tests)
   - Developer B: User Story 3 (expiry/restart tests)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Verify tests fail before implementing (constitution Principle IV)
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
