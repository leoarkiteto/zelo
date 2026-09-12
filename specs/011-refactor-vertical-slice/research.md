# Phase 0 Research: Vertical Slice Architecture Migration

**Feature**: 011-refactor-vertical-slice
**Date**: 2026-09-12

## Research task

Apply the Go vertical-slice organization principles from `sebajax/go-vertical-slice-architecture` to zelo, while respecting the zelo constitution and the user directive "remova interfaces o máximo possível" (remove interfaces as much as possible).

## Decision 1: Adopt feature-first folder organization, adapted to zelo naming

- **Decision**: Each feature keeps role-based sub-folders directly under `internal/features/<feature>/`: `domain/`, `services/`, `repositories/`, `handlers/`, `templates/`. The `core/` wrapper and `core/ports/` layer are deleted.
- **Rationale**: The reference repo organizes all code for a domain under one folder (`internal/product/` with `handler/`, `service/`, `infrastructure/`). zelo already isolates features under `internal/features/<feature>/`; the refactor flattens the `core/` wrapper and renames nothing else, which minimizes churn while achieving the same "one folder per feature" outcome. The clarified spec (`handlers/`, `services/`, `repositories/`, `domain/`, `templates/`) is authoritative.
- **Alternatives considered**:
  - Rename `repositories/` → `infrastructure/` to mirror the reference repo: rejected — `repositories/` is already used across zelo specs/scripts and renaming adds noise without value.
  - Put `product.go` domain files at the feature root like the reference repo: rejected — zelo clarified `domain/` as the home for feature-local types.

## Decision 2: Remove interfaces; concrete dependencies only

- **Decision**: Delete all `core/ports/*.go` interface declarations. Services become plain structs with concrete store fields (`*store.UserStore`, `*repositories.ListingStore`, etc.), constructed and wired manually in `cmd/web/main.go`. No service interfaces, no repository interfaces, no DI container.
- **Rationale**: The user explicitly asked to remove interfaces as much as possible. zelo is a modular monolith with one composition root and a solo developer; the swappability that interfaces provide is not currently needed, and hand-written fakes already test the services in their own package. The reference repo keeps `port.go` + `mockery`, but that is exactly the ceremony being removed.
- **Alternatives considered**:
  - Keep a single `port.go` per feature for DI, as in the reference repo: rejected — violates the user directive and FR-002.
  - Use Uber `dig` for DI: rejected — violates constitution Principle VI (no DI containers) and adds a dependency.
  - Use `mockery` generated mocks: rejected — interface-based and adds tooling; hand-written test doubles are the established zelo pattern.

## Decision 2a: Test seams are function-typed fields, not interfaces

- **Decision**: When a feature service or handler needs a test double, express the narrow collaborator as a function-typed field (a method value in production wiring, a closure in tests) instead of declaring an interface. Concrete store/service types are used everywhere else.
- **Rationale**: This keeps the "no interfaces" directive while preserving fast, external-service-free unit tests (`go test ./...` must pass without PostgreSQL/Redis).
- **Alternatives considered**: keeping a minimal consumer-owned interface for the seam (rejected — the user asked to remove interfaces as much as possible); requiring real stores in unit tests (rejected — unit tests must not need infrastructure).

## Decision 3: One-file-per-use-case style (adopted where it aids navigation, not mandatory)

- **Decision**: Follow the reference repo's `service/createProduct.go` style by splitting large service files into one file per use case when a feature has multiple independent use cases (e.g., auth registration/login/password-reset already follow this). Existing cohesive service files may stay as-is if splitting adds no clarity.
- **Rationale**: The reference repo shows that one file per use case makes a vertical slice scannable; zelo already does this in `auth` and `finance`. Making it a universal hard rule would force churn with no user-visible benefit.
- **Alternatives considered**: none material.

## Decision 4: Hand-written slice-level test doubles (no generated mocks)

- **Decision**: Unit tests for services keep using hand-written fakes/in-memory stores colocated in the test files, exactly as the current `auth` service tests do. When tests move from `core/services/` to `services/`, the fakes move with them.
- **Rationale**: This is the existing zelo pattern, needs no new tooling, and works without interfaces.
- **Alternatives considered**: `mockery` (rejected, see Decision 2).

## Decision 5: Composition root stays manual

- **Decision**: `cmd/web/main.go` keeps constructing shared stores/services and per-feature `Deps`; only the import paths and struct field types change (concrete types instead of interface fields, no `core/services` path).
- **Rationale**: One composition root for a modular monolith is simple and grep-able; a DI container is unnecessary.
- **Alternatives considered**: per-feature `NewDeps` constructors (allowed, but not required; tasks may use them where they reduce main.go noise).

## Decision 6: Architecture checks are extended, not replaced

- **Decision**: Extend `scripts/check-feature-boundaries.sh` to fail on any `internal/features/<feature>/core/` or `internal/features/<feature>/core/ports/` path, and to continue rejecting cross-feature imports. Update `scripts/_feature-template` and `scripts/new-feature.sh` to scaffold the new layout.
- **Rationale**: SC-001 requires an automated structure check; the existing boundary script is the natural home for it.
- **Alternatives considered**: a separate `check-vertical-slice.sh` (rejected — one check script is simpler to run and maintain).

## Decision 7: Constitution amendment is part of this feature

- **Decision**: Amend the constitution's Principle II/III (and any repository-layout guidance) to describe vertical slices only, removing the hexagonal ports/adapters mandate. The amendment is authored during implementation and approved via PR review, per the constitution's Governance section.
- **Rationale**: FR-011 and US3 explicitly require documentation to reflect the new architecture; leaving the constitution unchanged would leave contradictory guidance.
- **Alternatives considered**: deferring the constitution change to a later feature (rejected — the architecture checks and docs would be inconsistent in the meantime).
