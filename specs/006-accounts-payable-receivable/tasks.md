---
description: "Task list for feature implementation"
---

# Tasks: Gestão de Contas a Pagar e Receber

**Input**: Design documents from `/specs/006-accounts-payable-receivable/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: INCLUDED — the zelo constitution mandates TDD for all feature work (red-green-refactor). Every story phase starts with failing tests before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Go module root: `github.com/leoarkiteto/zelo`; app code under `internal/`, entrypoint `cmd/web/`
- Vertical slice: `internal/features/finance/` with `core/{domain,ports,services}`, `handlers/`, `repositories/`, `templates/`
- Go tests live next to the code they test (`*_test.go`), matching the existing repo
- Templates: `internal/features/finance/templates/*.templ` (+ generated `*_templ.go` via `scripts/templ-generate.sh`)
- Migrations: `migrations/NNNN_description.sql` (forward-only, applied by `make migrate`)
- i18n: `internal/shared/i18n/catalog.go` (both `enMessages` and `ptbrMessages`)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Slice skeleton and shared configuration/i18n touched by the finance feature

- [X] T001 Create the finance vertical slice directories under `internal/features/finance/`: `core/domain`, `core/ports`, `core/services`, `handlers`, `repositories`, `templates`
- [X] T002 [P] Add `UploadDir` to `internal/shared/config/config.go` (env `UPLOAD_DIR`, default `uploads/`) and document `UPLOAD_DIR` in `.env.example`
- [X] T003 [P] Add finance i18n keys to `internal/shared/i18n/catalog.go` in both `enMessages` and `ptbrMessages`: navigation (`nav.finance`), page titles/subtitles, form labels, status labels (`pending`/`settled`/`overdue`/`canceled`), category labels per `data-model.md`, validation messages, and flash messages
- [X] T004 [P] Add `uploads/` to `.gitignore` so receipt files are never committed

**Checkpoint**: `go build ./...` still compiles; `go test ./internal/shared/config ./internal/shared/i18n` passes.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Migration and core domain/persistence types every user story depends on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T005 Write migration `migrations/0009_create_finance_accounts.sql` per `specs/006-accounts-payable-receivable/data-model.md`: `finance_accounts` table with type/category/settlement/amount CHECK constraints, indexes (`idx_finance_accounts_list`, `idx_finance_accounts_unit_due`, `idx_finance_accounts_settled`), and drop/re-add of the `audit_events.event_type` CHECK to add `account_created`, `account_edited`, `account_settled`, `account_canceled`
- [X] T006 [P] Write failing domain tests in `internal/features/finance/core/domain/finance_test.go` covering money parsing (decimal string → cents, reject zero/negative/more than 2 decimals), category validity per `account_type`, and status transition rules
- [X] T007 Implement `internal/features/finance/core/domain/finance.go` per `data-model.md`: `AccountType`, `AccountStatus`, `Category`, `FinancialAccount`, money parser, and transition helpers (`CanEdit`, `CanSettle`, `CanCancel`) until T006 is green (depends on T006)
- [X] T008 [P] Create `internal/features/finance/core/ports/repositories.go` with `AccountStore` (Create/Get/Update/List/StatusUpdate), `SummaryStore` (SummaryForMonth), `UnitResolver`, and `AuditRecorder` ports per `research.md` R1/R7
- [X] T009 Implement `internal/features/finance/repositories/accounts.go` with `database/sql` adapters for `AccountStore` and `SummaryStore` per `data-model.md` (parameterized SQL, `effective_status` projection for `overdue`, escaped `ILIKE` search) (depends on T005, T007, T008)
- [X] T010 [P] Write failing repository integration tests in `internal/features/finance/repositories/accounts_test.go` (create/get/list with filters/summary queries against dockerized PostgreSQL) and make them green

**Checkpoint**: `make migrate` applies `0009` cleanly; `go test ./internal/features/finance/...` passes; foundation ready for story work.

---

## Phase 3: User Story 1 - Gestor cadastra uma despesa (conta a pagar) (Priority: P1) 🎯 MVP

**Goal**: The syndic can create a payable account with title, category, amount, due date, initial status, supplier/payee, and receipt attachment.

**Independent Test**: Sign in as syndic, open `/finance/new?type=payable`, submit `Manutenção do portão` / `maintenance` / `350.00` / future due date, and confirm it appears in `/finance` as `Pendente`.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T011 [P] [US1] Write failing service test in `internal/features/finance/core/services/finance_test.go` for payable `CreateAccount` (required fields, positive amount, `supplier_payee`, receipt metadata validation, audit event)
- [X] T012 [P] [US1] Write failing handler test in `internal/features/finance/handlers/finance_test.go` for `GET /finance/new?type=payable` and `POST /finance` (syndic-only access, CSRF, validation errors, multipart receipt upload, redirect)

### Implementation for User Story 1

- [X] T013 [US1] Create `FinanceService` in `internal/features/finance/core/services/finance.go` with `CreateAccount` payable path (title/category/amount/due date/status/`supplier_payee`/receipt metadata validation, audit `account_created`) until T011 is green (depends on T011)
- [X] T014 [US1] Create `internal/features/finance/handlers/deps.go` (`Deps` struct) and `internal/features/finance/handlers/routes.go` registering syndic-only finance routes with `middleware.RequireAuth` + `middleware.RequireRole(model.RoleSyndic)` per `contracts/http-routes.md`
- [X] T015 [US1] Implement `GET /finance/new?type=payable` and `POST /finance` handlers in `internal/features/finance/handlers/finance.go` (parse form, multipart receipt upload with 5 MB / PDF-JPEG-PNG limits, store under `UploadDir`, map service errors to i18n messages, redirect to `/finance`) until T012 is green
- [X] T016 [US1] Implement `GET /finance/{id}/receipt` in `internal/features/finance/handlers/receipt.go` (syndic-only, streams the stored comprovante; `404` when absent)
- [X] T017 [US1] Create `internal/features/finance/templates/finance.templ` with the list page and payable creation form (Templ + Tailwind + i18n, file input for receipt) and run `scripts/templ-generate.sh`
- [X] T018 [US1] Wire finance deps and `RegisterRoutes` into `cmd/web/main.go` (construct repository/service/handlers and register routes)
- [X] T019 [US1] Add template smoke test `internal/features/finance/templates/smoke_test.go` and run `scripts/tailwind-build.sh`

**Checkpoint**: User Story 1 fully functional — payable creation + receipt upload/download works and is independently testable.

---

## Phase 4: User Story 2 - Gestor lança uma receita (conta a receber) (Priority: P1)

**Goal**: The syndic can create a receivable account linked to a unit (or to the condominium/external origin) with payment code/line.

**Independent Test**: Sign in as syndic, open `/finance/new?type=receivable`, submit a `condo_fee` of `500.00` linked to a unit, and confirm it appears in `/finance` as `Pendente`.

### Tests for User Story 2 ⚠️

- [X] T020 [P] [US2] Write failing service test in `internal/features/finance/core/services/finance_test.go` for receivable `CreateAccount` (`unit_id` validation against session condominium, `payment_code`, category validation, no payable-only fields)
- [X] T021 [P] [US2] Write failing handler test in `internal/features/finance/handlers/finance_test.go` for `GET /finance/new?type=receivable` and `POST /finance` (unit select data, payment code, type-specific errors)

### Implementation for User Story 2

- [X] T022 [US2] Implement the receivable path in `FinanceService.CreateAccount` in `internal/features/finance/core/services/finance.go` (`unit_id`, `payment_code`, unit belongs to condominium) until T020 is green (depends on T020)
- [X] T023 [US2] Implement the `type=receivable` branch in `handlers/finance.go` (load units for the select, parse `unit_id`/`payment_code`) until T021 is green
- [X] T024 [US2] Add the receivable creation form (unit select, payment code field) to `internal/features/finance/templates/finance.templ` and regenerate with `scripts/templ-generate.sh`

**Checkpoint**: User Stories 1 AND 2 both work independently — payable and receivable creation complete.

---

## Phase 5: User Story 3 - Gestor atualiza o status de uma conta (Priority: P1)

**Goal**: The syndic settles (with transaction date) or cancels pending/overdue accounts; settled/canceled accounts become read-only; overdue highlighting is shown automatically.

**Independent Test**: Create a payable with a past due date, confirm it shows `Em Atraso`, settle it with a date, and confirm it becomes `Liquidado`; cancel another pending account and confirm `Cancelado`.

### Tests for User Story 3 ⚠️

- [X] T025 [P] [US3] Write failing service tests in `internal/features/finance/core/services/finance_test.go` for `SettleAccount` (requires `settlement_date`), `CancelAccount` (only pending/overdue), `EditAccount` (only pending/overdue), and overdue derivation
- [X] T026 [P] [US3] Write failing handler tests in `internal/features/finance/handlers/finance_test.go` for `GET/POST /finance/{id}/settle`, `GET/POST /finance/{id}/cancel`, and `GET/POST /finance/{id}/edit` (status guards, CSRF, `404`/`400` behavior)

### Implementation for User Story 3

- [X] T027 [US3] Implement `SettleAccount`, `CancelAccount`, and `EditAccount` in `internal/features/finance/core/services/finance.go` (settlement date required, pending/overdue only, read-only settled/canceled, audit events `account_settled`/`account_canceled`/`account_edited`) until T025 is green
- [X] T028 [US3] Implement settle/cancel/edit handlers in `internal/features/finance/handlers/finance.go` (confirmation pages for cancel, settlement date form, edit form reusing create form fields) until T026 is green
- [X] T029 [US3] Add settle/cancel/edit forms and the overdue visual badge to `internal/features/finance/templates/finance.templ`; regenerate with `scripts/templ-generate.sh`
- [X] T030 [US3] Ensure repository status updates enforce the transition contract (e.g., `UPDATE ... WHERE status IN ('pending','overdue')` via derived guard) in `internal/features/finance/repositories/accounts.go`

**Checkpoint**: Account lifecycle complete — create, edit (while pending/overdue), settle, cancel, and automatic overdue signalling all work.

---

## Phase 6: User Story 4 - Gestor filtra e busca lançamentos (Priority: P2)

**Goal**: The syndic filters/search the account list by period, type, category, and status (including the derived `overdue`/inadimplentes filter).

**Independent Test**: With mixed accounts in `/finance`, apply `type=receivable`, `status=overdue`, `category=condo_fee`, `period=YYYY-MM`, and `q` search individually and confirm only matching rows appear.

### Tests for User Story 4 ⚠️

- [X] T031 [P] [US4] Write failing repository test in `internal/features/finance/repositories/accounts_test.go` for `ListAccounts` filters (q, type, category, status incl. `overdue` expansion, period)
- [X] T032 [P] [US4] Write failing handler test in `internal/features/finance/handlers/finance_test.go` for `GET /finance` query-param parsing and empty state

### Implementation for User Story 4

- [X] T033 [US4] Implement `ListAccounts` filter SQL in `internal/features/finance/repositories/accounts.go` (escaped `ILIKE` on `title`/`supplier_payee`, `status='pending' AND due_date < CURRENT_DATE` for `overdue`, month bounds) until T031 is green
- [X] T034 [US4] Implement `FinanceService.ListAccounts` and handler filter parsing in `internal/features/finance/handlers/finance.go` until T032 is green
- [X] T035 [US4] Add the filter form (search text, type, category, status, period) and friendly empty state to `internal/features/finance/templates/finance.templ`; regenerate with `scripts/templ-generate.sh`

**Checkpoint**: The full list page supports all spec filters, including the inadimplentes (`overdue`) filter.

---

## Phase 7: User Story 5 - Gestor visualiza o resumo financeiro (dashboard) (Priority: P2)

**Goal**: The syndic sees Total a Receber no mês, Total a Pagar no mês, Saldo Previsto, and Saldo Realizado, recalculated on every load/status change.

**Independent Test**: With known accounts due/settled in the current month, open `/finance` and verify all four dashboard numbers match the accounts.

### Tests for User Story 5 ⚠️

- [X] T036 [P] [US5] Write failing service test in `internal/features/finance/core/services/finance_test.go` for `Summary` (totals by due month, realized balance by settlement month)
- [X] T037 [P] [US5] Write failing handler test in `internal/features/finance/handlers/finance_test.go` for dashboard data rendering

### Implementation for User Story 5

- [X] T038 [US5] Implement `SummaryForMonth` SQL in `internal/features/finance/repositories/accounts.go` and `FinanceService.Summary` in `core/services/finance.go` until T036 is green
- [X] T039 [US5] Render the dashboard section (totais + saldos) in `internal/features/finance/handlers/finance.go` and `internal/features/finance/templates/finance.templ`; regenerate with `scripts/templ-generate.sh`

**Checkpoint**: Dashboard totals are correct and update automatically whenever an account status changes.

---

## Phase 8: User Story 6 - Morador visualiza taxas/cotas pendentes da sua unidade (Priority: P2)

**Goal**: A resident sees only the receivables linked to their active unit, with value, due date, and status.

**Independent Test**: Sign in as an owner with a pending receivable linked to their unit and open `/finance/my-charges`; confirm the charge appears and other units' charges do not.

### Tests for User Story 6 ⚠️

- [X] T040 [P] [US6] Write failing service test in `internal/features/finance/core/services/finance_test.go` for `ResidentCharges` (active unit scoping, no cross-unit leakage)
- [X] T041 [P] [US6] Write failing handler test in `internal/features/finance/handlers/finance_test.go` for `GET /finance/my-charges` (member access, no-active-unit state)

### Implementation for User Story 6

- [X] T042 [US6] Implement `FinanceService.ResidentCharges` in `internal/features/finance/core/services/finance.go` using `UnitResolver.GetActiveUnitForUser` until T040 is green
- [X] T043 [US6] Implement `GET /finance/my-charges` route/handler (member middleware: `owner`, `tenant`, `syndic`) in `internal/features/finance/handlers/routes.go` and `handlers/finance.go` until T041 is green
- [X] T044 [US6] Add the my-charges template (pendências list with overdue highlight) to `internal/features/finance/templates/finance.templ`; regenerate with `scripts/templ-generate.sh`

**Checkpoint**: Residents can see their own pending charges and nothing else.

---

## Phase 9: User Story 7 - Morador visualiza a saúde financeira do condomínio (Priority: P3)

**Goal**: A resident sees aggregated condominium totals (total gasto, total arrecadado) without confidential details.

**Independent Test**: Sign in as a tenant and open `/finance/health`; confirm aggregated totals appear and no payee/supplier/receipt detail is exposed.

### Tests for User Story 7 ⚠️

- [X] T045 [P] [US7] Write failing service test in `internal/features/finance/core/services/finance_test.go` for `Health` (period aggregates, no confidential detail in output)
- [X] T046 [P] [US7] Write failing handler test in `internal/features/finance/handlers/finance_test.go` for `GET /finance/health` (member access)

### Implementation for User Story 7

- [X] T047 [US7] Implement `FinanceService.Health` in `internal/features/finance/core/services/finance.go` (condominium-level totals by period) until T045 is green
- [X] T048 [US7] Implement `GET /finance/health` route/handler and the health template in `internal/features/finance/handlers/` and `templates/finance.templ`; regenerate with `scripts/templ-generate.sh`

**Checkpoint**: Residents can see aggregated financial health with no confidential data.

---

## Phase 10: Polish & Cross-Cutting Concerns

**Purpose**: Navigation, full-suite verification, and the quickstart validation pass

- [X] T049 [P] Add role-aware finance navigation entries to the shared navigation in `internal/shared/templates/` (syndic sees `/finance`; members see `/finance/my-charges` and `/finance/health`)
- [X] T050 [P] Ensure all new i18n keys exist in both languages by running the existing catalog completeness test and fixing any gaps in `internal/shared/i18n/catalog.go`
- [X] T051 Run `scripts/templ-generate.sh`, `scripts/tailwind-build.sh`, `make migrate`, `go test ./...`, and `scripts/check-feature-boundaries.sh`; fix all failures
- [X] T052 Run every scenario in `specs/006-accounts-payable-receivable/quickstart.md` end-to-end and confirm expected outcomes
- [X] T053 Review the final diff for accidental leftovers, generated files committed, and no cross-feature imports

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 and US2 can proceed in parallel (if staffed)
  - US3 builds on US1/US2 (needs existing accounts)
  - US4/US5 build on US1-US3
  - US6/US7 build on US2/US3
- **Polish (Phase 10)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — no dependencies on other stories
- **User Story 2 (P1)**: Can start after Foundational (Phase 2) — no dependencies on other stories
- **User Story 3 (P1)**: Depends on US1/US2 (accounts must exist to settle/cancel/edit)
- **User Story 4 (P2)**: Depends on US1/US2/US3 (list data + overdue derivation)
- **User Story 5 (P2)**: Depends on US1/US2/US3 (totals need settled/pending accounts)
- **User Story 6 (P2)**: Depends on US2/US3 (receivables + statuses)
- **User Story 7 (P3)**: Depends on US1/US2/US3 (aggregates over both types)

### Within Each User Story

- Tests (included — constitution TDD) MUST be written and FAIL before implementation
- Domain/ports before services/repositories
- Services before handlers
- Handlers before templates render correctly
- Core implementation before integration/wiring
- Story complete before moving to next priority

### Parallel Opportunities

- Phase 1: T002, T003, T004 can run in parallel
- Phase 2: T006/T008/T010 can start in parallel with T005 (different files)
- Phase 3-9: the two test tasks in each story are [P] and can be written together; US1 and US2 can be implemented in parallel by different developers
- Phase 10: T049 and T050 are independent

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "T011 service test for payable CreateAccount in internal/features/finance/core/services/finance_test.go"
Task: "T012 handler test for payable form in internal/features/finance/handlers/finance_test.go"

# After tests fail, implement together where files differ:
Task: "T013 FinanceService.CreateAccount in internal/features/finance/core/services/finance.go"
Task: "T014 deps/routes in internal/features/finance/handlers/"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (payable creation + receipt)
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → foundation ready
2. Add User Story 1 → test independently → deploy/demo (MVP!)
3. Add User Story 2 → test independently → deploy/demo
4. Add User Story 3 → full account lifecycle → deploy/demo
5. Add US4/US5 (filters + dashboard) → deploy/demo
6. Add US6/US7 (resident views) → deploy/demo
7. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (payable create)
   - Developer B: User Story 2 (receivable create)
3. Then Developer A: US3 (status lifecycle); Developer B: US4 (filters)
4. Continue in priority order, committing after each task or logical group

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps the task to a user story for traceability
- Each user story is independently completable and testable
- Verify tests fail before implementing (constitution TDD)
- Commit after each task or logical group
- Stop at any checkpoint to validate the story independently
- Avoid: vague tasks, same-file conflicts, cross-story dependencies that break independence

---

## Phase 11: Convergence

**Purpose**: Close gaps found when assessing the codebase against spec/plan/tasks (outcome: tasks_appended)

- [X] T054 Include both payable and receivable categories in the `/finance` category filter when no `type` filter is selected — pass an empty/union category list instead of `domain.CategoriesFor("")` in `internal/features/finance/handlers/finance.go` per FR-014 (partial)
- [X] T055 Add a dedicated "inadimplentes" filter (receivable + overdue) so the filter shows only overdue receivables — update filter parsing in `internal/features/finance/handlers/finance.go` and the filter form in `internal/features/finance/templates/finance.templ` per FR-015 / SC-005 (partial)
- [X] T056 Add handler tests for receipt upload validation (5 MB, PDF/JPEG/PNG) and authenticated download (syndic-only, resident `403`) in `internal/features/finance/handlers/finance_test.go` per T012 / plan R6 (partial)
- [X] T057 Sniff the uploaded receipt's content type with `http.DetectContentType` and reject mismatches before storing in `internal/features/finance/handlers/receipt.go` per plan R6 (partial)
