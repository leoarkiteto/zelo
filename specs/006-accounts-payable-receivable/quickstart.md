# Quickstart Validation Guide: Gestão de Contas a Pagar e Receber

Feature: `specs/006-accounts-payable-receivable/spec.md`

Runnable validation scenarios that prove the feature end-to-end after implementation.
Details live in [data-model.md](./data-model.md) and
[contracts/http-routes.md](./contracts/http-routes.md).

## Prerequisites

- Go 1.27+
- Docker with Compose
- Existing auth/RBAC slice working (users, roles, sessions, CSRF)
- Existing i18n slice working (`en` / `pt-br`)
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
#    UPLOAD_DIR=uploads   (optional; defaults to uploads/)

# 3. Apply migrations (includes 0009_create_finance_accounts.sql)
make migrate

# 4. Generate templates and run the server
make templ
make run
```

Seed or create, via the existing invitation flow: one syndic, one owner, and one tenant
in the same condominium, with the owner/tenant occupying a unit.

## Validation scenarios

### 1. Access control

1. Open `/finance` signed out → redirected to `/login`.
2. Sign in as the owner → `/finance` returns `403` (syndic-only management area).
3. Sign in as the syndic → `/finance` renders the dashboard and list.
4. Sign in as the owner and open `/finance/my-charges` → renders.
5. Sign in as the tenant and open `/finance/health` → renders aggregates.

**Expected**: only the syndic manages accounts; residents reach only the two member
routes, per [contracts/http-routes.md](./contracts/http-routes.md).

### 2. Create payable and receivable accounts

1. As the syndic, open `/finance/new?type=payable`.
2. Submit: title `Manutenção do portão`, category `maintenance`, amount `350.00`,
   due date in the future → account appears as `Pendente`.
3. Open `/finance/new?type=receivable` and submit a `condo_fee` of `500.00` linked to
   the owner's unit → account appears as `Pendente`.
4. Try to create an account with amount `0` and with `-10` → form re-renders with a
   validation error and nothing is saved.
5. Try to create an account without a title → form re-renders with a validation error.

**Expected**: valid accounts are created in under 1 minute each; zero/negative amounts
are blocked (spec FR-002, FR-005, FR-011, SC-001, SC-002).

### 3. Backdated settled creation and overdue derivation

1. Create a payable with `status=settled`, a past `settlement_date`, and a past
   `due_date` → account appears as `Liquidado` with the date recorded.
2. Create a payable with `status=pending` and a `due_date` of yesterday → the list
   shows it as `Em Atraso` with a visual alert, without any manual status change.
3. Create a receivable with `due_date` of today → it remains `Pendente` (not overdue).

**Expected**: settled creation requires the transaction date; overdue is automatic and
visual (spec FR-008, FR-009, FR-010, FR-012, SC-003).

### 4. Settle and cancel

1. As the syndic, open `/finance/{id}/settle` for an overdue account, submit with a
   `settlement_date` → account becomes `Liquidado` and leaves the overdue highlight.
2. Try to settle without a date → form re-renders with an error and no state change.
3. Open `/finance/{id}/cancel` for a pending account and confirm → account becomes
   `Cancelado`.
4. Try to edit a `Liquidado` or `Cancelado` account → rejected; the account is
   read-only.

**Expected**: settlement always records the transaction date; cancellation and
read-only rules match spec FR-010, FR-019, SC-007.

### 5. Search and filters

1. With mixed accounts in place, on `/finance` filter by `type=receivable` → only
   receivables appear.
2. Filter by `status=overdue` → only receivables/payables with `due_date` in the past
   and `status=pending` appear (the "inadimplentes" filter).
3. Filter by `category=condo_fee` → only condo fees appear.
4. Filter by `period=YYYY-MM` of the current month → only accounts due that month.
5. Search `portão` → only the matching title appears; search `zzz` → friendly empty
   state.

**Expected**: filters combine correctly and the overdue filter shows exactly the
overdue accounts (spec FR-014, FR-015, SC-005, SC-006).

### 6. Dashboard math

1. In the current month, create a receivable of `500.00` and a payable of `350.00`.
2. Open `/finance` → dashboard shows Total a Receber `500.00`, Total a Pagar `350.00`,
   Saldo Previsto `150.00`.
3. Settle the receivable (with a settlement date in the current month) → Saldo
   Realizado increases by `500.00`; settle the payable → Saldo Realizado shows
   `150.00`.
4. Change any account status and re-open `/finance` → totals reflect the change with no
   manual refresh action.

**Expected**: totals and saldos are recalculated automatically on every status change
(spec FR-013, FR-018, SC-004).

### 7. Resident views

1. Sign in as the owner with a pending receivable linked to their unit → open
   `/finance/my-charges` and see the charge with value, due date, and status.
2. Confirm no other unit's charges appear and no supplier/receipt data is shown.
3. Open `/finance/health` → see aggregated totals gasto/arrecadado for the period, with
   no confidential details.
4. Sign in as a member with no active unit (if available) → `/finance/my-charges`
   shows the friendly no-unit state.

**Expected**: residents see only their own pendências and aggregates (spec FR-016,
FR-017, SC-008).

### 8. Receipt upload and download

1. As the syndic, create a payable with a PDF receipt attached (under 5 MB).
2. Open `/finance/{id}/receipt` → the file downloads.
3. Sign in as the owner and try `/finance/{id}/receipt` → `403`.
4. Try to upload a file over 5 MB or a non-PDF/JPEG/PNG type → form re-renders with a
   validation error.

**Expected**: receipts are syndic-only and validated (spec Assumptions, R6 in
[research.md](./research.md)).

### 9. Automated tests

```bash
go test ./...
scripts/check-feature-boundaries.sh
```

**Expected**: stdlib-based unit and integration tests pass, covering the finance
domain, repositories, handlers, access control, overdue derivation, dashboard math,
receipt upload/download, and the `0009_create_finance_accounts.sql` migration; the
feature boundary check reports `Feature boundaries OK`.
