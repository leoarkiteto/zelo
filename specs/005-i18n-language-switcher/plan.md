# Implementation Plan: i18n Language Switcher

**Branch**: `005-i18n-language-switcher` | **Date**: 2026-08-23 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/005-i18n-language-switcher/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Primary requirement: a signed-in user can switch the application interface between English (`en`) and Brazilian Portuguese (`pt-br`) using a toggle on a new profile page. Each language option shows its country flag. The choice is saved to the user account and applied automatically at login, with English as the default language.

Technical approach (from Phase 0 research): add a nullable `language_preference` column to `users`, introduce a shared `internal/shared/i18n` translation catalog resolved per request, and create a new `internal/features/profile` vertical slice exposing `GET /profile` and `POST /profile/language`. The toggle uses an HTMX POST that swaps the authenticated shell so the whole UI updates in one round-trip without losing page state.

## Technical Context

**Language/Version**: Go 1.27

**Primary Dependencies**: Templ v0.3.1020, pgx/v5, golang.org/x/crypto; TailwindCSS 3.4 and HTMX (static asset) on the front end

**Storage**: PostgreSQL via `database/sql` — `users` table gains a `language_preference` column

**Testing**: `go test ./...`, handler tests with `net/http/httptest`, Templ smoke render tests

**Target Platform**: Linux server (single `cmd/web` binary)

**Project Type**: Web application (server-rendered GOTTH stack)

**Performance Goals**: One request round-trip for a language toggle; authenticated page renders stay under 200ms p95

**Constraints**: Go standard library first (no web framework, no ORM, no i18n framework); CSS-first with HTMX-only JavaScript; vertical-slice isolation enforced by `scripts/check-feature-boundaries.sh`

**Scale/Scope**: 2 locales (`en`, `pt-br`), 1 new feature slice (`profile`), 1 new shared module (`i18n`), 1 forward-only migration

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

1. **GOTTH stack** — PASS: server-rendered Templ pages, HTMX progressive enhancement for the toggle, no SPA.
2. **Modular monolith with hexagonal architecture** — PASS: new `internal/features/profile` slice with `core/{domain,ports,services}`; business logic depends only on ports, implemented by adapters in `internal/shared/store`.
3. **Vertical slice feature isolation** — PASS: profile feature is self-contained; the translation catalog is a new explicit shared module `internal/shared/i18n`, not another feature's internals.
4. **Test-first development (NON-NEGOTIABLE)** — PASS with mandate: tests for domain, service, handler, store, and shared i18n must be written first and seen failing before implementation.
5. **SOLID, design patterns & generated code** — PASS: consumer-owned ports, small focused services, Templ output generated via `scripts/templ-generate.sh`.
6. **Standard library first & CSS-first** — PASS: no new runtime dependencies; flags use Unicode emoji; interaction uses HTMX only.

**Post-design re-check**: PASS — see [research.md](./research.md) for the decisions behind each gate.

## Project Structure

### Documentation (this feature)

```text
specs/005-i18n-language-switcher/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

```text
cmd/web/main.go                            # wire profile deps, routes, WithLocale middleware
internal/features/profile/                 # NEW vertical slice (scripts/new-feature.sh profile)
├── core/domain/language.go                # Language value object (en, pt-br; default en)
├── core/ports/repositories.go             # LanguagePreferenceReader/Updater ports
├── core/ports/services.go                 # Profile service port
├── core/services/profile.go               # GetProfile / ChangeLanguage use cases
├── handlers/deps.go                       # handler dependencies
├── handlers/routes.go                     # GET /profile, POST /profile/language
├── handlers/profile.go                    # page + toggle handlers
└── templates/profile.templ                # profile page with language toggle
internal/shared/i18n/                      # NEW shared translation module
├── language.go                            # Language constants + validation
├── catalog.go                             # en/pt-br message catalog
└── context.go                             # per-request locale context helpers
internal/shared/middleware/locale.go       # NEW WithLocale (runs after WithUser)
internal/shared/templates/layout.templ     # render <html lang> + #app-shell lang from locale
internal/shared/model/user.go              # add LanguagePreference field
internal/shared/store/users.go             # read/write language_preference
migrations/0008_add_user_language_preference.sql
```

**Structure Decision**: single Go module using the existing feature-slice convention. `profile` is scaffolded with `scripts/new-feature.sh profile`; the translation catalog is a shared module under `internal/shared/i18n` because every slice's templates consume it, which is the constitution's explicit "shared functionality" path.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations — no entries required.
