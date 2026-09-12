---
description: "Task list for Vertical Slice Architecture Migration"
---

# Tasks: Vertical Slice Architecture Migration

**Input**: Design documents from `/specs/011-refactor-vertical-slice/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, quickstart.md

**Tests**: This refactor keeps the project's existing automated tests (constitution Principle IV and spec FR-007). Test files move together with the code they cover and must pass at the end of every feature migration. No new feature tests are added beyond adapting existing ones to the new layout and dependency style.

**Organization**: Tasks are grouped by user story. Each story is independently verifiable per its "Independent Test" in spec.md.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Exact file paths are included in task descriptions.

## Seam rule for this refactor (from research.md Decision 2a)

In feature packages, replace interface declarations with concrete types. Where a unit test needs a test double, use **function-typed fields** (method values/closures) instead of interfaces. Follow this rule in every `[US1]`/`[US2]` rewire task.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish a green baseline and a shared inventory before any code moves.

- [X] T001 Verify the pre-refactor baseline: run `go test ./...`, `make check`, and `go build ./cmd/web` and confirm all three pass.
- [X] T002 [P] Inventory the hexagonal files to migrate using the file lists in this tasks.md (no code changes yet).

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add the migration guardrail and the seam rule before touching feature code.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T003 Update `scripts/check-feature-boundaries.sh` to add a structure check: reject `internal/features/<feature>/core/` and `internal/features/<feature>/core/ports/` for any feature **not** listed in a new `scripts/vertical-slice-exempt.txt`; keep the existing cross-feature import check unchanged. Initialize `scripts/vertical-slice-exempt.txt` with all 7 features (`auth`, `directory`, `finance`, `home`, `management`, `profile`, `tickets`).
- [X] T004 [P] Add the seam rule to `specs/011-refactor-vertical-slice/research.md` as "Decision 2a": in feature packages use concrete types; where a unit test needs a double, use function-typed fields instead of interfaces.

**Checkpoint**: guardrail ready — feature migrations can begin.

---

## Phase 3: User Story 1 - Solo developer changes a feature inside one self-contained slice (Priority: P1) 🎯 MVP

**Goal**: Migrate one small feature (`home`) to the target vertical-slice layout, proving a developer can change it inside a single folder with no `core/` wrapper and no `ports/` layer.

**Independent Test**: `internal/features/home/` contains only `domain/`, `handlers/`, `services/`, `templates/` (no `core/`, no `ports/`); `go test ./internal/features/home/...` and `go build ./cmd/web` pass; `scripts/check-feature-boundaries.sh` passes after removing `home` from the exempt list.

### Implementation for User Story 1

- [X] T005 [US1] Move `internal/features/home/core/domain/doc.go` to `internal/features/home/domain/doc.go` (package `domain`).
- [X] T006 [US1] Move `internal/features/home/core/services/doc.go` to `internal/features/home/services/doc.go` (package `services`).
- [X] T007 [US1] Delete `internal/features/home/core/ports/doc.go` and remove the now-empty `internal/features/home/core/` directory.
- [X] T008 [US1] In `internal/features/home/handlers/deps.go`, replace the local `Roles` and `AuditRecorder` interfaces with function-typed fields (per the seam rule); update call sites in `internal/features/home/handlers/home.go` and `internal/features/home/handlers/areas.go`.
- [X] T009 [US1] Update `internal/features/home/handlers/home_test.go` fakes to closures/method values for the new function fields; run `go test ./internal/features/home/...`.
- [X] T010 [US1] Update `cmd/web/main.go` home wiring to pass `roles.ActiveRolesForUser` and `audit.RecordEvent` method values; run `go build ./cmd/web`.
- [X] T011 [US1] Remove `home` from `scripts/vertical-slice-exempt.txt`; run `scripts/check-feature-boundaries.sh` and `go test ./...`.

**Checkpoint**: User Story 1 complete — the pilot slice proves the new layout and DX.

---

## Phase 4: User Story 2 - All existing features are migrated without changing behavior (Priority: P2)

**Goal**: Migrate the remaining 6 features (`auth`, `directory`, `finance`, `management`, `profile`, `tickets`) to the same layout, preserving user-visible behavior.

**Independent Test**: After each feature's group completes, `go build ./cmd/web`, `go test ./internal/features/<feature>/...`, and the full `go test ./...` pass; `find internal/features -type d \( -name core -o -name ports \)` returns nothing after the final feature.

**⚠️ Execution order**: Migrate features **sequentially in the order below** so the build stays green after each feature (FR-008). Each feature group follows the same pattern: restructure → rewire services → rewire handlers → rewire `cmd/web/main.go` → verify and drop from the exempt list.

### 4.1 auth

- [X] T012 [US2] Restructure `internal/features/auth/`: move `core/domain/doc.go` → `domain/doc.go`; move `core/services/{auth.go,auth_test.go,password_reset.go,registration.go,registration_test.go}` → `services/`; delete `core/ports/{repositories.go,services.go}`; remove `core/`.
- [X] T013 [US2] Rewire `internal/features/auth/services/`: replace `core/ports` imports and interface fields with concrete `internal/shared/store` types (`*store.UserStore`, `*store.RoleStore`, `*store.InvitationStore`, `*store.AuditStore`) and `internal/shared/security` types (`*security.PasswordHasher`, `security.TokenHasher`); apply function-typed fields where `auth_test.go`/`registration_test.go` need doubles; update those test files.
- [X] T014 [US2] Rewire `internal/features/auth/handlers/deps.go` to import `internal/features/auth/services` instead of `core/services`; update `internal/features/auth/handlers/public_test.go` if it referenced ports.
- [X] T015 [US2] Update `cmd/web/main.go` auth wiring: import `authservices "github.com/leoarkiteto/zelo/internal/features/auth/services"`, construct services with concrete dependencies, and remove any `core/services`/`core/ports` imports; run `go build ./cmd/web` and `go test ./internal/features/auth/...`.
- [X] T016 [US2] Remove `auth` from `scripts/vertical-slice-exempt.txt`; run `scripts/check-feature-boundaries.sh`.

### 4.2 directory

- [X] T017 [P] [US2] Restructure `internal/features/directory/`: move `core/domain/doc.go` → `domain/doc.go`; move `core/services/{directory.go,directory_test.go}` → `services/`; delete `core/ports/{doc.go,repositories.go,services.go}`; remove `core/`.
- [X] T018 [US2] Rewire `internal/features/directory/services/`: replace ports interface fields with concrete `*repositories.ListingStore` and `*repositories.CategoryStore` (same feature) or shared store types; apply function-typed fields where `directory_test.go` needs doubles; update that test file.
- [X] T019 [US2] Rewire `internal/features/directory/handlers/deps.go` and `internal/features/directory/handlers/directory.go` to depend on `*services.DirectoryService` and concrete/function-typed collaborators.
- [X] T020 [US2] Update `cmd/web/main.go` directory wiring to the new import paths and concrete dependencies; run `go build ./cmd/web` and `go test ./internal/features/directory/...`.
- [X] T021 [US2] Remove `directory` from `scripts/vertical-slice-exempt.txt`; run `scripts/check-feature-boundaries.sh`.

### 4.3 finance

- [X] T022 [P] [US2] Restructure `internal/features/finance/`: move `core/domain/{finance.go,finance_test.go}` → `domain/`; move `core/services/{finance.go,finance_test.go}` → `services/`; delete `core/ports/repositories.go`; remove `core/`.
- [X] T023 [US2] Rewire `internal/features/finance/services/`: replace ports interface fields with the concrete `*repositories.AccountStore` (same feature) or shared store types; update `finance_test.go` doubles per the seam rule.
- [X] T024 [US2] Rewire `internal/features/finance/handlers/deps.go`, `finance.go`, and `receipt.go` to depend on `*services.FinanceService` and concrete/function-typed collaborators; update `internal/features/finance/handlers/finance_test.go`.
- [X] T025 [US2] Update `cmd/web/main.go` finance wiring to the new import paths and concrete dependencies; run `go build ./cmd/web` and `go test ./internal/features/finance/...`.
- [X] T026 [US2] Remove `finance` from `scripts/vertical-slice-exempt.txt`; run `scripts/check-feature-boundaries.sh`.

### 4.4 management

- [X] T027 [P] [US2] Restructure `internal/features/management/`: move `core/domain/doc.go` → `domain/doc.go`; move `core/services/{roles.go,roles_test.go,fakes_test.go}` → `services/`; delete `core/ports/{repositories.go,services.go}`; remove `core/`.
- [X] T028 [US2] Rewire `internal/features/management/services/`: replace ports interface fields with concrete shared store types; update `roles_test.go` and `fakes_test.go` per the seam rule.
- [X] T029 [US2] Rewire `internal/features/management/handlers/deps.go`, `roles.go`, and `invitations.go` to depend on `*services.RolesService` and concrete/function-typed collaborators.
- [X] T030 [US2] Update `cmd/web/main.go` management wiring to the new import paths and concrete dependencies; run `go build ./cmd/web` and `go test ./internal/features/management/...`.
- [X] T031 [US2] Remove `management` from `scripts/vertical-slice-exempt.txt`; run `scripts/check-feature-boundaries.sh`.

### 4.5 profile

- [X] T032 [P] [US2] Restructure `internal/features/profile/`: move `core/domain/{doc.go,language.go}` → `domain/`; move `core/services/{doc.go,profile.go,profile_test.go}` → `services/`; delete `core/ports/{repositories.go,services.go}`; remove `core/`.
- [X] T033 [US2] Rewire `internal/features/profile/services/`: replace `ports.LanguagePreferenceReader`/`ports.LanguagePreferenceUpdater` interface fields with concrete shared store types or function-typed fields per the seam rule; update `profile_test.go`.
- [X] T034 [US2] Rewire `internal/features/profile/handlers/deps.go` and `profile.go` to depend on `*services.ProfileService` instead of `ports.ProfileService`; update `language_toggle_test.go` and `profile_test.go`.
- [X] T035 [US2] Update `cmd/web/main.go` profile wiring to the new import paths and concrete dependencies; run `go build ./cmd/web` and `go test ./internal/features/profile/...`.
- [X] T036 [US2] Remove `profile` from `scripts/vertical-slice-exempt.txt`; run `scripts/check-feature-boundaries.sh`.

### 4.6 tickets

- [X] T037 [P] [US2] Restructure `internal/features/tickets/`: move `core/domain/{tickets.go,tickets_test.go}` → `domain/`; move `core/services/{tickets.go,tickets_test.go}` → `services/`; delete `core/ports/repositories.go`; remove `core/`.
- [X] T038 [US2] Rewire `internal/features/tickets/services/`: replace ports interface fields with the concrete `*repositories.TicketStore` (same feature) or shared store types; update `tickets_test.go` per the seam rule.
- [X] T039 [US2] Rewire `internal/features/tickets/handlers/deps.go` and `tickets.go` to depend on `*services.TicketService` and concrete/function-typed collaborators; update `internal/features/tickets/handlers/tickets_test.go`.
- [X] T040 [US2] Update `cmd/web/main.go` tickets wiring to the new import paths and concrete dependencies; run `go build ./cmd/web` and `go test ./internal/features/tickets/...`.
- [X] T041 [US2] Remove `tickets` from `scripts/vertical-slice-exempt.txt`; run `scripts/check-feature-boundaries.sh`.

**Checkpoint**: All features migrated; `find internal/features -type d \( -name core -o -name ports \)` returns nothing.

---

## Phase 5: User Story 3 - Tooling, scaffolding, and documentation enforce vertical slices only (Priority: P3)

**Goal**: Make the new layout the default and prevent the hexagonal ports/adapters pattern from returning.

**Independent Test**: `scripts/new-feature.sh demo` generates the new layout (no `core/`, no `ports/`); the structure check passes with the exempt list empty; the constitution, README, and REASONIX.md describe vertical slices only.

### Implementation for User Story 3

- [X] T042 [P] [US3] Update `scripts/_feature-template/`: delete `core/` template files; create `domain/`, `services/`, `repositories/`, `handlers/`, `templates/` doc files; update `scripts/_feature-template/README.md` to the new layout and remove ports guidance.
- [X] T043 [P] [US3] Update `scripts/new-feature.sh` "Next steps" text to reference the new layout (`domain/`, `services/`, `handlers/`, `repositories/`, `templates/`) and remove ports instructions.
- [X] T044 [US3] Make the structure check in `scripts/check-feature-boundaries.sh` unconditional: empty `scripts/vertical-slice-exempt.txt` and update the script header comment to describe the enforced vertical-slice layout.
- [X] T045 [P] [US3] Update `specs/004-feature-folder-isolation/contracts/feature-folder-layout.md` to the new layout: role-based sub-folders at the feature root, no `core/` wrapper, no `ports/` layer.
- [X] T046 [P] [US3] Update `.specify/memory/constitution.md` Principles II/III and the Repository Layout section to describe vertical slices only (remove hexagonal ports/adapters mandates).
- [X] T047 [P] [US3] Update `REASONIX.md` and `README.md` architecture sections to the vertical-slice-only structure.

**Checkpoint**: Tooling and docs enforce the new architecture.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup across the whole refactor.

- [X] T048 [P] Run `gofmt -l .` and `go vet ./...`; fix any findings introduced by the file moves.
- [X] T049 [P] Run the integration suite with real services: `TEST_DATABASE_URL=... TEST_REDIS_URL=... go test ./tests/...`.
- [X] T050 Run the full `quickstart.md` validation: `find internal/features -type d \( -name core -o -name ports \)` returns nothing, `go test ./...` passes, `make check` passes, and a `scripts/new-feature.sh demo` scaffold smoke test produces the new layout (then remove `internal/features/demo`).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Setup; blocks all user stories.
- **User Story 1 (Phase 3)**: Depends on Foundational; no dependency on other stories.
- **User Story 2 (Phase 4)**: Depends on Foundational; features must be migrated sequentially in the given order (each group leaves the build green).
- **User Story 3 (Phase 5)**: Depends on User Story 2 completing the structure check's subject (all features migrated).
- **Polish (Phase 6)**: Depends on all user stories.

### User Story Dependencies

- **US1 (P1)**: after Foundational — pilot `home` migration.
- **US2 (P2)**: after Foundational (can start after US1) — migrate `auth` → `directory` → `finance` → `management` → `profile` → `tickets`.
- **US3 (P3)**: after US2 — scaffold, boundary check hardening, docs, constitution.

### Within Each Feature Migration (US2)

1. Restructure folders (move domain/services, delete ports/core).
2. Rewire services + tests to concrete/function-typed dependencies.
3. Rewire handlers + tests.
4. Rewire `cmd/web/main.go` and verify `go build` + feature tests.
5. Drop the feature from the exempt list and run the boundary check.

### Parallel Opportunities

- T002, T004 (Setup/Foundational) can run in parallel.
- T017, T022, T027, T032, T037 (folder restructures) touch different feature folders and can be prepared in parallel, **but** each feature's `cmd/web/main.go` rewire must complete before the next feature starts to keep the build green (FR-008).
- T042/T043 (scaffold files) and T045/T046/T047 (different docs) can run in parallel.
- T048 (fmt/vet) and T049 (integration tests) can run in parallel.

### Parallel Example: US3 documentation

```bash
Task: "Update scripts/_feature-template/ ..."
Task: "Update scripts/new-feature.sh ..."
Task: "Update specs/004-feature-folder-isolation/contracts/feature-folder-layout.md ..."
Task: "Update .specify/memory/constitution.md ..."
Task: "Update REASONIX.md and README.md ..."
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational (guardrail + seam rule).
3. Complete Phase 3: User Story 1 (`home` pilot).
4. **STOP AND VALIDATE**: verify the pilot per its Independent Test.
5. Review the pattern with the user before proceeding to US2 (optional but recommended).

### Incremental Delivery

1. Setup + Foundational → guardrail ready.
2. US1 (`home`) → first vertical slice proven.
3. US2 feature by feature → each leaves the app green (auth → directory → finance → management → profile → tickets).
4. US3 → scaffold, checks, docs, constitution updated.
5. Polish → full validation (`go test ./...`, `make check`, integration tests, quickstart).

### Solo Developer Strategy

This refactor is designed for one developer: the phases are sequential, each feature migration is a complete green unit, and the exempt-list guardrail lets `make check` pass between migrations while still failing on any regression after a feature is removed from the list.

---

## Notes

- [P] tasks = different files, no dependencies.
- [Story] label maps each task to spec.md US1/US2/US3.
- Keep the build green after every feature (FR-008): never start the next feature before the current one's `go build ./cmd/web` + tests pass.
- Follow the seam rule from research.md Decision 2a: concrete types; function-typed fields for test doubles (no new interfaces).
- Do not hand-edit `*_templ.go` or `web/static/css/output.css`; file moves do not require regeneration, but run `templ generate`/`make tailwind` if any `.templ` or `input.css` file actually moves.
- Commit after each task or logical group.
