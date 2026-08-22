# Session Cookie Contract

Feature: `specs/001-rbac-auth/spec.md`

The application uses server-side sessions: the browser stores only a session ID; the
session record and user association live in PostgreSQL (`sessions` table). The session
manager is a small stdlib-only component in `internal/auth`.

## Cookie

| Attribute | Value |
|-----------|-------|
| Name | `zelo_session` |
| Value | Random session ID (32 random bytes, base64url-encoded; ≥ 256 bits of entropy) |
| `HttpOnly` | Always set — not readable from JavaScript |
| `Secure` | Set when `APP_ENV=production` |
| `SameSite` | `Lax` |
| `Path` | `/` |
| `Domain` | Not set (host-only) |
| Lifetime | 12 hours idle / 7 days absolute, whichever comes first |

## Lifecycle

1. **Sign-in success** — the server generates a random session ID, stores
   `sha256(session_id)` plus the user id and a CSRF token in the `sessions` table, and
   sends `Set-Cookie: zelo_session=<id>; HttpOnly; Secure(prod); SameSite=Lax; Path=/`.
2. **Authenticated request** — the server hashes the cookie value, loads the session
   row, and checks it is not expired and the user is active.
3. **Sign-out** — the server deletes only the current `sessions` row and sends an
   expired cookie to clear the browser value.
4. **Expiry** — expired rows are rejected and cleaned up by a background job or on
   access.

## Concurrency

A user may hold multiple simultaneous sessions across devices (spec FR-017). Signing
out ends only the current session; signing in on a new device does not invalidate
existing sessions.

## Security guarantees

- The raw session ID is never logged and only its SHA-256 hash is stored.
- Session fixation is prevented by issuing a new random session ID on sign-in.
- CSRF protection uses a per-session synchronizer token stored in the `sessions` row
  and rendered as a hidden form field (`csrf_token`); state-changing requests with a
  missing or mismatched token receive `403`.
- Changing a user's role or disabling an account takes effect on their next protected
  request (the session is re-validated against current roles).
- A disabled user's sessions stop granting access immediately (spec edge case).
