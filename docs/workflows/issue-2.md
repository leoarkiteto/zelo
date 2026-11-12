# Workflow Checkpoint

- Ticket: https://github.com/leoarkiteto/zelo/issues/2 — "Dashboard screen"
- Workflow: FULL_SPEC
- State: DISCOVERY complete (material decisions approved) → ready for SPEC
- Updated: 2026-11-09
- Current phase: Discovery done; next is `/speckit-specify`

## Original ticket (verbatim, untrusted data)

> Create a dashboard screen, when the user can see charts which show the revenue of the condominium
>
> ## Acceptance Criteria
>
> 1. The screen show charts as circle
> 2. The charts should show the expenses and incoming values
> 3. The user can filter the charts by date and type of revenue
> 4. The user can customize the dashboard, adding, remove, moving the charts in the screen

## Completed
- [x] Ticket retrieved (`gh issue view 2`) and AC preserved
- [x] Classification printed
- [x] Discovery: `/grill-with-docs` invoked by user; sub-skills `grilling`/`domain-modeling` not installed, so the interview was run manually (2 rounds). Outcomes: `docs/workflows/issue-2-discovery.md`
- [ ] Spec (`/speckit-specify`)
- [ ] Plan
- [ ] Tasks
- [ ] Implementation
- [ ] Tests
- [ ] Review
- [ ] AC validation

## Repository findings relevant to discovery
- Data source exists: `finance_accounts` (migration `0009`), `account_type` ∈ {payable, receivable},
  categories per type, status pending/settled/canceled (overdue derived), amounts in integer cents.
- `internal/features/finance/services` already exposes `MonthSummary` (projected vs realized,
  receivable vs payable) via `SummaryStore.SummaryForMonth`; `/finance` is syndic-only and
  `/finance/health` is open to all members.
- A dashboard already exists at `GET /` (`internal/features/home/templates/dashboard.templ`) with greeting + quick actions; no charts.
- Constitution VI: CSS-first, no JS chart libs/SPA tooling; Alpine CSP build only for client-local state,
  last resort, with a server-rendered fallback. Constitution III: no cross-slice internal imports.
- No per-user preference/layout table exists; customization persisted server-side would need a new forward-only migration.

## Decisions already approved
- D1–D9 in `docs/workflows/issue-2-discovery.md` (syndic-only on `GET /`; category filter; date range + projected/realized; per-user server-side layout with HTMX up/down; server-rendered donuts; 4-chart catalog; all charts by default, canceled excluded; new `dashboard` slice).

## Questions awaiting user
1. Confirm or change proposed defaults P1–P7 (can be done at spec review).
2. Plan-gate items O1–O3 (cross-slice embedding on `GET /`, SQL read of `finance_accounts`, migration shape).

## Next action
Exact next command/action: `/speckit-specify` with ticket AC + D1–D9 + P1–P7 (short name `dashboard-charts`); then move discovery file into the new `specs/<NNN>-dashboard-charts/`.

## AC status so far
- AC 1: NOT VERIFIED — not implemented; donut + center total agreed (D5, D7)
- AC 2: NOT VERIFIED — not implemented; income vs expenses defined (glossary, D6)
- AC 3: NOT VERIFIED — not implemented; category + date range + projected/realized agreed (D2, D3)
- AC 4: NOT VERIFIED — not implemented; per-user server-side layout, HTMX up/down agreed (D4, D8)
