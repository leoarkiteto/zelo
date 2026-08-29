# Feature Specification: In-Memory Session Storage

**Feature Branch**: `008-in-memory-session-storage`

**Created**: 2026-08-27

**Status**: Draft

**Input**: User description: "The session should be saved in a Database 'in-memory', not in a persistent database. Because this is a ephemeral data"

## Clarifications

### Session 2026-08-27

- Q: Should this feature remove the persistent `sessions` table from the database, or leave it in the schema unused? → A: Drop the table now via a new forward-only migration; old session rows are permanently removed.
- Q: With sessions moved to Redis (a separate in-memory service), what should happen to active sessions when the application restarts? → A: Sessions survive application restarts; they are lost only if Redis itself restarts or loses its data.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - User logs in and stays authenticated during active use (Priority: P1)

A user logs in with valid credentials. The system creates a login session in a dedicated in-memory data store (Redis) that runs independently of the application, and gives the user a session cookie. On every subsequent request, the system validates the cookie against the in-memory session data and identifies the user, so protected pages keep working without re-login while the session is active.

**Why this priority**: This is the core of the change — session storage must move out of the persistent database and into a dedicated in-memory data store without breaking the login experience.

**Independent Test**: Log in as a registered user, navigate to protected pages across multiple requests, and confirm the user stays authenticated without any session record being written to the persistent database.

**Acceptance Scenarios**:

1. **Given** a registered user with valid credentials, **When** they log in, **Then** the system creates a session in the in-memory data store (Redis), issues the session cookie, and loads the authenticated area.
2. **Given** an active session, **When** the user requests a protected page, **Then** the system validates the session cookie against the in-memory session and renders the page for that user.
3. **Given** an active session, **When** the user keeps using the application within the idle timeout, **Then** the session remains valid and the idle lifetime is refreshed.
4. **Given** the persistent database, **When** the user logs in or uses an active session, **Then** no session record is written to or read from the persistent database.

---

### User Story 2 - User logs out and the session ends immediately (Priority: P2)

A user chooses to log out. The system removes the session from the in-memory data store and clears the session cookie immediately, so the browser can no longer access the authenticated area with that cookie.

**Why this priority**: Immediate logout is an existing security expectation that must continue working after the storage change.

**Independent Test**: Log in, then log out, and confirm the old session cookie is rejected on the next request and the session is gone from storage.

**Acceptance Scenarios**:

1. **Given** an active session, **When** the user logs out, **Then** the session is removed from storage, the cookie is cleared, and the user is taken to the unauthenticated area.
2. **Given** a cleared or unknown session cookie, **When** the browser makes a request, **Then** the system treats the request as anonymous and redirects to the login page.
3. **Given** a user who logs out without an active session cookie, **When** they access the logout flow, **Then** the system completes without errors.

---

### User Story 3 - Sessions are ephemeral: expiry and Redis restart discard them (Priority: P2)

Because session data is ephemeral, sessions survive application restarts but do not survive a restart of the in-memory data store (Redis). A user with a cookie from before a Redis restart is treated as anonymous and must log in again. Sessions that reach their expiry time are also discarded so the in-memory store does not grow without bound.

**Why this priority**: Ephemerality is the reason for the change; proving that expiry and Redis restart discard sessions — while ordinary application restarts do not — confirms the requirement is actually met.

**Independent Test**: Log in, restart only the application, and confirm the same cookie still works; then restart Redis (or flush it) and confirm the cookie is rejected; also verify expired sessions are automatically removed over time.

**Acceptance Scenarios**:

1. **Given** a session created before an application restart, **When** only the application restarts and the browser reuses the old cookie, **Then** the session still validates and the user stays logged in.
2. **Given** sessions that have passed their expiry time, **When** expiry handling runs, **Then** the expired sessions are removed from the in-memory store.
3. **Given** a long-running application with many logins, **When** sessions expire, **Then** the in-memory store returns to a bounded size instead of growing indefinitely.
4. **Given** an active session, **When** the in-memory data store restarts or loses its data, **Then** the session no longer validates and the user must log in again.

---

### Edge Cases

- What happens when a cookie is present but no matching session exists (e.g., after a Redis restart, expiry, or logout)? The request is treated as anonymous and redirected to login; no error is shown to the user.
- What happens when a session has passed its expiry time? The system rejects it and the user must log in again.
- What happens when a user is active but the absolute session lifetime is reached? The session expires; continued activity does not extend it beyond the absolute lifetime.
- What happens when many requests arrive concurrently for the same session? Each request is validated safely; the most recent idle refresh wins and no request fails due to the concurrent access.
- What happens when the same user logs in from two devices? Each login creates its own independent session; logging out from one device does not end the other session.
- What happens to session rows that existed in the persistent database before this change? They are discarded by the new forward-only migration that drops the `sessions` table; they are never used for validation.
- What happens when a user without any session accesses the logout flow? The flow completes gracefully without errors.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All session data MUST be held in a dedicated in-memory data store (Redis) that runs independently of the application process.
- **FR-002**: The persistent database MUST NOT be read from or written to for session creation, validation, refresh, or deletion.
- **FR-003**: On successful login, the system MUST create a session containing the user identity, the condominium, the CSRF token, the creation time, and the expiry time, and MUST issue the corresponding session cookie.
- **FR-004**: The system MUST validate the session cookie on each request against the in-memory session data and load the authenticated user when the session is valid.
- **FR-005**: A session MUST remain valid until the earliest of: logout, expiry, or loss/restart of the in-memory data store. Restarting the application alone MUST NOT invalidate sessions.
- **FR-006**: The system MUST keep the existing session lifetimes: an absolute lifetime of 7 days and an idle timeout of 12 hours, refreshed on activity.
- **FR-007**: Logout MUST immediately remove the session from the in-memory data store and clear the session cookie in the browser.
- **FR-008**: Expired sessions MUST be removed automatically so the in-memory store remains bounded over time.
- **FR-009**: The CSRF protection MUST continue to use the per-session token held in the in-memory session record.
- **FR-010**: Concurrent requests for the same session MUST be handled safely without corrupting or losing session data.
- **FR-011**: The same user MAY hold multiple concurrent sessions from different devices or browsers; ending one session MUST NOT affect the others.
- **FR-012**: The system MUST remove the persistent `sessions` table from the database schema through a new forward-only migration; existing session rows are discarded as part of this change.

### Key Entities *(include if feature involves data)*

- **Session**: a login-session record held in the dedicated in-memory data store (Redis); attributes: hashed session token, user identity, condominium, CSRF token, creation time, expiry time. It is referenced only through the session cookie and disappears on logout, expiry, or when Redis restarts or loses data.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of successful logins create a session that immediately authenticates the user's subsequent requests until logout, expiry, or loss/restart of the in-memory data store.
- **SC-002**: 100% of session cookies are rejected after logout or after the in-memory data store restarts or loses data; restarting only the application MUST NOT reject valid session cookies.
- **SC-003**: Zero session records are written to or read from the persistent database during login, authenticated use, idle refresh, and logout.
- **SC-004**: Expired sessions are removed automatically; after 24 hours of normal use, the number of stored sessions reflects only active, unexpired sessions with no unbounded growth.
- **SC-005**: Existing login, logout, session-validation, and CSRF behavior passes 100% of the applicable automated tests with no user-visible regression.
- **SC-006**: Authenticated page loads complete in under 1 second.

## Assumptions

- The application continues to run as a single process; Redis is a separate in-memory service added to `docker-compose.yml` and used only for session data. Multi-instance application deployments remain out of scope.
- Sessions survive application restarts; they are lost only when Redis restarts or loses its data, which is the accepted ephemeral boundary. Redis is configured as an ephemeral store with no disk persistence.
- The session cookie format and token hashing behavior remain unchanged; only the storage location changes.
- The existing session lifetimes (7-day absolute, 12-hour idle) remain the defaults.
- Dropping the historical `sessions` table (see FR-012) is accepted because those rows are ephemeral; no session data is migrated.
- No migration or import of existing persistent sessions is needed; users with sessions from before the change simply log in again.
- This change affects server-side login sessions only; other persistent data (users, condominiums, finance records, tickets, audit events) remains in the persistent database.
