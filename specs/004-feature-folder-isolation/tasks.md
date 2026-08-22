---
description: "Task list for Feature Folder Isolation"
---

# Tasks: Feature Folder Isolation

**Input**: Design documents from `/specs/004-feature-folder-isolation/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: No new test tasks were requested in the specification. The existing `go test ./...` suite is the regression gate for this behavior-preserving refactor; test files move with the code they cover and must stay green after every task.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare the target directory skeleton and baseline before any code moves

- [x] T001 [P] Create the target skeleton per `specs/004-feature-folder-isolation/plan.md`: add `doc.go` placeholders under `internal/features/{home,directory,management,auth}/core/{domain,ports,services}`, `internal/features/{home,directory,management,auth}/handlers`, `internal/features/directory/repositories`, `internal/features/{home,directory,management,auth}/templates`, and `internal/shared/{model,config,security,middleware,store,templates,testutil,httpx}`
- [x] T002 Run `go test ./...` and `go build ./...` from repo root; confirm the baseline is green before any file moves (record the output for the PR)
- [x] T003 [P] Add `scripts/check-feature-boundaries.sh` that exits non-zero if any file under `internal/features/<feature>/` imports another `internal/features/<feature>/` path (imports from `cmd/web` and `internal/shared/` are allowed)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Relocate shared modules to `internal/shared/` so every feature can depend on them from their new homes

**⚠️ CRITICAL**: No feature migration (Phase 3+) can begin until this phase is complete

- [x] T00404Move `internal/model/` → `internal/shared/model/` (package name stays `model`); update every import of `github.com/leoarkiteto/zelo/internal/model` to `github.com/leoarkiteto/zelo/internal/shared/model`; run `go test ./...` from repo root and confirm green
- [x] T00505[P] Move `internal/config/config.go` → `internal/shared/config/config.go`; update the import in `cmd/web/main.go`; run `go build ./...` and confirm green
- [x] T00606Move `internal/auth/{csrf,password,session}.go` and their tests → `internal/shared/security/`; rename the package declaration to `security` and update all references in `internal/middleware/`, `internal/handler/`, `internal/service/`, and `cmd/web/` from `auth.` to `security.`; run `go test ./...` and confirm green
- [x] T00707Move `internal/middleware/` → `internal/shared/middleware/`; update imports in `internal/handler/` and `cmd/web/main.go`; run `go test ./...` and confirm green
- [x] T00808Move `internal/testutil/testdb.go` → `internal/shared/testutil/testdb.go`; update test imports in `internal/store/` and `internal/handler/`; run `go test ./...` and confirm green
- [x] T00909Move the whole `internal/store/` package → `internal/shared/store/` (package name stays `store`); update all imports in `internal/handler/`, `internal/middleware/`, and `cmd/web/`; run `go test ./...` and confirm green
- [x] T01010Move the whole `web/templates/` package → `internal/shared/templates/`; update imports in `internal/handler/` from `github.com/leoarkiteto/zelo/web/templates` to `github.com/leoarkiteto/zelo/internal/shared/templates`; run `templ generate` then `go test ./...` and confirm green
- [x] T01111Move `internal/handler/helpers.go` → `internal/shared/httpx/helpers.go` (package `httpx`); export the helpers (`Render`, `RenderError`, `SetAnonymousCSRF`, `CurrentUser`, `CurrentSession`, `AnonymousCSRFValue`); update all `internal/handler/` call sites to use `httpx.*`; run `go build ./...` and confirm green
- [x] T01212Move the `TokenHasher` adapter from `internal/handler/deps.go` → `internal/shared/security/token_hasher.go` (package `security`); update `cmd/web/main.go` to use `security.TokenHasher{}`; run `go build ./...` and confirm green

**Checkpoint**: Shared modules live under `internal/shared/` and the suite is green — feature migration can now begin

---

## Phase 3: User Story 1 - Developer finds a whole feature in one folder (Priority: P1) 🎯 MVP

**Goal**: Migrate the smallest feature (`home`) into `internal/features/home/` as the pilot vertical slice, proving the hexagonal folder pattern end-to-end.

**Independent Test**: Open `internal/features/home/` and verify its handlers and templates are the only place home code lives; run `go test ./...` and confirm `/`, `/condominium`, `/unit`, and `/tenancy` still behave exactly as before.

### Tests for User Story 1

No new tests requested. Existing home-related scenarios move with the code (T017) and must pass unchanged.

### Implementation for User Story 1

- [x] T013 [US1] Create `internal/features/home/handlers/deps.go` defining home `Deps` with the collaborators `home.go`, `areas.go`, and `shell.go` need (logger, roles reader, sessions, users, audit), using narrow interfaces compatible with `internal/shared/` implementations
- [x] T014 [US1] Move `internal/handler/{home.go,areas.go,shell.go}` → `internal/features/home/handlers/`; change the package to `handlers`; adapt receivers to home `Deps`; update imports to `internal/shared/httpx`, `internal/shared/model`, and `internal/shared/middleware`
- [x] T015 [US1] Add `internal/features/home/handlers/routes.go` with `RegisterRoutes(mux *http.ServeMux, deps Deps)` registering `GET /`, `GET /condominium`, `GET /unit`, and `GET /tenancy` with the same shared-middleware role guards; update `internal/handler/router.go` to delegate these routes to it and remove the local registrations
- [x] T016 [US1] Move `internal/shared/templates/{dashboard,area}.templ` and their generated `*_templ.go` → `internal/features/home/templates/`; update package declarations, imports, and component references in the moved files and home handlers; run `templ generate` then `go build ./...`
- [x] T017 [US1] Move the home-related scenarios from `internal/handler/{public_test.go,integration_test.go}` into `internal/features/home/handlers/home_test.go` with a home `Deps` fixture; keep the same assertions; run `go test ./...` and confirm green
- [x] T018 [US1] Run `go test ./...` and `go build ./...` from repo root; confirm all home code lives under `internal/features/home/` and no `internal/handler/home.go`, `internal/handler/areas.go`, or `internal/handler/shell.go` remain

**Checkpoint**: User Story 1 is functional and independently testable — the pilot feature is fully isolated

---

## Phase 4: User Story 2 - Existing features are migrated without changing behavior (Priority: P2)

**Goal**: Migrate the remaining features (`directory`, `management`, `auth`) into their own folders and rewire the composition root, preserving all routes and behavior.

**Independent Test**: Run `go test ./...` and `go build ./...`; walk the route inventory in `contracts/http-routes.md` and confirm every route behaves identically.

### Tests for User Story 2

No new tests requested. Existing tests move with their code and must pass unchanged at every step.

### Implementation for User Story 2 — directory

- [x] T01919[US2] Create `internal/features/directory/core/ports/{repositories.go,services.go}` with `ListingStore`, `CategoryStore`, `UnitResolver`, and `AuditRecorder` interfaces copied from `internal/service/ports.go` (identical method sets)
- [x] T02020[US2] Move `internal/service/directory.go` → `internal/features/directory/core/services/directory.go` (package `services`); update imports to `internal/shared/model` and the new ports package; keep `DirectoryService` behavior identical
- [x] T02121[US2] Move `internal/service/directory_test.go` → `internal/features/directory/core/services/directory_test.go`; update package and imports; run `go test ./...` and confirm green
- [x] T02222[US2] Split `internal/shared/store/listings.go`: create `internal/features/directory/repositories/listings.go` (`ListingStore`) and `internal/features/directory/repositories/categories.go` (`CategoryStore`) with the existing SQL; delete those types from `internal/shared/store/listings.go`; update all imports
- [x] T02323[US2] Move `internal/shared/store/listings_test.go` → `internal/features/directory/repositories/listings_test.go`; update package and imports; run `go test ./...` and confirm green
- [x] T02424[US2] Create `internal/features/directory/handlers/deps.go` and move `internal/handler/directory.go` → `internal/features/directory/handlers/directory.go` (package `handlers`); adapt to directory `Deps`
- [x] T02525[US2] Add `internal/features/directory/handlers/routes.go` with `RegisterRoutes` for all `/directory` and `/directory/categories` routes with the same role guards; update `internal/handler/router.go` to delegate
- [x] T02626[US2] Move `internal/shared/templates/directory.templ` and its generated `*_templ.go` → `internal/features/directory/templates/`; update component references; run `templ generate` then `go build ./...`
- [x] T02727[US2] Move directory-related handler tests into `internal/features/directory/handlers/directory_test.go` with a directory `Deps` fixture; run `go test ./...` and confirm green

### Implementation for User Story 2 — management

- [x] T02828[US2] Create `internal/features/management/core/ports/{repositories.go,services.go}` with `RoleStore`, `InvitationStore`, `UnitReader`, `AuditRecorder`, and `TokenHasher` interfaces copied from `internal/service/ports.go`
- [x] T02929[US2] Move `internal/service/roles.go` → `internal/features/management/core/services/roles.go` (package `services`); update imports to `internal/shared/model` and the new ports package; keep behavior identical
- [x] T03030[US2] Move `internal/service/roles_test.go` → `internal/features/management/core/services/roles_test.go`; update package and imports; run `go test ./...` and confirm green
- [x] T03131[US2] Create `internal/features/management/handlers/deps.go`; move `internal/handler/{roles.go,invitations.go}` → `internal/features/management/handlers/` (package `handlers`); adapt to management `Deps`
- [x] T03232[US2] Add `internal/features/management/handlers/routes.go` with `RegisterRoutes` for `/roles*` and `/invitations*` with the same syndic-only guards; update `internal/handler/router.go` to delegate
- [x] T03333[US2] Move `internal/shared/templates/{roles,invitations}.templ` and their generated `*_templ.go` → `internal/features/management/templates/`; update component references; run `templ generate` then `go build ./...`
- [x] T03434[US2] Move management-related handler tests into `internal/features/management/handlers/management_test.go`; run `go test ./...` and confirm green

### Implementation for User Story 2 — auth

- [x] T035[US2] Create `internal/features/auth/core/ports/{repositories.go,services.go}` with `UserStore`, `InvitationStore`, `RoleReader`, `AuditRecorder`, `PasswordHasher`, and `TokenHasher` interfaces copied from `internal/service/ports.go`
- [x] T036[US2] Move `internal/service/{registration.go,auth.go,password_reset.go}` → `internal/features/auth/core/services/` (package `services`); update imports to `internal/shared/model` and the new ports package; keep behavior identical
- [x] T037[US2] Move `internal/service/{registration_test.go,auth_test.go}` → `internal/features/auth/core/services/`; update package and imports; run `go test ./...` and confirm green
- [x] T038[US2] Create `internal/features/auth/handlers/deps.go`; move `internal/handler/{login.go,register.go,logout.go,password.go}` → `internal/features/auth/handlers/` (package `handlers`); adapt to auth `Deps`
- [x] T039[US2] Add `internal/features/auth/handlers/routes.go` with `RegisterRoutes` for `/register`, `/login`, `/password/*`, and `POST /logout` with the same guards; update `internal/handler/router.go` to delegate
- [x] T040[US2] Move `internal/shared/templates/{auth,login,register,forgot_password,reset_password}.templ` and their generated `*_templ.go` → `internal/features/auth/templates/`; update component references; run `templ generate` then `go build ./...`
- [x] T041[US2] Move auth-related handler tests into `internal/features/auth/handlers/auth_test.go`; run `go test ./...` and confirm green

### Implementation for User Story 2 — composition root

- [x] T042[US2] Rewrite `cmd/web/main.go` to construct `internal/shared/{config,security,middleware,store}` adapters and each feature's `Deps`, then register `auth`, `home`, `directory`, and `management` routes on one `http.ServeMux` and apply the shared middleware stack in the order documented in `contracts/http-routes.md`
- [x] T043[US2] Delete the old `internal/handler/` and `internal/service/` packages once `cmd/web/main.go` no longer imports them; run `go test ./...` and confirm green
- [x] T044[US2] Move the full-app integration tests from the deleted handler package into `tests/integration/` exercising the composed router from `cmd/web`; keep coverage of the route inventory in `contracts/http-routes.md`
- [x] T045[US2] Run `go test ./...` and `go build ./...` from repo root; confirm all four features live under `internal/features/` and every test passes

**Checkpoint**: All existing features are isolated; the public route contract is unchanged

---

## Phase 5: User Story 3 - New features start from a standard folder template (Priority: P3)

**Goal**: Formalize the feature folder template and a scaffold script so future features start isolated by default.

**Independent Test**: Run `scripts/new-feature.sh smoke`, confirm the generated structure matches `contracts/feature-folder-layout.md`, then build and remove the throwaway feature.

### Tests for User Story 3

No new tests requested. The scaffold validation (T048) is the acceptance check.

### Implementation for User Story 3

- [x] T04646[US3] Create `scripts/feature-template/` with the canonical skeleton from `contracts/feature-folder-layout.md`: `core/{domain,ports,services}`, `handlers/`, `repositories/`, `templates/`, each with `doc.go` placeholders and a short `README.md`
- [x] T04747[US3] Add `scripts/new-feature.sh <feature-name>` that copies `scripts/feature-template/` to `internal/features/<feature-name>/`, replaces the placeholder package names, and prints the next steps
- [x] T04848[US3] Validate the scaffold: run `scripts/new-feature.sh smoke`, then `go build ./...` and `go test ./...` from repo root; remove `internal/features/smoke/` afterward
- [x] T04949[US3] Document the new-feature template in `contracts/feature-folder-layout.md` and link it from `README.md`

**Checkpoint**: New features can be scaffolded in under 15 minutes with a consistent structure

---

## Phase 6: User Story 4 - Shared capabilities stay shared (Priority: P4)

**Goal**: Enforce the shared/feature boundary, remove leftovers, and make the new layout the documented standard.

**Independent Test**: Run `scripts/check-feature-boundaries.sh` (clean), audit `internal/shared/` for feature-exclusive code, and confirm `README.md` + the constitution describe the new layout.

### Tests for User Story 4

No new tests requested. The boundary script (T050) is the acceptance check.

### Implementation for User Story 4

- [x] T05050[US4] Run `scripts/check-feature-boundaries.sh`; fix any cross-feature import violations it reports
- [x] T05151[US4] Audit `internal/shared/` for feature-exclusive code; move anything used by a single feature into that feature's folder (e.g., any directory-only types into `internal/features/directory/`)
- [x] T05252[US4] Verify `internal/shared/templates/` contains only shared layout/error components and every feature-specific template lives under `internal/features/<feature>/templates/`
- [x] T05353[US4] Update `README.md` "Directory layout" section to the new `internal/features/` + `internal/shared/` structure
- [x] T05454[US4] Amend `.specify/memory/constitution.md` "Repository Layout & Conventions" (and Principle III wording if needed) to the new layout; bump the version to 1.2.0 and add a Sync Impact Report entry

**Checkpoint**: The shared/feature boundary is enforced and documented

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and review before merge

- [x] T055 Run the full `specs/004-feature-folder-isolation/quickstart.md` validation: `make test`, `make build`, `make templ`, `git diff --exit-code -- '*_templ.go'`, and the route smoke test
- [x] T056 Final review: run `go vet ./...` and `git diff --stat` from repo root; confirm `git status` shows only intended moves and no behavior changes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup — BLOCKS all user stories (features must import shared modules from their new paths)
- **User Stories (Phase 3+)**: All depend on Foundational completion
  - US1 (pilot) must finish before US2 so the pattern is proven
  - US3 can proceed after US1/US2 (uses the real features as examples)
  - US4 must finish after US2 (audits the fully migrated tree)
- **Polish (Phase 7)**: Depends on all user stories

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational — no dependencies on other stories
- **User Story 2 (P2)**: Can start after US1 (mirrors the proven pilot pattern) — the three features (`directory`, `management`, `auth`) are independent of each other
- **User Story 3 (P3)**: Can start after Foundational in parallel with US1/US2; T049 (documentation link) should land after US2
- **User Story 4 (P4)**: Can start only after US2 — it audits the final tree

### Within Each User Story

- Ports before services (services import the feature's own ports)
- Services before handlers (handlers depend on services)
- Handlers before templates move (handlers render the feature's templates)
- Tests move last within each feature so the new package compiles first
- `go test ./...` must be green before moving to the next feature

### Parallel Opportunities

- T001 and T003 in Setup can run in parallel
- T005 can run in parallel with the Foundational chain (disjoint files)
- Within US2, the three features can be migrated in parallel by different developers once US1 established the pattern
- Within each feature, ports and templates moves touch disjoint files and can be prepared in parallel

---

## Parallel Example: User Story 2 (directory)

```bash
# After ports exist, these can be prepared in parallel:
Task: "Move internal/service/directory.go → internal/features/directory/core/services/directory.go"
Task: "Create internal/features/directory/repositories/{listings.go,categories.go} from internal/shared/store/listings.go"
Task: "Move internal/shared/templates/directory.templ → internal/features/directory/templates/"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (pilot `home` feature)
4. **STOP and VALIDATE**: open `internal/features/home/`, run `go test ./...`, smoke-test `/`
5. Review the pilot pattern before migrating the remaining features

### Incremental Delivery

1. Setup + Foundational → shared modules in place
2. Add US1 (`home` isolated) → validate → continue
3. Add US2 (`directory`, `management`, `auth` isolated) → validate each feature → validate routes
4. Add US3 (scaffold script) → validate with a throwaway feature
5. Add US4 (boundary enforcement + docs) → validate with boundary script
6. Each story adds isolation without changing behavior

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (pilot `home`)
3. Once the pilot pattern is approved:
   - Developer A: `directory`
   - Developer B: `management`
   - Developer C: `auth`
4. Rejoin for composition root (T042–T045), then US3/US4 together

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps a task to its user story for traceability
- `go test ./...` must stay green after every task — this is a behavior-preserving refactor
- Run `templ generate` after every `.templ` move and commit the regenerated `*_templ.go`
- Commit after each task or logical group so each move is reviewable in isolation
- Stop at any checkpoint to validate a story independently
- Avoid: moving a file without updating its imports, deleting old packages before the composition root is rewired, and any change that alters route behavior
