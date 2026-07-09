#!/usr/bin/env bash
set -euo pipefail

version="1.13.13"
data_dir="${HOME}/.singbox-manager"
base_url="https://github.com/SagerNet/sing-box/releases/download"
arches="amd64 arm64"
checksum_amd64="bb99cabf47694625db421ee17898f36cdc1f9c2cb5decf65b12bac8d8437e842"
checksum_arm64="d7fab87b921933eb281d8ee7bd5377cdd8228089f1f7c807c9363a6a2329286c"

usage() {
  cat <<'USAGE'
Usage: download-singbox.sh [--version VERSION] [--data-dir DIR] [--base-url URL]

Prefetch official sing-box Linux amd64/arm64 runtime archives into:
  {data_dir}/runtime-cache/sing-box/{version}/
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      version="${2:?missing --version value}"
      shift 2
      ;;
    --data-dir)
      data_dir="${2:?missing --data-dir value}"
      shift 2
      ;;
    --base-url)
      base_url="${2:?missing --base-url value}"
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

cache_dir="${data_dir}/runtime-cache/sing-box/${version}"
mkdir -p "$cache_dir"

if [ "$version" != "1.13.13" ]; then
  echo "unsupported version for built-in checksum metadata: ${version}" >&2
  exit 2
fi

tmp_manifest="${cache_dir}/checksums.json.tmp"
printf '{\n' >"$tmp_manifest"
first=true

for arch in $arches; do
  archive="sing-box-${version}-linux-${arch}.tar.gz"
  url="${base_url}/v${version}/${archive}"
  target="${cache_dir}/linux-${arch}.tar.gz"

  echo "Downloading ${url}"
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$target"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$target" "$url"
  else
    echo "curl or wget is required" >&2
    exit 1
  fi

  checksum="$(sha256sum "$target" | awk '{print $1}')"
  expected_checksum=""
  case "$arch" in
    amd64) expected_checksum="$checksum_amd64" ;;
    arm64) expected_checksum="$checksum_arm64" ;;
  esac
  if [ "$checksum" != "$expected_checksum" ]; then
    echo "checksum mismatch for ${archive}: expected ${expected_checksum}, got ${checksum}" >&2
    exit 1
  fi
  if [ "$first" = true ]; then
    first=false
  else
    printf ',\n' >>"$tmp_manifest"
  fi
  printf '  "linux-%s": "%s"' "$arch" "$checksum" >>"$tmp_manifest"
done

printf '\n}\n' >>"$tmp_manifest"
mv "$tmp_manifest" "${cache_dir}/checksums.json"
echo "Runtime cache ready: ${cache_dir}"
