# Discovery — Issue #2 "Dashboard screen"

- Ticket: https://github.com/leoarkiteto/zelo/issues/2
- Date: 2026-11-09
- Method: `/grill-with-docs` was invoked by the user, but its two sub-skills (`grilling`, `domain-modeling`)
  are not installed (`Unknown skill`). The interview was therefore run manually by the Tech Lead
  (two rounds of structured questions). No ADR/glossary files were produced by the skill.
- Status: material decisions APPROVED by the user; minor defaults PROPOSED (confirm at spec gate).
- Destination: move to `specs/<NNN>-dashboard-charts/discovery.md` once `/speckit-specify` creates the feature dir.
  This is discovery input to the spec, not a competing spec.

## Original Acceptance Criteria (verbatim)
1. The screen show charts as circle
2. The charts should show the expenses and incoming values
3. The user can filter the charts by date and type of revenue
4. The user can customize the dashboard, adding, remove, moving the charts in the screen

## Glossary (domain terms as agreed)
- **Revenue (ticket wording)**: in this ticket, all finance movements of the condominium, i.e. both
  **expenses** (`finance_accounts.account_type = 'payable'`) and **income** (`'receivable'`).
- **Type of revenue (AC 3)**: the finance **category** (e.g. `condo_fee`, `maintenance`), not account type or status.
- **Projected amount**: non-canceled entries (pending + overdue + settled) bucketed by `due_date`.
- **Realized amount**: `settled` entries bucketed by `settlement_date`.
- **Chart card**: one donut chart instance on the dashboard; can be added, removed, and moved.
- **Layout**: ordered set of chart cards for a (user, condominium) pair.

## Approved decisions
| # | Topic | Decision | Maps to AC |
|---|-------|----------|-----------|
| D1 | Audience & location | Syndic only, rendered on the existing home dashboard `GET /`. Other roles keep the current home unchanged. | all |
| D2 | Type filter | Filter by finance category. | AC 3 |
| D3 | Date filter | From/to date range; user toggles projected (by due date) vs realized (by settlement date). | AC 3 |
| D4 | Customization | Persisted server-side per user (new table + forward-only migration). Add/remove/move via HTMX buttons (move up/down); no drag-and-drop, no new JS. | AC 4 |
| D5 | Rendering | Server-rendered donuts (SVG/CSS), no JS chart library: constitution VI compliant. | AC 1 |
| D6 | Catalog | 4 chart kinds: Income vs Expenses; Expenses by category; Income by category; Status breakdown (pending/settled/overdue). | AC 1, AC 2 |
| D7 | Shape | Donut with period total in the center + accessible legend (values and %). | AC 1 |
| D8 | Defaults | A user with no saved layout sees all 4 charts. Canceled entries are always excluded (consistent with existing finance totals). | AC 2, AC 4 |
| D9 | Ownership | New vertical slice `internal/features/dashboard/` owns read-only aggregate queries on `finance_accounts`, the layout table, and chart templates. | — |

## Proposed defaults (not yet confirmed; confirm or change at spec review)
- P1: Default date range = current calendar month; default mode = projected.
- P2: Layout is keyed by (user_id, condominium_id), since the active condominium comes from the session.
- P3: Each chart kind appears at most once in a layout (catalog of 4; "add" only offers kinds not on screen).
- P4: Filters live in URL query params (bookmarkable, server-rendered fallback); filters are not persisted.
- P5: Category filter applies across charts; selecting only income categories makes the expense-by-category chart show its empty state (and vice versa). Status breakdown in realized mode shows settled only, so it shows a hint that it is meaningful in projected mode.
- P6: Empty state per chart when there is no data in range; all amounts formatted as BRL from integer cents; labels via existing i18n.
- P7: Filter changes and layout actions use HTMX partial swaps of the chart section; full-page POST/GET works without JS.

## Open for the plan phase (architecture; needs approval at plan gate)
- O1: How `GET /` (home slice) shows the dashboard slice's section without a cross-slice import.
  Candidates: (a) home renders a container with `hx-get="/dashboard/charts"` (plus a server-side
  fallback link); (b) composition root (`cmd/web`) injects a section renderer into home `Deps` via an interface.
- O2: Reading `finance_accounts` from the dashboard slice's own repository: a SQL-level dependency on
  another slice's table. Confirm this is acceptable under constitution III (no Go import), or expose
  aggregates through a shared module.
- O3: Migration `0013_create_dashboard_layouts.sql` shape (e.g. `user_id`, `condominium_id`, `chart_kind`, `position`).
