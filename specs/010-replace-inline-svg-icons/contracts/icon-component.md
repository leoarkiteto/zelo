# Contract: Shared Icon Component

**Feature**: 010-replace-inline-svg-icons
**Audience**: Feature template authors and reviewers
**Source**: `internal/shared/templates/atoms` (Templ component `Icon`)

## Purpose

The shared `Icon` component is the **only** supported way for feature templates
to render an icon. It encapsulates the Google Material Symbols library so no
template embeds hand-authored icon artwork.

## Component Signature

```text
Icon(props IconProps)
```

### IconProps

| Field | Type | Required | Default | Meaning |
|-------|------|----------|---------|---------|
| `Kind` | `IconKind` | yes | — | Which library icon to render (see catalog). |
| `Class` | `string` | no | `h-5 w-5` | Additional CSS classes for size/color overrides. |
| `DataIcon` | `string` | no | empty | Optional `data-icon` attribute value (password toggle hook). |
| `Style` | `string` | no | empty | Optional inline CSS (e.g. initial `display:none` for toggled icons); emitted only when non-empty. |
| `AriaHidden` | `bool` | no | `true` | Hides the icon from assistive technology when decorative. |

### IconKind catalog

`building`, `mail`, `users`, `home`, `key`, `info`, `chevron-left`,
`arrow-right`, `clipboard`, `tag`, `eye`, `eye-off`, `menu`.

Unknown values MUST render the fallback ligature `info` (FR-008).

## Rendered Output Contract

For `Icon({Kind: "eye", Class: "h-5 w-5", DataIcon: "eye", AriaHidden: true})`:

```html
<span class="material-symbols-outlined h-5 w-5" data-icon="eye" aria-hidden="true">visibility</span>
```

Rules:

1. The element is a `span`, never an `svg`.
2. The class list starts with `material-symbols-outlined`, followed by the
   resolved `Class` value.
3. `data-icon` is present only when `DataIcon` is non-empty.
4. `style` is present only when `Style` is non-empty.
5. `aria-hidden` reflects `AriaHidden` (default `true`).
6. The element's text content is the Material Symbols ligature name from the
   catalog mapping (see [data-model.md](../data-model.md)).

## Required Static Assets

The following MUST exist for icons to render:

| Asset | Path | Notes |
|-------|------|-------|
| Font face + icon class | `assets/css/input.css` | Defines `@font-face` for `Material Symbols Outlined` and the `.material-symbols-outlined` rules. |
| Vendored & served font | `web/static/fonts/material-symbols-outlined.woff2` | Apache 2.0; committed; served at `/static/fonts/material-symbols-outlined.woff2`. |
| Built stylesheet | `web/static/css/output.css` | Produced by `make tailwind`; committed. |

The font MUST be served from the application's own origin; no external CDN.

## Accessibility Contract

- **Decorative icons** (default): `aria-hidden="true"` and no focusable
  behavior.
- **Interactive icons** (e.g. password toggle, menu button): the accessible
  name and state live on the enclosing `button`/`link` (`aria-label`,
  `aria-pressed`); the icon span remains `aria-hidden="true"`.
- No icon may introduce text content that is announced by screen readers
  unless the caller opts out of hiding (future need only).

## Behavior Contract

- The password visibility toggle MUST continue to switch `visibility` /
  `visibility_off` icons and update `aria-label` / `aria-pressed` exactly as
  the current page does.
- Changing a catalog ligature in the shared component updates every usage;
  feature templates MUST NOT override icon artwork locally.

## Validation

- `go test ./internal/shared/templates/...` covers the rendered span, class,
  and fallback behavior.
- `rg '<svg' internal --glob '*.templ'` must return no production template
  matches after migration.
