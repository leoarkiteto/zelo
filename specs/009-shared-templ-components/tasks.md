---
description: "Task list for Shared Templ Components (Atomic Design)"
---

# Tasks: Shared Templ Components (Atomic Design)

**Input**: Design documents from `/specs/009-shared-templ-components/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Tests are included because the project constitution mandates TDD for all feature implementations (write tests first, see them fail, then implement). All render smoke tests are Go standard-library tests; no new test tooling is introduced.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- Shared Templ layers: `internal/shared/templates/{atoms,molecules,organisms}/`
- Feature templates: `internal/features/<feature>/templates/*.templ`
- Feature design docs: `specs/009-shared-templ-components/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Create the atomic layer skeleton and shared helpers before any component work

- [X] T001 Create the three atomic layer directories `internal/shared/templates/atoms/`, `internal/shared/templates/molecules/`, and `internal/shared/templates/organisms/` per `specs/009-shared-templ-components/plan.md`
- [X] T002 [P] Add `internal/shared/templates/atoms/doc.go` with `package atoms` and a package doc stating atoms are the smallest reusable UI pieces and must not import molecules/organisms
- [X] T003 [P] Add `internal/shared/templates/molecules/doc.go` with `package molecules` and a package doc stating molecules compose atoms and must not import organisms
- [X] T004 [P] Add `internal/shared/templates/organisms/doc.go` with `package organisms` and a package doc stating organisms are composite page sections that may compose molecules and atoms

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Test helper and boundary enforcement that every user story depends on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T005 Create `internal/shared/templates/testutil/render.go` exposing `RenderToString(t *testing.T, c templ.Component) string` so every layer's smoke tests render components without duplicating helper code
- [X] T006 Create `scripts/check-template-atomic-boundaries.sh` that exits non-zero when a file under `internal/shared/templates/atoms` imports `molecules` or `organisms`, a file under `molecules` imports `organisms`, or any shared template package imports `internal/features/**`
- [X] T007 Run `make templ && go test ./...` from repository root and confirm the baseline suite passes before any template moves (record the output as the pre-migration baseline for `specs/009-shared-templ-components/quickstart.md`)

**Checkpoint**: Layers exist, render helper and boundary check ready, baseline green — user story work can begin

---

## Phase 3: User Story 1 - Developer finds shared Templ components in a predictable structure (Priority: P1) 🎯 MVP

**Goal**: Make the atomic layers discoverable and documented so a developer can find any shared component by its layer.

**Independent Test**: Open `internal/shared/templates/` and confirm `atoms/`, `molecules/`, and `organisms/` each exist with a `doc.go` that describes the layer; read `internal/shared/templates/README.md` and confirm it matches `specs/009-shared-templ-components/contracts/shared-templ-components.md`.

### Tests for User Story 1

No render tests needed yet; this story is about structure and documentation.

### Implementation for User Story 1

- [X] T008 [US1] Create `internal/shared/templates/README.md` documenting the three atomic layers, the component inventory from `specs/009-shared-templ-components/data-model.md`, and the import rules from `specs/009-shared-templ-components/contracts/shared-templ-components.md`
- [X] T009 [P] [US1] Expand `internal/shared/templates/atoms/doc.go` to list the atom components (Button, Badge, Icon, TextInput, TextArea, Select) and the rule that atoms never import higher layers
- [X] T010 [P] [US1] Expand `internal/shared/templates/molecules/doc.go` to list the molecule components (Card, FormField, Alert, EmptyState) and the rule that molecules may only import atoms
- [X] T011 [P] [US1] Expand `internal/shared/templates/organisms/doc.go` to list the organism components (Layout, Shell, ErrorPage) and the rule that organisms may compose molecules and atoms

**Checkpoint**: Any developer can locate the layer of every shared component in under 2 minutes using the README and package docs

---

## Phase 4: User Story 2 - Features reuse shared components instead of copying markup (Priority: P2)

**Goal**: Extract repeated markup from feature templates into shared atoms/molecules and update feature pages to compose them, eliminating duplication.

**Independent Test**: Run `go test ./...`, then grep feature templates per `specs/009-shared-templ-components/quickstart.md` and confirm repeated `btn`/`card`/`badge`/`alert`/`empty-state`/inline-SVG blocks now come from shared components.

### Tests for User Story 2 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T012 [P] [US2] Write failing render smoke tests for atoms in `internal/shared/templates/atoms/atoms_test.go`: Button, Badge, Icon, TextInput, TextArea, Select render with expected classes/text and do not panic on empty props
- [X] T013 [P] [US2] Write failing render smoke tests for molecules in `internal/shared/templates/molecules/molecules_test.go`: Card, FormField, Alert, EmptyState render with expected structure and content

### Implementation for User Story 2

- [X] T014 [P] [US2] Implement `Button` with `ButtonProps`/`ButtonVariant` in `internal/shared/templates/atoms/button.templ` (wraps existing `btn btn-<variant>` classes; renders anchor when `Href` is set)
- [X] T015 [P] [US2] Implement `Badge` with `BadgeProps`/`BadgeVariant` in `internal/shared/templates/atoms/badge.templ` (wraps existing `badge badge-<variant>` classes)
- [X] T016 [P] [US2] Implement `Icon` with `IconProps`/`IconKind` in `internal/shared/templates/atoms/icon.templ` consolidating the inline SVGs and the `QuickActionIcon` switch from `internal/features/home/templates/dashboard.templ`
- [X] T017 [P] [US2] Implement `TextInput`, `TextArea`, and `Select` in `internal/shared/templates/atoms/input.templ` with the current form-control classes
- [X] T018 [US2] Implement `Card` with `CardProps` in `internal/shared/templates/molecules/card.templ` (wraps `card card-pad` with optional title/body/footer)
- [X] T019 [US2] Implement `FormField` with `FormFieldProps` in `internal/shared/templates/molecules/form_field.templ` composing the atom form controls with label/error/hint
- [X] T020 [P] [US2] Implement `Alert` with `AlertProps`/`AlertVariant` in `internal/shared/templates/molecules/alert.templ` (wraps `alert alert-<variant>`)
- [X] T021 [P] [US2] Implement `EmptyState` with `EmptyStateProps` in `internal/shared/templates/molecules/empty_state.templ` (wraps `empty-state` with icon/title/copy/action)
- [X] T022 [US2] Run `make templ` from repository root to generate `*_templ.go` for atoms/molecules, then run `go test ./internal/shared/templates/atoms/... ./internal/shared/templates/molecules/...` and confirm the new tests pass
- [X] T023 [P] [US2] Update `internal/features/home/templates/dashboard.templ` to use `atoms.Icon` for quick-action icons and `atoms.Badge` for role badges; remove the local `QuickActionIcon` component and the old role badge helper call
- [X] T024 [P] [US2] Update `internal/features/auth/templates/login.templ`, `register.templ`, `forgot_password.templ`, and `reset_password.templ` to use shared `atoms.Button`, atom form controls, and `molecules.Alert`; keep `AuthShell` in `internal/features/auth/templates/auth.templ` feature-local
- [X] T025 [P] [US2] Update `internal/features/directory/templates/directory.templ` to use shared Card/Button/Badge/Alert/EmptyState components for its repeated blocks
- [X] T026 [P] [US2] Update `internal/features/finance/templates/finance.templ` to use shared Card/Button/Badge/Alert components for its repeated blocks
- [X] T027 [P] [US2] Update `internal/features/management/templates/invitations.templ` and `internal/features/management/templates/roles.templ` to use shared Card/Button/Badge/Alert/EmptyState components
- [X] T028 [P] [US2] Update `internal/features/profile/templates/profile.templ` to use shared atom form controls and `molecules.Card`/`molecules.Alert`
- [X] T029 [P] [US2] Update `internal/features/tickets/templates/tickets.templ` to use shared Button/Badge/EmptyState/Card components for its repeated blocks
- [X] T030 [US2] Run `make templ && go test ./...` from repository root; confirm every test passes and the grep checks in `specs/009-shared-templ-components/quickstart.md` show no remaining duplicated shared markup

**Checkpoint**: Feature templates compose shared components; pages still render identically and no feature-local copies of shared markup remain

---

## Phase 5: User Story 3 - Existing shared Templ components migrate without visual change (Priority: P3)

**Goal**: Move the existing shared components (`Layout`/`Shell`, `ErrorPage`, role badge helper) into their atomic layers and update every feature import, preserving rendered output.

**Independent Test**: Run `go test ./...` and the manual page smoke in `specs/009-shared-templ-components/quickstart.md`; confirm `/`, `/login`, `/directory`, `/tickets`, `/finance`, and the error page look identical to before.

### Tests for User Story 3 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before the move**

- [X] T031 [P] [US3] Write failing organisms render tests in `internal/shared/templates/organisms/organisms_test.go` covering Layout (html lang/title/head), Shell (app-shell element, topbar, sidebar nav), and ErrorPage (status/message/dashboard link)

### Implementation for User Story 3

- [X] T032 [US3] Move `Layout`, `Shell`, `ShellData`, `NavItem`, and `UserView` from `internal/shared/templates/layout.templ` into `internal/shared/templates/organisms/layout.templ` (package `organisms`); adjust internal references and imports per `specs/009-shared-templ-components/contracts/shared-templ-components.md`
- [X] T033 [US3] Move `ErrorPage` and `ErrorPageData` from `internal/shared/templates/error.templ` into `internal/shared/templates/organisms/error.templ` (package `organisms`)
- [X] T034 [US3] Run `make templ` from repository root to generate `internal/shared/templates/organisms/*_templ.go`, then run `go test ./internal/shared/templates/organisms/...` and confirm the new organisms tests pass
- [X] T035 [P] [US3] Update imports in `internal/features/auth/templates/{login,register,forgot_password,reset_password}.templ` from `github.com/leoarkiteto/zelo/internal/shared/templates` to `github.com/leoarkiteto/zelo/internal/shared/templates/organisms`
- [X] T036 [P] [US3] Update imports in `internal/features/directory/templates/directory.templ` to `.../internal/shared/templates/organisms`
- [X] T037 [P] [US3] Update imports in `internal/features/finance/templates/finance.templ` to `.../internal/shared/templates/organisms`
- [X] T038 [P] [US3] Update imports in `internal/features/home/templates/dashboard.templ` and `internal/features/home/templates/area.templ` to `.../internal/shared/templates/organisms`
- [X] T039 [P] [US3] Update imports in `internal/features/management/templates/invitations.templ` and `internal/features/management/templates/roles.templ` to `.../internal/shared/templates/organisms`
- [X] T040 [P] [US3] Update imports in `internal/features/profile/templates/profile.templ` and `internal/features/tickets/templates/tickets.templ` to `.../internal/shared/templates/organisms`
- [X] T041 [US3] Remove the now-unused flat files `internal/shared/templates/layout.templ`, `error.templ`, `badges.go`, `layout_templ.go`, `error_templ.go`, and `smoke_test.go` after confirming no remaining references to the old package
- [X] T042 [US3] Run `go test ./...` from repository root; confirm all tests pass and the rendered pages match the pre-migration baseline from `specs/009-shared-templ-components/quickstart.md`

**Checkpoint**: Existing shared components now live in organisms/atoms and every page renders identically

---

## Phase 6: User Story 4 - New shared components are added in the right place (Priority: P4)

**Goal**: Enforce the placement rules so future components land in the correct layer automatically.

**Independent Test**: Run `scripts/check-feature-boundaries.sh` and `make check`; deliberately moving an atom import upward should fail, and adding a feature-only component should stay out of `internal/shared/templates`.

### Tests for User Story 4

No new render tests needed; this story is about enforcement tooling.

### Implementation for User Story 4

- [X] T043 [US4] Extend `scripts/check-feature-boundaries.sh` to also invoke `scripts/check-template-atomic-boundaries.sh` so a single command enforces both feature and atomic layer boundaries
- [X] T044 [US4] Add a `check` target to `Makefile` that runs `go test ./...`, `scripts/check-feature-boundaries.sh`, and `scripts/check-template-atomic-boundaries.sh`
- [X] T045 [US4] Run the full validation flow in `specs/009-shared-templ-components/quickstart.md` end-to-end (`make templ`, `go test ./...`, `scripts/check-feature-boundaries.sh`, manual page smoke) and confirm every expected outcome
- [X] T046 [US4] Verify no duplicated shared markup remains by running the grep checks listed in `specs/009-shared-templ-components/quickstart.md`

**Checkpoint**: Placement rules are automated; new shared components have exactly one correct home

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final consistency, documentation sync, and release readiness

- [X] T047 [P] Update `specs/009-shared-templ-components/data-model.md` and `specs/009-shared-templ-components/contracts/shared-templ-components.md` if implementation revealed any drift from the designed props/signatures
- [X] T048 [P] Update `internal/shared/templates/README.md` with the final component inventory and any lessons learned during migration
- [X] T049 Run final `make templ && go test ./... && scripts/check-feature-boundaries.sh` from repository root; review `git diff` for only intended template changes plus committed generated `*_templ.go` files
- [X] T050 Commit the generated `*_templ.go` files together with their source `.templ` changes, following the project's generated-code convention in `scripts/`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 first (structure/documentation), then US2 and US3 (US2 and US3 both depend on the US1 layer skeleton; US3's import updates are cleaner after US2's feature-template edits)
  - US4 depends on US2 and US3 (it validates and enforces the finished structure)
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) — no dependencies on other stories
- **User Story 2 (P2)**: Can start after US1 — extracts atoms/molecules and updates feature markup
- **User Story 3 (P3)**: Can start after US1 — migrates existing shared components; run after US2's feature edits to avoid touching the same files twice
- **User Story 4 (P4)**: Can start after US2 and US3 — wires up enforcement and final validation

### Within Each User Story

- Tests MUST be written and FAIL before implementation (constitution TDD)
- Atoms before molecules (molecules compose atoms)
- Components before feature-template updates
- Story complete before moving to the next priority

### Parallel Opportunities

- Phase 1: T002, T003, T004 are parallel (different directories)
- Phase 2: T005 and T006 are parallel (different files); T007 runs after them
- US1: T009, T010, T011 are parallel (different package docs)
- US2: T012/T013 (tests) parallel; T014–T017 (atoms) parallel; T020/T021 (molecules) parallel after atoms; T023–T029 (feature templates) parallel after components exist
- US3: T035–T040 (per-feature import updates) parallel; T032/T033 sequential before generation
- Polish: T047/T048 parallel

---

## Parallel Example: User Story 2

```bash
# Launch all atom tests together (must fail first):
Task: "Write failing render smoke tests for atoms in internal/shared/templates/atoms/atoms_test.go"
Task: "Write failing render smoke tests for molecules in internal/shared/templates/molecules/molecules_test.go"

# Launch all atom implementations together:
Task: "Implement Button in internal/shared/templates/atoms/button.templ"
Task: "Implement Badge in internal/shared/templates/atoms/badge.templ"
Task: "Implement Icon in internal/shared/templates/atoms/icon.templ"
Task: "Implement TextInput/TextArea/Select in internal/shared/templates/atoms/input.templ"

# After atoms pass, launch feature template updates together:
Task: "Update internal/features/home/templates/dashboard.templ ..."
Task: "Update internal/features/auth/templates/... ..."
Task: "Update internal/features/directory/templates/directory.templ ..."
Task: "Update internal/features/finance/templates/finance.templ ..."
Task: "Update internal/features/management/templates/... ..."
Task: "Update internal/features/profile/templates/profile.templ ..."
Task: "Update internal/features/tickets/templates/tickets.templ ..."
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1 (atomic layer skeleton + docs)
4. **STOP and VALIDATE**: open `internal/shared/templates/` and confirm the three layers are documented and discoverable
5. Deploy/demo the structural change if desired (no behavior change yet)

### Incremental Delivery

1. Complete Setup + Foundational → foundation ready
2. Add US1 → structure discoverable → validate
3. Add US2 → repetition removed from features → run `go test ./...` and visual smoke
4. Add US3 → existing shared components migrated → run `go test ./...` and visual smoke
5. Add US4 → automated enforcement + full quickstart validation
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. After Foundational:
   - Developer A: US1 docs, then atoms (US2)
   - Developer B: molecules (US2) after atoms land
   - Developer C: prepare US3 organisms move (only after A/B finish feature edits to avoid conflicts)
3. US4 runs after all component work lands

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify render smoke tests fail before implementing their components (constitution TDD)
- Commit after each task or logical group; generated `*_templ.go` is committed with its source `.templ`
- Stop at any checkpoint to validate the story independently
- Avoid: vague tasks, same-file conflicts, cross-story dependencies that break independence
