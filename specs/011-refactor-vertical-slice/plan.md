# Implementation Plan: Vertical Slice Architecture Migration

**Branch**: `011-refactor-vertical-slice` | **Date**: 2026-09-12 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/011-refactor-vertical-slice/spec.md`

**Note**: This file is the `/speckit-plan` output. `tasks.md` is created later by `/speckit-tasks`.

## Summary

Refactor the whole zelo codebase from the current "hexagonal + vertical slice" hybrid (every feature has a `core/{domain,ports,services}` wrapper with consumer-owned interfaces) to a pure Vertical Slice Architecture: each feature keeps role-based sub-folders (`handlers/`, `services/`, `repositories/`, `domain/`, `templates/`) directly under `internal/features/<feature>/`, with no `core/` wrapper and no `ports/` layer. Interfaces are removed as much as possible: services depend on concrete stores, tests use hand-written slice-level doubles, and the composition root (`cmd/web/main.go`) keeps manual per-feature wiring. The reference repo `sebajax/go-vertical-slice-architecture` informs the feature-first organization and one-use-case-per-file style, but its Fiber framework, Uber `dig` DI container, `port.go` interfaces, and `mockery` mocks are explicitly rejected because they violate the zelo constitution (stdlib-first, CSS-first, no new frameworks) and the user's directive to remove interfaces as much as possible.

## Technical Context

**Language/Version**: Go 1.27

**Primary Dependencies**: Go standard library (`net/http`, `database/sql`), Templ, Tailwind CSS v4, HTMX v4, PostgreSQL driver, Redis client, `golang.org/x/crypto` (Argon2id). No new dependencies.

**Storage**: PostgreSQL 16 + Redis 7 (existing, unchanged)

**Testing**: `go test ./...` (colocated unit tests) + `tests/integration` (full-app flows against real PostgreSQL/Redis, skipped unless `TEST_DATABASE_URL`/`TEST_REDIS_URL` are set)

**Target Platform**: Linux server (single binary); local development on darwin/arm64

**Project Type**: Server-rendered web application (GOTTH stack)

**Performance Goals**: No change from current baseline; user-facing latency and throughput must stay equivalent to pre-migration behavior.

**Constraints**: No new frameworks, ORMs, DI containers, or mock generators; stdlib-first; CSS-first (no new JS beyond HTMX); no cross-feature imports; TDD non-negotiable; generated assets (`*_templ.go`, `output.css`) regenerated via scripts only; config via env only.

**Scale/Scope**: 7 feature slices (`auth`, `directory`, `finance`, `home`, `management`, `profile`, `tickets`), 9 shared modules, 12 `core/ports/*.go` files to delete, 10 `core/services/*.go` files to move to `services/`, 5 `repositories/` packages to keep/rewire, 7 `handlers/` + `templates/` packages to update, plus `scripts/new-feature.sh`, `scripts/_feature-template`, `scripts/check-feature-boundaries.sh`, `cmd/web/main.go`, and architecture docs (constitution, REASONIX.md, README, spec 004 contract).

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. GOTTH Stack | PASS | No stack change. |
| II. Modular Monolith with Hexagonal Architecture | JUSTIFIED VIOLATION | This feature exists to amend Principle II/III: remove the hexagonal ports/adapters mandate and keep only vertical slices. Amendment is part of FR-011 and must go through PR review per Governance. See Complexity Tracking. |
| III. Vertical Slice Feature Isolation | PASS (strengthened) | The refactor tightens this principle; no cross-feature imports remain enforced by `scripts/check-feature-boundaries.sh`. |
| IV. Test-First Development | PASS | Tests move with code and are kept green; new/changed contracts get tests before implementation. |
| V. SOLID, Design Patterns & Generated Code | PASS | No hand-editing of generated files; generation scripts updated, then `templ generate`/`make tailwind` run. |
| VI. Standard Library First & CSS-First | PASS | Fiber, Uber `dig`, and `mockery` from the reference repo are rejected; no new dependencies. |
| Feature wiring | PASS | `cmd/web/main.go` rewired to per-feature `Deps` using concrete stores/services. |

**Gate result**: PASS with one justified violation (constitution amendment), documented in Complexity Tracking.

**Post-Phase-1 re-check**: PASS. `research.md` confirms no new dependencies (Fiber/dig/mockery rejected), no new interfaces (ports deleted, concrete wiring), and the layout in `data-model.md` matches the clarified spec. No new constitution violations introduced by the design.

## Project Structure

### Documentation (this feature)

```text
specs/011-refactor-vertical-slice/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── checklists/
│   └── requirements.md  # Spec quality checklist
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

`contracts/` is intentionally **not** created: this is a purely internal code-organization refactor with no new external interface (no new API/CLI/UI contract). The target folder layout is documented in `data-model.md` and enforced by the updated `scripts/check-feature-boundaries.sh`.

### Source Code (repository root)

Target layout after migration:

```text
internal/features/<feature>/
├── domain/          # feature-local domain types only (shared types stay in internal/shared/model)
├── services/        # use cases; plain structs with concrete dependencies, no interfaces
├── repositories/    # feature-exclusive persistence (concrete stores)
├── handlers/        # HTTP handlers + Deps + RegisterRoutes
└── templates/       # feature-exclusive Templ views

internal/shared/     # unchanged: config, httpx, i18n, middleware, model, security, store, templates, testutil

scripts/
├── new-feature.sh                  # updated to scaffold the layout above
├── _feature-template/              # updated template (no core/, no ports/)
├── check-feature-boundaries.sh     # updated to also reject core/ and ports/ packages
└── check-template-atomic-boundaries.sh  # unchanged
```

**Structure Decision**: Keep the clarified role-based sub-folders directly under each feature root (no `core/` wrapper, no `ports/`). Shared domain entities stay in `internal/shared/model`; feature-local types live in the feature's `domain/`. The reference repo's `handler/` + `service/` + `infrastructure/` organization is adopted in spirit, but zelo keeps its existing folder names (`handlers/`, `repositories/`, `templates/`) to minimize churn and stay consistent with the project's specs and scripts.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Removing the hexagonal ports/adapters layer conflicts with current constitution Principle II until it is amended | The entire purpose of this feature is to remove the `core/ports` indirection that adds complexity for a solo developer | Keeping the ports layer alongside vertical slices (hybrid) is exactly the complexity the user asked to remove |
| Reference repo practices (Fiber, Uber `dig`, `mockery`, generated mocks) are not adopted | They conflict with constitution Principle VI (stdlib-first, no frameworks/DI containers) and the user's "remove interfaces as much as possible" directive | Adopting `dig`/`mockery` would add dependencies and interface ceremony, not remove it |
