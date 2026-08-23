# Data Model: Gestão de Contas a Pagar e Receber

Feature: `specs/006-accounts-payable-receivable/spec.md` — persistence model and
validation rules for the `finance` slice. Access is via `database/sql` + the `pgx`
stdlib driver with plain SQL. Implementation details (migration SQL, repository code)
belong to `tasks.md` and the implementation phase.

## Entities

### FinancialAccount

A single financial movement: an account to pay (despesa) or to receive (receita).

| Field | Type | Rules |
|-------|------|-------|
| `id` | UUID, PK, default `gen_random_uuid()` | Immutable |
| `condominium_id` | UUID, FK → `condominiums.id` ON DELETE CASCADE | Required; set from the session condominium |
| `account_type` | text, CHECK `IN ('payable','receivable')` | Required; fixed after creation |
| `title` | text | Required, 1–120 chars |
| `category` | text | Required; must be valid for `account_type` (see category keys below) |
| `amount_cents` | bigint | Required; `> 0` (FR-011) |
| `due_date` | date | Required; `overdue` is derived from it |
| `status` | text, CHECK `IN ('pending','settled','canceled')` | Required; `overdue` is never persisted |
| `settlement_date` | date, nullable | Required when `status = 'settled'`; must be `NULL` otherwise (FR-010) |
| `supplier_payee` | text, nullable | Optional; payable only; max 120 chars |
| `receipt_path` | text, nullable | Optional; payable only; relative path of the uploaded comprovante |
| `receipt_name` | text, nullable | Optional; payable only; original uploaded filename |
| `unit_id` | UUID, nullable, FK → `units.id` ON DELETE SET NULL | Optional; receivable only; unidade responsável |
| `payment_code` | text, nullable | Optional; receivable only; código/linha de pagamento, max 200 chars |
| `created_by` | UUID, FK → `users.id` ON DELETE RESTRICT | Required; the gestor who created the account |
| `created_at` | timestamptz | Default `now()` |
| `updated_at` | timestamptz | Updated on edit/status change |

Type-specific integrity checks (in the migration):

```text
CHECK (amount_cents > 0)
CHECK (
  (account_type = 'payable'   AND category IN ('maintenance','cleaning','utilities','payroll','third_party_services','other'))
  OR
  (account_type = 'receivable' AND category IN ('condo_fee','fine_interest','common_area_reservation','extraordinary_income'))
)
CHECK ((status = 'settled' AND settlement_date IS NOT NULL)
    OR (status <> 'settled' AND settlement_date IS NULL))
CHECK (account_type <> 'payable'   OR (unit_id IS NULL AND payment_code IS NULL))
CHECK (account_type <> 'receivable' OR (supplier_payee IS NULL AND receipt_path IS NULL AND receipt_name IS NULL))
```

### Category (value object, not a table)

Stable keys persisted in `category`; display labels come from the i18n catalog.

| account_type | category key | pt-br label | en label |
|---|---|---|---|
| payable | `maintenance` | Manutenção | Maintenance |
| payable | `cleaning` | Limpeza | Cleaning |
| payable | `utilities` | Utilidades (Água/Luz/Gás) | Utilities (Water/Electricity/Gas) |
| payable | `payroll` | Folha de Pagamento | Payroll |
| payable | `third_party_services` | Serviços de Terceiros | Third-party services |
| payable | `other` | Diversos | Other |
| receivable | `condo_fee` | Cota Condominial | Condominium fee |
| receivable | `fine_interest` | Multa/Juros | Fines/Interest |
| receivable | `common_area_reservation` | Reserva de Espaço Comum | Common area reservation |
| receivable | `extraordinary_income` | Receitas Extraordinárias | Extraordinary income |

### AccountStatus (value object)

| Stored value | Effective status shown | Meaning |
|---|---|---|
| `pending` | `pending` | Pendente when `due_date >= CURRENT_DATE` |
| `pending` | `overdue` | Em Atraso when `due_date < CURRENT_DATE` (derived, FR-009) |
| `settled` | `settled` | Liquidado (Pago/Recebido) |
| `canceled` | `canceled` | Cancelado |

### DashboardSummary (derived, not stored)

Computed by SQL aggregation per condominium and month:

- `total_receivable_cents` — sum of non-canceled receivables with `due_date` in month.
- `total_payable_cents` — sum of non-canceled payables with `due_date` in month.
- `projected_balance_cents` — `total_receivable_cents − total_payable_cents`.
- `realized_balance_cents` — settled receivables in month (by `settlement_date`) minus
  settled payables in month (by `settlement_date`).

## Relationships

```text
FinancialAccount N ──── 1 Condominium  (condominium_id)
FinancialAccount N ──── 1 User         (created_by)
FinancialAccount N ──── 0..1 Unit      (unit_id, receivable only)
```

- A `FinancialAccount` belongs to exactly one `Condominium`.
- A `FinancialAccount` is created by exactly one `User` (the gestor).
- A receivable may reference at most one `Unit`; the reference is nullable so
  condominium-wide receivables exist and historical rows survive unit deletion.
- Payable-only fields (`supplier_payee`, `receipt_*`) are `NULL` for receivables;
  receivable-only fields (`unit_id`, `payment_code`) are `NULL` for payables.

## State Transitions

```text
creation ──▶ pending ────────── settle(settlement_date) ──▶ settled (terminal, read-only)
              │  │
              │  └────────────── cancel ──────────────────▶ canceled (terminal, read-only)
              │
              │ (due_date < today, derived at read time — no write)
              ▼
            overdue ─────────── settle(settlement_date) ──▶ settled
              │
              └────────────── cancel ────────────────────▶ canceled
```

- Creation is allowed only as `pending` or `settled`; creating as `settled` requires
  `settlement_date` (clarification session 2026-08-23).
- `settle` requires a transaction date (FR-010) and moves `pending`/`overdue` to
  `settled`.
- `cancel` moves `pending`/`overdue` to `canceled`.
- `settled` and `canceled` are read-only (FR-019).
- `overdue` is a projection: `status = 'pending' AND due_date < CURRENT_DATE`.

## Planned migration

1. `0009_create_finance_accounts.sql`:
   - `finance_accounts` table with the columns and CHECK constraints above.
   - Indexes:
     - `idx_finance_accounts_list` on `(condominium_id, account_type, status, due_date)`
       for the list/filters/dashboard.
     - `idx_finance_accounts_unit_due` on `(unit_id, due_date) WHERE account_type = 'receivable' AND status = 'pending'`
       for resident pendências.
     - `idx_finance_accounts_settled` on `(condominium_id, account_type, settlement_date)`
       for saldo realizado.
   - Extend `audit_events.event_type` CHECK allow-list with `account_created`,
     `account_edited`, `account_settled`, `account_canceled` (drop/re-add constraint,
     same pattern as migration `0007`).

## Validation Rules

- `title`: required, 1–120 characters.
- `category`: required; must be one of the keys valid for the chosen `account_type`.
- `amount`: decimal string with at most two decimal places; parsed to integer cents;
  must be `> 0`.
- `due_date`: required; any valid calendar date (past dates allowed so backdated
  entries are possible).
- `status` at creation: `pending` or `settled`; `settled` requires `settlement_date`.
- `settlement_date`: required for `settled` rows; must be `NULL` for other rows.
- `supplier_payee` / `payment_code`: optional, length-limited; only for their
  respective account types.
- `unit_id`: optional for receivables; must be a unit of the session condominium when
  provided.
- Receipt upload: optional; max 5 MB; content type PDF, JPEG, or PNG.
