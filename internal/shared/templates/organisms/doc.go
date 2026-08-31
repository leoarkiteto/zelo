// Package organisms contains composite Templ page sections shared across
// zelo features.
//
// Components in this package:
//   - Layout (the full HTML document)
//   - Shell (the authenticated app shell with topbar and sidebar)
//   - ErrorPage
//
// Layer rules (enforced by scripts/check-template-atomic-boundaries.sh):
// organisms MAY compose molecules and atoms, and MUST NOT import
// internal/features/** code.
package organisms
