#!/usr/bin/env bash
# Fail if any production Templ template still contains hand-authored inline
# <svg> markup. Icons must be rendered through atoms.Icon (Material Symbols).
set -euo pipefail

matches="$(rg '<svg' internal --glob '*.templ' || true)"
if [ -n "$matches" ]; then
  echo "Inline <svg> found in templates - use atoms.Icon instead:" >&2
  echo "$matches" >&2
  exit 1
fi

echo "OK: no inline <svg> in templates"
