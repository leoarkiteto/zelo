# HTTP Routes Contract: RBAC Authentication & Authorization

Feature: `specs/001-rbac-auth/spec.md`

All routes are server-rendered HTML. State-changing routes (`POST`) require a valid
CSRF token submitted as a form field (`csrf_token`); an invalid or missing token
returns `403 Forbidden`.

## Public routes

| Method | Path | Purpose | Success | Errors |
|--------|------|---------|---------|--------|
| GET | `/register?token=<invitation>` | Show registration form pre-filled from the invitation | `200` | `410` invalid/expired/used invitation |
| POST | `/register` | Create account using the invitation token and chosen credentials | `303` → `/login` | `400` validation errors re-rendered; `409` email already registered; `410` invalid/expired/used invitation |
| GET | `/login` | Show sign-in form | `200` | — |
| POST | `/login` | Authenticate user, start session | `303` → `/` | `401` invalid credentials (generic); `423` account temporarily locked |
| POST | `/logout` | End current session, delete server session | `303` → `/login` | `401` if not signed in |
| GET | `/password/forgot` | Show password reset request form | `200` | — |
| POST | `/password/forgot` | Send reset link (does not reveal account existence) | `303` → `/login` | — |
| GET | `/password/reset` | Show reset form for a valid token | `200` | `410` expired/invalid token |
| POST | `/password/reset` | Set a new password | `303` → `/login` | `400` validation errors; `410` expired/invalid token |

## Authenticated routes

Requires a valid session. If unauthenticated, redirect to `/login` with a
`next` query parameter (or return `401` for non-browser requests).

| Method | Path | Allowed roles | Purpose |
|--------|------|---------------|---------|
| GET | `/` | any authenticated role | Role-appropriate landing/dashboard |
| GET | `/condominium` | `syndic` only | Condominium management area |
| GET | `/unit` | `owner`, `syndic` | Own unit management area |
| GET | `/tenancy` | `tenant` only | Own tenancy area |
| GET | `/invitations` | `syndic` only | List pending/accepted invitations |
| POST | `/invitations` | `syndic` only | Create an owner/tenant invitation |
| POST | `/invitations/{id}/revoke` | `syndic` only | Revoke a pending invitation |
| GET | `/roles` | `syndic` only | List users and roles in the condominium |
| POST | `/roles/assign` | `syndic` only | Grant owner/tenant/syndic role (eligibility enforced) |
| POST | `/roles/revoke` | `syndic` only | Revoke owner/tenant/syndic role |

Role restriction behavior: a signed-in user who requests a route outside their role
receives `403 Forbidden` with a friendly explanation (spec US3 scenario 4). Direct
link access from a signed-out user redirects to `/login`.

## Registration form contract

| Field | Required | Validation |
|-------|----------|------------|
| `token` | yes | Valid pending invitation token; role comes from the invitation, never from the form |
| `email` | yes | Valid email; unique case-insensitively; must match `invited_email` when the invitation has one |
| `password` | yes | 12–256 characters |
| `password_confirm` | yes | Must match `password` |
| `csrf_token` | yes | Valid CSRF token |

The form never offers a `role` selector; submitting `role=syndic` is rejected.

## Invitation form contract (syndic only)

| Field | Required | Validation |
|-------|----------|------------|
| `unit_id` | yes | A unit in the syndic's condominium |
| `invited_role` | yes | Exactly `owner` or `tenant` |
| `invited_email` | no | Valid email if provided; pre-fills registration |
| `csrf_token` | yes | Valid CSRF token |

## Sign-in form contract

| Field | Required | Validation |
|-------|----------|------------|
| `email` | yes | Present |
| `password` | yes | Present |
| `csrf_token` | yes | Valid CSRF token |

Sign-in failure returns one generic error ("invalid email or password") and must not
reveal whether the email is registered. After 5 consecutive failures for the same
account, the account is locked for 15 minutes and sign-in returns `423 Locked` with a
generic message.
