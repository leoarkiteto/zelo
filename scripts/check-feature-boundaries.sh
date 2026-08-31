#!/usr/bin/env bash
# check-feature-boundaries.sh
#
# Fails when a file inside one feature folder imports another feature's
# internal/features/ path. A feature may import its own internal/features/
# sub-packages (e.g. core/ports from core/services). Imports of
# internal/shared/ and cmd/web are outside this check and always allowed.
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

# Enforce the shared Templ atomic layer boundaries too.
if ! scripts/check-template-atomic-boundaries.sh; then
  fail=1
fi

if [[ "$fail" -ne 0 ]]; then
  echo "Feature boundary violations found." >&2
  exit 1
fi

echo "Feature boundaries OK."
