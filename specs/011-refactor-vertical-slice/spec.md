# Feature Specification: Vertical Slice Architecture Migration

**Feature Branch**: `011-refactor-vertical-slice`

**Created**: 2026-09-12

**Status**: Draft

**Input**: User description: "Refatorar todo o projeto para mudar de "Hexagonal Architecture" para "Vertical Slice Architecture", pois o projeto tem um desenvolvedor solo e a arquitetura atual tem muita complexidade"

## Clarifications

### Session 2026-09-12

- Q: What internal structure should a migrated feature use? → A: Role-based sub-folders (`handlers/`, `services/`, `repositories/`, `domain/`, `templates/`) directly under the feature root, without the `core/` wrapper and without a `ports/` layer.
- Q: Where should domain types (user, role, invitation, condominium, etc.) live after removing ports? → A: Keep the shared model module for cross-feature entities, and allow feature-local types in the feature's own `domain/` sub-folder only when they are exclusive to that feature.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Solo developer changes a feature inside one self-contained slice (Priority: P1)

The solo developer opens a single feature folder and finds everything needed to understand and change that feature — request handling, use cases, data access, and templates — together in one place, without tracing through a mandatory ports/adapters interface layer. Making a change requires touching files in that feature folder only, plus shared modules when the change genuinely affects cross-feature code.

**Why this priority**: This is the core pain point in the request. The current hexagonal layering adds indirection and files that a solo developer must maintain for every feature, which is the complexity the refactor aims to remove.

**Independent Test**: Pick any migrated feature, implement a small behavior change, and confirm that only files inside that feature's folder (and, if applicable, shared modules) are edited, while the application keeps working and the existing tests for that feature pass.

**Acceptance Scenarios**:

1. **Given** a migrated feature folder, **When** the developer opens it, **Then** the request handling, use cases, data access, and templates are organized in role-based sub-folders (`handlers/`, `services/`, `repositories/`, `domain/`, `templates/`) directly inside that feature folder, without a `core/` wrapper or a `ports/` layer.
2. **Given** a feature folder, **When** the developer reads its contents, **Then** there is no mandatory ports/adapters interface layer and no `core/` wrapper to update for a use-case change.
3. **Given** a developer changes one feature, **When** they implement the change, **Then** they do not need to edit files in another feature's folder.
4. **Given** two features need the same capability, **When** the developer implements it, **Then** the shared capability lives in a shared module, not inside one of the feature folders.

---

### User Story 2 - All existing features are migrated without changing behavior (Priority: P2)

Every existing feature is restructured from the current hexagonal layout into the simplified vertical-slice layout. For end users, the application continues to behave exactly as before; only the internal organization changes.

**Why this priority**: The complexity goal is only met when the features that already exist are migrated. This is the largest piece of work and must be done carefully so nothing user-visible breaks.

**Independent Test**: After migrating each feature, run the full automated test suite and exercise the application end to end, then confirm every flow (sign-in, roles, directory, finance, tickets, home, management, profile, language switching) behaves exactly as before the migration.

**Acceptance Scenarios**:

1. **Given** an existing feature still uses the old `core/` + `ports/` layout, **When** the migration is performed, **Then** all of that feature's code is reorganized into the simplified vertical-slice layout.
2. **Given** a migrated feature, **When** the application is exercised end to end, **Then** its user-visible behavior is identical to before the migration.
3. **Given** a migrated feature, **When** its automated tests run, **Then** the tests pass without requiring the removed hexagonal ports layer.
4. **Given** a migration step, **When** the step is complete, **Then** the application still builds and the full test suite still passes.

---

### User Story 3 - Tooling, scaffolding, and documentation enforce vertical slices only (Priority: P3)

The project's new-feature scaffolding, architecture checks, and developer documentation are updated to describe and enforce the simplified vertical-slice layout, so the hexagonal ports/adapters pattern does not creep back in.

**Why this priority**: The refactor only sticks if the tooling and documentation a developer relies on point to the new structure. Without it, future features would recreate the complexity that was just removed.

**Independent Test**: Run the new-feature scaffold, confirm the generated structure is the simplified vertical slice, run the architecture checks against the migrated codebase, and read the architecture documentation to confirm it no longer instructs hexagonal ports/adapters layering.

**Acceptance Scenarios**:

1. **Given** the new-feature scaffold, **When** a developer generates a new feature, **Then** the scaffold creates the role-based sub-folders (`handlers/`, `services/`, `repositories/`, `domain/`, `templates/`) directly under the feature root, without a `core/` wrapper or a `ports/` layer.
2. **Given** the architecture checks, **When** they run against the migrated codebase, **Then** they reject cross-feature imports and pass on the simplified layout.
3. **Given** the developer documentation, **When** the developer reads it, **Then** it describes the vertical-slice-only structure and no longer instructs the removed hexagonal layering.
4. **Given** the constitution, **When** it is reviewed, **Then** its architecture principle reflects vertical slices only, without the hexagonal ports/adapters requirement.

### Edge Cases

- What happens when two features need the same data or behavior? The shared capability must live in a shared module; a feature must not import another feature's internals.
- What happens to existing tests that relied on ports interfaces with fake implementations? They must be adapted to slice-level test doubles and must still verify the same behavior.
- What happens to cross-cutting concerns such as sessions, security, middleware, and i18n? They remain centralized in shared modules and are not duplicated inside feature folders.
- What happens if a feature is only partially migrated and still references the old `core/` or `ports/` layer? The architecture/structure check must fail for that feature until its migration is complete, and the build must stay green between migration steps.
- What happens to generated files when templates or styles move? They must be regenerated and committed after the move so the output stays in sync.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The codebase MUST organize each feature's complete behavior — request handling, use cases, data access, and templates — inside its own feature folder, using role-based sub-folders (`handlers/`, `services/`, `repositories/`, `domain/`, `templates/`) directly under the feature root with no `core/` wrapper.
- **FR-002**: The codebase MUST NOT require a mandatory hexagonal ports/adapters interface layer inside feature folders.
- **FR-003**: All existing features MUST be migrated to the simplified vertical-slice layout.
- **FR-004**: A feature MUST NOT import or reference the internals of another feature.
- **FR-005**: Cross-feature capabilities MUST remain centralized in shared modules, which are the only allowed cross-feature dependency.
- **FR-006**: All user-facing behavior MUST remain unchanged during and after the migration, including sign-in, roles, service directory, finance, tickets, home, management, profile, and language switching.
- **FR-007**: Each migrated feature MUST retain automated tests that verify its behavior, and those tests MUST run without requiring a hexagonal ports layer.
- **FR-008**: The application MUST build and pass all quality gates after each feature's migration, with no all-at-once breakage.
- **FR-009**: The new-feature scaffolding MUST generate the simplified vertical-slice structure: role-based sub-folders (`handlers/`, `services/`, `repositories/`, `domain/`, `templates/`) at the feature root, without a `core/` wrapper or a `ports/` layer.
- **FR-010**: The architecture checks MUST enforce the simplified layout: they MUST reject cross-feature imports and any reintroduction of the removed `core/` wrapper or `ports/` layer.
- **FR-011**: The architecture documentation and constitution MUST describe the vertical-slice-only structure and remove hexagonal ports/adapters guidance.
- **FR-012**: Shared domain types used by more than one feature MUST remain in the shared model module; a feature MAY define feature-local domain types in its own `domain/` sub-folder only when no other feature uses them.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of existing features are migrated to self-contained feature folders, verified by an automated structure check that finds no remaining `core/` wrapper or `ports/` layer.
- **SC-002**: The full automated test suite passes with 100% success after the migration, and no user-visible regression is found during end-to-end exercise of all existing flows.
- **SC-003**: A developer can add a new feature by creating and wiring files inside a single feature folder only, with zero edits required in other feature folders.
- **SC-004**: A developer can locate all of the code for any migrated feature inside one feature folder in under 2 minutes.
- **SC-005**: The project's automated quality gates pass on the migrated codebase, including the architecture boundary checks.

## Assumptions

- The migration is performed incrementally, one feature at a time, keeping the application building and all tests passing after each feature (no big-bang cutover).
- The hexagonal `core/` wrapper and `ports/` interface layer are removed entirely rather than kept alongside the new structure.
- Cross-cutting concerns (sessions, security, middleware, i18n, shared templates) remain centralized in shared modules and are not duplicated per feature.
- The existing shared model module remains the home of cross-feature domain entities; feature-local types move into the feature's `domain/` sub-folder only when exclusive to that feature.
- Test-Driven Development and the existing quality gates remain mandatory during the migration.
- Generated assets (templated views, compiled styles) are regenerated and committed after any file moves.
- No new user-facing functionality is added in this refactor; the scope is internal structure only.
