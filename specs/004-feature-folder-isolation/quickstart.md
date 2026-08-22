# Quickstart: Feature Folder Isolation Validation

**Feature**: [spec.md](./spec.md) | **Date**: 2026-08-22

This guide proves the reorganization works end-to-end without changing behavior. It is a validation walkthrough — implementation details belong in `tasks.md` and the implementation phase.

## Prerequisites

- Go 1.27+ (`go version`)
- PostgreSQL running; `DATABASE_URL` set in `.env` (see `.env.example`)
- `TEST_DATABASE_URL` optionally set for integration tests (tests skip when unset)
- Templ CLI available (`templ generate` via `make templ`)
- Node.js only if the Tailwind CSS output must be rebuilt (not required for this refactor)

## Validation scenarios

### 1. Baseline before migration

```bash
go test ./...
go build ./...
```

**Expected**: all tests pass (integration tests may skip if `TEST_DATABASE_URL` is unset); build succeeds with no output.

### 2. After each feature migration

After moving the shared modules and then each feature (`home` first, then `directory`, `management`, and `auth`), run:

```bash
go test ./...
go build ./...
make templ
git status
```

**Expected**: tests pass after every feature; `templ generate` produces no uncommitted drift in `*_templ.go`; only the moved files appear in `git status`.

### 3. Structure check

```bash
find internal -maxdepth 3 -type d | sort
```

**Expected**: `internal/features/{auth,directory,management,home}` and `internal/shared/{config,model,middleware,security,store,templates,testutil}` exist; the old layer folders (`internal/handler`, `internal/service`, `internal/store`, `internal/auth`) are gone.

```bash
grep -rn "internal/features" internal/features --include='*.go' | grep -v "_test.go"
```

**Expected**: no feature file imports another feature folder (imports of `internal/features/` from `cmd/web` are expected and allowed as the composition root).

### 4. Route smoke test

Start the app and exercise the public surface documented in `contracts/http-routes.md`:

```bash
make run
# in another shell:
curl -i http://localhost:8080/login
curl -i http://localhost:8080/register
curl -i http://localhost:8080/            # should redirect to /login when unauthenticated
```

With a seeded development database (`make migrate` then `go run ./cmd/web -seed`):

1. Sign in as `syndic@example.com` / `syndic-password-123`.
2. Open `/directory` — the directory page renders with its search/filter controls.
3. Open `/roles` and `/invitations` — management pages render.
4. Open `/` — the dashboard renders with the area shell.

**Expected**: every route returns the same status/redirect/page as before the refactor; no 500s; templates render with the same layout.

### 5. Full test suite + generated code gate

```bash
make test
make build
make templ
git diff --exit-code -- '*_templ.go'
```

**Expected**: all tests pass; `bin/web` builds; generated template files are committed and unchanged by a fresh generation.

## Definition of done

- Steps 1–5 pass.
- Code review confirms each feature folder matches `contracts/feature-folder-layout.md` and no cross-feature imports exist (spec FR-004).
- `README.md` and the constitution's "Repository Layout & Conventions" section describe the new layout (spec FR-009).
