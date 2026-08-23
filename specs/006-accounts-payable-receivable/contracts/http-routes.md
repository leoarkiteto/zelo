# HTTP Routes Contract: Gestão de Contas a Pagar e Receber

Feature: `specs/006-accounts-payable-receivable/spec.md`

All routes are server-rendered HTML. State-changing routes (`POST`) require a valid
CSRF token submitted as a form field (`csrf_token`); an invalid or missing token
returns `403 Forbidden`.

Access model:
- Unauthenticated visitors are redirected to `/login` (existing `RequireAuth`).
- Authenticated users with no active role in their session condominium receive `403`
  with a friendly explanation (existing `RequireRole` deny path).
- Finance management routes allow the `syndic` role only.
- Resident routes (`/finance/my-charges`, `/finance/health`) allow `owner`, `tenant`,
  and `syndic`.

## Syndic management routes

| Method | Path | Purpose | Success | Errors |
|--------|------|---------|---------|--------|
| GET | `/finance` | Dashboard + list with filters: `q` (keyword), `type` (`payable`/`receivable`), `category` (key), `status` (`pending`/`settled`/`overdue`/`canceled`), `period` (`YYYY-MM`) | `200` | — |
| GET | `/finance/new?type=payable\|receivable` | Show creation form for the given account type | `200` | `400` invalid/missing `type` |
| POST | `/finance` | Create an account (multipart when a receipt is attached) | `303` → `/finance` | `400` validation errors re-rendered |
| GET | `/finance/{id}/edit` | Show edit form (only `pending`/`overdue`) | `200` | `404` not found or not in condominium; `400` if `settled`/`canceled` |
| POST | `/finance/{id}/edit` | Update editable fields (multipart when replacing receipt) | `303` → `/finance` | `400` validation errors re-rendered; `404` not found |
| GET | `/finance/{id}/settle` | Show settlement form (only `pending`/`overdue`) | `200` | `404` not found; `400` if already `settled`/`canceled` |
| POST | `/finance/{id}/settle` | Mark settled with `settlement_date` | `303` → `/finance` | `400` missing/invalid date re-rendered; `404` not found |
| GET | `/finance/{id}/cancel` | Show cancel confirmation page (only `pending`/`overdue`) | `200` | `404` not found; `400` if already `settled`/`canceled` |
| POST | `/finance/{id}/cancel` | Mark canceled | `303` → `/finance` | `404` not found |
| GET | `/finance/{id}/receipt` | Download the attached comprovante (payable only) | `200` file stream | `404` not found or no receipt |

## Resident routes

| Method | Path | Purpose | Success | Errors |
|--------|------|---------|---------|--------|
| GET | `/finance/my-charges` | List receivables linked to the user's active unit | `200` | `403`/`404`-style message when the user has no active unit |
| GET | `/finance/health?period=YYYY-MM` | Aggregated financial health: total gasto, total arrecadado | `200` | — |

`/finance/health` never exposes payees, suppliers, receipts, or other units' individual
accounts — only condominium-level aggregates (spec FR-017).

## Create/edit form contract

Used by `GET/POST /finance/new` (create) and `GET/POST /finance/{id}/edit` (edit).

| Field | Create | Edit | Validation |
|-------|--------|------|------------|
| `type` | required (select `payable`/`receivable`) | not editable | `payable` or `receivable` |
| `title` | required | required | 1–120 characters |
| `category` | required | required | Valid key for the account type |
| `amount` | required | required | Decimal, max 2 places, `> 0` |
| `due_date` | required | required | Valid date (`YYYY-MM-DD`) |
| `status` | required (select `pending`/`settled`) | not editable | `pending` or `settled` on create |
| `settlement_date` | required when `status=settled` | read-only | Valid date |
| `supplier_payee` | optional (payable) | optional (payable) | Max 120 characters |
| `receipt` | optional file (payable) | optional file, replaces existing (payable) | Max 5 MB; PDF/JPEG/PNG |
| `unit_id` | optional (receivable) | optional (receivable) | Must be a unit of the session condominium |
| `payment_code` | optional (receivable) | optional (receivable) | Max 200 characters |
| `csrf_token` | required | required | Valid CSRF token |

The server always supplies `condominium_id`, `created_by`, and type-specific defaults;
users never submit those values. On edit, the account must be `pending` or `overdue`;
`settled` and `canceled` accounts reject edits (FR-019).

## Settle form contract

| Field | Required | Validation |
|-------|----------|------------|
| `settlement_date` | required | Valid date (`YYYY-MM-DD`) |
| `csrf_token` | required | Valid CSRF token |

## Cancel form contract

| Field | Required | Validation |
|-------|----------|------------|
| `csrf_token` | required | Valid CSRF token |

The cancel confirmation page is a plain form with only `csrf_token`; the target account
must be `pending` or `overdue`.

## Filter query contract (`GET /finance`)

| Param | Values | Behavior |
|-------|--------|----------|
| `q` | free text | Escaped `ILIKE` match on `title` and `supplier_payee` |
| `type` | `payable` / `receivable` | Exact match on `account_type` |
| `category` | category key | Exact match on `category` |
| `status` | `pending` / `settled` / `overdue` / `canceled` | `overdue` expands to `status='pending' AND due_date < CURRENT_DATE` |
| `period` | `YYYY-MM` | Restricts `due_date` to that month |

Combining filters ANDs all conditions. No matches → friendly empty state.
