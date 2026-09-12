# Feature Template

This directory is the canonical scaffold for a new zelo feature. Copy it with:

```bash
scripts/new-feature.sh <feature-name>
```

The script creates `internal/features/<feature-name>/` and replaces the
`{{FEATURE}}` placeholder in comments. Folder and package names follow the
vertical-slice layout (`specs/004-feature-folder-isolation/contracts/feature-folder-layout.md`):

```text
internal/features/<feature>/
├── domain/          # feature-local domain types only (shared types stay in internal/shared/model)
├── services/        # use cases; plain structs with concrete dependencies, no ports layer
├── handlers/        # HTTP handlers + Deps + RegisterRoutes
├── repositories/    # feature-exclusive persistence (optional)
└── templates/       # feature-specific Templ views (optional)
```

Rules:

- Feature-exclusive code lives inside the feature folder.
- Cross-feature code moves to `internal/shared/` and is never duplicated.
- A feature never imports another feature's internals.
- There is no `core/` wrapper and no `ports/` layer: handlers wire concrete
  services and shared collaborators; services take their dependencies as plain
  fields (concrete types, or function values where a test needs a seam).
- Run `scripts/check-feature-boundaries.sh` after wiring the feature in `cmd/web/main.go`.
