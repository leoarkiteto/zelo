# Phase 0 Research: RBAC Authentication & Authorization

Feature: `specs/001-rbac-auth/spec.md`
Constraint input: "CSS-First over Javascript and Standard lib as much as possible (no
frameworks and ORM)". All Technical Context unknowns resolved below.

## R1 — Server-side session strategy

- **Decision**: Build a small custom session manager in `internal/auth` using the Go
  standard library: `crypto/rand` for session IDs (32 random bytes, base64url-encoded),
  a `zelo_session` cookie (`HttpOnly`, `Secure` in production, `SameSite=Lax`), and a
  `sessions` table in PostgreSQL accessed through `database/sql`.
- **Rationale**: Matches the user's "session id and cookie" direction and the
  standard-library-first constraint; the code surface is small and fully testable.
- **Alternatives considered**:
  - `alexedwards/scs/v2` + `pgxstore` — maintained but is a third-party session
    framework (rejected under the no-frameworks constraint).
  - `gorilla/sessions` — archived (rejected).
  - Signed/encrypted client-side cookies — session data would live in the cookie,
    conflicting with the server-side/Postgres direction (rejected).

## R2 — Password storage and verification

- **Decision**: Hash passwords with **Argon2id** via `golang.org/x/crypto/argon2` using
  OWASP-recommended parameters and store a PHC-formatted string in
  `users.password_hash`.
- **Rationale**: Argon2id is the OWASP first choice; the Go standard library has no
  safe password-hashing primitive, and `golang.org/x/crypto` is the official Go
  extension, so this is the minimal compliant dependency.
- **Alternatives considered**:
  - Pure stdlib `crypto/sha256` — fast hashes are not suitable for passwords (rejected).
  - `bcrypt` via `x/crypto` — acceptable but Argon2id is preferred for new systems
    (rejected).
  - Third-party `argon2` wrappers — unnecessary; `x/crypto` is sufficient (rejected).

## R3 — PostgreSQL access

- **Decision**: Use stdlib `database/sql` with the `github.com/jackc/pgx/v5/stdlib`
  driver and hand-written SQL. No ORM and no query builder.
- **Rationale**: `database/sql` is the standard-library database layer; `pgx` is used
  strictly as a driver registration, not as a framework or ORM.
- **Alternatives considered**:
  - GORM / ent / sqlc — ORM or code-generation layers, rejected by the no-ORM
    constraint.
  - `lib/pq` — in maintenance mode (rejected in favor of `pgx` stdlib driver).

## R4 — Migrations

- **Decision**: Plain forward-only SQL files in `migrations/` named
  `NNNN_description.sql`, applied by a minimal `internal/store` migration runner that
  records applied versions in a `schema_migrations` table inside the same transaction.
- **Rationale**: Satisfies "no migration framework" while keeping the constitution's
  forward-only, committed migration rule.
- **Alternatives considered**:
  - `pressly/goose` or `golang-migrate` — full migration frameworks (rejected).
  - `psql` manual runs — error-prone for reproducible environments (rejected).

## R5 — CSRF protection

- **Decision**: Custom synchronizer-token middleware in `internal/middleware`: a random
  per-session CSRF token is stored server-side and rendered in every state-changing
  form; `POST` requests are rejected with `403` when the submitted token is missing or
  mismatched.
- **Rationale**: Standard pattern, implementable with `crypto/rand` and the custom
  session store; no third-party middleware needed.
- **Alternatives considered**:
  - `justinas/nosurf` / `gorilla/csrf` — third-party middleware (rejected under the
    standard-library-first constraint).
  - Double-submit cookie alone — weaker; server-side token preferred (rejected).

## R6 — Local PostgreSQL via docker-compose

- **Decision**: Add a root-level `docker-compose.yml` with a `postgres` service
  (PostgreSQL 16-alpine), named volume for persistence, fixed local port, database/user
  configured via environment, and a healthcheck.
- **Rationale**: Gives every developer the same local database and satisfies the
  requirement to save users in a Postgres container.
- **Alternatives considered**:
  - Cloud/hosted Postgres — unnecessary for local dev (rejected).
  - SQLite — user explicitly requested Postgres and RBAC data needs referential
    integrity (rejected).

## R7 — RBAC data model shape

- **Decision**: Roles as a join table (`user_roles`) scoped by `condominium_id`; unit
  relationships as `unit_occupancies` (`owner` or `tenant`); `invitations` table for
  syndic-issued registration links; `sessions` table for server-side sessions. A user
  may hold `syndic` only in addition to `owner`, never alongside `tenant`.
- **Rationale**: Standard RBAC assignments keep `syndic + owner` representable, the
  "syndic must be an owner" rule enforceable, and invite-based registration explicit.
- **Alternatives considered**:
  - Single `role` column on `users` — cannot represent syndic + owner or role history
    (rejected).
  - `role` column on `memberships` only — conflates property relationships with roles
    (rejected).

## R8 — CSS-first front-end behavior

- **Decision**: Tailwind CSS handles layout, styling, and as much interaction state as
  practical (transitions, focus/hover/disabled states, progressive disclosure via CSS).
  HTMX is limited to server-driven partial updates (e.g., form submission without full
  reload) and is loaded as the only JavaScript asset. No custom JS framework or build
  step beyond compiling Tailwind.
- **Rationale**: Honors "CSS-first over Javascript" while preserving the
  constitution-mandated GOTTH stack.
- **Alternatives considered**:
  - Alpine.js or a JS component framework — violates CSS-first and no-frameworks
    (rejected).
  - No JS at all — would break the constitution's HTMX requirement (rejected).
- **Superseded (2026-09-13)**: the Alpine.js rejection above was revisited when the
  login password-reveal toggle turned out to need client-local interaction state that
  Tailwind CSS cannot express and HTMX cannot serve. Constitution Principle VI now
  carries a ratified, narrowly-scoped Alpine exception (CSP build only, strictly
  last-resort, logic confined to `web/static/js/app.js`); see
  `.specify/memory/constitution.md` and `REASONIX.md`. The claim that HTMX is "loaded
  as the only JavaScript asset" therefore no longer holds. The rest of R8 stands:
  Tailwind CSS still owns layout, styling, and interaction state wherever it can, and
  HTMX still owns server-driven partial updates.

## R9 — Config and cookie flags

- **Decision**: `internal/config` reads `DATABASE_URL`, `SESSION_SECRET`, `APP_ENV`
  (`development`/`production`), and cookie flags from `os.Getenv` with validation; the
  session cookie defaults to `HttpOnly`, `SameSite=Lax`, and `Secure` when
  `APP_ENV=production`.
- **Rationale**: Stdlib-only configuration matches the constitution's environment-based
  config rule and keeps secrets out of source control.
- **Alternatives considered**: third-party config/env libraries (rejected under the
  standard-library-first constraint).
