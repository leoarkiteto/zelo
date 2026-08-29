# Data Model: In-Memory Session Storage (Redis)

## Entity: Session

A server-side login session referenced by the `zelo_session` browser cookie. After this feature, sessions exist only in Redis; PostgreSQL has no `sessions` table.

| Field | Type | Required | Notes |
|-------|------|----------|-------|
| TokenHash | string (SHA-256 hex) | yes | Redis key discriminator; never the raw cookie token |
| UserID | UUID string | yes | Identity loaded into the request context |
| CondominiumID | UUID string | no | The condominium the user is operating in; may be empty |
| CSRFToken | string | yes | Per-session CSRF token stored with the session |
| CreatedAt | RFC 3339 timestamp | yes | Recorded at creation; used to cap absolute lifetime |
| ExpiresAt | RFC 3339 timestamp | yes | Defensive expiry check; Redis TTL is the primary enforcement |

## Redis representation

- **Key**: `zelo:session:{TokenHash}` where `TokenHash = security.HashToken(rawID)` (existing SHA-256 hex).
- **Value**: JSON object:

```json
{
  "user_id": "uuid",
  "condominium_id": "uuid-or-empty",
  "csrf_token": "random-token",
  "created_at": "2026-08-27T23:00:00Z",
  "expires_at": "2026-09-03T23:00:00Z"
}
```

- **TTL**: set to the remaining lifetime in seconds on create and on idle refresh. Redis removes the key automatically when TTL reaches zero (spec FR-008).

## Identity & uniqueness

- `TokenHash` is the unique identifier. One key exists per login (device/browser), so the same user may hold multiple concurrent sessions (spec FR-011).
- The raw session token exists only in the browser cookie; storage holds only its SHA-256 hash.

## Lifecycle / state transitions

```text
        login                     idle activity                 logout
[absent] -----> [active] ------------------------> [active] ----------> [absent]
                   |                                  |
                   | expiry (TTL or expires_at)       | Redis restart/loss
                   v                                  v
               [absent]                           [absent]
```

- **Create** (login): generate raw token + CSRF token, compute hash, write JSON with `CreatedAt = now`, `ExpiresAt = now + 7 days`, TTL = 7 days.
- **Read** (request validation): GET key. Missing key → not found (anonymous). `expires_at` in the past → treat as expired and delete key. Otherwise valid.
- **Idle refresh** (valid read while idle window active): if `now + 12h < ExpiresAt`, set `ExpiresAt = now + 12h` and update TTL accordingly. The absolute 7-day lifetime is still capped via `CreatedAt` + 7 days (SessionManager keeps current logic).
- **Delete** (logout): DEL key.
- **Expire**: Redis TTL removes the key automatically; the read path also defensively rejects stale `expires_at`.

## Validation rules (from spec)

- Absolute lifetime: 7 days from creation; idle timeout: 12 hours, refreshed on activity (spec FR-006).
- Logout deletes immediately (FR-007).
- Application restart does not invalidate sessions (FR-005); Redis restart/loss does.
- No session data may be written to PostgreSQL (FR-001/FR-002).

## Scale

- One key per active session. TTL guarantees the Redis memory footprint is bounded by active sessions (spec SC-004).
- Session volume follows the app's single-condominium scale; no special sharding or eviction policy is required.
