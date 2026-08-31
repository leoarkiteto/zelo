// Package molecules contains Templ components composed from atoms and
// shared across zelo features.
//
// Components in this package:
//   - Card
//   - FormField
//   - Alert
//   - EmptyState
//
// Layer rules (enforced by scripts/check-template-atomic-boundaries.sh):
// molecules MAY import atoms, but MUST NOT import organisms.
package molecules
