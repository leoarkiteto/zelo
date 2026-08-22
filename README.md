# zelo

GOTTH web app — **Go** + **OAuth** + **Tailwind CSS** + **HTMX** + **Templ**.

Layout follows the [golang-standards/project-layout](https://github.com/golang-standards/project-layout) conventions.

## Directory layout

```
.
├── cmd/
│   └── web/                # main entrypoint — `go run ./cmd/web`, builds the binary `web`
├── internal/               # private application code (not importable from outside this module)
│   ├── auth/               # OAuth2 flows (Google/GitHub), session & token handling, user identity
│   ├── config/             # env-based configuration loading + validation
│   ├── handler/            # HTTP handlers — parse request, call service, render templ component
│   ├── middleware/         # request logging, auth guard, CSRF, panic recovery, security headers
│   ├── model/              # domain models / DTOs shared across layers
│   ├── service/            # business logic / use cases — the orchestration layer
│   └── store/              # persistence layer — SQLC queries, repositories, DB access
├── assets/                 # source assets, compiled/built into web/static
│   ├── css/                # Tailwind input source (e.g. input.css)
│   └── js/                 # htmx script, custom JS
├── web/                    # web-app-specific components served to the client
│   ├── static/             # built assets served by the web server
│   │   ├── css/            # compiled Tailwind output
│   │   ├── js/             # bundled htmx / app JS
│   │   └── img/            # images, favicons
│   └── templates/          # templ templates (.templ) + generated *_templ.go
├── migrations/             # SQL migrations (e.g. goose/golang-migrate)
├── scripts/                # build/dev helpers (Tailwind watch, templ generate, migrate)
├── test/                   # external integration/e2e test harnesses (optional)
├── build/                  # packaging & CI — Dockerfile, .dockerignore
├── docs/                   # design docs, ADRs
└── tools/                  # helper tooling (templ, sqlc, air, ...) pinned in tools.go
```

## GOTTH conventions

- **Go**: server-side app in `cmd/web` + `internal/*`; module is `github.com/leoarkiteto/zelo`.
- **OAuth**: auth flows live in `internal/auth`; protect routes via `internal/middleware`.
- **Tailwind**: edit sources in `assets/css`, output the compiled stylesheet to `web/static/css` (keep built artifacts out of git or gitignore them). Tailwind is pinned as an npm devDependency — run `npm install` once, then `make tailwind`.
- **HTMX**: vendored/bundled under `assets/js`, served from `web/static/js`.
- **Templ**: `*.templ` files in `web/templates`; run `templ generate` and commit the generated `*_templ.go`.
- **Design system**: tokens and reusable components are documented in [`docs/design-system.md`](docs/design-system.md); the UI follows the reference screenshots in `docs/ui`.

## Root files to add next

- `Makefile` (targets: `run`, `build`, `dev`, `migrate`, `templ`, `tailwind`)
- `.air.toml` — hot-reload dev server
- `.env.example` — config template read by `internal/config`
- `tailwind.config.js` — content globs incl. `web/templates/**/*.templ`
