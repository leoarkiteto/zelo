# zelo Design System

The UI follows the visual language of the reference screenshots in `docs/ui`
(condominium client portal). All design decisions below are derived from those
screenshots (verified by OCR + pixel sampling).

## Design tokens

Defined as Tailwind v4 theme tokens (`@theme` in `assets/css/input.css`) and
exposed as Tailwind color scales.

| Token | Hex | Usage |
|---|---|---|
| `chrome-700` | `#3A4B56` | Topbar background (dark slate) |
| `chrome-500` | `#5E6B76` | Avatar background, muted chrome accents |
| `chrome-100` | `#E3E7EA` | Sidebar background |
| `chrome-200` | `#D0D6DA` | Borders, table dividers |
| `canvas` | `#EEEFEF` | Default page background |
| `canvas-blue` | `#F2F9FB` | Blue-tinted canvas (dashboard hero) |
| `steel-500` | `#4090C0` | Primary accent — buttons, links, focus rings |
| `steel-50/100` | `#F0F6FA` / `#DCEAF4` | Info badges, icon bubbles, hover tints |
| `ink-900` | `#0B1B2B` | Headings / primary text (navy) |
| `ink-500` | `#55677A` | Muted / secondary text |

Status colors use the Tailwind defaults: `green` (approved/used/success),
`amber` (pending/in-process), `red` (rejected/revoked/error).

## Layout

- **App shell** — `web/templates/layout.templ`: dark slate topbar (brand, user
  identity, sign out) + light gray sidebar with a "MAIN NAVIGATIONS" group.
  The sidebar is a CSS-only drawer on mobile (`#nav-toggle` checkbox +
  `peer-checked:` — no JavaScript).
- **Auth pages** — `AuthShell` in `web/templates/auth.templ`: centered card
  with brand mark; used by login, register, forgot/reset password.
- **Content area** — centered `max-w-6xl` container; pages compose header +
  cards + tables.

## Components (Tailwind `@layer components` in `assets/css/input.css`)

| Class | Purpose |
|---|---|
| `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost`, `.btn-outline`, `.btn-danger`, `.btn-sm`, `.btn-block` | Buttons; primary = steel `#4090C0` |
| `.card`, `.card-pad` | White rounded cards with subtle border/shadow |
| `.page-title`, `.page-subtitle`, `.page-head` | Page header block (title + optional action) |
| `.label`, `.input`, `.select` | Form controls with steel focus ring |
| `.badge`, `.badge-info`, `.badge-neutral`, `.badge-success`, `.badge-warning`, `.badge-danger` | Status / role pills (leading dot) |
| `.alert`, `.alert-success`, `.alert-error`, `.alert-info` | Flash / error banners |
| `.table-wrap`, `.table` | Data tables: header on `chrome-50`, row hover, dividers |
| `.topbar`, `.sidebar`, `.nav-group`, `.nav-link`, `.nav-link-active` | App shell chrome |
| `.breadcrumb` | Secondary navigation (area pages) |
| `.quick-card`, `.quick-card-icon` | Dashboard quick-action cards |
| `.empty-state`, `.empty-state-title`, `.empty-state-copy` | Empty collections |
| `.stepper`, `.stepper-item`, `.stepper-dot*`, `.stepper-line` | Multi-step progress (reference pattern; ready for wizard flows) |

## Page → reference mapping

| zelo page | Reference |
|---|---|
| Dashboard (`dashboard.templ`) | `ui-ref-4` — greeting + date, hero + CTA, quick-action cards, roles |
| Roles / Invitations (`roles.templ`, `invitations.templ`) | `ui-ref-1` — page header + toolbar, table, status badges, actions |
| Area pages (`area.templ`) | `ui-ref-2` — breadcrumb + title + card content |
| Auth pages | no reference screenshot — kept as focused centered cards in the same palette |

## Tooling

Tailwind is pinned as a dev dependency (`npm install`), matching the version
used to build the committed `web/static/css/output.css`. Rebuild with
`make tailwind`; regenerated `*_templ.go` with `make templ`.
Content sources: Tailwind v4 auto-detects source files (including `.templ`)
from the project root, so no `content` config or `@source` directive is needed.
