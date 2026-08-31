# Contract: Shared Templ Components

**Feature**: [spec.md](../spec.md) | **Plan**: [plan.md](../plan.md) | **Date**: 2026-08-31

This is the component API contract for the shared Templ library. It defines the import paths, component signatures, and HTML expectations that all feature templates rely on after this feature is implemented.

## Package layout

```text
internal/shared/templates/
├── atoms/       # package atoms
├── molecules/   # package molecules (components live in components.templ)
└── organisms/   # package organisms
```

Import paths (module `github.com/leoarkiteto/zelo`):

- `internal/shared/templates/atoms`
- `internal/shared/templates/molecules`
- `internal/shared/templates/organisms`

## Layer rules

1. `atoms` MUST NOT import `molecules` or `organisms`.
2. `molecules` MUST NOT import `organisms`; it MAY import `atoms`.
3. `organisms` MAY import `molecules` and `atoms`.
4. No shared template package MAY import `internal/features/**`.
5. Feature templates MAY import any shared template package but MUST NOT import another feature's templates.
6. Generated `*_templ.go` files are committed and MUST NOT be hand-edited.

## Component signatures

Signatures use Go-style notation; the exact field set is specified in [data-model.md](../data-model.md).

```go
// atoms
templ Button(p ButtonProps)
templ Badge(p BadgeProps)
templ Icon(p IconProps)
templ TextInput(p InputProps)
templ TextArea(p InputProps)
templ Select(p SelectProps)

// molecules
templ Card(p CardProps)
templ FormField(p FormFieldProps)
templ Alert(p AlertProps)
templ EmptyState(p EmptyStateProps)

// organisms
templ Layout(title string, locale i18n.Language, shell *ShellData, body templ.Component)
templ Shell(shell *ShellData, body templ.Component)
templ ErrorPage(p ErrorPageData)
```

## HTML expectations

| Component | Expected output (summary) |
|-----------|---------------------------|
| `Button` | `<a class="btn btn-<variant>">` or `<button class="btn btn-<variant>">` |
| `Badge` | `<span class="badge badge-<variant>">label</span>` |
| `Icon` | inline SVG for the named kind, existing `h-* w-*` sizing |
| `Card` | `<section class="card card-pad">` with optional heading/body/footer |
| `FormField` | `<label>` + atom control + optional error/hint text |
| `Alert` | `<div class="alert alert-<variant>">` with message |
| `EmptyState` | `<div class="empty-state">` with icon, title, copy, optional action |
| `Layout` | full HTML document with `<head>` (CSS + HTMX) and body shell |
| `Shell` | `<div id="app-shell" lang="...">` topbar + sidebar + `<main>` body |
| `ErrorPage` | full page with status, message, and dashboard link |

## Naming conventions

- Component files are `kebab-case` of the component: `button.templ`, `form_field.templ`, `empty_state.templ`.
- Component functions are `PascalCase`; props structs are `<Component>Props` (except carried-over `ShellData`, `ErrorPageData`, `NavItem`, `UserView`, which keep their existing names for minimal churn).
- Variant/kind types are `<Component>Variant` or `<Component>Kind` with typed constants.

## Feature migration contract

Existing feature template files update imports as follows, with no rendered-output change:

```text
old: sharedtemplates "github.com/leoarkiteto/zelo/internal/shared/templates"
new: organisms "github.com/leoarkiteto/zelo/internal/shared/templates/organisms"
     atoms     "github.com/leoarkiteto/zelo/internal/shared/templates/atoms"
     molecules "github.com/leoarkiteto/zelo/internal/shared/templates/molecules"
     (only the layers the file actually uses)
```

Call-site equivalents:

- `sharedtemplates.Layout(...)` → `organisms.Layout(...)`
- `sharedtemplates.ShellData` → `organisms.ShellData`
- `sharedtemplates.RoleBadgeClass(role)` → `atoms.Badge(...)` with the role mapped to a `BadgeVariant`
- Repeated `btn`, `card`, `alert`, `empty-state`, inline SVG blocks → the corresponding atom/molecule components

## Boundary checks

- `scripts/check-feature-boundaries.sh` continues to pass and MUST still flag any feature importing another feature's templates.
- A new atomic-layer check MUST fail when an atom imports a molecule/organism or a molecule imports an organism.
