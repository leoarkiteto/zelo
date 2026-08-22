# Data Model: Feature Folder Isolation

**Feature**: [spec.md](./spec.md) | **Date**: 2026-08-22

This feature does not change database entities. Its "data" is the organization of the codebase itself, modeled below so the reorganization has the same rigor as a data-bearing feature.

## Organizational Entities

### Feature

Represents a user-facing capability that owns a complete vertical slice.

- **Attributes**: name (kebab-case, e.g. `auth`, `directory`), folder path (`internal/features/<name>/`), route prefix (e.g. `/directory`), tests (colocated with the code they cover).
- **Relationships**: depends only on `Shared Module` interfaces and its own `core/`; never on another `Feature`'s internals.
- **Validation rules** (from spec):
  - FR-001: exactly one dedicated folder per feature.
  - FR-002: all code used exclusively by the feature lives inside that folder.
  - FR-004: no imports/references into another feature's folder.

### Feature Folder

The canonical shape every feature follows (the contract in `contracts/feature-folder-layout.md`).

- **Attributes**: `core/domain/`, `core/ports/`, `core/services/`, `handlers/`, `repositories/`, `templates/` (repositories/templates optional when the feature has none).
- **Relationships**: one `Feature Folder` per `Feature`.
- **Validation rules**: FR-005 (new features start from the template), FR-007 (single naming convention), FR-008 (tests colocated).

### Port

An interface owned by a feature's core, defining what the core needs from the outside world.

- **Attributes**: kind (`repository` or `service`), file (`core/ports/repositories.go` or `core/ports/services.go`), consumer (the feature core that declares it).
- **Relationships**: declared by a `Feature`; implemented by `Shared Module` adapters or by the feature's own `repositories/`.
- **Validation rules**: ports are local to their feature (no shared port package); implementation is satisfied structurally.

### Adapter

A concrete implementation or driver at the edge of a feature.

- **Kinds**: `handlers/` (driving adapters — HTTP), `repositories/` (driven adapters — persistence), `templates/` (presentation).
- **Relationships**: a feature's adapters may implement that feature's ports or shared ports; shared adapters live in `internal/shared/`.
- **Validation rules**: FR-002 (feature-exclusive adapters stay in the feature), FR-003 (multi-feature adapters stay in shared and are not duplicated).

### Shared Module

Cross-cutting code used by more than one feature.

- **Instances**: `config`, `model`, `middleware`, `security`, `store`, `templates`, `httpx`, `testutil`.
- **Attributes**: path (`internal/shared/<module>/`), consumers (the features/middleware that use it).
- **Relationships**: may be imported by any `Feature`; must not import a `Feature`.
- **Validation rules**: FR-003 (single copy, no duplication inside features).

### Composition Root

The wiring point that assembles the application.

- **Attributes**: `cmd/web/main.go` (and `seed.go`).
- **Relationships**: imports all feature `handlers/` packages and shared modules; registers each feature's routes and applies shared middleware.
- **Validation rules**: the only place allowed to know about all features at once.

## Current → Target Package Map

| Current package | Target location | Notes |
|---|---|---|
| `internal/config` | `internal/shared/config` | moved as-is |
| `internal/model` | `internal/shared/model` | shared data definitions |
| `internal/middleware` | `internal/shared/middleware` | moved as-is |
| `internal/auth` (password, session, csrf) | `internal/shared/security` | shared security primitives |
| `internal/store/{db,migrate,errors}` | `internal/shared/store` | shared persistence infrastructure |
| `internal/store/{users,roles,invitations,sessions,audit,units}` | `internal/shared/store` | multi-feature persistence adapters |
| `internal/store/listings.go` (ListingStore + CategoryStore) | `internal/features/directory/repositories` | directory-exclusive adapters |
| `internal/service/ports.go` | split into each feature's `core/ports/` | consumer-owned ports |
| `internal/service/{registration,auth,password_reset}.go` | `internal/features/auth/core/services` | auth use cases |
| `internal/service/directory.go` | `internal/features/directory/core/services` | directory use cases |
| `internal/service/roles.go` | `internal/features/management/core/services` | management use cases |
| `internal/handler/{login,register,logout,password}.go` | `internal/features/auth/handlers` | auth routes |
| `internal/handler/directory.go` | `internal/features/directory/handlers` | directory routes |
| `internal/handler/{roles,invitations}.go` | `internal/features/management/handlers` | management routes |
| `internal/handler/{home,areas,shell}.go` | `internal/features/home/handlers` | dashboard shell |
| `internal/handler/{deps,helpers,router}.go` | composition root (`cmd/web`) + per-feature `handlers` | wiring split |
| `internal/testutil` | `internal/shared/testutil` | shared test helper |
| `web/templates` (feature-specific) | `internal/features/<feature>/templates` | per-feature views |
| `web/templates` (layout, error) | `internal/shared/templates` | shared view components |

## State transitions

Not applicable — this is a structural refactor. The only "transition" is the migration of each current package to its target folder, which must happen with zero behavior change.
