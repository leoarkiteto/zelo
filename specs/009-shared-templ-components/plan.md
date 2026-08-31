# Implementation Plan: Shared Templ Components (Atomic Design)

**Branch**: `009-shared-templ-components` | **Date**: 2026-08-31 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/009-shared-templ-components/spec.md`, refined by the user request: "Implement the Atomic design to all Templ components to avoid repeatition of the components"

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Reorganize the shared Templ components under `internal/shared/templates` into atomic design layers (`atoms/`, `molecules/`, `organisms/`) so every feature can reuse the same UI pieces instead of repeating markup. The work migrates the existing shared components (layout/shell, error page, role badge helper) into those layers, extracts repeated UI patterns currently copied across feature templates (buttons, badges, icons, cards, alerts, empty states) into shared atoms/molecules, updates feature template imports, and guarantees the rendered pages stay identical. Research decisions: one Go sub-package per atomic layer with a strict dependency direction (organisms → molecules → atoms), CSS-first components wrapping the existing Tailwind classes, per-component render smoke tests, and an automated atomic-boundary check.

## Technical Context

**Language/Version**: Go 1.27 (module `github.com/leoarkiteto/zelo`); Templ v0.3.1020

**Primary Dependencies**: Templ (server-side templates), Tailwind CSS (existing utility classes: `btn`, `card`, `badge`, `alert`, `empty-state`, `nav-link`), HTMX (progressive enhancement, unchanged)

**Storage**: N/A (no persistence changes; UI organization only)

**Testing**: Go standard `testing` package + Templ render smoke tests (`go test ./...`)

**Target Platform**: Server-rendered web application (GOTTH stack)

**Project Type**: Web application — modular monolith (Go + Templ + TailwindCSS + HTMX)

**Performance Goals**: No measurable render regression vs current inline markup; shared components add no extra HTTP requests or client-side overhead

**Constraints**: CSS-first (no JS frameworks, no SPA tooling); generated `*_templ.go` files committed and never hand-edited; feature boundary check must keep passing; shared templates must not import feature code; standard library first

**Scale/Scope**: One shared templates module reorganized into 3 layers; 7 feature template packages updated for the new imports; roughly 10–15 shared components extracted (icons, buttons, badges, form fields, cards, alerts, empty states, layout/shell, error page)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Assessment | Gate |
|-----------|------------|------|
| I. GOTTH Stack | Uses Templ for all components; no SPA/JS framework introduced; Tailwind classes reused | PASS |
| II. Modular Monolith / Hexagonal | Touches only the template layer; business logic, handlers, services, and store unchanged | PASS |
| III. Vertical Slice Feature Isolation | Shared UI lives in `internal/shared/templates`; feature-only templates stay in `internal/features/<feature>/templates`; no feature imports another feature | PASS |
| IV. Test-First Development | Render smoke tests are written first for each shared component and are seen to fail before the component is extracted | PASS |
| V. SOLID, Design Patterns & Generated Code | Atomic design is an established UI pattern; `*_templ.go` produced by `templ generate` and committed, never hand-edited | PASS |
| VI. Standard Library First & CSS-First | No new runtime dependencies; components are wrappers over existing CSS classes; JS remains HTMX-only | PASS |

**Verdict**: No violations. All gates pass; no complexity justification required.

**Post-design re-check (after Phase 1)**: `data-model.md`, `contracts/shared-templ-components.md`, and `quickstart.md` introduce no new runtime dependencies, keep all shared UI in `internal/shared/templates`, preserve feature-owned page templates, and keep generated files committed via the existing `templ generate` workflow. Gates remain PASS.

## Project Structure

### Documentation (this feature)

```text
specs/009-shared-templ-components/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   └── shared-templ-components.md
├── checklists/
│   └── requirements.md  # Spec quality checklist (from /speckit-specify)
└── spec.md              # Feature specification
```

### Source Code (repository root)

```text
internal/shared/templates/
├── atoms/                 # package atoms — smallest reusable UI pieces
│   ├── button.templ       # Button, ButtonVariant
│   ├── badge.templ        # Badge, BadgeVariant
│   ├── icon.templ         # Icon, IconKind (shared SVG icon set)
│   └── input.templ        # TextInput, TextArea, Select (form controls)
├── molecules/             # package molecules — compositions of atoms
│   ├── card.templ         # Card (card container + header/body/footer)
│   ├── form_field.templ   # FormField (label + control + error text)
│   ├── alert.templ        # Alert (variant + icon + message)
│   └── empty_state.templ  # EmptyState (icon + title + copy + action)
├── organisms/             # package organisms — composite page sections
│   ├── layout.templ       # Layout, Shell, ShellData, NavItem, UserView
│   └── error.templ        # ErrorPage, ErrorPageData
├── *_templ.go             # generated by `templ generate`, committed
└── *_test.go              # per-component render smoke tests (colocated)

internal/features/<feature>/templates/
└── *.templ                # feature pages unchanged; imports updated to the new layer packages
```

**Structure Decision**: One Go sub-package per atomic layer inside the existing `internal/shared/templates` module. Features import the layer they need (`.../internal/shared/templates/atoms|molecules|organisms`). Dependency direction is enforced: `organisms` may import `molecules` and `atoms`, `molecules` may import `atoms`, and `atoms` imports nothing above it. Feature-level page templates remain in each feature's `templates/` folder and compose shared components for repeated UI.

## Complexity Tracking

> No constitution violations; no entries required.
