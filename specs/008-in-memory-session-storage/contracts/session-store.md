# Contract: Session Store

This feature changes the storage adapter behind an existing internal port. The contract below is what `SessionManager` (and, through it, auth middleware and CSRF middleware) depends on.

## Port: SessionRepository

Defined in `internal/shared/security/session.go` (unchanged by this feature):

```go
type SessionRepository interface {
    CreateSession(ctx context.Context, sess model.Session) error
    GetSessionByTokenHash(ctx context.Context, tokenHash string) (model.Session, error)
    UpdateSessionExpiry(ctx context.Context, tokenHash string, expiresAt time.Time) error
    DeleteSession(ctx context.Context, tokenHash string) error
}
```

### Semantics

| Operation | Behavior |
|-----------|----------|
| `CreateSession` | Store the session keyed by `TokenHash` with TTL = remaining lifetime; error if the store is unavailable |
| `GetSessionByTokenHash` | Return the session; return `model.ErrNotFound` when the key is absent; wrap other failures with context |
| `UpdateSessionExpiry` | Update `ExpiresAt` and reset TTL to the new remaining lifetime; `model.ErrNotFound` if key absent |
| `DeleteSession` | Remove the key; deleting a missing key is not an error (logout is idempotent) |

## Storage schema (Redis adapter)

- **Key**: `zelo:session:{TokenHash}` — `TokenHash` is the SHA-256 hex of the raw cookie token (`security.HashToken`).
- **Value**: JSON object (fields per `data-model.md`).
- **TTL**: seconds until `ExpiresAt`; Redis expires the key automatically.

## Cookie contract (unchanged)

| Property | Value |
|----------|-------|
| Name | `zelo_session` |
| Value | 32-byte base64url raw session token |
| Path | `/` |
| HttpOnly | `true` |
| Secure | `true` in production (`Config.IsProduction()`), `false` in development |
| SameSite | `Lax` |
| Expires | now + 7-day absolute lifetime |
| Clear on logout | `MaxAge = -1`, `Expires = Unix(0,0)` |

## Error contract

- Missing key → `model.ErrNotFound` → `SessionManager.Read` returns `ErrNoSession` (anonymous request).
- Expired session → `SessionManager.Read` returns `ErrSessionExpired` (defensive `expires_at` check).
- Redis unavailable → wrapped error surfaces as a 500 on login/validation paths; startup ping fails fast before serving traffic.
