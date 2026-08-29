# Phase 0 Research: In-Memory Session Storage (Redis)

Feature: move server-side login sessions from PostgreSQL to Redis, drop the PostgreSQL `sessions` table.

## Decision 1: Redis client library

- **Decision**: Use `github.com/redis/go-redis/v9` as the Redis client.
- **Rationale**: It is the de facto standard, actively maintained Go Redis client with native `context.Context` support, connection pooling, URL parsing (`redis.ParseURL`), and `redis.Nil` sentinel for missing keys. It keeps the adapter small and testable.
- **Alternatives considered**:
  - `github.com/gomodule/redigo` — older, less active, lower-level API; rejected.
  - Hand-written RESP client — violates the "justify complexity" principle; rejected (see plan Complexity Tracking).

## Decision 2: Redis in docker-compose

- **Decision**: Add a `redis:7-alpine` service to `docker-compose.yml` with `command: ["redis-server", "--save", "", "--appendonly", "no"]`, no named volume, port `6379`, and a `redis-cli ping` healthcheck.
- **Rationale**: Matches the user instruction ("Add a Redis in the `docker-compose.yml`"). Alpine image is small; disabling RDB/AOF persistence keeps the store ephemeral, which is the feature's core property. No volume means `docker compose down` discards sessions.
- **Alternatives considered**:
  - Named volume for Redis — rejected: would persist sessions to disk, contradicting "ephemeral data".
  - Default Redis persistence — rejected: default RDB snapshots could resurrect sessions after a Redis restart, weakening the ephemeral boundary.

## Decision 3: Session representation in Redis

- **Decision**: One key per session: `zelo:session:{sha256-hex(session-token)}` → JSON value `{"user_id","condominium_id","csrf_token","created_at","expires_at"}` with Redis `SET ... EX <remaining-ttl>`. Idle refresh updates `expires_at` in the JSON and resets the TTL to the new remaining lifetime. Expiry is enforced primarily by Redis TTL, with a defensive `expires_at` check on read.
- **Rationale**: A single JSON value keeps the adapter simple and mirrors the existing `model.Session` struct exactly. TTL gives automatic, bounded cleanup (spec FR-008) with no background sweeper. The token hash is the existing SHA-256 hex hash from `security.HashToken`, preserving cookie semantics.
- **Alternatives considered**:
  - Redis hash (`HSET`) with per-field storage — works but adds field-by-field marshalling without meaningful benefit for five fields.
  - No TTL + periodic sweeper — rejected: more moving parts; Redis TTL is the idiomatic expiry mechanism.
  - Plain token as key (no hash) — rejected: key would hold the raw session token, which should not appear in storage even transiently.

## Decision 4: Configuration

- **Decision**: Add `REDIS_URL` to `internal/shared/config.Config`, defaulting to `redis://localhost:6379/0` in development; `redis.ParseURL` validates/normalizes it; the app pings Redis at startup and fails fast if unreachable. Add `REDIS_URL=redis://localhost:6379/0` to `.env.example` and export it in the `Makefile`.
- **Rationale**: Mirrors how `DATABASE_URL` is handled and keeps the existing env-based configuration convention. Failing fast on ping gives an actionable startup error instead of mysterious 500s on first login.
- **Alternatives considered**:
  - `REDIS_HOST`/`REDIS_PORT` pair — rejected: a single URL is the project's existing pattern (`DATABASE_URL`) and matches go-redis's `ParseURL`.
  - Optional Redis with graceful degradation — rejected: sessions are required for login; running without Redis must fail loudly.

## Decision 5: PostgreSQL `sessions` table removal

- **Decision**: Add forward-only migration `migrations/0012_drop_sessions.sql` containing `DROP TABLE IF EXISTS sessions;` and remove `sessions` from every `TRUNCATE` list in tests (`internal/shared/store/users_test.go`, `tests/integration/integration_test.go`).
- **Rationale**: The clarification session chose to drop the table now. `IF EXISTS` keeps the migration idempotent for environments where the table was already removed manually. Removing the table from test truncation avoids errors after the migration runs.
- **Alternatives considered**:
  - Leave the table unused — rejected by user clarification.
  - Delete `0005_create_sessions.sql` — rejected: migrations are forward-only and already applied in existing environments; deleting history breaks the migration ledger.

## Decision 6: Testing strategy

- **Decision**: Unit-test the new `RedisSessionStore` against `miniredis` (fast, deterministic, no external service). Keep the existing `fakeSessionRepo` tests for `SessionManager` unchanged. Update `tests/integration` to build the app with a `RedisSessionStore` when `TEST_REDIS_URL` is set (skipping otherwise), and add `internal/shared/testutil/testredis.go` to resolve a per-test Redis database number.
- **Rationale**: `go test ./...` must stay runnable without Docker (existing convention: integration tests skip without `TEST_DATABASE_URL`). miniredis provides real Redis protocol behavior in-process. Integration coverage proves the composed app still works with real Redis.
- **Alternatives considered**:
  - Integration tests only (docker Redis) — rejected: plain `go test ./...` would fail or silently skip core adapter coverage.
  - Another fake repository for the adapter — rejected: it would not exercise marshalling, TTL, or error mapping.

## Decision 7: Error mapping

- **Decision**: Map `redis.Nil` (key missing) to `model.ErrNotFound` so `SessionManager.Read` keeps returning `ErrNoSession` for unknown cookies. Wrap other Redis errors with context (`create session`, `get session`, `update session expiry`, `delete session`).
- **Rationale**: The `SessionRepository` contract already relies on `store.ErrNotFound`/`model.ErrNotFound`; preserving it means `SessionManager` and middleware need zero changes.
- **Alternatives considered**: New sentinel errors in the security package — rejected: would ripple into `SessionManager` and its tests for no benefit.

## Open questions

None. All technical unknowns are resolved above.
