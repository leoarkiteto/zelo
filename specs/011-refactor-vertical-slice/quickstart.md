# Quickstart / Validation Guide: Vertical Slice Architecture Migration

**Feature**: 011-refactor-vertical-slice
**Date**: 2026-09-12

This guide proves the migration works end to end after implementation. It validates structure, build, tests, and the scaffold. It is a validation guide, not an implementation manual — implementation steps live in `tasks.md`.

## Prerequisites

- Go 1.27, `templ`, `make`, and Tailwind tooling installed.
- `.env` configured from `.env.example` (per the repo README).
- Optional for integration tests: `docker compose up -d` (PostgreSQL 16 + Redis 7) and `TEST_DATABASE_URL`/`TEST_REDIS_URL` exported.

## Validation scenarios

### 1. No hexagonal layer remains

```bash
find internal/features -type d \( -name core -o -name ports \) | grep . && echo "VIOLATION" || echo "OK: no core/ or ports/ folders"
find internal/features -path '*/core/ports/*' | grep . && echo "VIOLATION" || echo "OK: no ports files"
```

Expected: `OK` for both.

### 2. Unit tests pass without ports

```bash
go test ./...
```

Expected: all packages pass; no test imports `core/ports`.

### 3. Full quality gates pass

```bash
make check
```

Expected: `Feature boundaries OK.` plus all tests passing. This runs the extended boundary check that rejects `core/` paths and cross-feature imports.

### 4. End-to-end behavior is unchanged (regression gate)

```bash
make run
```

Then exercise each flow manually against the running app: sign-in/registration, role management, service directory, finance, tickets, home dashboard, profile, and language switching. Expected: identical behavior to before the migration.

With docker services up, also run:

```bash
TEST_DATABASE_URL=... TEST_REDIS_URL=... go test ./tests/...
```

Expected: integration tests pass.

### 5. New-feature scaffold produces the vertical-slice layout

```bash
scripts/new-feature.sh demo
find internal/features/demo -maxdepth 2 -type d | sort
rm -rf internal/features/demo   # cleanup after inspection
```

Expected folder list:

```text
internal/features/demo
internal/features/demo/domain
internal/features/demo/handlers
internal/features/demo/repositories
internal/features/demo/services
internal/features/demo/templates
```

Expected: no `core/`, no `ports/`, and the scaffold's printed "Next steps" no longer mention ports.

### 6. Composition root wires features directly

```bash
go build ./cmd/web
```

Expected: build succeeds with concrete store/service dependencies wired in `cmd/web/main.go` and no `core/services` imports.

## References

- Target layout and structural entities: [data-model.md](./data-model.md)
- Decisions and rejected alternatives: [research.md](./research.md)
- Requirements and success criteria: [spec.md](./spec.md)
