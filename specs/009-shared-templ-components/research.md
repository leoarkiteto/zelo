# Research: Shared Templ Components (Atomic Design)

**Feature**: `specs/009-shared-templ-components` | **Date**: 2026-08-31

This document resolves the design questions for reorganizing the shared Templ components into atomic design layers. The feature spec contained no unresolved `[NEEDS CLARIFICATION]` markers; the questions below are the technical decisions required to turn the spec into a plan.

## R1. How to map atomic design onto a Go/Templ component library

**Decision**: Use the classic atomic design levels but only the three that belong in a shared library: **atoms**, **molecules**, and **organisms**. The remaining levels — *templates* and *pages* — stay feature-owned: each feature's `templates/` folder remains the place for page-level templates composed from shared components.

**Rationale**:
- Atoms, molecules, and organisms are the reusable, context-free building blocks; templates/pages are always feature-specific in this codebase and already live in `internal/features/<feature>/templates`.
- Keeping only three layers in `internal/shared/templates` respects the project's vertical-slice isolation: shared folders hold only what more than one feature needs.

**Alternatives considered**:
- Five folders including `templates/` and `pages/` in shared — rejected: it would pull feature-specific pages into shared code and break Principle III.
- One flat shared folder with file-name prefixes (`atoms-*`, `molecules-*`) — rejected: the user explicitly asked for a specific folder structure, and folders make the layering and boundaries explicit.

## R2. Package layout for the layers

**Decision**: One Go sub-package per atomic layer under `internal/shared/templates`:
`internal/shared/templates/atoms`, `internal/shared/templates/molecules`, `internal/shared/templates/organisms`.

**Rationale**:
- Go packages are directory-scoped, so folder-per-layer naturally becomes package-per-layer.
- Separate packages make the dependency direction enforceable with a simple import scan: `atoms` imports nothing above it, `molecules` imports only `atoms` (plus shared modules), `organisms` imports `molecules`/`atoms`.
- Feature templates import exactly the layer they need, making the composition explicit.

**Alternatives considered**:
- Keep one `templates` package and put files in subfolders with the same package name — rejected: multiple directories with the same package name are distinct packages anyway and create confusing import aliases.
- Single flat package with no folders — rejected: does not satisfy the "specific folder structure" requirement.

## R3. Which components belong to which layer

**Decision**: Initial component inventory:

| Layer | Components | Source today |
|-------|------------|--------------|
| atoms | `Button`, `Badge`, `Icon`, `TextInput`, `TextArea`, `Select` | repeated `btn`/`badge`/`svg`/form markup across features |
| molecules | `Card`, `FormField`, `Alert`, `EmptyState` | repeated `card`/`alert`/`empty-state` blocks across features |
| organisms | `Layout`, `Shell` (+ `ShellData`, `NavItem`, `UserView`), `ErrorPage` | existing `layout.templ`, `error.templ` |

The `RoleBadgeClass` helper is replaced by an `atoms.Badge` component with a `Variant` parameter; call sites switch from the helper to the component.

**Rationale**:
- The inventory matches the repeated markup found in the current feature templates (grep for `card`, `btn`, `badge`, `alert`, `empty-state`, inline SVGs) and the components that already exist in `internal/shared/templates`.
- Atoms stay small and context-free; molecules combine atoms; organisms combine molecules and atoms into page-level sections.

**Alternatives considered**:
- Extract only what already exists in shared (`Layout`, `ErrorPage`, `RoleBadgeClass`) — rejected: it leaves the repetition the user wants to remove.
- Extract every possible UI piece now — rejected: speculative extractions would violate "shared only when a second feature actually needs it" and enlarge the change surface.

## R4. Component API style

**Decision**: Each shared component is a small Templ function with an explicit data struct for props; variants are typed string/enum-like parameters (e.g., `ButtonVariant`, `BadgeVariant`, `AlertVariant`, `IconKind`). Content is passed through parameters (labels, icons, children/body components) rather than hard-coded strings.

**Rationale**:
- Matches the existing codebase style (`ShellData`, `ErrorPageData`, `QuickAction` structs).
- Explicit props keep the components reusable across features and testable in isolation.
- Typed variants prevent typos and make the allowed styles discoverable.

**Alternatives considered**:
- Positional parameters — rejected: many optional props would make call sites unreadable.
- Templ children/`{children...}` for every component — used only where whole body blocks are needed (e.g., `Card` body, `Layout` body); fixed props are clearer for small components.

## R5. Migration strategy that guarantees no visual change

**Decision**: Migrate in layers from the bottom up: first create `atoms`, then `molecules`, then `organisms`, then update feature templates to the new imports. For each component, write the render smoke test first (TDD), extract the markup into the shared component, then replace the feature-local markup and compare rendered output via the tests.

**Rationale**:
- Bottom-up means each layer only depends on already-migrated lower layers.
- Test-first extraction proves the component renders the same markup before call sites change.
- Existing render tests (`internal/shared/templates/smoke_test.go` and any feature handler tests) act as a regression net.

**Alternatives considered**:
- Move existing shared files first, then extract — workable, but atoms/molecules would not exist yet for organisms to use, forcing extra churn.
- Big-bang move of everything in one step — rejected: harder to keep pages identical and to review.

## R6. Testing strategy

**Decision**: Colocated Go render smoke tests for every shared component, plus an atomic-boundary test. Each component test renders the component with representative props and asserts the key output (text, class, or `id`) is present and that invalid/edge inputs (empty content, no variant) do not panic. The existing `smoke_test.go` moves/expands into the layer packages.

**Rationale**:
- Constitution IV requires TDD and tests for all contracts.
- Render smoke tests are the cheapest meaningful verification for Templ components and directly support SC-003/SC-004.

**Alternatives considered**:
- Golden HTML files — useful later, but brittle for this migration and not necessary now.
- Browser-based visual tests — rejected: adds tooling that violates the CSS-first/keep-it-small directive.

## R7. Enforcing atomic boundaries

**Decision**: Add a small automated check (extend or mirror `scripts/check-feature-boundaries.sh`) that fails if any file under `internal/shared/templates/atoms` imports `molecules` or `organisms`, or any file under `molecules` imports `organisms`. The existing feature-boundary check continues to enforce that shared templates never import `internal/features/**` and features never import each other.

**Rationale**:
- The layer dependency direction is the core structural rule of atomic design; an automated check keeps it honest.
- Reuses the existing script pattern, so no new tooling.

**Alternatives considered**:
- Code-review-only enforcement — rejected: the project already uses scripts for boundary enforcement and the rule is cheap to automate.
- `go vet`/custom linter — overkill for this scope.

## R8. Generated code handling

**Decision**: `templ generate` is run after every template change; the generated `*_templ.go` files are committed and never hand-edited (existing project rule). No build step changes.

**Rationale**: Constitution V and the existing Makefile (`make templ`) already define this workflow.

**Alternatives considered**: Generating at build time only — rejected: the project convention is committed generated files.

## R9. CSS and JS scope

**Decision**: Shared components are CSS-first wrappers over the existing Tailwind classes (`btn`, `card`, `badge`, `alert`, `empty-state`, `nav-link`, `quick-card`). No new JavaScript, no Tailwind reconfiguration, and no new CSS files unless a shared component exposes a style that truly has no existing class.

**Rationale**: Constitution VI requires CSS-first and minimal JS; the existing classes already cover the repeated patterns.

**Alternatives considered**:
- Component-specific CSS files — rejected: Tailwind utilities are the established styling approach.
- HTMX-based component behaviors — rejected: not needed for these presentational components.
