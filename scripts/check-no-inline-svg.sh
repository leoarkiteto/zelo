#!/usr/bin/env bash
# Fail if any production Templ template contains an inline <svg> that is not
# a data chart. Icons must be rendered through atoms.Icon (Material Symbols)
# and are never hand-drawn. The only exception is a server-rendered data
# chart, which has no JS chart library behind it and must carry an explicit
# data-chart attribute on the same line as <svg (e.g. <svg data-chart="bars").
set -euo pipefail

matches="$(rg '<svg' internal --glob '*.templ' | rg -v 'data-chart' || true)"
if [ -n "$matches" ]; then
  echo "Inline <svg> without data-chart found in templates." >&2
  echo "Icons must use atoms.Icon; only data charts may use inline <svg data-chart=\"...\"> (same line as <svg)." >&2
  echo "$matches" >&2
  exit 1
fi

echo "OK: no inline <svg> in templates (other than data-chart charts)"
