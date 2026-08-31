// Package atoms contains the smallest reusable Templ UI pieces shared
// across zelo features.
//
// Components in this package:
//   - Button
//   - Badge
//   - Icon
//   - TextInput, TextArea, Select
//
// Layer rules (enforced by scripts/check-template-atomic-boundaries.sh):
// atoms MUST NOT import molecules or organisms. Atoms wrap the existing
// Tailwind CSS utility classes and are styled CSS-first.
package atoms
