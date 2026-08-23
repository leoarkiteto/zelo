# Phase 0 Research: Gestão de Contas a Pagar e Receber

Feature: `specs/006-accounts-payable-receivable/spec.md`

Constraint input: GOTTH stack, standard library first (no frameworks/ORM), CSS-first
over JavaScript, vertical slice isolation, TDD, i18n (`en` + `pt-br`). All Technical
Context unknowns resolved below.

## R1 — Feature slice placement and shape

- **Decision**: One new self-contained vertical slice at `internal/features/finance/`
  with `core/{domain,ports,services}`, `handlers/`, `repositories/`, and `templates/`.
  Domain types (`FinancialAccount`, `AccountType`, `AccountStatus`, `Category`, money
  parsing) live in `core/domain`; repositories and handlers import only that
  same-feature package or `internal/shared/*`.
- **Rationale**: Matches the constitution's vertical slice layout and the existing
  `internal/features/*` convention. Keeping the persisted model inside the slice
  (rather than `internal/shared/model`) makes the slice fully self-contained, as
  required by Principle III.
- **Alternatives considered**:
  - Placing `FinancialAccount` in `internal/shared/model` (like the older directory
    listing model) — pollutes shared code with a feature-specific model and weakens
    slice isolation (rejected).
  - A separate `payables` + `receivables` pair of slices — both share the same status
    lifecycle, list, and dashboard; one slice avoids duplicated code and keeps the
    feature cohesive (rejected).

## R2 — Money representation

- **Decision**: Store amounts as `BIGINT` integer cents (`amount_cents`), strictly
  greater than zero. Forms submit the amount as a decimal string (e.g. `1234.56`);
  the domain parser converts it to cents and rejects values with more than two decimal
  places, zero, or negative amounts.
- **Rationale**: Integer cents avoid floating-point rounding in sums and comparisons
  and make `SUM`/ordering exact. A positive check plus a two-decimal parse enforces
  spec FR-011 (no zero/negative values).
- **Alternatives considered**:
  - `NUMERIC(12,2)` — exact too, but requires care scanning into Go values and is
    heavier than integer arithmetic for summaries (rejected).
  - `float8` — rounding risk for financial data (rejected).
  - Storing decimal strings — breaks ordering and aggregation (rejected).

## R3 — Status model and `Em Atraso` derivation

- **Decision**: Persist only `pending`, `settled`, and `canceled`. `overdue` (`Em
  Atraso`) is derived at query time: a row is overdue when `status = 'pending'` and
  `due_date < CURRENT_DATE`. List/filter/summary queries expose an `effective_status`
  computed column; no background job flips rows.
- **Rationale**: Spec FR-009 requires automatic, always-correct overdue signalling.
  Derivation makes it impossible for a row to be stale and removes the need for a
  scheduler. A persisted `overdue` would need a job to roll it back if the due date is
  edited or a payment is backdated.
- **Alternatives considered**:
  - Scheduled job updating `status` to `overdue` — adds a background worker and stale
    windows; the codebase has no scheduler today (rejected).
  - Persisting `overdue` and recomputing on every read — duplicates the same logic in
    writes instead of reads and still needs a job (rejected).

## R4 — Categories as stable keys

- **Decision**: Store `account_type` (`payable` | `receivable`) and a stable `category`
  key. Display labels come from the existing i18n catalog (`finance.cat.payable.*`,
  `finance.cat.receivable.*`) in `en` and `pt-br`. A `CHECK` constraint enforces the
  valid key set per `account_type`.
- **Rationale**: The app is bilingual (spec 005). Stable keys keep the data
  language-neutral and make filtering deterministic; labels are a presentation concern
  and belong in the i18n catalog.
- **Alternatives considered**:
  - Storing Portuguese display text — breaks the `pt-br`/`en` switcher and makes
    filtering depend on UI text (rejected).
  - A `categories` table — the spec fixes the category lists and says customization is
    out of scope; a table adds joins and management UI for no v1 benefit (rejected).

## R5 — Dashboard metric definitions

- **Decision**: All summary numbers are SQL aggregations, not stored:
  - **Total a Receber no mês**: sum of `amount_cents` for non-canceled `receivable`
    accounts whose `due_date` falls in the selected month.
  - **Total a Pagar no mês**: same for `payable` accounts.
  - **Saldo Previsto**: Total a Receber − Total a Pagar (same month).
  - **Saldo Realizado**: receivables with `settlement_date` in the month minus payables
    with `settlement_date` in the month.
- **Rationale**: This matches the spec's wording ("Total a Receber/Pagar no mês" by
  vencimento; "Saldo Previsto vs. Realizado" by previsão vs. liquidação) and keeps the
  dashboard a read-only projection that can never drift from the accounts.
- **Alternatives considered**:
  - A materialized `finance_summaries` table — would need refresh logic and can drift
    (rejected).
  - Computing in Go — would load every account into memory; SQL aggregates are
    simpler and faster at this scale (rejected).

## R6 — Receipt attachment (comprovante) storage

- **Decision**: Receipts are uploaded as part of the payable create/edit form
  (multipart). Files are stored on the server filesystem under `UPLOAD_DIR` (new
  config, default `uploads/`), with the relative path and original filename stored in
  `finance_accounts.receipt_path` / `receipt_name`. Downloads go through an
  authenticated `GET /finance/{id}/receipt` route restricted to the syndic — never
  through the public static file server. Limits: 5 MB per file; allowed types PDF,
  JPEG, PNG.
- **Rationale**: Receipts are confidential financial documents (spec Assumptions:
  "restritos ao gestor"), so they must not sit under `web/static`. Filesystem storage
  keeps the database small and streams easily; the existing stdlib HTTP server handles
  file serving without new dependencies.
- **Alternatives considered**:
  - Storing bytes in the database (`bytea`) — bloats rows and backups with no benefit
    at this scale (rejected).
  - Serving from `web/static/uploads` — exposes receipts to anyone with the URL
    (rejected).
  - External object storage — new dependency/infrastructure not justified for v1
    (rejected).

## R7 — Access model and resident scoping

- **Decision**: Reuse the existing `middleware.RequireAuth` + `middleware.RequireRole`
  stack. All `/finance` management routes require `syndic`. Resident routes
  (`GET /finance/my-charges`, `GET /finance/health`) allow `owner`, `tenant`, and
  `syndic`. `my-charges` resolves the current user's active unit via the existing
  `UnitStore.GetActiveUnitForUser` and lists only receivables with that `unit_id`.
  `health` shows condominium-level aggregates with no payee/receipt detail.
- **Rationale**: Matches spec FR-016/FR-017 and the existing directory access pattern;
  no new middleware is needed.
- **Alternatives considered**:
  - New `RequireManager` middleware — duplicates `RequireRole` (rejected).
  - Deriving the unit from `unit_occupancies` in the handler — the existing
    `GetActiveUnitForUser` already encodes the "one active unit" v1 rule (rejected).

## R8 — List, search, and filter implementation

- **Decision**: Plain parameterized SQL over `finance_accounts`. `GET /finance` accepts
  `q`, `type`, `category`, `status`, and `period` (`YYYY-MM`). Keyword search uses
  escaped `ILIKE` on `title` and `supplier_payee`; `status=overdue` translates to
  `status='pending' AND due_date < CURRENT_DATE`; `period` bounds `due_date` to the
  month. No full-text search engine.
- **Rationale**: At condominium scale, `ILIKE` over a few thousand rows is fast and
  dependency-free. Translating the derived `overdue` status in SQL keeps the filter and
  list consistent.
- **Alternatives considered**:
  - PostgreSQL `tsvector` — unnecessary moving parts for this scale (rejected).
  - Client-side filtering — violates CSS-first/server-rendered constraints (rejected).

## R9 — Audit events

- **Decision**: Record `audit_events` for the four state-changing finance actions:
  `account_created`, `account_edited`, `account_settled`, `account_canceled`. Migration
  `0009` drops and re-adds the `audit_events.event_type` CHECK constraint (same pattern
  as migration `0007`) to extend the allow-list.
- **Rationale**: Financial mutations are security-relevant and the constitution's
  existing audit model already records role/directory events. Reusing it preserves the
  audit trail required for prestação de contas.
- **Alternatives considered**:
  - No audit events — inconsistent with the rest of the app (rejected).
  - A separate finance audit table — duplicates the existing infrastructure (rejected).

## R10 — Retention, cancellation, and deletion

- **Decision**: No hard delete in v1. Cancel is a state transition available only for
  `pending`/`overdue` accounts; `settled` and `canceled` are read-only. Rows are never
  physically removed, which trivially satisfies the 5-year retention rule (FR-022);
  archiving after 5 years is deferred to a later version.
- **Rationale**: Matches the spec's status rules and the clarification (creation only
  as `Pendente` or `Liquidado`). Avoiding delete keeps accounting history intact and
  removes the need for retention timers in v1.
- **Alternatives considered**:
  - Soft delete `deleted_at` — redundant with `canceled` and adds filtering noise
    (rejected).
  - Hard delete with a retention check — unnecessary risk to the accounting record
    (rejected).
