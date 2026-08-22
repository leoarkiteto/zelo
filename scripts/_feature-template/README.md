# Feature Template

This directory is the canonical scaffold for a new zelo feature. Copy it with:

```bash
scripts/new-feature.sh <feature-name>
```

The script creates `internal/features/<feature-name>/` and replaces the
`{{FEATURE}}` placeholder in comments. Folder and package names follow
`contracts/feature-folder-layout.md`:

```text
internal/features/<feature>/
├── core/
│   ├── domain/        # feature-specific domain types and rules
│   ├── ports/         # interfaces the core needs (repositories.go, services.go)
│   └── services/      # application use cases
├── handlers/          # driving adapters: HTTP handlers + RegisterRoutes
├── repositories/      # driven adapters: feature-exclusive persistence (optional)
└── templates/         # feature-specific Templ views (optional)
```

Rules:

- Feature-exclusive code lives inside the feature folder.
- Cross-feature code moves to `internal/shared/` and is never duplicated.
- A feature never imports another feature's internals.
- `core/ports` interfaces are consumer-owned: declare only what this feature needs.
- Run `scripts/check-feature-boundaries.sh` after wiring the feature in `cmd/web/main.go`.
