---
description: "Task list for Replace Inline SVG Icons"
---

# Tasks: Replace Inline SVG Icons

**Input**: Design documents from `/specs/010-replace-inline-svg-icons/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Included — the project constitution mandates test-first development for all feature work.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- GOTTH modular monolith: shared UI in `internal/shared/templates/`, feature UI in `internal/features/<feature>/templates/`
- Static assets: source in `assets/`, served from `web/static/`
- Generated artifacts: `templ generate` → `*_templ.go`, `make tailwind` → `web/static/css/output.css`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Vendor the Google Material Symbols font and add the CSS foundation every icon depends on.

- [X] T001 [P] Download the Material Symbols Outlined variable font `woff2` from `https://fonts.gstatic.com/s/materialsymbolsoutlined/v368/kJEhBvYX7BgnkSrUwT8OhrdQw4oELdPIeeII9v6oFsLjBuVY.woff2` into `web/static/fonts/material-symbols-outlined.woff2` (create the `web/static/fonts/` directory), verify the file starts with the wOFF2 magic bytes `wOF2`, and add `web/static/fonts/README.md` noting the source and Apache-2.0 license.
- [X] T002 [P] Add the Material Symbols CSS to `assets/css/input.css`: an `@font-face` for `Material Symbols Outlined` with `src: url('/static/fonts/material-symbols-outlined.woff2') format('woff2')`, `font-weight: 100 700`, and `font-display: swap`; a `.material-symbols-outlined` base class (24px font-size, `line-height: 1`, `-webkit-font-feature-settings: 'liga'`, `-webkit-font-smoothing: antialiased`); and size rules so `.material-symbols-outlined.h-3\.5`, `.h-4`, `.h-5`, and `.h-6` set `font-size: 0.875rem`, `1rem`, `1.25rem`, and `1.5rem` respectively.
- [X] T003 Run `make tailwind` and verify `web/static/css/output.css` now contains `@font-face`, `.material-symbols-outlined`, and the `/static/fonts/material-symbols-outlined.woff2` URL.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Upgrade the shared `Icon` component to render Material Symbols. This MUST be complete before any user story template migration.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T004 Update `internal/shared/templates/atoms/atoms_test.go` first (TDD): change `TestIconRendersKnownKind` to expect `<span class="material-symbols-outlined h-5 w-5" aria-hidden="true">info</span>` instead of `<svg`, and add `TestIconRendersDataIcon` (expects `data-icon="eye"` when `DataIcon` is set) and `TestIconFallsBackForUnknownKind` (expects ligature `info`). These tests must fail until T006 lands.
- [X] T005 Update `internal/shared/templates/atoms/types.go`: add `IconKindEye`, `IconKindEyeOff`, and `IconKindMenu` constants; extend `IconProps` with `DataIcon string` and `AriaHidden *bool` (nil means hidden); add a `materialName(kind IconKind) string` mapping per `specs/010-replace-inline-svg-icons/data-model.md` (building→`apartment`, mail→`mail`, users→`group`, home→`home`, key→`key`, info→`info`, chevron-left→`chevron_left`, arrow-right→`arrow_forward`, clipboard→`content_paste`, tag→`label`, eye→`visibility`, eye-off→`visibility_off`, menu→`menu`, unknown→`info`).
- [X] T006 Replace the switch of SVG cases in `internal/shared/templates/atoms/icon.templ` with a single `<span class={ iconClass(p) } data-icon?={ p.DataIcon } aria-hidden={ iconAriaHidden(p) }>{ materialName(p.Kind) }</span>`; remove all hand-authored SVG path data; run `templ generate` to regenerate `internal/shared/templates/atoms/icon_templ.go`.
- [X] T007 Run `go test ./internal/shared/templates/atoms` and confirm the T004 tests are green.

**Checkpoint**: Shared icon foundation ready — all stories can now migrate their templates to `atoms.Icon`.

---

## Phase 3: User Story 1 - Existing icons render through the icon library with no visible change (Priority: P1) 🎯 MVP

**Goal**: Migrate the mobile menu icon and the home area breadcrumb/link icons from inline SVG to the shared Material Symbols component.

**Independent Test**: Render the app shell and `AreaPage`; every migrated icon appears in the same position and size, and neither template contains inline `<svg>`.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T008 [P] [US1] Update `internal/shared/templates/organisms/organisms_test.go` first (TDD): extend `TestShellRendersAppChrome` to assert the output contains `material-symbols-outlined` and `>menu<`, and add a check that the shell output contains no `<svg`.
- [X] T009 [P] [US1] Update `internal/features/home/templates/smoke_test.go` first (TDD): extend the `area` case in `TestPagesRender` to assert the output contains `material-symbols-outlined`, `chevron_left`, and `arrow_forward`, and add a check that the area page output contains no `<svg`.

### Implementation for User Story 1

- [X] T010 [P] [US1] Update `internal/shared/templates/organisms/layout.templ`: import `atoms "github.com/leoarkiteto/zelo/internal/shared/templates/atoms"` and replace the inline hamburger `<svg>` in the mobile menu `label` with `@atoms.Icon(atoms.IconProps{Kind: atoms.IconKindMenu, Class: "h-6 w-6"})`.
- [X] T011 [P] [US1] Update `internal/features/home/templates/area.templ`: import the shared atoms package, replace the breadcrumb chevron `<svg>` with `@atoms.Icon(atoms.IconProps{Kind: atoms.IconKindChevronLeft, Class: "h-3.5 w-3.5"})`, and replace the dashboard link arrow `<svg>` with `@atoms.Icon(atoms.IconProps{Kind: atoms.IconKindArrowRight, Class: "h-4 w-4 text-steel-500"})`.
- [X] T012 [US1] Run `templ generate` then `go test ./internal/shared/templates/organisms ./internal/features/home/templates` and confirm the T008/T009 tests are green.

**Checkpoint**: User Story 1 should be fully functional and testable independently.

---

## Phase 4: User Story 2 - The shared icon component becomes the single way to use icons (Priority: P2)

**Goal**: Prove the icon catalog is complete and enforce that no template can reintroduce hand-authored inline SVG icons.

**Independent Test**: Run the catalog completeness test, read the design-system documentation, and run `make check` — the new no-inline-SVG script fails if any `.templ` file contains `<svg`.

### Tests for User Story 2 ⚠️

- [X] T013 [P] [US2] Add `TestIconCatalogIsComplete` to `internal/shared/templates/atoms/atoms_test.go`: iterate every `IconKind` constant, assert `materialName` returns its documented Material Symbols ligature, and assert `Icon` renders a span containing that ligature.

### Implementation for User Story 2

- [X] T014 [P] [US2] Add an "Icons" section to `docs/design-system.md` documenting the shared `Icon` component: Material Symbols source, the `IconKind` catalog table, usage example `@atoms.Icon(atoms.IconProps{Kind: atoms.IconKindMail})`, size classes, `DataIcon` hook, and the accessibility rule that decorative icons stay hidden while interactive callers put the accessible name on the enclosing button/link.
- [X] T015 [P] [US2] Create `scripts/check-no-inline-svg.sh` that fails with a non-zero exit if `rg '<svg' internal --glob '*.templ'` finds any match, and wire it into `make check` in `Makefile` by adding `scripts/check-no-inline-svg.sh` after the existing boundary checks.
- [X] T016 [US2] Run `make check` and confirm the new script passes together with the catalog completeness test.

**Checkpoint**: User Stories 1 AND 2 should both work independently; the single-entry-point rule is enforced by CI.

---

## Phase 5: User Story 3 - Password visibility toggle keeps its behavior and accessibility (Priority: P3)

**Goal**: Migrate the login password visibility eye/eye-off icons to the shared component while preserving the toggle script and accessible labels.

**Independent Test**: Render `LoginPage` and confirm both library icons are present with their `data-icon` hooks and no inline `<svg>`; then exercise the login page toggle manually.

### Tests for User Story 3 ⚠️

- [X] T017 [P] [US3] Update `internal/features/auth/templates/smoke_test.go` first (TDD): assert the login page output contains `material-symbols-outlined`, `data-icon="eye"`, `data-icon="eye-off"`, `visibility`, and `visibility_off`, and contains no `<svg`.

### Implementation for User Story 3

- [X] T018 [US3] Update `internal/features/auth/templates/login.templ`: replace the two inline `<svg>` elements inside the `[data-password-toggle]` button with `@atoms.Icon(atoms.IconProps{Kind: atoms.IconKindEye, Class: "h-5 w-5", DataIcon: "eye"})` and `@atoms.Icon(atoms.IconProps{Kind: atoms.IconKindEyeOff, Class: "h-5 w-5", DataIcon: "eye-off"})`; keep the `data-password-toggle` script and the button's `aria-label`/`aria-pressed` attributes unchanged.
- [X] T019 [US3] Run `templ generate` then `go test ./internal/features/auth/templates/...` and confirm the T017 test is green.

**Checkpoint**: All user stories should now be independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and artifact hygiene across all stories.

- [X] T020 [P] Run `rg '<svg' internal --glob '*.templ'` and confirm there are no remaining inline SVG icons in production templates.
- [X] T021 [P] Run `make templ` and `make tailwind`, then confirm `git status` shows the regenerated `*_templ.go` files and `web/static/css/output.css` (plus the new font) are ready to commit together.
- [X] T022 Run the full quickstart validation from `specs/010-replace-inline-svg-icons/quickstart.md`: `make check` green, password toggle verified on `/login`, and (with Docker services up) `TEST_DATABASE_URL=... TEST_REDIS_URL=... go test ./tests/...` green.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Enforces and documents the component built in Phase 2
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Migrates the remaining login template independently

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Shared component implementation before template migrations
- Template migration before the phase's verification command
- Story complete before moving to next priority

### Parallel Opportunities

- Setup: T001 and T002 can run in parallel (different files)
- US1: T008 and T009 (tests) can run in parallel; T010 and T011 (template edits) can run in parallel
- US2: T013, T014, and T015 can run in parallel (different files)
- Polish: T020 and T021 can run in parallel
- Once Foundational completes, US1, US2, and US3 can proceed in parallel by different developers

---

## Parallel Example: User Story 1

```bash
# Launch both failing tests together:
Task: "Update internal/shared/templates/organisms/organisms_test.go ..."
Task: "Update internal/features/home/templates/smoke_test.go ..."

# After they fail, edit the two templates together:
Task: "Update internal/shared/templates/organisms/layout.templ ..."
Task: "Update internal/features/home/templates/area.templ ..."
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → shared icon component ready
2. Add User Story 1 → menu and area icons migrated → Test independently (MVP!)
3. Add User Story 2 → catalog enforced and documented → Test independently
4. Add User Story 3 → login toggle migrated → Test independently
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (layout + area templates)
   - Developer B: User Story 2 (catalog test + docs + no-inline-SVG script)
   - Developer C: User Story 3 (login password toggle)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
