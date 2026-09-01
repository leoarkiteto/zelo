# Data Model: Replace Inline SVG Icons

**Feature**: 010-replace-inline-svg-icons
**Date**: 2026-09-01

This feature has **no database persistence**. The data model below documents
the icon catalog and the rendering props that the shared component exposes.

## Entities

### Icon

A named entry in the curated Google Material Symbols icon library available to
the application.

| Field | Type | Description | Rules |
|-------|------|-------------|-------|
| `kind` | `IconKind` | Public identifier used by templates (e.g. `building`, `eye`, `menu`). | Must be one of the defined constants. Unknown values fall back to `info`. |
| `ligature` | `string` | Material Symbols ligature name rendered inside the icon span (e.g. `apartment`, `visibility`). | Must match a glyph in the Material Symbols Outlined font. |
| `meaning` | human label | What the icon communicates to users. | Used only in docs/tests; not rendered. |
| `default_size` | CSS class set | Default size classes when no override is supplied. | Default `h-5 w-5` (20 px). |

### Icon catalog

| kind | ligature | meaning |
|------|----------|---------|
| `building` | `apartment` | Condominium/building |
| `mail` | `mail` | Email/contact |
| `users` | `group` | People/roles |
| `home` | `home` | Dashboard/home |
| `key` | `key` | Credentials/password |
| `info` | `info` | Information and fallback |
| `chevron-left` | `chevron_left` | Breadcrumb separator |
| `arrow-right` | `arrow_forward` | Forward/navigation arrow |
| `clipboard` | `content_paste` | Clipboard/records |
| `tag` | `label` | Tag/category |
| `eye` | `visibility` | Show password |
| `eye-off` | `visibility_off` | Hide password |
| `menu` | `menu` | Mobile navigation menu |
| *(fallback)* | `info` | Fallback for unknown kinds |

### IconProps (rendering contract)

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Kind` | `IconKind` | yes | Which library icon to render. |
| `Class` | `string` | no | Extra CSS classes for size/color overrides; empty means default `h-5 w-5`. |
| `DataIcon` | `string` | no | Optional `data-icon` attribute value used by the password toggle script. |
| `Style` | `string` | no | Optional inline CSS (e.g. initial `display:none` for toggled icons); emitted only when non-empty. |
| `AriaHidden` | `bool` | no | Whether to hide the icon from assistive technology; defaults to true (decorative). |

## Validation Rules

1. `Kind` must be a defined `IconKind` constant. If not, render the fallback
   ligature `info` (FR-008).
2. `Class` is an opaque string of CSS classes and is never validated beyond
   non-empty concatenation.
3. `DataIcon` is emitted only when non-empty; it must match the selector used
   by the password toggle script (`data-icon="eye"` / `data-icon="eye-off"`).
4. `AriaHidden` defaults to true so decorative icons stay hidden; interactive
   callers provide the accessible name on the button/link, not the icon.

## State Transitions

None. Icons are stateless render output; the only stateful behavior (password
visibility toggle) is driven by the existing `data-password-toggle` script and
is unchanged by this data model.
