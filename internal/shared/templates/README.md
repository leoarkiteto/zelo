# Shared Templ Components

Shared Templ UI library for zelo, organized by atomic design layers. All
components are server-rendered, CSS-first wrappers over the existing Tailwind
classes.

## Layers

| Layer | Package | Contents |
|-------|---------|----------|
| atoms | `internal/shared/templates/atoms` | Smallest reusable UI pieces: `Button`, `Badge`, `Icon`, `TextInput`, `TextArea`, `Select` |
| molecules | `internal/shared/templates/molecules` | Compositions of atoms in `components.templ`: `Card`, `FormField`, `Alert`, `EmptyState` |
| organisms | `internal/shared/templates/organisms` | Composite page sections: `Layout`, `Shell`, `ErrorPage` |

## Import rules

1. `atoms` MUST NOT import `molecules` or `organisms`.
2. `molecules` MUST NOT import `organisms`; it MAY import `atoms`.
3. `organisms` MAY compose `molecules` and `atoms`.
4. No shared template package MAY import `internal/features/**`.
5. Feature templates MAY import any shared template layer, but MUST NOT import
   another feature's templates.
6. Generated `*_templ.go` files are committed and MUST NOT be hand-edited.

Enforcement: `scripts/check-template-atomic-boundaries.sh` (also wired into
`scripts/check-feature-boundaries.sh`).

## Component inventory

See `specs/009-shared-templ-components/data-model.md` for the props of each
component and `specs/009-shared-templ-components/contracts/shared-templ-components.md`
for signatures and HTML expectations.

## Adding a new shared component

1. Decide the atomic layer: standalone UI piece → `atoms`; composition of
   atoms → `molecules`; page section → `organisms`.
2. Create the `.templ` file in that layer and export a `PascalCase` component
   plus a `<Component>Props` struct.
3. Write a failing render smoke test in that layer's `*_test.go`.
4. Implement the component using the existing Tailwind classes.
5. Run `make templ`, then `go test ./...`.
6. Run `scripts/check-feature-boundaries.sh` before committing.
