#!/usr/bin/env bash
# new-feature.sh <feature-name>
#
# Scaffolds a new feature folder under internal/features/ from the canonical
# template in scripts/_feature-template/ (see contracts/feature-folder-layout.md).
#
# Usage:
#   scripts/new-feature.sh <feature-name>

set -euo pipefail

feature="${1:-}"
if [[ -z "$feature" ]]; then
  echo "usage: scripts/new-feature.sh <feature-name>" >&2
  exit 1
fi

target="internal/features/$feature"
if [[ -e "$target" ]]; then
  echo "error: $target already exists" >&2
  exit 1
fi

mkdir -p "$target"
cp -R scripts/_feature-template/. "$target/"
find "$target" -type f -print0 | xargs -0 perl -pi -e "s/{{FEATURE}}/$feature/g"

echo "Scaffolded feature at $target"
echo
echo "Next steps:"
echo "  1. Add domain types under $target/core/domain/"
echo "  2. Declare ports in $target/core/ports/{repositories,services}.go"
echo "  3. Implement use cases in $target/core/services/"
echo "  4. Add handlers + RegisterRoutes in $target/handlers/"
echo "  5. Add feature-exclusive persistence in $target/repositories/ (if needed)"
echo "  6. Add Templ views in $target/templates/ (if needed) and run 'templ generate'"
echo "  7. Wire the feature in cmd/web/main.go and run scripts/check-feature-boundaries.sh"
