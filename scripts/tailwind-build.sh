#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
npx tailwindcss -i ./assets/css/input.css -o ./web/static/css/output.css
