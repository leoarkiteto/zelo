---

description: "Task list for feature implementation"
---

# Tasks: Service Provider Directory

**Input**: Design documents from `/specs/003-service-directory/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: INCLUDED — the zelo constitution mandates TDD for all feature work (red-green-refactor). Every story phase starts with failing tests.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Go module root: `github.com/leoarkiteto/zelo`; app code under `internal/`, entrypoint `cmd/web/`
- Go tests live next to the code they test (`*_test.go`), matching the existing repo
- Templates: `web/templates/*.templ` (+ generated `*_templ.go` via `scripts/templ-generate.sh`)
- Migrations: `migrations/NNNN_description.sql` (forward-only, applied by `make migrate`)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Database schema and shared domain types for the directory slice

- [X] T001 Create migration `migrations/0007_create_service_directory.sql` with `service_categories` (partial unique index on active name) and `service_provider_listings` (phone/phone_digits/unit snapshot columns, FKs, search indexes) per `specs/003-service-directory/data-model.md`
- [X] T002 Create `internal/model/listing.go` with `ServiceCategory` and `ServiceProviderListing` structs per `specs/003-service-directory/data-model.md`
- [X] T003 Extend `internal/service/ports.go` with `CategoryStore` and `ListingStore` interfaces per `specs/003-service-directory/data-model.md` (depends on T002)
- [X] T004 [P] Extend `internal/model/audit.go` with directory audit event types: `listing_created`, `listing_edited`, `listing_deleted`, `category_created`, `category_renamed`, `category_deactivated`

**Checkpoint**: `make migrate` applies `0007` cleanly and `go build ./...` compiles.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Persistence adapters that every user story depends on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T005 Write failing integration tests for category + listing persistence (CRUD, search, duplicate lookup, unit lookup) in `internal/store/listings_test.go`
- [X] T006 Implement `internal/store/listings.go` with `CategoryStore` and `ListingStore` database/sql adapters (plain SQL; include `GetActiveUnitForUser` and `FindDuplicatePhone`) until T005 passes
- [X] T007 Run `go test ./internal/store/...` and confirm T005 passes against the dockerized PostgreSQL

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Resident registers a recommended professional (Priority: P1) 🎯 MVP

**Goal**: A resident submits name, category, phone, notes; the system records their unit automatically and shows the listing in the directory.

**Independent Test**: Sign in as a resident, open `GET /directory/new`, submit a valid listing, and see it appear on `GET /directory` with the resident's unit shown.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T008 [P] [US1] Write failing service test for `CreateListing` (validation, automatic unit capture, duplicate warning flag) in `internal/service/directory_test.go`
- [X] T009 [P] [US1] Write failing handler test for `GET /directory/new` (200 for members, 403 for non-members) and `POST /directory` (valid save → 303; missing/invalid fields → 400 re-render) in `internal/handler/directory_test.go`

### Implementation for User Story 1

- [X] T010 [US1] Implement `internal/service/directory.go` with `DirectoryService.CreateListing` (validate per `contracts/http-routes.md`, resolve active unit via store, normalize `phone_digits`, duplicate check) until T008 passes
- [X] T011 [US1] Extend `internal/handler/deps.go` and `cmd/web/main.go` to construct and inject `ListingStore`, `CategoryStore`, and `DirectoryService`
- [X] T012 [US1] Implement `internal/handler/directory.go` with `directoryNewGET` and `directoryCreatePOST` handlers (member access, CSRF-protected POST, re-render with validation/duplicate warnings) and register `GET /directory`, `GET /directory/new`, `POST /directory` in `internal/handler/router.go`
- [X] T013 [US1] Create `web/templates/directory.templ` with a directory list component and a listing submission form (name, category select, phone, notes, CSRF); run `scripts/templ-generate.sh` and commit generated `directory_templ.go`
- [X] T014 [US1] Run `go test ./...`; confirm T008 and T009 pass

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - Resident searches and filters the directory (Priority: P2)

**Goal**: Residents can search the directory by keyword and filter by category, with a clear empty state.

**Independent Test**: With several listings present, sign in as a resident and verify `GET /directory?q=plumbing` and `GET /directory?category=<id>` return only matching listings.

### Tests for User Story 2 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T015 [P] [US2] Write failing service test for `SearchListings` (keyword across name/category/notes, category filter, empty results) in `internal/service/directory_test.go`
- [X] T016 [P] [US2] Write failing handler test for `GET /directory` with `q` and `category` query params (filtering, empty state) in `internal/handler/directory_test.go`

### Implementation for User Story 2

- [X] T017 [US2] Implement `DirectoryService.SearchListings` in `internal/service/directory.go` (escaped `ILIKE` keyword + active-category filter per `research.md` R4) until T015 passes
- [X] T018 [US2] Extend `internal/handler/directory.go` list handler to parse `q`/`category`, call `SearchListings`, and render results or empty state until T016 passes
- [X] T019 [US2] Update `web/templates/directory.templ` with a search input, category filter, and empty-state message; run `scripts/templ-generate.sh`
- [X] T020 [US2] Run `go test ./...`; confirm T015 and T016 pass

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - Syndic moderates, edits, and deletes listings (Priority: P3)

**Goal**: The syndic can edit or delete any listing and can add, rename, and deactivate categories.

**Independent Test**: Sign in as the syndic, edit one listing, delete another, add/rename/deactivate a category, and verify all changes are reflected for residents.

### Tests for User Story 3 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T021 [P] [US3] Write failing service tests for `EditListing`, `DeleteListing`, and category management (`CreateCategory`, `RenameCategory`, `DeactivateCategory`) including audit events and resident-denied rules in `internal/service/directory_test.go`
- [X] T022 [P] [US3] Write failing handler tests for syndic edit/delete routes and category routes (syndic-only 403 checks, 404 handling, validation) in `internal/handler/directory_test.go`

### Implementation for User Story 3

- [X] T023 [US3] Implement `DirectoryService.EditListing` and `DirectoryService.DeleteListing` in `internal/service/directory.go` (syndic-only, audit events per `research.md` R6) until T021 passes
- [X] T024 [US3] Implement category management use cases in `internal/service/directory.go` (`CreateCategory`, `RenameCategory`, `DeactivateCategory`) per `data-model.md` state transitions until T021 passes
- [X] T025 [US3] Implement syndic handlers in `internal/handler/directory.go` for `GET/POST /directory/{id}/edit`, `POST /directory/{id}/delete`, and category routes `GET/POST /directory/categories`, `POST /directory/categories/{id}/rename`, `POST /directory/categories/{id}/deactivate`; register them in `internal/handler/router.go` with syndic-only middleware
- [X] T026 [US3] Update `web/templates/directory.templ` with listing edit form, delete confirmation, and category management UI; run `scripts/templ-generate.sh`
- [X] T027 [US3] Run `go test ./...`; confirm T021 and T022 pass

**Checkpoint**: All user stories should now be independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Generated assets, full-suite validation, and review readiness

- [X] T028 Rebuild Tailwind output with `make tailwind` and regenerate templates with `make templ`; commit generated `web/static/css/output.css` and `web/templates/*_templ.go`
- [X] T029 Run `make migrate` against a fresh dockerized PostgreSQL, then execute every scenario in `specs/003-service-directory/quickstart.md`
- [X] T030 Run `go test ./...` and `go vet ./...`; review the final diff for leftover debug files and confirm only intended changes remain

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 must finish before US2 because US2 extends the `GET /directory` handler and template created in US1
  - US3 can start after US1 (it reuses the directory page and listing model); it does not require US2
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Depends on US1's directory page/handler (T012, T013) but is otherwise independently testable
- **User Story 3 (P3)**: Depends on US1's listing model/persistence (T001-T007) and directory page; search (US2) is not required

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Models before services
- Services before endpoints
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- Phase 1: T004 is parallel with T002/T003 (different files)
- Within US1: T008 and T009 can be written in parallel (different files)
- Within US2: T015 and T016 can be written in parallel (different files)
- Within US3: T021 and T022 can be written in parallel (different files)
- US3 can be developed in parallel with US2 after US1 completes

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: "Write failing service test for CreateListing in internal/service/directory_test.go"
Task: "Write failing handler test for GET /directory/new and POST /directory in internal/handler/directory_test.go"
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

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (blocking for US2)
   - After US1: Developer B takes User Story 2 while Developer A takes User Story 3
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
