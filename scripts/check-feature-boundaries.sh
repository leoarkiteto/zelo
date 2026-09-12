#!/usr/bin/env bash
# check-feature-boundaries.sh
#
# Enforces the vertical-slice architecture:
#
#   1. No cross-feature imports. A file inside one feature folder must not
#      import another feature's internal/features/ path. A feature may import
#      its own sub-packages. Imports of internal/shared/ and cmd/web are always
#      allowed.
#
#   2. No hexagonal layer. A feature folder must not contain a core/ wrapper or
#      a ports/ package once it is migrated. Features still migrating may be
#      listed in scripts/vertical-slice-exempt.txt (one feature name per line,
#      '#' comments allowed); remove a feature from that list as soon as its
#      migration is verified, after which this check fails if core/ or ports/
#      reappears.
#
# Usage: scripts/check-feature-boundaries.sh

set -uo pipefail

fail=0

while IFS= read -r f; do
  feature=$(printf '%s' "$f" | awk -F/ '{print $3}')
  # Each import of the module's internal/features/ tree.
  while IFS= read -r imp; do
    imported=$(printf '%s' "$imp" | sed -E 's#.*/internal/features/([^/]+)/.*#\1#')
    if [[ "$imported" != "$feature" ]]; then
      echo "CROSS-FEATURE IMPORT in $f: $imp"
      fail=1
    fi
  done < <(grep -oE '"github\.com/leoarkiteto/zelo/internal/features/[^"]+"' "$f" | tr -d '"' || true)
done < <(find internal/features -name '*.go' -type f 2>/dev/null || true)

# Enforce the vertical-slice layout: no core/ wrapper, no ports/ package.
exempt_file="scripts/vertical-slice-exempt.txt"
is_exempt() {
  [[ -f "$exempt_file" ]] && grep -qx -- "$1" "$exempt_file" 2>/dev/null
}
while IFS= read -r legacy_dir; do
  [[ -z "$legacy_dir" ]] && continue
  feature=$(printf '%s' "$legacy_dir" | awk -F/ '{print $3}')
  if ! is_exempt "$feature"; then
    echo "HEXAGONAL LAYOUT in $feature: $legacy_dir must be removed (vertical slices have no core/ or ports/ layer)"
    fail=1
  fi
done < <(find internal/features -mindepth 2 -maxdepth 2 -type d \( -name core -o -name ports \) 2>/dev/null || true)

# Enforce the shared Templ atomic layer boundaries too.
if ! scripts/check-template-atomic-boundaries.sh; then
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo "Feature boundary violations found." >&2
  exit 1
fi

echo "Feature boundaries OK."
