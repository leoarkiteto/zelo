# Implementation Plan: In-Memory Session Storage (Redis)

**Branch**: `008-in-memory-session-storage` | **Date**: 2026-08-27 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/008-in-memory-session-storage/spec.md`, plus planning input: "Add a Redis in the `docker-compose.yml` to store the sessions as we will drop this table from Postgres."

**Note**: This plan is the output of `/speckit-plan`. Implementation tasks are generated separately by `/speckit-tasks`.

## Summary

Move server-side login sessions out of PostgreSQL and into Redis, a dedicated in-memory data store running as a `docker-compose.yml` service. The PostgreSQL `sessions` table is dropped through a new forward-only migration. Sessions survive application restarts (Redis runs independently) and are lost only on logout, expiry (enforced by Redis TTL), or Redis restart/data loss. The existing `SessionRepository` port in `internal/shared/security` is kept; only the adapter changes from SQL to Redis.

## Technical Context

**Language/Version**: Go 1.27 (`go.mod`), GOTTH stack (Go + Templ + Tailwind CSS + HTMX)

**Primary Dependencies**: `github.com/jackc/pgx/v5` (PostgreSQL, unchanged); NEW `github.com/redis/go-redis/v9` (Redis client); NEW test-only `github.com/alicebob/miniredis/v2` (in-memory Redis for unit tests)

**Storage**: PostgreSQL remains the persistent store for all domain data; Redis 7 (`redis:7-alpine`) becomes the in-memory store for sessions only. The `sessions` table is removed by migration `0012_drop_sessions.sql`.

**Testing**: `go test ./...` (unit + integration). Session manager tests keep the existing `fakeSessionRepo`; a new `RedisSessionStore` adapter gets unit tests against `miniredis`; `tests/integration` is updated to use Redis and is gated by `TEST_DATABASE_URL` + `TEST_REDIS_URL`.

**Target Platform**: Linux server via Docker Compose (PostgreSQL + Redis); local darwin development.

**Project Type**: Server-rendered web application (single deployable `cmd/web`).

**Performance Goals**: Authenticated page loads complete in under 1 second (spec SC-006); session validation is O(1) Redis GET.

**Constraints**: Go standard library first — Redis client is a justified exception (no stdlib Redis client). Migrations remain plain forward-only SQL. Vertical-slice isolation is preserved: the change lives in `internal/shared/*` and `cmd/web` only. No session data may be written to PostgreSQL.

**Scale/Scope**: Single condominium-management application, single process; Redis TTL bounds session growth; no horizontal scaling in scope.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **I. GOTTH Stack** — PASS. No frontend changes; server-side session plumbing only.
- **II. Modular Monolith with Hexagonal Architecture** — PASS. The `security.SessionRepository` port stays unchanged; the Redis store is a swappable adapter. `SessionManager` (domain logic) remains storage-agnostic.
- **III. Vertical Slice Feature Isolation** — PASS. Sessions are a shared concern already located in `internal/shared/security` + `internal/shared/store`; no feature slice internals are touched, and no feature imports another feature.
- **IV. Test-First Development (NON-NEGOTIABLE)** — MUST follow. Write/adjust tests first (Redis adapter unit tests, updated integration wiring) and see them fail before implementing.
- **V. SOLID, Design Patterns & Generated Code** — PASS. No generated artifacts involved.
- **VI. Standard Library First & CSS-First** — VIOLATION, JUSTIFIED. `github.com/redis/go-redis/v9` is required because Go's standard library has no Redis client; `alicebob/miniredis` is a test-only dependency that avoids requiring a live Redis in unit tests. Recorded in Complexity Tracking.
- **Repository Layout & Conventions** — PASS. Changes touch `docker-compose.yml`, `.env.example`, `migrations/`, `internal/shared/*`, `cmd/web`, and tests, all in the constitution-mandated layout.
- **Development Workflow & Quality Gates** — MUST verify. `go test ./...` must pass; migrations committed and forward-only; code review confirms compliance.

## Project Structure

### Documentation (this feature)

```text
specs/008-in-memory-session-storage/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   └── session-store.md
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/web/main.go                              # wire Redis session store instead of SQL
internal/shared/security/session.go          # UNCHANGED port + SessionManager
internal/shared/store/redis.go               # NEW OpenRedis client helper
internal/shared/store/redis_sessions.go      # NEW RedisSessionStore adapter
internal/shared/store/sessions.go            # REMOVED SQL session store
internal/shared/store/redis_sessions_test.go # NEW adapter tests (miniredis)
internal/shared/config/config.go             # add REDIS_URL loading/validation
internal/shared/testutil/testredis.go        # NEW TEST_REDIS_URL helper
migrations/0012_drop_sessions.sql            # NEW forward-only migration
docker-compose.yml                           # add redis service
.env.example                                 # add REDIS_URL
Makefile                                     # export REDIS_URL
tests/integration/integration_test.go        # use RedisSessionStore + TEST_REDIS_URL
```

**Structure Decision**: Single-project layout already established by the repository. No new top-level directories; the feature is a shared-module change plus a new migration and compose service. `internal/shared/store` gains a Redis adapter because that package is the existing persistence-adapter home; the domain port remains in `internal/shared/security`.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Third-party Redis client (`go-redis/v9`) despite stdlib-first rule | Go's standard library has no Redis client; writing a production-quality RESP client by hand would be larger, riskier, and harder to review than the dependency | A hand-written RESP client was rejected: more code, more edge cases, no benefit |
| Test-only `miniredis/v2` dependency | Unit-test the Redis adapter deterministically without requiring a live Redis service in `go test ./...` | Running unit tests against docker Redis was rejected: makes plain `go test ./...` environment-dependent |
