#!/usr/bin/env bash
# Ensure web/dist has a minimal index.html so //go:embed in web/embed.go succeeds.
# Does not overwrite an existing build artifact.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
stub="$root/web/dist/index.html"
if [[ -f "$stub" ]]; then
  exit 0
fi
mkdir -p "$(dirname "$stub")"
cat >"$stub" <<'HTML'
<!doctype html><html><head><meta charset="utf-8"><title>sbm embed stub</title></head><body>ok</body></html>
HTML
echo "created $stub"
