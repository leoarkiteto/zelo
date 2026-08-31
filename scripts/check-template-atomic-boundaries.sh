#!/usr/bin/env bash
# check-template-atomic-boundaries.sh
#
# Fails when the shared Templ atomic layers violate their dependency
# direction:
#   - atoms must not import molecules or organisms
#   - molecules must not import organisms
#   - no shared template package may import internal/features/**
#
# Usage: scripts/check-template-atomic-boundaries.sh

set -uo pipefail

fail=0
base="internal/shared/templates"

check_no_import() {
  local file="$1"
  local pattern="$2"
  local rule="$3"
  while IFS= read -r imp; do
    echo "ATOMIC-BOUNDARY VIOLATION in $file: $imp ($rule)"
    fail=1
  done < <(grep -oE "github\.com/leoarkiteto/zelo/${pattern}[^\" ]*" "$file" | tr -d '"' || true)
}

while IFS= read -r f; do
  check_no_import "$f" "internal/shared/templates/molecules" "atoms must not import molecules"
  check_no_import "$f" "internal/shared/templates/organisms" "atoms must not import organisms"
done < <(find "$base/atoms" -name '*.go' -type f 2>/dev/null || true)

while IFS= read -r f; do
  check_no_import "$f" "internal/shared/templates/organisms" "molecules must not import organisms"
done < <(find "$base/molecules" -name '*.go' -type f 2>/dev/null || true)

while IFS= read -r f; do
  while IFS= read -r imp; do
    echo "ATOMIC-BOUNDARY VIOLATION in $f: $imp (shared templates must not import features)"
    fail=1
  done < <(grep -oE "github\.com/leoarkiteto/zelo/internal/features/[^\" ]*" "$f" | tr -d '"' || true)
done < <(find "$base" -name '*.go' -type f 2>/dev/null || true)

if [[ "$fail" -ne 0 ]]; then
  echo "Atomic template boundary violations found." >&2
  exit 1
fi

echo "Atomic template boundaries OK."
