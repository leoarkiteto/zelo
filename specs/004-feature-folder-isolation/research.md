# Research: Feature Folder Isolation

**Feature**: [spec.md](./spec.md) | **Date**: 2026-08-22

This document resolves the unknowns from the plan's Technical Context before the Phase 1 design artifacts are produced.

## 1. Screenshot interpretation

The user attached a screenshot of a suggested hexagonal folder structure (`.reasonix/attachments/clipboard-20260822-225428.126388-000001.png`). OCR and coordinate reconstruction yield the following tree:

```text
cmd/
pkg/
internal/
├── core/
│   ├── domain/
│   │   ├── game.go
│   │   └── board.go
│   ├── ports/
│   │   ├── repositories.go
│   │   └── services.go
│   └── services/
│       └── gamesrv/
│           └── service.go
├── handlers/
└── repositories/
```

The suggestion is a hexagonal (ports & adapters) slice: a `core/` package holding `domain/`, `ports/`, and `services/`, with `handlers/` and `repositories/` as the surrounding adapters. Applied to zelo's requirement that each feature live in its own folder, each feature becomes:

```text
internal/features/<feature>/
├── core/
│   ├── domain/
│   ├── ports/
│   └── services/
├── handlers/
├── repositories/
└── templates/          # added for zelo's GOTTH stack (see R6)
```

`cmd/` and `pkg/` from the screenshot are a generic Go project convention; zelo already uses `cmd/web` and keeps everything else private under `internal/`, so no root-level `pkg/` is introduced.

## 2. Decisions

### R1 — Target per-feature folder structure

- **Decision**: Use the screenshot's hexagonal shape per feature: `core/{domain,ports,services}`, `handlers/`, `repositories/`, plus `templates/` for Templ views.
- **Rationale**: It matches the user's screenshot, the constitution's Principle II (hexagonal ports & adapters) and Principle III (vertical slice), and spec FR-002 (all feature-exclusive code in one folder).
- **Alternatives considered**: Keep `handler/service/store` naming inside each feature (less conceptual churn, but does not follow the screenshot and is less explicit about ports); nest the whole feature under `internal/<feature>/` directly without a `features/` parent (weaker grouping, harder to distinguish features from shared code).

### R2 — Feature inventory and file mapping

- **Decision**: Split the codebase into four features — `auth`, `directory`, `management`, `home` — plus shared modules.
- **Rationale**: These are the user-facing capabilities visible in the current routes and templates; each can be migrated and tested independently (spec US1–US4).
- **Alternatives considered**: One feature per existing `handler/*.go` file (too fine-grained, would fragment shared stores); a single `features/` folder with everything (recreates the maintenance problem).

| Feature | Current handlers | Current services | Current stores | Current templates | New location |
|---|---|---|---|---|---|
| `auth` | `login.go`, `register.go`, `logout.go`, `password.go` | `auth.go`, `registration.go`, `password_reset.go` | uses shared `users`, `invitations` | `auth.templ`, `login.templ`, `register.templ`, `forgot_password.templ`, `reset_password.templ` | `internal/features/auth/` |
| `directory` | `directory.go` | `directory.go` | `listings.go` (ListingStore + CategoryStore), uses shared `units`, `audit` | `directory.templ` | `internal/features/directory/` |
| `management` | `roles.go`, `invitations.go` | `roles.go` | uses shared `roles`, `invitations`, `units`, `audit` | `roles.templ`, `invitations.templ` | `internal/features/management/` |
| `home` | `home.go`, `areas.go`, `shell.go` | — | uses shared `roles`, `audit` | `dashboard.templ`, `area.templ` | `internal/features/home/` |

### R3 — Where shared modules live

- **Decision**: Move cross-feature code to `internal/shared/` (`config`, `model`, `middleware`, `security`, `store`, `templates`, `httpx`, `testutil`).
- **Rationale**: Spec FR-003 requires a clearly designated shared location; a single `shared/` bucket makes the feature/shared split obvious and reviewable (anything outside `features/` or `shared/` is misplaced).
- **Alternatives considered**: Keep shared modules at `internal/` root (`internal/model`, `internal/middleware`, …) — less churn but the boundary is less explicit; put shared code in `pkg/` — leaks internals outside `internal/`, contrary to the constitution.

### R4 — Where shared persistence adapters live

- **Decision**: Persistence adapters for tables used by more than one feature (`users`, `roles`, `invitations`, `sessions`, `audit`, `units`) live in `internal/shared/store/`. Each feature declares its own narrow port interfaces in `core/ports/`; the shared adapters satisfy them structurally.
- **Rationale**: Spec FR-003 (shared code is not duplicated) and FR-004 (features don't reference each other) rule out putting these adapters in any one feature. Go's implicit interfaces make this clean: the consumer owns the port.
- **Alternatives considered**: Duplicate a repository per feature (violates FR-003); have features import another feature's repository (violates FR-004); keep one central `ports.go` (works, but centralizes ports outside the features that need them — less self-contained).

### R5 — Route registration without cross-feature references

- **Decision**: Each feature's `handlers/` package exposes `RegisterRoutes(mux *http.ServeMux, deps <Feature>Deps)`; `cmd/web/main.go` acts as the composition root and registers all features, applying shared middleware around the final mux.
- **Rationale**: Features stay independent (no feature imports another), wiring is explicit in one place, and middleware stays shared (spec US4).
- **Alternatives considered**: Keep one central `router.go` with all routes (keeps feature boundaries weak); let each feature wrap its own middleware (duplicates middleware composition).

### R6 — Where templates live

- **Decision**: Feature-specific Templ files move into `internal/features/<feature>/templates/` with their generated `*_templ.go`; shared layout/error components move to `internal/shared/templates/`.
- **Rationale**: Spec FR-002 says a feature folder must contain all code used exclusively by that feature, including its entry points; in the GOTTH stack, templates are part of the feature's presentation. `templ generate` scans the module tree, so generation works regardless of directory.
- **Alternatives considered**: Keep templates in `web/templates/<feature>/` (less churn, but presentation stays outside the feature folder); keep one flat `web/templates/` package (weakest isolation). Shared layout/error components stay shared because every feature uses them (FR-003).

### R7 — Migration strategy

- **Decision**: Incremental migration in two waves. Wave 1 (foundational): move shared modules first — `model` → `config` → `auth`(→`security`) → `middleware` → `testutil` → `store` → `web/templates`(→`shared/templates`) → handler helpers (→`httpx`) → `TokenHasher`. Wave 2 (features): pilot `home`, then `directory`, `management`, and `auth` in parallel, then rewrite `cmd/web/main.go` as the composition root and delete the old `handler/` and `service/` packages. Use `git mv` for history, rewrite import paths, run `templ generate`, and require `go test ./...` + `go build ./...` to pass after each task.
- **Rationale**: Shared modules must be at their final paths before feature folders import them, avoiding a second global import rewrite. The suite stays green at every step (constitution quality gates), and each feature is independently verifiable (spec US1/US2).
- **Alternatives considered**: Big-bang move of all files at once (faster in raw edits but produces a long broken window and hard-to-review diff); feature-first with shared cleanup last (each feature would temporarily import old shared paths and need a second import rewrite pass).

### R8 — Naming conventions

- **Decision**: Feature folders use kebab-case matching the feature name (`auth`, `directory`, `management`, `home`); Go package names stay lowercase and match their directory; port files are named `repositories.go` and `services.go` per the screenshot; service implementation folders use a `<feature>srv`-style subfolder only when multiple implementations exist.
- **Rationale**: Matches the screenshot's naming and keeps folders greppable and predictable (spec FR-007).
- **Alternatives considered**: snake_case or PascalCase folder names (not idiomatic for Go directories).

## 3. Consolidated outcome

- No `NEEDS CLARIFICATION` items remain.
- Target layout is `internal/features/<feature>/` + `internal/shared/`.
- Existing behavior (routes, schema, migrations) is preserved by contract — see `contracts/http-routes.md`.
- The canonical feature folder contract is documented in `contracts/feature-folder-layout.md`.
