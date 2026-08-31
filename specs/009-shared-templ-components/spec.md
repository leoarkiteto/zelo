# Feature Specification: Shared Templ Components

**Feature Branch**: `009-shared-templ-components`

**Created**: 2026-08-31

**Status**: Draft

**Input**: User description: "Organize the Templ components in a specific folder structure to be share to all features"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Developer finds shared Templ components in a predictable structure (Priority: P1)

A developer working on any feature can open the shared templates area and immediately locate the app shell, reusable UI components, icons, and feedback components organized by atomic design layers (atoms, molecules, organisms). They do not have to read every file or guess whether a component is shared or where it lives.

**Why this priority**: This is the core request — organize the shared Templ components so they can be discovered and reused by all features.

**Independent Test**: Open the shared templates area and verify each documented component category has its own folder and that each existing shared component sits in exactly one documented category.

**Acceptance Scenarios**:

1. **Given** a developer opens the shared templates area, **When** they look for a reusable UI component, **Then** it is located in a folder named after its atomic layer and its purpose is obvious from its name.
2. **Given** the shared templates area, **When** a developer reads the documentation, **Then** the folder structure matches the documentation exactly.
3. **Given** a developer needs the app shell, **When** they browse the shared templates area, **Then** the layout components are in a dedicated layout folder and not mixed with page-specific templates.

---

### User Story 2 - Features reuse shared components instead of copying markup (Priority: P2)

When building or updating a page, a developer imports the shared component from the shared templates area instead of copying its markup into their feature's templates. As a result, shared UI renders consistently across all features, and fixes to a shared component apply everywhere at once.

**Why this priority**: The value of a shared structure is realized only if features actually reuse the components rather than duplicating them.

**Independent Test**: Pick a shared component and use it from two different features; verify both render the same markup and neither feature contains a private copy of that component.

**Acceptance Scenarios**:

1. **Given** a shared component exists, **When** a feature needs that UI, **Then** the feature imports the shared component and does not create a local copy.
2. **Given** two features use the same shared component, **When** it is rendered in each feature, **Then** the output is identical apart from the content each feature supplies.
3. **Given** a shared component is updated, **When** any feature renders it, **Then** the update is reflected in all features without editing feature templates.

---

### User Story 3 - Existing shared Templ components migrate without visual change (Priority: P3)

The existing shared Templ components — the app shell/layout, the error page, and the role badge helper — move into the new folder structure. Every page keeps rendering exactly as before; only the internal organization and import paths change.

**Why this priority**: The repository already has shared Templ components; the new structure is only complete when they are migrated without breaking the application.

**Independent Test**: After migration, run the existing page-render tests and exercise key pages; verify the rendered pages are identical to before and tests that passed before still pass.

**Acceptance Scenarios**:

1. **Given** the existing shared Templ components live flat in the shared templates area, **When** the migration runs, **Then** each component is moved to the folder for its category.
2. **Given** a migrated shared component, **When** a feature renders a page that uses it, **Then** the page output is identical to before the migration.
3. **Given** a migrated shared component, **When** its tests run, **Then** all tests that passed before still pass without weakening assertions.

---

### User Story 4 - New shared components are added in the right place (Priority: P4)

When a developer needs a UI piece used by more than one feature, they create it in the shared templates area under the correct category folder, following the documented naming and test rules. Components used by only one feature stay in that feature's templates.

**Why this priority**: A clear, enforced placement rule keeps the shared area organized as the project grows and prevents it from becoming a dumping ground.

**Independent Test**: Add a throwaway shared component following the documentation, then verify it lands in the correct category folder, has a smoke test, and passes the boundary check.

**Acceptance Scenarios**:

1. **Given** a UI component is needed by more than one feature, **When** a developer adds it, **Then** it is placed in the shared templates area under its category folder and named per the convention.
2. **Given** a UI component is used by only one feature, **When** a developer adds it, **Then** it stays inside that feature's templates and is not added to the shared area.
3. **Given** a new shared component, **When** the boundary check runs, **Then** no shared component imports feature-specific code and the check passes.

---

### Edge Cases

- What happens when a component is used by only one feature today but might be shared later? It stays in the feature's templates; it moves to the shared area only when a second feature actually needs it.
- What happens when two features need similar but not identical UI? The shared component is parameterized to accept feature-specific content; if the differences are too large, the component stays feature-specific and is not forced into the shared area.
- What happens during migration when a page renders differently? The migration is incomplete for that component until render tests pass and the output matches the previous version.
- What happens when a shared component needs to show feature-specific data? It receives the data through parameters; it never imports feature code.
- What happens when a developer hand-edits a generated Templ file? The change is rejected; generated files are regenerated by the standard generation command and committed.
- What happens when a feature template still contains a private copy of a shared component? Code review rejects the duplication and the feature is updated to import the shared component.
- What happens when the shared area grows? New category folders may be added through the documented process, but existing categories keep stable names.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The shared Templ components MUST live in a single shared templates area organized into stable category folders.
- **FR-002**: The shared templates area MUST be organized using atomic design layers: atoms (smallest reusable UI pieces such as buttons, badges, icons, and form controls), molecules (compositions of atoms such as forms, cards, and alerts), and organisms (larger composite sections such as the app shell/navigation and error page). Feature-level templates and pages remain in each feature's templates area.
- **FR-003**: Every shared Templ component MUST be placed in exactly one category folder and named using a documented naming convention.
- **FR-004**: Features MUST import shared UI components from the shared templates area and MUST NOT duplicate their markup inside feature templates.
- **FR-005**: Feature-specific templates and components that are used by only one feature MUST remain inside that feature's templates area.
- **FR-006**: The existing shared Templ components (app layout/shell, error page, role badge helper) MUST be migrated into the new category folders with no change to rendered output.
- **FR-007**: Shared Templ components MUST accept feature-specific content through parameters; features MUST NOT create modified copies of a shared component.
- **FR-008**: The shared templates area MUST NOT depend on any feature's internal code; it may depend only on other shared modules.
- **FR-009**: Each shared Templ component MUST have an automated smoke test that renders it and verifies its key content appears.
- **FR-010**: The documentation MUST describe the shared templates folder structure, the rules for deciding whether a component belongs in the shared area or a feature, and the steps to add a new shared component.
- **FR-011**: Generated Templ artifacts MUST be produced by the standard generation command and committed; they MUST NOT be hand-edited.
- **FR-012**: The existing feature-boundary check MUST continue to pass and MUST treat the shared templates area as shared code that no feature may import from another feature's templates.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer unfamiliar with the codebase can locate any shared Templ component in under 2 minutes using the documented folder structure.
- **SC-002**: 100% of shared Templ components reside in their documented category folder after migration.
- **SC-003**: 0 feature templates contain a private copy of shared component markup, as verified by code review.
- **SC-004**: 100% of page-render tests that passed before migration still pass after migration.
- **SC-005**: A developer can add a new shared Templ component, including its smoke test, in under 30 minutes following the documentation.
- **SC-006**: 100% of new components used by more than one feature are added to the shared templates area rather than duplicated, as verified by code review.

## Assumptions

- The feature covers only the organization of Templ components; Go handlers, services, and persistence are out of scope.
- The shared templates area already exists as `internal/shared/templates`; this feature reorganizes its contents and defines conventions inside that area rather than creating a new top-level module.
- The proposed atomic folders are `atoms/`, `molecules/`, and `organisms/`; the exact list of components in each folder is confirmed during planning.
- Existing feature page templates remain in their feature folders; only genuinely reusable UI is extracted into the shared area.
- The initial extraction candidates are the currently shared components (layout/shell, error page, role badge helper) plus repeated UI patterns such as icons; the full extraction list is identified during planning.
- Import paths in existing feature templates are updated as part of the migration; no user-visible behavior changes.
- Documentation lives with the repository documentation and is kept in sync with the actual folder structure.
- Enforcement relies on the existing Spec Kit workflow and code review, consistent with the project's feature-isolation rules.
