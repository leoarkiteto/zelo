---
description: "Task list for feature implementation"
---

# Tasks: i18n Language Switcher

**Input**: Design documents from `/specs/005-i18n-language-switcher/`

**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: REQUIRED — the project constitution (Principle IV) and `plan.md` mandate test-first development. Tests for domain, service, handler, store, and shared i18n must be written first and seen failing before the corresponding implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Single Go module `github.com/leoarkiteto/zelo`, binary entrypoint `cmd/web`
- Feature slices under `internal/features/<feature>/` with `core/{domain,ports,services}`, `handlers/`, `templates/`
- Shared modules under `internal/shared/` (`model`, `store`, `middleware`, `templates`, `httpx`, `i18n`)
- Migrations are forward-only SQL files in `migrations/`
- Generated Templ artifacts (`*_templ.go`) are produced by `scripts/templ-generate.sh` (`make templ`) and committed

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Scaffold the new feature slice and prepare the storage migration before any feature code is written.

- [X] T001 Run `scripts/new-feature.sh profile` to scaffold `internal/features/profile/` from `scripts/_feature-template/`
- [X] T002 Create `migrations/0008_add_user_language_preference.sql` with `ALTER TABLE users ADD COLUMN language_preference TEXT;` plus `ALTER TABLE users ADD CONSTRAINT users_language_preference_check CHECK (language_preference IN ('en','pt-br'));`, then apply it with `make migrate`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared i18n module, locale middleware, persistence support, and localized shell that EVERY user story depends on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

### Foundational Tests (write FIRST, ensure they FAIL before implementation) ⚠️

- [X] T003 [P] Write failing tests for the `Language` value object in `internal/shared/i18n/language_test.go`: constants `en`/`pt-br`, `Valid()` rejects anything else, `Default()` returns `en`
- [X] T004 [P] Write failing tests for the message catalog in `internal/shared/i18n/catalog_test.go`: every key exists in both `en` and `pt-br`, `T()` returns the requested language's value, missing key falls back to English
- [X] T005 [P] Write failing tests for per-request locale context helpers in `internal/shared/i18n/context_test.go`
- [X] T006 [P] Write failing tests for `WithLocale` in `internal/shared/middleware/locale_test.go`: anonymous request resolves `en`, user with `pt-br` preference resolves `pt-br`, missing/invalid preference resolves `en`
- [X] T007 [P] Write failing store tests in `internal/shared/store/users_test.go` using `testutil.TestDatabaseURL("store")`: `GetUserByID`/`GetUserByEmail` scan `language_preference`, `UpdateLanguagePreference` persists it, `NULL` reads as empty
- [X] T008 [P] Write failing layout locale smoke test in `internal/shared/templates/smoke_test.go`: `<html lang="{locale}">` and `<div id="app-shell" lang="{locale}">` render the locale from `ShellData`

### Foundational Implementation

- [X] T009 [P] Implement the `Language` type in `internal/shared/i18n/language.go` with `LanguageEN`, `LanguagePTBR`, `Valid()`, and `Default()` per `data-model.md`
- [X] T010 [P] Implement the message catalog and `T(lang, key)` lookup with English fallback in `internal/shared/i18n/catalog.go`; include the initial shared shell keys (`nav.dashboard`, `nav.directory`, `nav.management`, `nav.invitations`, `nav.roles`, `nav.unit`, `nav.tenancy`, `topbar.sign_out`, etc.) in both languages
- [X] T011 [P] Implement locale context helpers in `internal/shared/i18n/context.go` (`WithLanguage`, `LanguageFrom`, resolution from a raw preference string)
- [X] T012 [P] Add `LanguagePreference string` field to `model.User` in `internal/shared/model/user.go` (empty string = unset/default)
- [X] T013 Update `UserStore.GetUserByEmail` and `UserStore.GetUserByID` in `internal/shared/store/users.go` to scan `language_preference` into the new field (`NULL` → empty)
- [X] T014 Add `UpdateLanguagePreference(ctx context.Context, userID, language string) error` to `UserStore` in `internal/shared/store/users.go` (`UPDATE users SET language_preference = $1, updated_at = now() WHERE id = $2`)
- [X] T015 Implement `WithLocale` middleware in `internal/shared/middleware/locale.go`: reads `middleware.UserFrom(ctx)`, resolves `en` default from `LanguagePreference`, stores locale via `i18n.WithLanguage`
- [X] T016 Update `internal/shared/templates/layout.templ`: add `Locale` to `ShellData`, render `<html lang="{Locale}">`, change the shell root to `<div id="app-shell" lang="{Locale}">`, and localize shell strings via the catalog
- [X] T017 Update `internal/shared/httpx/shell.go`: set `ShellData.Locale` from `i18n.LanguageFrom(r.Context())` and build nav labels through the catalog
- [X] T018 Run `scripts/templ-generate.sh` and commit the regenerated `internal/shared/templates/layout_templ.go`
- [X] T019 Wire `WithLocale` between `WithUser` and `CSRF` in `cmd/web/main.go`

**Checkpoint**: Foundation ready — user story implementation can now begin. `go test ./...` passes for the foundational packages.

---

## Phase 3: User Story 1 - User toggles the interface language from the profile page (Priority: P1) 🎯 MVP

**Goal**: A signed-in user opens `/profile`, sees a language toggle with English and Português (BR), and switching updates the entire authenticated shell in one HTMX round-trip without losing page state.

**Independent Test**: Sign in, open `/profile`, toggle `en` → `pt-br`, and confirm all visible interface text changes to Brazilian Portuguese immediately. Toggle back and confirm it reverts to English.

### Tests for User Story 1 ⚠️

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T020 [P] [US1] Write failing service tests in `internal/features/profile/core/services/profile_test.go`: `GetProfile` returns the profile with the current language, `ChangeLanguage` validates and persists via fake ports, invalid language returns an error
- [X] T021 [P] [US1] Write failing handler tests in `internal/features/profile/handlers/profile_test.go`: `GET /profile` returns 200 with the toggle and current locale selected; `POST /profile/language` with `HX-Request: true` returns 200 with `<div id="app-shell" lang="pt-br">` and calls the persistence port; invalid language returns 400; anonymous requests return 302; plain form (no HTMX) returns 302 to `/profile`
- [X] T022 [P] [US1] Write failing Templ smoke test in `internal/features/profile/templates/smoke_test.go`: the profile page renders both language options and the selected state

### Implementation for User Story 1

- [X] T023 [P] [US1] Create the `LanguagePreference` domain value object in `internal/features/profile/core/domain/language.go` (wraps/validates `i18n.Language`, default `en`)
- [X] T024 [P] [US1] Declare consumer-owned ports in `internal/features/profile/core/ports/repositories.go` (`LanguagePreferenceReader`, `LanguagePreferenceUpdater`) and `internal/features/profile/core/ports/services.go` (`ProfileService`)
- [X] T025 [US1] Implement `ProfileService` in `internal/features/profile/core/services/profile.go` with `GetProfile` and `ChangeLanguage` use cases (depends on T023, T024)
- [X] T026 [P] [US1] Create `internal/features/profile/templates/profile.templ` with the language toggle per `contracts/ui-contract.md`: HTMX form `hx-post="/profile/language"`, `hx-target="#app-shell"`, `hx-swap="outerHTML"`, hidden `csrf_token`, two options (`en`/`pt-br`) with labels and flags, current locale selected
- [X] T027 [US1] Create handler dependencies in `internal/features/profile/handlers/deps.go` (logger, profile service, roles/audit for the shell)
- [X] T028 [US1] Implement `GET /profile` and `POST /profile/language` in `internal/features/profile/handlers/profile.go` per `contracts/http-endpoints.md`: HTMX shell swap, plain-form 302 fallback, 400 invalid input, 500 persistence failure
- [X] T029 [US1] Register `GET /profile` and `POST /profile/language` in `internal/features/profile/handlers/routes.go`
- [X] T030 Run `scripts/templ-generate.sh` and commit the generated `internal/features/profile/templates/profile_templ.go`
- [X] T031 Wire profile deps, service, and routes into `cmd/web/main.go`

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently — toggle switches the whole UI immediately.

---

## Phase 4: User Story 2 - Language preference is saved and applied at login (Priority: P2)

**Goal**: The language choice is persisted to `users.language_preference` and automatically applied at login on any device, with English as the default.

**Independent Test**: Sign in, toggle to `pt-br`, sign out, sign in again — the application appears in Brazilian Portuguese immediately after login.

### Tests for User Story 2 ⚠️

- [X] T032 [P] [US2] Write failing integration test in `tests/integration/language_preference_test.go`: login as syndic → `POST /profile/language` with `pt-br` → sign out → sign in → dashboard renders `lang="pt-br"` shell and `SELECT language_preference FROM users` returns `pt-br`; also assert a user with no preference sees English after login

### Implementation for User Story 2

- [X] T033 [US2] Wire profile handlers, profile service, and `WithLocale` (after `WithUser`) into `newApp` in `tests/integration/integration_test.go` so the US2 integration test exercises the composed app

**Checkpoint**: At this point, User Stories 1 AND 2 both work independently — preference survives logout/login.

---

## Phase 5: User Story 3 - Each language option shows its country flag (Priority: P3)

**Goal**: The toggle presents exactly two accessible options: `🇺🇸 English` and `🇧🇷 Português (BR)`, with the current language clearly selected.

**Independent Test**: Open `/profile` and confirm the toggle shows exactly two options with the United States flag for English and the Brazil flag for Português (BR).

### Tests for User Story 3 ⚠️

- [X] T034 [P] [US3] Write failing accessibility/flag handler test in `internal/features/profile/handlers/language_toggle_test.go`: `GET /profile` renders exactly two options, 🇺🇸 for `en`, 🇧🇷 for `pt-br`, flags are `aria-hidden="true"`, each option has an accessible text label, and the active language is marked selected

### Implementation for User Story 3

- [X] T035 [US3] Finalize flag markup and selected-state indication in `internal/features/profile/templates/profile.templ` (`aria-hidden` flags, visible labels, `aria-current`/checked state for the active option)
- [X] T036 [US3] Run `scripts/templ-generate.sh` and commit the regenerated `internal/features/profile/templates/profile_templ.go`

**Checkpoint**: Flags are visible and the toggle remains accessible; US1–US3 all pass independently.

---

## Phase 6: User Story 4 - Every user-facing message appears in the selected language (Priority: P4)

**Goal**: All user-facing text — navigation, buttons, labels, help text, validation messages, error messages, empty states, confirmations — is available in both `en` and `pt-br`, with English fallback for any missing message and locale-aware date/number formatting.

**Independent Test**: Walk through the main flows once in English and once in Brazilian Portuguese and confirm every user-facing message appears in the selected language, with no mixed-language screens or technical codes.

### Tests for User Story 4 ⚠️

- [X] T037 [P] [US4] Write failing catalog coverage test in `internal/shared/i18n/catalog_test.go`: enumerate required message keys for shared layout/error, auth, home, directory, management, and profile and assert non-empty `en` and `pt-br` values
- [X] T038 [P] [US4] Write failing tests for locale-aware formatting helpers in `internal/shared/i18n/format_test.go` (date and number conventions for `en` vs `pt-br`)

### Implementation for User Story 4

- [X] T039 [US4] Implement date/time/number formatting helpers for `en` and `pt-br` in `internal/shared/i18n/format.go` (depends on T038 failing)
- [X] T040 [P] [US4] Add a `Locale` field to auth template data structs and pass `i18n.LanguageFrom(r.Context())` from `internal/features/auth/handlers/`
- [X] T041 [P] [US4] Add a `Locale` field to home template data structs and pass `i18n.LanguageFrom(r.Context())` from `internal/features/home/handlers/`
- [X] T042 [P] [US4] Add a `Locale` field to directory template data structs and pass `i18n.LanguageFrom(r.Context())` from `internal/features/directory/handlers/`
- [X] T043 [P] [US4] Add a `Locale` field to management template data structs and pass `i18n.LanguageFrom(r.Context())` from `internal/features/management/handlers/`
- [X] T044 [US4] Translate the shared error page in `internal/shared/templates/error.templ` to use the catalog (`i18n.T(data.Locale, key)`)
- [X] T045 [P] [US4] Translate auth templates in `internal/features/auth/templates/` (`auth.templ`, `login.templ`, `register.templ`, `forgot_password.templ`, `reset_password.templ`) to use the catalog (depends on T040)
- [X] T046 [P] [US4] Translate home templates in `internal/features/home/templates/` (`dashboard.templ`, `area.templ`) to use the catalog (depends on T041)
- [X] T047 [P] [US4] Translate the directory template in `internal/features/directory/templates/directory.templ` to use the catalog (depends on T042)
- [X] T048 [P] [US4] Translate management templates in `internal/features/management/templates/` (`invitations.templ`, `roles.templ`) to use the catalog (depends on T043)
- [X] T049 [US4] Add every translated message key to `internal/shared/i18n/catalog.go` with both `en` and `pt-br` values so the catalog coverage test passes (no keys missing)
- [X] T050 [US4] Apply the `internal/shared/i18n/format.go` helpers wherever dates or numbers are rendered across templates/handlers
- [X] T051 Run `scripts/templ-generate.sh` and commit all regenerated `*_templ.go` files

**Checkpoint**: All user stories should now be independently functional; full interface text is available in both languages.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Verification gates and final quality sweeps that affect the whole feature.

- [X] T052 Run `go test ./...` and fix any failures
- [X] T053 Run `scripts/check-feature-boundaries.sh` and fix any cross-slice violations
- [X] T054 Run `make templ` and `make tailwind` and commit any regenerated/compiled assets (`web/static/css/output.css`, `*_templ.go`)
- [X] T055 Execute all validation scenarios in `specs/005-i18n-language-switcher/quickstart.md` (scenarios 1–6) in both languages
- [X] T056 Sweep templates for remaining hardcoded user-facing strings and move any found into `internal/shared/i18n/catalog.go`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - US1 (P1) is the MVP and must land first
  - US2, US3, US4 can then proceed; US2 depends on the US1 endpoint wiring, US3 depends on the US1 toggle, US4 touches all feature templates
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational; integration test needs the US1 `/profile/language` route wired into `newApp`
- **User Story 3 (P3)**: Can start after Foundational; refines the US1 toggle markup
- **User Story 4 (P4)**: Can start after Foundational; translates all slices and requires the `Locale` plumbing from Phase 2

### Within Each User Story

- Tests MUST be written and FAIL before implementation (constitution Principle IV)
- Models/domain before services
- Services before handlers/endpoints
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All foundational test tasks (T003–T008) are independent files and can run in parallel
- Foundational implementation tasks marked [P] (T009–T012) can run in parallel
- All User Story test tasks marked [P] can run in parallel
- Within US1, T023/T024/T026 touch different files and can run in parallel
- US4 Locale plumbing (T040–T043) can run in parallel across feature slices
- US4 translation tasks (T045–T048) can run in parallel after their slice's Locale field exists

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together (write them first, see them fail):
Task: "Write failing service tests in internal/features/profile/core/services/profile_test.go"
Task: "Write failing handler tests in internal/features/profile/handlers/profile_test.go"
Task: "Write failing Templ smoke test in internal/features/profile/templates/smoke_test.go"

# Launch independent US1 implementation files together after the tests fail:
Task: "Create LanguagePreference in internal/features/profile/core/domain/language.go"
Task: "Declare ports in internal/features/profile/core/ports/*.go"
Task: "Create profile.templ in internal/features/profile/templates/profile.templ"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Toggle `en` ↔ `pt-br` on `/profile` and confirm the whole shell updates
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Verify preference survives login → Deploy/Demo
4. Add User Story 3 → Verify flags and accessibility → Deploy/Demo
5. Add User Story 4 → Verify complete translation coverage → Deploy/Demo
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (profile slice + toggle)
   - Developer B: User Story 2 (integration test + login application) — after A wires the route
   - Developer C: User Story 4 catalog coverage + formatting helpers (independent files)
3. US3 joins after US1's toggle exists; stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to a specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing (constitution Principle IV)
- Commit after each task or logical group
- Stop at any checkpoint to validate a story independently
- Generated Templ output must come from `scripts/templ-generate.sh`, never hand-edited
- Avoid: vague tasks, same-file conflicts, cross-story dependencies that break independence

---

## Phase 8: Convergence

**Purpose**: Close gaps found by `/speckit-converge` after the initial implementation — localized runtime messages in the authenticated slices and gap recording in the catalog fallback.

- [X] T057 Localize all handler-generated user-facing messages in `internal/features/directory/handlers/directory.go` (validation errors, not-found messages, duplicate warnings, and the `flashMessage()` confirmations) through the i18n catalog via `i18n.T(i18n.LanguageFrom(r.Context()), key)`; keep the English catalog values identical to the current strings so the existing assertions in `tests/integration/directory_test.go` keep passing; add any new keys to `internal/shared/i18n/catalog.go` (en + pt-br) and to the coverage list in `internal/shared/i18n/catalog_test.go` per FR-008 (partial)
- [X] T058 Localize all handler-generated user-facing messages in `internal/features/management/handlers/roles.go`, `internal/features/management/handlers/invitations.go`, and the generic 500 responses in `internal/features/home/handlers/` through the i18n catalog; keep English values unchanged; add new keys to `internal/shared/i18n/catalog.go` and `internal/shared/i18n/catalog_test.go` per FR-008 (partial)
- [X] T059 Record catalog fallback gaps in `internal/shared/i18n/catalog.go`: when `T()` (via `lookup`) falls back because a key is missing for the requested language, emit a warning log (e.g. `slog.Warn("missing translation", "lang", lang, "key", key)`) so missing messages are recorded for correction, satisfying the spec edge case per FR-009 (partial)
