# Research: Replace Inline SVG Icons

**Feature**: 010-replace-inline-svg-icons
**Date**: 2026-09-01

## Context

The application currently renders every icon as hand-authored inline `<svg/>`
markup: the shared `Icon` component keeps ten hand-drawn SVG paths, and three
feature/layout templates keep additional inline SVGs (password visibility
eye/eye-off, breadcrumb chevron, link arrow, and mobile menu icon). The
planning directive is to replace all inline SVG icons with **Google icons**
(Material Symbols).

## Decisions

### Decision 1: Use Google Material Symbols Outlined as the icon library

- **Decision**: Adopt **Material Symbols Outlined** from Google Fonts.
- **Rationale**: Explicitly selected by the user during planning. The current
  icons are 24 px outline-style glyphs, which is the closest visual family to
  Material Symbols Outlined. Material Symbols is a variable font (weight, fill,
  optical size, grade), licensed under Apache 2.0, and provides ligature names
  that map cleanly onto the existing `IconKind` values.
- **Alternatives considered**:
  - Lucide / Heroicons SVG sets — rejected because the user selected Google
    icons and those sets would keep us in hand-authored SVG management.
  - Keeping inline SVGs — rejected; that is the status quo this feature removes.

### Decision 2: Self-host the variable font instead of using the Google Fonts CDN

- **Decision**: Vendor the `woff2` file at
  `web/static/fonts/material-symbols-outlined.woff2`, define `@font-face` in
  `assets/css/input.css` pointing at `/static/fonts/material-symbols-outlined.woff2`,
  and rebuild `web/static/css/output.css`.
- **Rationale**: Matches the project's vendored-asset pattern (HTMX is vendored
  from `assets/js` and served from `web/static/js`). No runtime third-party
  dependency, works offline, avoids CSP exceptions for an external font host,
  and keeps page loads within our own origin.
- **Alternatives considered**:
  - Google Fonts CDN `<link>` — simpler, but adds an external runtime
    dependency, extra DNS/TLS, and a possible CSP/privacy concern. Rejected.
  - Per-icon SVG files — rejected; it would preserve the SVG-management burden
    the feature removes and does not match the ligature-based font approach.

### Decision 3: Render icons as Material Symbols ligature spans

- **Decision**: The shared `Icon` component renders
  `<span class="material-symbols-outlined {size}" data-icon?="{...}"
  aria-hidden="true">{ligature-name}</span>` instead of `<svg>…</svg>`.
- **Rationale**: This is the standard Material Symbols integration: the font
  turns the ligature name into the glyph. It removes all hand-authored path
  data and keeps a single component as the only icon entry point.
- **Alternatives considered**:
  - Building an SVG sprite from Material Symbols — rejected; more moving parts
    and no benefit over the font ligature approach for this project.

### Decision 4: Map existing `IconKind` values to Material Symbols names

- **Decision**: Extend `IconKind` with the missing kinds (`eye`, `eye-off`,
  `menu`) and add a mapping from kind to Material Symbols ligature name.
- **Mapping** (ligature names use underscores):

| IconKind | Material Symbols name |
|----------|-----------------------|
| building | `apartment` |
| mail | `mail` |
| users | `group` |
| home | `home` |
| key | `key` |
| info | `info` |
| chevron-left | `chevron_left` |
| arrow-right | `arrow_forward` |
| clipboard | `content_paste` |
| tag | `label` |
| eye | `visibility` |
| eye-off | `visibility_off` |
| menu | `menu` |
| unknown/fallback | `info` |

- **Rationale**: Each name is the Material Symbols ligature for the current
  icon's meaning. Exact names are validated against the Material Symbols
  catalogue during implementation.
- **Alternatives considered**: Renaming `IconKind` values to match Material
  Symbols directly — rejected because existing call sites and tests use the
  current names; a mapping keeps the public API stable.

### Decision 5: Preserve the password toggle hooks

- **Decision**: Keep `data-icon="eye"` and `data-icon="eye-off"` attributes on
  the two icon elements inside the password toggle button, now emitted by the
  shared `Icon` component via an extended `IconProps`. The existing
  `data-password-toggle` script keeps working unchanged.
- **Rationale**: The toggle is behavior-sensitive (FR-005, User Story 3); the
  smallest change is to render the same hooks with library icons and leave the
  JS untouched.
- **Alternatives considered**: Rewriting the toggle to a pure HTMX/CSS
  mechanism — rejected as scope creep for this feature.

### Decision 6: Size handling stays class-based

- **Decision**: Keep `IconProps.Class` as the styling hook. The default remains
  `h-5 w-5`; the CSS layer adds `.material-symbols-outlined` font-size rules so
  the existing Tailwind size classes (`h-3.5`, `h-4`, `h-5`, `h-6`) also size
  the glyph.
- **Rationale**: Minimizes call-site churn and preserves FR-007 (size/styling
  overrides) while matching Material Symbols' 24 px base size.
- **Alternatives considered**: A dedicated `Size` enum on `IconProps` — cleaner
  API but requires changing every call site; rejected for this migration.

## Open Questions

None. All Technical Context unknowns were resolved by the user's directive
("use the Google icons as icon library") and the decisions above.
