# Phase 1 Data Model: Vertical Slice Architecture Migration

**Feature**: 011-refactor-vertical-slice
**Date**: 2026-09-12

This refactor changes code organization, not application data. No database tables, columns, or user-facing entities change. The "entities" below are the structural units the migration operates on.

## Entity 1: Feature Slice

- **Represents**: one self-contained feature under `internal/features/<feature>/`.
- **Attributes**: `name` (auth, directory, finance, home, management, profile, tickets); sub-folders `domain/`, `services/`, `repositories/`, `handlers/`, `templates/`; `RegisterRoutes` handler; `Deps` struct.
- **Rules**:
  - Must not contain `core/` or `core/ports/`.
  - Must not import another feature's `internal/features/<other>/` packages (enforced by `scripts/check-feature-boundaries.sh`).
  - Services must depend on concrete types, not interfaces.
- **State transitions**:
  - `hexagonal` (has `core/{domain,ports,services}`) → `migrating` (old paths still compile while being moved) → `migrated` (new layout only, structure check passes).
  - A slice may only be `migrating` within one task; the build and tests must be green when the task ends.

## Entity 2: Shared Module

- **Represents**: cross-feature code under `internal/shared/` (`config`, `httpx`, `i18n`, `middleware`, `model`, `security`, `store`, `templates`, `testutil`).
- **Attributes**: `name`, allowed importers (any feature, never another feature).
- **Rules**:
  - Cross-feature domain entities (`User`, `RoleAssignment`, `Invitation`, `Condominium`, etc.) stay in `internal/shared/model`.
  - Cross-cutting concerns (sessions, middleware, i18n, shared templates) are not duplicated into features.
  - Shared modules do not import `internal/features/**` (template-boundary script continues to enforce this for `internal/shared/templates`).

## Entity 3: Feature-local Domain Type

- **Represents**: a type used by exactly one feature (e.g., a query/filter struct or an enum that no other feature needs).
- **Attributes**: owning feature, location `internal/features/<feature>/domain/`.
- **Rules**: may only live in the feature's `domain/` when no other feature uses it; otherwise it belongs in `internal/shared/model`. No duplication of a shared type inside a feature.

## Entity 4: Architecture Boundary Check

- **Represents**: the automated structure gate (`scripts/check-feature-boundaries.sh`, plus `scripts/check-template-atomic-boundaries.sh`).
- **Attributes**: `pass`/`fail`; violations list.
- **Rules**:
  - Rejects cross-feature imports.
  - After this feature, also rejects any `internal/features/<feature>/core/` path (the removed hexagonal layer).
  - Runs as part of `make check`.

## Entity 5: Feature Scaffold Template

- **Represents**: `scripts/_feature-template/` + `scripts/new-feature.sh`.
- **Attributes**: generated sub-folders (`domain/`, `services/`, `repositories/`, `handlers/`, `templates/`), placeholder replacement.
- **Rules**:
  - Must not generate `core/` or `core/ports/`.
  - Must instruct the developer to wire `RegisterRoutes` in `cmd/web/main.go` and run the boundary check.

## Relationship Summary

```text
Feature Slice ──imports──▶ Shared Module (allowed)
Feature Slice ──imports──▶ another Feature Slice (forbidden)
Feature Slice ──contains──▶ Feature-local Domain Type (only if exclusive)
Architecture Boundary Check ──validates──▶ Feature Slice + Shared Module
Feature Scaffold Template ──generates──▶ Feature Slice (new layout)
```
