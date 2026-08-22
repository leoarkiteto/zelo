<!--
Sync Impact Report
- Version change: unversioned template scaffold → 1.0.0
- Modified principles: n/a (template placeholders [PRINCIPLE_1..5_*] replaced on initial ratification)
- Added sections: Core Principles (I–V), Repository Layout & Conventions,
  Development Workflow & Quality Gates, Governance
- Removed sections: n/a (template placeholder tokens removed)
- Follow-up TODOs: none
-->

# zelo Constitution

## Core Principles

### I. GOTTH Stack

zelo is a condominium management web application. All application functionality
MUST be delivered on the GOTTH stack: Go (server), Templ (templates),
Tailwind CSS (styling), and HTMX (interactivity). The server renders HTML with
progressive enhancement; no client-side SPA framework is permitted. The Go
module is `github.com/leoarkiteto/zelo` and the only binary entrypoint is
`cmd/web`. Rationale: a single, composable stack keeps the codebase small,
fast, and maintainable.

### II. Modular Monolith with Hexagonal Architecture

The application MUST be a single deployable modular monolith. Business logic in
`internal/service` MUST NOT depend on web frameworks, databases, or external
clients; it communicates only through ports (interfaces) implemented by
adapters in `internal/store`, `internal/auth`, and `internal/handler`. Domain
models live in `internal/model` and MUST be shared across layers without
leaking infrastructure concerns. Rationale: the core stays independently
testable and adapters remain swappable.

### III. Vertical Slice Feature Isolation

Each feature MUST be fully self-contained within its own vertical slice,
spanning handler → service → store for that feature only. A slice MUST NOT
reach into another slice's internals; shared functionality MAY only be used
through explicit shared modules (`internal/model`, `internal/middleware`,
`internal/config`). Rationale: self-contained slices keep changes local and
make feature behavior predictable.

### IV. Test-First Development (NON-NEGOTIABLE)

TDD is mandatory for all feature implementations: tests are written first,
approved by the user, seen to fail, and only then is the implementation added.
The red-green-refactor cycle is strictly enforced. Handlers, services, and
store adapters MUST have automated tests; new contracts and contract changes
MUST add integration tests. Rationale: tests written first lock in intent and
prevent untested behavior from shipping.

### V. SOLID, Design Patterns & Generated Code

All code MUST adhere to industry best practice, the SOLID principles, and
established design patterns; complexity MUST be justified. Generated artifacts
(`*_templ.go` from Templ, sqlc queries, compiled Tailwind output) MUST be
produced by the scripts in `scripts/` and never hand-edited. Rationale:
generation sources are the single source of truth for generated output.

## Repository Layout & Conventions

The repository MUST follow the golang-standards/project-layout conventions
documented in `README.md`:

- `cmd/web`: main entrypoint; everything else is in `internal/`.
- `internal/`: private packages — `handler`, `service`, `store`, `model`,
  `auth`, `middleware`, `config`.
- `assets/`: source assets (CSS/JS) compiled into `web/static`.
- `web/templates`: Templ sources (`*.templ`) plus generated `*_templ.go`.
- `migrations/`: SQL migrations, committed and forward-only.
- `scripts/`: build/dev helpers (Tailwind watch, templ generate, migrate).
- `build/`: packaging and CI (Dockerfile, .dockerignore).
- `tools/`: helper tooling pinned via `tools.go`.

Configuration MUST be environment-based, loaded and validated by
`internal/config`, and documented in `.env.example`. OAuth2 flows (Google /
GitHub) live in `internal/auth`; route protection, CSRF, request logging,
panic recovery, and security headers are applied in `internal/middleware`.
Root-level tooling files (`Makefile`, `.air.toml`, `tailwind.config.js`) MUST
be added before feature work begins so every developer uses the same commands.

## Development Workflow & Quality Gates

No feature MAY merge until all of the following hold:

- `go test ./...` passes.
- `templ generate` has been run and the generated `*_templ.go` files are
  committed.
- Tailwind output has been rebuilt and committed (or explicitly gitignored).
- New migrations are committed and forward-only.
- A code review confirms compliance with this constitution: SOLID, TDD,
  vertical-slice isolation, and hexagonal boundaries.

New features MUST be built through the Spec Kit workflow — spec, plan, tasks,
then implementation — with user approval at each gate. Complexity MUST be
justified in the review.

## Governance

This constitution supersedes all other practices where they conflict.
Amendments MUST be documented, approved through PR review, and accompanied by a
migration plan when behavior changes. Versioning follows semantic versioning:
MAJOR for principle removals or redefinitions, MINOR for added or materially
expanded guidance, PATCH for clarifications and wording fixes. Every PR and
review MUST verify compliance; runtime development guidance is captured per
feature in `.specify/memory` (spec, plan, tasks).

**Version**: 1.0.0 | **Ratified**: 2026-08-22 | **Last Amended**: 2026-08-22
