#!/usr/bin/env bash
set -euo pipefail

output="dist/sbm-deployment-assets.tar.gz"

usage() {
  cat <<'USAGE'
Usage: package-deployment-assets.sh [--output PATH]

Package standalone proxy-node deployment scripts and verification notes.
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --output)
      output="${2:?missing --output value}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

mkdir -p "$(dirname "$output")"
tar -czf "$output" \
  scripts/verify-deployment-vps-matrix.sh \
  scripts/verify-deployment-vps.sh \
  scripts/templates/probe-system.sh \
  scripts/templates/security-basic.sh \
  scripts/templates/singbox-vless-reality.sh \
  scripts/runtime-cache/download-singbox.sh \
  docs/proxy-node-deployment-verification.md

echo "Deployment assets package ready: $output"
