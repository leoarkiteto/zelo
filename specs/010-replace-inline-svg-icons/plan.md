# Implementation Plan: Replace Inline SVG Icons

**Branch**: `010-replace-inline-svg-icons` | **Date**: 2026-09-01 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/010-replace-inline-svg-icons/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Replace all hand-authored inline `<svg/>` icon artwork in page templates with the Google Material Symbols icon library, rendered through the existing shared `Icon` component in `internal/shared/templates/atoms`. The Material Symbols variable font is self-hosted with the application's static assets, the shared icon component switches from hand-drawn SVG paths to Material Symbols ligatures, and the feature templates (login password toggle, breadcrumbs/links, mobile menu) migrate to the shared component. Existing behavior — especially the password visibility toggle — and accessibility labels are preserved.

## Technical Context

**Language/Version**: Go 1.27

**Primary Dependencies**: Templ (server-rendered components), Tailwind CSS v4 (styling), HTMX v4 (unchanged), Google Material Symbols Outlined (self-hosted variable font, Apache 2.0)

**Storage**: N/A — no database changes. One vendored font file added under `assets/fonts/` and served from `web/static/fonts/`.

**Testing**: `go test ./...` (unit + Templ render tests), `make check` (tests + boundary checks), `tests/integration/` full-app flows when `TEST_DATABASE_URL` and `TEST_REDIS_URL` are set.

**Target Platform**: Server-rendered GOTTH web application, modern browsers.

**Project Type**: Web application (modular monolith).

**Performance Goals**: No perceptible icon rendering regression; icon font is a single static asset loaded via normal CSS, cached by the browser.

**Constraints**: CSS-first (no client-side JS for icon rendering); no cross-feature imports; generated `*_templ.go` and compiled Tailwind output committed; assets vendored under `assets/` and served from `web/static/`.

**Scale/Scope**: 14 icon kinds (13 existing + fallback), 1 shared component, 4 template files touched, 1 static font + CSS integration.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. GOTTH Stack | PASS | Icons render server-side via Templ; appearance via CSS font; HTMX behavior unchanged. |
| II. Modular Monolith / Hexagonal | PASS | Change is confined to the shared templates module and two feature templates; no business-logic change. |
| III. Vertical Slice Feature Isolation | PASS | Features import `internal/shared/templates/atoms` (explicitly allowed shared module); no cross-feature imports. |
| IV. Test-First Development | PASS | Icon render tests and password-toggle integration assertions are updated/added before implementation. |
| V. SOLID & Generated Code | PASS | Single-responsibility `Icon` component; `templ generate` and `make tailwind` regenerate committed artifacts. |
| VI. Standard Library First & CSS-First | PASS | No Go framework/ORM added. The Material Symbols font is a front-end asset, not a JS framework; CSS-first is preserved. The asset dependency is user-mandated and recorded below. |

## Project Structure

### Documentation (this feature)

```text
specs/010-replace-inline-svg-icons/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   └── icon-component.md
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
internal/shared/templates/
├── atoms/
│   ├── types.go             # IconKind + IconProps (mapping to Material Symbols)
│   ├── icon.templ           # Icon component (SVG internals replaced by span)
│   └── atoms_test.go        # Render assertions updated
└── organisms/
    └── layout.templ         # Mobile menu icon migrates to atoms.Icon

internal/features/
├── auth/templates/login.templ    # Eye/eye-off toggle migrates to atoms.Icon
└── home/templates/area.templ     # Breadcrumb + link arrows migrate to atoms.Icon

assets/
└── css/input.css                # @font-face + .material-symbols-outlined rules

web/static/
├── css/output.css               # Rebuilt by make tailwind
└── fonts/
    └── material-symbols-outlined.woff2   # Vendored icon font, served at /static/fonts/
```

**Structure Decision**: Single GOTTH web application. Icon library code stays in the shared atoms package; the font asset follows the existing vendored-asset pattern (`assets/` → `web/static/`). Feature templates only change their icon call sites.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | — | — |

The Material Symbols font is a new front-end asset dependency. It does not violate a constitution principle (it is CSS-first and not a JS framework), and it is user-mandated: the planning directive explicitly selects Google icons. It is vendored like the existing HTMX asset.
