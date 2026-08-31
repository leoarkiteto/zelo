# Quickstart: Shared Templ Components (Atomic Design)

**Feature**: `specs/009-shared-templ-components` | **Date**: 2026-08-31

This guide lists the runnable validation scenarios that prove the atomic design reorganization works end-to-end. It references [data-model.md](./data-model.md) and [contracts/shared-templ-components.md](./contracts/shared-templ-components.md) instead of repeating them.

## Prerequisites

- Go toolchain (the project targets Go 1.27)
- `templ` CLI available (generates `*_templ.go`)
- Node/npx for the Tailwind build (only if CSS classes change; not expected for this feature)

## Validation scenarios

### 1. Generate templates and build

```bash
make templ
go build ./...
```

**Expected outcome**: `templ generate` regenerates all `*_templ.go` files under `internal/shared/templates/{atoms,molecules,organisms}` and the project compiles with no errors.

### 2. Run the full test suite

```bash
go test ./...
```

**Expected outcome**: all tests pass, including the new per-component render smoke tests in `internal/shared/templates/*` and the existing feature tests that render pages through the migrated templates.

### 3. Verify the atomic layer boundaries

```bash
scripts/check-feature-boundaries.sh
```

**Expected outcome**: `Feature boundaries OK.` — no feature imports another feature's templates, and the atomic-layer check reports no upward imports (`atoms` → `molecules/organisms`, `molecules` → `organisms`).

### 4. Verify shared component coverage

```bash
go test ./internal/shared/templates/...
```

**Expected outcome**: every shared component (Button, Badge, Icon, TextInput, TextArea, Select, Card, FormField, Alert, EmptyState, Layout/Shell, ErrorPage) has a render smoke test that passes and asserts its key output per the contract.

### 5. Verify pages render unchanged (manual smoke)

```bash
make run
```

Then load the main pages in a browser:

- `/` (dashboard)
- `/login` (auth page without shell)
- `/directory` (cards, badges, forms, buttons)
- `/tickets` (lists, empty states, buttons)
- `/finance` (cards, buttons, badges)
- A non-existent route (shared error page)

**Expected outcome**: every page looks identical to the current application; only the internal organization changed. Buttons, badges, cards, alerts, and icons keep the same styling and behavior.

### 6. Verify no duplicated shared markup remains

Review the feature templates and grep for the patterns that were extracted:

```bash
grep -rn "class=\"btn btn-" internal/features/*/templates/
grep -rn "class=\"card card-pad\"" internal/features/*/templates/
grep -rn "<svg" internal/features/*/templates/
```

**Expected outcome**: the grep hits shrink to the intended feature-specific usages; repeated shared markup now comes from the shared components via imports, not local copies.

## Definition of done for this feature

- [ ] `make templ` runs clean and generated files are committed
- [ ] `go test ./...` passes
- [ ] `scripts/check-feature-boundaries.sh` passes
- [ ] Atomic layers match [contracts/shared-templ-components.md](./contracts/shared-templ-components.md)
- [ ] Manual smoke shows no visual differences on the pages listed above
