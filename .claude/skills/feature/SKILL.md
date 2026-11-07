---
name: feature
description: Run full Spec-Driven Development for substantial features, architecture changes, or unclear requirements.
---

# Full Feature Workflow

Use only for substantial features, cross-module work, architectural changes, or high uncertainty.

1. Inspect repository and existing product/technical conventions.
2. Invoke Grill With Docs if iterative requirement discovery or ADR/glossary work is useful.
3. Use the installed GitHub Spec Kit skills/commands for specify, clarify when needed, plan, tasks, analyze, implement, and converge.
4. Confirm command names against the installed Spec Kit version; do not invent commands.
5. Preserve approved requirements and AC.
6. Create a dependency-aware task breakdown.
7. Delegate independent tasks to appropriate workers.
8. Avoid parallel edits to overlapping files.
9. Integrate, inspect diffs, run tests, and invoke reviewer.
10. Converge implementation against requirements/spec/plan/tasks.
11. Pause for approval at scope, high-impact architecture/security, destructive, external side-effect, and deployment gates.
12. Report outcomes, tests, AC coverage, risks, and outstanding work.

Do not use this workflow for routine localized bugs or simple UI changes.
