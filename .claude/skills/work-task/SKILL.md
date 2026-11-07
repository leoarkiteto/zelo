---
name: work-task
description: Process a sprint ticket from Linear, GitHub Issues/Projects, or pasted ticket content. Classify and execute Quick, Standard, or Full Spec workflow with selective approval.
---

# Work Task — Sprint Ticket Workflow

## Input
Accept a ticket URL, ID, or pasted ticket. Identify source as Linear, GitHub, or manual.
If only URL/ID is supplied and no authorized connector/CLI can retrieve it, ask the user to paste title, description, and AC. Do not pretend access.

## Step 1 — Normalize
Create a private working summary with:
- ticket ID/source/title
- description
- original Acceptance Criteria
- labels/priority if available
- constraints and links
Preserve original AC verbatim where possible. Do not edit the source ticket.

## Step 2 — Inspect repository
Inspect relevant files and project conventions. Determine languages/frameworks, test commands, and likely scope. Do not run unsafe commands.

## Step 3 — Classify
Choose one:
- QUICK: small localized low-risk fix, clear AC, little dependency.
- STANDARD: multiple components/files, integration, moderate uncertainty, or meaningful tests.
- FULL_SPEC: new capability, cross-module change, significant architectural impact, high uncertainty, or requirements needing discovery.

Explain classification briefly. Honor explicit user override.

## QUICK
1. Make a concise implementation plan (few bullets).
2. Implement directly in current Claude Code session.
3. Run relevant local checks.
4. Validate each AC.
5. Review diff for scope creep and regressions.
6. Report.

Do not invoke Spec Kit, Grill With Docs, or subagents unless genuinely needed.

## STANDARD
1. Write a short plan and dependency/file map.
2. If requirements are ambiguous, ask one focused question or use Grill With Docs for substantial discovery.
3. Delegate implementation to the best-fit worker when useful.
4. Use parallel workers only for independent tasks with non-overlapping files.
5. Ask workers to report changes, tests, assumptions, and blockers.
6. Integrate and inspect diffs.
7. Invoke reviewer for meaningful changes.
8. Run tests and validate every AC.
9. Report.

## FULL_SPEC
1. Use Grill With Docs only when the request needs iterative discovery, domain decisions, ADRs, or glossary.
2. Use the installed Spec Kit commands/skills for specify, clarify (if needed), plan, tasks, analyze, implement, and converge as available in the installed version.
3. Do not assume exact slash-command names if they are not present; inspect available commands/skills and use the installed version.
4. Obtain approval at product/architecture gates if decisions materially affect scope, security, or cost.
5. Delegate independent tasks to specialized workers.
6. Review integration and run checks.
7. Converge implementation against spec, plan, tasks, and AC.
8. Report.

## Selective approval
Proceed for routine local work. Pause before:
- changing AC or product scope;
- high-impact architecture/security choices;
- destructive changes or risky migrations;
- production or external side effects;
- commit, push, PR, merge, release, deployment.

## Required AC validation
For each original AC, report PASS / FAIL / NOT VERIFIED and evidence. No unsupported PASS claims.

## Final report format
- Ticket
- Workflow + why
- Summary
- Files changed
- AC validation table
- Tests run (exact commands/results)
- Review findings
- Risks/blockers
- Suggested commit message
