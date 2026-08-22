# zelo

GOTTH web app — **Go** + **OAuth** + **Tailwind CSS** + **HTMX** + **Templ**.

Layout follows the [golang-standards/project-layout](https://github.com/golang-standards/project-layout) conventions, extended with feature-first vertical slices (see the project constitution and `specs/004-feature-folder-isolation/contracts/feature-folder-layout.md`).

## Directory layout

```
.
├── cmd/
│   └── web/                # main entrypoint — composition root that wires all features + shared modules
├── internal/               # private application code (not importable from outside this module)
│   ├── features/           # one folder per user-facing feature — fully self-contained vertical slices
│   │   ├── auth/           # registration, login, logout, password reset
│   │   │   ├── core/       # domain, ports (interfaces), services (use cases)
│   │   │   ├── handlers/   # HTTP handlers + RegisterRoutes
│   │   │   └── templates/  # feature-specific templ components (+ *_templ.go)
│   │   ├── directory/      # service provider directory (listings + categories)
│   │   │   ├── core/
│   │   │   ├── handlers/
│   │   │   ├── repositories/   # listings + categories persistence adapters
│   │   │   └── templates/
│   │   ├── management/     # roles + invitations administration
│   │   │   ├── core/
│   │   │   ├── handlers/
│   │   │   └── templates/
│   │   └── home/           # dashboard + area shell
│   │       ├── handlers/
│   │       └── templates/
│   └── shared/             # cross-feature code — never duplicated into features
│       ├── config/         # env-based configuration loading + validation
│       ├── model/          # domain models / DTOs shared across layers
│       ├── middleware/     # request logging, auth guard, CSRF, panic recovery, security headers
│       ├── security/       # password hashing, session manager, CSRF, token primitives
│       ├── store/          # persistence: db open, migrations runner, shared repositories
│       ├── templates/      # shared layout + error templ components (+ *_templ.go)
│       ├── httpx/          # shared HTTP handler helpers (render, current user/session, CSRF form helper)
│       └── testutil/       # shared test database helper
├── assets/                 # source assets, compiled/built into web/static
│   ├── css/                # Tailwind input source (e.g. input.css)
│   └── js/                 # htmx script, custom JS
├── web/                    # web-app-specific components served to the client
│   └── static/             # built assets served by the web server
│       ├── css/            # compiled Tailwind output
│       ├── js/             # bundled htmx / app JS
│       └── img/            # images, favicons
├── migrations/             # SQL migrations (forward-only, applied by a minimal runner)
├── scripts/                # build/dev helpers (Tailwind watch, templ generate, migrate, new-feature)
├── tests/                  # full-app integration tests against PostgreSQL (require TEST_DATABASE_URL)
├── build/                  # packaging & CI — Dockerfile, .dockerignore
├── docs/                   # design docs, ADRs
└── tools/                  # helper tooling (templ, air, ...) pinned in tools.go
```

## GOTTH conventions

- **Go**: server-side app in `cmd/web` (composition root) + `internal/features/*` + `internal/shared/*`; module is `github.com/leoarkiteto/zelo`.
- **OAuth**: auth flows and session/CSRF primitives live in `internal/shared/security`; protect routes via `internal/shared/middleware`.
- **Tailwind**: edit sources in `assets/css`, output the compiled stylesheet to `web/static/css` (keep built artifacts out of git or gitignore them). Tailwind is pinned as an npm devDependency — run `npm install` once, then `make tailwind`.
- **HTMX**: vendored/bundled under `assets/js`, served from `web/static/js`.
- **Templ**: feature templates live in `internal/features/<feature>/templates`; shared layout/error components in `internal/shared/templates`; run `templ generate` and commit the generated `*_templ.go`.
- **Design system**: tokens and reusable components are documented in [`docs/design-system.md`](docs/design-system.md); the UI follows the reference screenshots in `docs/ui`.

## New features

Every feature is a self-contained vertical slice under `internal/features/<feature>/`
following the hexagonal layout in `specs/004-feature-folder-isolation/contracts/feature-folder-layout.md`.
Scaffold a new feature with:

```bash
scripts/new-feature.sh <feature-name>
```

Rules: feature-exclusive code stays in the feature folder; cross-feature code goes to
`internal/shared/`; features never import another feature's internals. Run
`scripts/check-feature-boundaries.sh` to verify, and wire the new feature's
`RegisterRoutes` in `cmd/web/main.go`.
