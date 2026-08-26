# HTTP Routes Contract: Ticket Management

Feature: `specs/007-ticket-management/spec.md`

All routes are server-rendered HTML. State-changing routes (`POST`) require a valid
CSRF token submitted as a form field (`csrf_token`); an invalid or missing token
returns `403 Forbidden`.

Access model:
- Unauthenticated visitors are redirected to `/login` (existing `RequireAuth`).
- Authenticated users with no active role in their session condominium receive `403`
  with a friendly explanation (existing `RequireRole` deny path).
- Resident routes (`/tickets`, `/tickets/new`, `/tickets/{id}`) allow `owner`,
  `tenant`, and `syndic`.
- Syndic routes (`/tickets/inbox`, `/tickets/{id}/reply`, `/tickets/{id}/close`)
  allow the `syndic` role only.

## Resident routes

| Method | Path | Purpose | Success | Errors |
|--------|------|---------|---------|--------|
| GET | `/tickets` | List the current user's own tickets, newest first, with status and reply count | `200` | — |
| GET | `/tickets/new` | Show the ticket creation form | `200` | `403` when the user has no active unit |
| POST | `/tickets` | Create a ticket | `303` → `/tickets/{id}` | `400` validation errors re-rendered; `403` no active unit |
| GET | `/tickets/{id}` | View one ticket: description, status, replies, and (for `open` tickets in the syndic's hands) the reply/close forms | `200` | `404` not found, not in condominium, or not the author (members); `400` if a member tries a foreign ticket id |

For `GET /tickets/{id}`: a resident (`owner`/`tenant`) may open only their own ticket;
the syndic may open any ticket of the session condominium. Foreign or missing tickets
return `404` so ticket existence is not leaked (FR-009).

## Syndic routes

| Method | Path | Purpose | Success | Errors |
|--------|------|---------|---------|--------|
| GET | `/tickets/inbox?status=open\|closed\|all` | List tickets for the condominium; `open` (default) oldest-first, `closed`/`all` newest-first | `200` | `400` invalid `status` value falls back to `open` |
| POST | `/tickets/{id}/reply` | Add a reply to an `open` ticket | `303` → `/tickets/{id}` | `400` empty/too-long content re-rendered; `404` not found; `400` if already `closed` |
| POST | `/tickets/{id}/close` | Close an `open` ticket | `303` → `/tickets/inbox` | `404` not found; `400` if already `closed` |

`POST /tickets/{id}/reply` and `POST /tickets/{id}/close` are rejected on `closed`
tickets (FR-008). Closing is allowed even when the ticket has no replies (FR-007).

## Create form contract

Used by `GET/POST /tickets/new`.

| Field | Required | Validation |
|-------|----------|------------|
| `title` | required | Trimmed non-empty, 1–120 characters |
| `category` | required | One of `repair`, `noise_complaint`, `assembly_topic`, `other` |
| `description` | required | Trimmed non-empty, 1–2000 characters |
| `csrf_token` | required | Valid CSRF token |

The server always supplies `condominium_id`, `author_id`, `unit_id` (from the active
unit), `status = open`, and `created_at`; users never submit those values.

## Reply form contract

Used on `GET /tickets/{id}` (syndic, open ticket) and submitted to
`POST /tickets/{id}/reply`.

| Field | Required | Validation |
|-------|----------|------------|
| `content` | required | Trimmed non-empty, 1–2000 characters |
| `csrf_token` | required | Valid CSRF token |

## Close form contract

Used on `GET /tickets/{id}` (syndic, open ticket) and submitted to
`POST /tickets/{id}/close`.

| Field | Required | Validation |
|-------|----------|------------|
| `csrf_token` | required | Valid CSRF token |

The close confirmation is a plain form with only `csrf_token`; the target ticket must
be `open`.

## Inbox filter contract (`GET /tickets/inbox`)

| Param | Values | Behavior |
|-------|--------|----------|
| `status` | `open` / `closed` / `all` | `open` (default): open tickets oldest-first; `closed`: closed tickets newest-first; `all`: every ticket newest-first. Invalid values fall back to `open`. |

No matches → friendly empty state with a hint to check back later (syndic) or to
create the first ticket (resident).
