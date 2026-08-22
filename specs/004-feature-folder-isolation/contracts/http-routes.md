# Contract: HTTP Routes (unchanged)

**Feature**: [spec.md](../spec.md) | **Date**: 2026-08-22

This reorganization MUST preserve the public HTTP surface exactly. The routes below are the current application contract; after migration, every route must resolve to the same behavior as before. No routes may be added, removed, or changed in method/path.

## Public routes

- `GET /register`
- `POST /register`
- `GET /login`
- `POST /login`
- `GET /password/forgot`
- `POST /password/forgot`
- `GET /password/reset`
- `POST /password/reset`
- `POST /logout`
- `GET /`

## Role-restricted routes

- `GET /condominium` — syndic only
- `GET /unit` — owner or syndic
- `GET /tenancy` — tenant only

## Service directory routes (member access + syndic moderation)

- `GET /directory`
- `GET /directory/new`
- `POST /directory`
- `GET /directory/{id}/edit` — syndic only
- `POST /directory/{id}/edit` — syndic only
- `GET /directory/{id}/delete` — syndic only
- `POST /directory/{id}/delete` — syndic only
- `GET /directory/categories` — syndic only
- `POST /directory/categories` — syndic only
- `POST /directory/categories/{id}/rename` — syndic only
- `POST /directory/categories/{id}/deactivate` — syndic only

## Management routes (syndic only)

- `GET /invitations`
- `POST /invitations`
- `POST /invitations/{id}/revoke`
- `GET /roles`
- `POST /roles/assign`
- `POST /roles/revoke`

## Static assets

- `GET /static/*` — served from `web/static` (unchanged)

## Middleware contract

The following cross-cutting behaviors MUST remain applied to the routes above, in the same order as today:

1. Panic recovery
2. Request logging
3. Security headers
4. User loading (`WithUser`)
5. CSRF protection

## Validation

After migration, the route inventory above is checked by:

- `go test ./...` (handler integration tests cover the routes)
- The quickstart walkthrough in [quickstart.md](../quickstart.md)
