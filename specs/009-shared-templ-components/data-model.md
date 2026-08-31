# Data Model: Shared Templ Components (Atomic Design)

**Feature**: `specs/009-shared-templ-components` | **Date**: 2026-08-31

This feature has no persistent data entities. The "data model" here is the set of component data contracts (props) that the shared Templ components expose. Field names are the Go struct fields used by the Templ components; types are expressed at the design level.

## Dependency direction

```text
organisms  ──▶  molecules  ──▶  atoms
```

- `atoms` MUST NOT import `molecules` or `organisms`.
- `molecules` MUST NOT import `organisms`.
- No shared template package may import `internal/features/**`.

## Atoms

### Button

| Field | Type | Notes |
|-------|------|-------|
| `Label` | localized text | visible button text |
| `Variant` | `ButtonVariant` | `primary`, `secondary`, `danger`, `ghost`, `outline`, `block` |
| `Type` | string | submit/button/link — defaults sensibly |
| `Href` | URL (optional) | renders an anchor when present, otherwise a button |

Renders an element with the existing `btn btn-<variant>` classes.

### Badge

| Field | Type | Notes |
|-------|------|-------|
| `Label` | localized text | visible badge text |
| `Variant` | `BadgeVariant` | `info`, `success`, `warning`, `danger`, `neutral` |

Replaces `RoleBadgeClass`; role call sites pass the role's badge variant instead of a raw class string.

### Icon

| Field | Type | Notes |
|-------|------|-------|
| `Kind` | `IconKind` | named icon set: `building`, `mail`, `users`, `home`, `key`, `info`, `chevron-left`, `arrow-right`, `clipboard`, `tag` |
| `Class` | string (optional) | size/color overrides; defaults to the existing `h-5 w-5` style |

Renders the shared SVG for the given kind. Consolidates `QuickActionIcon` and the inline SVGs currently repeated in feature templates.

### TextInput / TextArea / Select

| Field | Type | Notes |
|-------|------|-------|
| `Name` | string | form field name |
| `Value` | string | current value |
| `Placeholder` | text (optional) | |
| `Required` | bool | |
| `Class` | string (optional) | layout overrides |
| `Options` (Select only) | list of `{Value, Label}` | option list |

Render the existing form-control markup with the same classes.

## Molecules

Molecule components live in `internal/shared/templates/molecules/components.templ`.

### Card

| Field | Type | Notes |
|-------|------|-------|
| `Title` | text (optional) | card heading |
| `Body` | Templ component | body content |
| `Footer` | Templ component (optional) | actions row |

Renders `card card-pad` with consistent header/body/footer structure.

### FormField

| Field | Type | Notes |
|-------|------|-------|
| `Label` | localized text | field label |
| `For` | string | input id/name association |
| `Control` | Templ component | one of the atom form controls |
| `Error` | text (optional) | validation message |
| `Hint` | text (optional) | helper text |

Renders label + control + error/hint in the existing form layout.

### Alert

| Field | Type | Notes |
|-------|------|-------|
| `Variant` | `AlertVariant` | `info`, `success`, `error` |
| `Message` | localized text | alert text |
| `Dismissible` | bool (optional) | |

Renders the existing `alert alert-<variant>` markup.

### EmptyState

| Field | Type | Notes |
|-------|------|-------|
| `Icon` | `IconKind` | leading icon |
| `Title` | localized text | |
| `Copy` | localized text | supporting sentence |
| `Action` | Templ component (optional) | primary action |

Renders the existing `empty-state` block.

## Organisms

### Layout / Shell

Carried over from today's `layout.templ` with no semantic change:

| Type | Fields | Notes |
|------|--------|-------|
| `NavItem` | `Label`, `Path`, `Active` | sidebar navigation entry |
| `UserView` | `Email`, `Roles`, `Initial` | signed-in user shown in topbar |
| `ShellData` | `User`, `Nav`, `CSRF`, `Locale` | app shell data; `nil` renders the page without chrome |
| `Layout(title, locale, shell, body)` | — | top-level HTML document |

### ErrorPage

| Type | Fields | Notes |
|------|--------|-------|
| `ErrorPageData` | `Status`, `Message`, `Locale` | carried over from today's `error.templ` |

## State transitions

None. These are stateless presentational components.
