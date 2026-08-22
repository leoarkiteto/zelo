# Quickstart Validation Guide: RBAC Authentication & Authorization

Feature: `specs/001-rbac-auth/spec.md`

Runnable validation scenarios that prove the feature end-to-end after implementation.
Details live in [data-model.md](./data-model.md),
[contracts/http-routes.md](./contracts/http-routes.md), and
[contracts/session-cookie.md](./contracts/session-cookie.md).

## Prerequisites

- Go 1.27+
- Docker with Compose
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

# 3. Apply migrations (minimal forward-only runner)
go run ./cmd/web -migrate   # exact flag per implementation

# 4. Generate templates and run the server
templ generate
go run ./cmd/web
```

Bootstrap the first syndic using the dev seed command added during implementation (or
by direct database insert following [data-model.md](./data-model.md)); the application
itself does not self-select the first syndic.

## Validation scenarios

### 1. Invite-based registration

1. Sign in as the syndic and open `/invitations`.
2. Create an invitation for `owner@example.com` with role `owner` and a unit.
3. Create an invitation for `tenant@example.com` with role `tenant`.
4. Open the invitation link in a fresh browser session; the form is pre-filled with
   condominium, unit, and invited role, and there is no role selector.
5. Complete registration for both invitees.
6. Re-open an already-used invitation → `410`/rejected.

**Expected**: two users exist with the invited roles (check `user_roles` and
`invitations.status`).

### 2. Sign in and role-based access

1. Sign in as the owner → lands on `/`, can open `/unit`, but `/condominium` returns
   `403`.
2. Sign in as the tenant → can open `/tenancy`, but `/unit` and `/condominium` return
   `403`.
3. Sign out → protected routes redirect to `/login`.

**Expected**: each role only reaches the routes in
[contracts/http-routes.md](./contracts/http-routes.md).

### 3. Syndic role management and eligibility

1. As the syndic, grant `syndic` to the owner → accepted; the user now reaches
   `/condominium`.
2. As the syndic, attempt to grant `syndic` to the tenant → rejected (tenants are never
   eligible).
3. As the syndic, revoke the `owner` role from that user → the `syndic` role is revoked
   automatically and `/condominium` becomes `403`.

**Expected**: the "syndic must be an owner" rule holds in all transitions, and only the
current syndic can change roles.

### 4. Sign-in lockout

1. At `/login`, submit 5 consecutive wrong passwords for the owner account.
2. Attempt a 6th sign-in with the correct password → sign-in is rejected with a generic
   locked message (`423` per [contracts/http-routes.md](./contracts/http-routes.md)).
3. Wait for the lock to expire (or advance the lock in the test database), then sign in
   successfully.

**Expected**: account locks for 15 minutes after 5 consecutive failures, and the
lockout event is recorded.

### 5. Session behavior

1. Sign in and inspect the `zelo_session` cookie in browser dev tools.
2. Confirm the cookie is `HttpOnly` (and `Secure` when `APP_ENV=production`),
   `SameSite=Lax`, and contains no readable user data.
3. Sign in from a second browser → the first session stays signed in (FR-017).
4. Sign out in one browser → only that browser's session ends.
5. Delete the corresponding `sessions` row in the database → that session's next request
   redirects to `/login`.

**Expected**: server-side session behavior matches
[contracts/session-cookie.md](./contracts/session-cookie.md).

### 6. Automated tests

```bash
go test ./...
```

**Expected**: stdlib-based unit and integration tests pass, covering handlers,
services, store adapters, session lifecycle, lockout, invitations, and role rules.
