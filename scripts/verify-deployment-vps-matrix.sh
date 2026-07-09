#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
verify_script="${SBM_VERIFY_SCRIPT:-$script_dir/verify-deployment-vps.sh}"
runtime_cache_script="${SBM_RUNTIME_CACHE_SCRIPT:-$script_dir/runtime-cache/download-singbox.sh}"

default_cases="direct,managed,custom_socks5,custom_http_connect,import_export,runtime_cache,cancel,preserve_remote_run_dir"
case_list="${SBM_MATRIX_CASES:-$default_cases}"
mode="plan"
skip_unavailable="${SBM_MATRIX_SKIP_UNAVAILABLE:-false}"
prepare_cache="${SBM_MATRIX_PREPARE_CACHE:-false}"
runtime_version="${SBM_RUNTIME_VERSION:-1.13.13}"
data_dir="${SBM_DATA_DIR:-$HOME/.singbox-manager}"

usage() {
  cat <<'USAGE'
Usage:
  scripts/verify-deployment-vps-matrix.sh [--plan|--run] [--cases LIST]

Runs the repeatable Proxy Node Deployment VPS verification matrix by invoking
scripts/verify-deployment-vps.sh once per selected case.

Default mode is --plan, which prints the matrix without contacting the API/VPS.
Use --run to execute real deployments.

Cases:
  direct                  direct SSH + remote runtime deployment
  managed                 managed_proxy route, auto-selects first usable route unless SBM_MANAGED_CANDIDATE_ID is set
  custom_socks5           custom SOCKS5 route
  custom_http_connect     custom HTTP CONNECT route
  import_export           import generated node and run sing-box check on exported config
  runtime_cache           require valid pinned runtime cache, optionally prefetch with SBM_MATRIX_PREPARE_CACHE=true
  cancel                  create then cancel a run
  preserve_remote_run_dir preserve remote deployment temp dir and verify marker

Common required environment for --run:
  SBM_PASSWORD
  SBM_SSH_HOST
  SBM_SSH_PASSWORD, or SBM_SSH_AUTH_METHOD=private_key with SBM_SSH_PRIVATE_KEY/FILE

Custom SOCKS5 case environment:
  SBM_SOCKS5_PROXY_HOST       fallback: SBM_PROXY_HOST
  SBM_SOCKS5_PROXY_PORT       fallback: SBM_PROXY_PORT_LOCAL
  SBM_SOCKS5_PROXY_USERNAME   optional
  SBM_SOCKS5_PROXY_PASSWORD   optional

Custom HTTP CONNECT case environment:
  SBM_HTTP_CONNECT_PROXY_HOST       fallback: SBM_PROXY_HOST
  SBM_HTTP_CONNECT_PROXY_PORT       fallback: SBM_PROXY_PORT_LOCAL
  SBM_HTTP_CONNECT_PROXY_USERNAME   optional
  SBM_HTTP_CONNECT_PROXY_PASSWORD   optional

Other optional environment:
  SBM_MATRIX_CASES             comma-separated case list
  SBM_MATRIX_SKIP_UNAVAILABLE  true to skip custom proxy cases with missing proxy env; default false
  SBM_MATRIX_PREPARE_CACHE     true to download pinned runtime cache before runtime_cache; default false
  SBM_RUNTIME_VERSION          default 1.13.13
  SBM_DATA_DIR                 default ~/.singbox-manager
  SBM_SING_BOX_CHECK_BIN       required by import_export unless sing-box is on PATH or in ~/.singbox-manager/bin
USAGE
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --plan|--dry-run)
      mode="plan"
      shift
      ;;
    --run)
      mode="run"
      shift
      ;;
    --cases)
      case_list="${2:?missing --cases value}"
      shift 2
      ;;
    --skip-unavailable)
      skip_unavailable="true"
      shift
      ;;
    --prepare-cache)
      prepare_cache="true"
      shift
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

case "$mode" in
  plan|run) ;;
  *) echo "invalid mode: $mode" >&2; exit 2 ;;
esac
case "$skip_unavailable" in
  true|false) ;;
  *) echo "SBM_MATRIX_SKIP_UNAVAILABLE must be true or false" >&2; exit 2 ;;
esac
case "$prepare_cache" in
  true|false) ;;
  *) echo "SBM_MATRIX_PREPARE_CACHE must be true or false" >&2; exit 2 ;;
esac

if [ ! -x "$verify_script" ]; then
  echo "verify script is not executable: $verify_script" >&2
  exit 2
fi

split_cases() {
  python3 - "$case_list" <<'PY'
import sys
for item in sys.argv[1].replace(" ", "").split(","):
    if item:
        print(item)
PY
}

mask_status() {
  local name="$1"
  if [ -n "${!name:-}" ]; then
    printf '%s=<set>\n' "$name"
  else
    printf '%s=<missing>\n' "$name"
  fi
}

proxy_value() {
  local primary="$1"
  local fallback="$2"
  printf '%s' "${!primary:-${!fallback:-}}"
}

ensure_proxy_case_env() {
  local case_name="$1"
  local host="$2"
  local port="$3"
  if [ -n "$host" ] && [ -n "$port" ]; then
    return 0
  fi
  if [ "$skip_unavailable" = "true" ]; then
    echo "==> SKIP $case_name: proxy host/port not configured"
    return 1
  fi
  echo "$case_name requires proxy host and port; set case-specific env or use SBM_MATRIX_SKIP_UNAVAILABLE=true" >&2
  exit 2
}

print_case_plan() {
  local case_name="$1"
  shift
  echo "==> PLAN $case_name"
  if [ "$#" -gt 0 ]; then
    local rendered=()
    local pair key value
    for pair in "$@"; do
      key="${pair%%=*}"
      value="${pair#*=}"
      case "$key" in
        *PASSWORD*|*PRIVATE_KEY*|*PASSPHRASE*|*TOKEN*|*SECRET*)
          if [ -n "$value" ]; then
            rendered+=("$key=<set>")
          else
            rendered+=("$key=")
          fi
          ;;
        *)
          rendered+=("$pair")
          ;;
      esac
    done
    printf '    sets: %s\n' "${rendered[*]}"
  else
    echo "    sets: <base environment>"
  fi
}

run_verify_case() {
  local case_name="$1"
  shift
  if [ "$mode" = "plan" ]; then
    print_case_plan "$case_name" "$@"
    return 0
  fi
  echo "==> RUN $case_name"
  (
    cd "$repo_root"
    env "$@" "$verify_script"
  )
}

run_case() {
  local case_name="$1"
  case "$case_name" in
    direct)
      run_verify_case direct \
        SBM_CONNECTION_MODE=direct \
        SBM_RUNTIME_SOURCE=remote \
        SBM_IMPORT_NODE=false \
        SBM_CHECK_EXPORTED_CONFIG=false \
        SBM_CANCEL_AFTER_CREATE=false \
        SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=false
      ;;
    managed)
      run_verify_case managed \
        SBM_CONNECTION_MODE=managed_proxy \
        SBM_RUNTIME_SOURCE=remote \
        SBM_IMPORT_NODE=false \
        SBM_CHECK_EXPORTED_CONFIG=false \
        SBM_CANCEL_AFTER_CREATE=false \
        SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=false
      ;;
    custom_socks5)
      local host port username password
      host="$(proxy_value SBM_SOCKS5_PROXY_HOST SBM_PROXY_HOST)"
      port="$(proxy_value SBM_SOCKS5_PROXY_PORT SBM_PROXY_PORT_LOCAL)"
      username="${SBM_SOCKS5_PROXY_USERNAME:-${SBM_PROXY_USERNAME:-}}"
      password="${SBM_SOCKS5_PROXY_PASSWORD:-${SBM_PROXY_PASSWORD:-}}"
      if ! ensure_proxy_case_env "$case_name" "$host" "$port"; then
        return 0
      fi
      run_verify_case custom_socks5 \
        SBM_CONNECTION_MODE=custom_proxy \
        SBM_PROXY_TYPE=socks5 \
        SBM_PROXY_HOST="$host" \
        SBM_PROXY_PORT_LOCAL="$port" \
        SBM_PROXY_USERNAME="$username" \
        SBM_PROXY_PASSWORD="$password" \
        SBM_RUNTIME_SOURCE=remote \
        SBM_IMPORT_NODE=false \
        SBM_CHECK_EXPORTED_CONFIG=false \
        SBM_CANCEL_AFTER_CREATE=false \
        SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=false
      ;;
    custom_http_connect)
      local host port username password
      host="$(proxy_value SBM_HTTP_CONNECT_PROXY_HOST SBM_PROXY_HOST)"
      port="$(proxy_value SBM_HTTP_CONNECT_PROXY_PORT SBM_PROXY_PORT_LOCAL)"
      username="${SBM_HTTP_CONNECT_PROXY_USERNAME:-${SBM_PROXY_USERNAME:-}}"
      password="${SBM_HTTP_CONNECT_PROXY_PASSWORD:-${SBM_PROXY_PASSWORD:-}}"
      if ! ensure_proxy_case_env "$case_name" "$host" "$port"; then
        return 0
      fi
      run_verify_case custom_http_connect \
        SBM_CONNECTION_MODE=custom_proxy \
        SBM_PROXY_TYPE=http_connect \
        SBM_PROXY_HOST="$host" \
        SBM_PROXY_PORT_LOCAL="$port" \
        SBM_PROXY_USERNAME="$username" \
        SBM_PROXY_PASSWORD="$password" \
        SBM_RUNTIME_SOURCE=remote \
        SBM_IMPORT_NODE=false \
        SBM_CHECK_EXPORTED_CONFIG=false \
        SBM_CANCEL_AFTER_CREATE=false \
        SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=false
      ;;
    import_export)
      run_verify_case import_export \
        SBM_CONNECTION_MODE=direct \
        SBM_RUNTIME_SOURCE=remote \
        SBM_IMPORT_NODE=true \
        SBM_CHECK_EXPORTED_CONFIG=true \
        SBM_CANCEL_AFTER_CREATE=false \
        SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=false
      ;;
    runtime_cache)
      if [ "$prepare_cache" = "true" ] && [ ! -x "$runtime_cache_script" ]; then
        echo "runtime cache script is not executable: $runtime_cache_script" >&2
        exit 2
      fi
      if [ "$mode" = "run" ] && [ "$prepare_cache" = "true" ]; then
        echo "==> PREPARE runtime_cache"
        "$runtime_cache_script" --version "$runtime_version" --data-dir "$data_dir"
      elif [ "$mode" = "plan" ] && [ "$prepare_cache" = "true" ]; then
        echo "==> PLAN runtime_cache prefetch: $runtime_cache_script --version $runtime_version --data-dir $data_dir"
      fi
      run_verify_case runtime_cache \
        SBM_CONNECTION_MODE=direct \
        SBM_RUNTIME_SOURCE=cache \
        SBM_IMPORT_NODE=false \
        SBM_CHECK_EXPORTED_CONFIG=false \
        SBM_CANCEL_AFTER_CREATE=false \
        SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=false
      ;;
    cancel)
      run_verify_case cancel \
        SBM_CONNECTION_MODE=direct \
        SBM_RUNTIME_SOURCE=remote \
        SBM_IMPORT_NODE=false \
        SBM_CHECK_EXPORTED_CONFIG=false \
        SBM_CANCEL_AFTER_CREATE=true \
        SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=false
      ;;
    preserve_remote_run_dir)
      run_verify_case preserve_remote_run_dir \
        SBM_CONNECTION_MODE=direct \
        SBM_RUNTIME_SOURCE=remote \
        SBM_IMPORT_NODE=false \
        SBM_CHECK_EXPORTED_CONFIG=false \
        SBM_CANCEL_AFTER_CREATE=false \
        SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR=true
      ;;
    *)
      echo "unknown matrix case: $case_name" >&2
      exit 2
      ;;
  esac
}

echo "Proxy Node Deployment VPS verification matrix ($mode)"
mask_status SBM_PASSWORD
mask_status SBM_SSH_HOST
mask_status SBM_SSH_PASSWORD
mask_status SBM_SSH_PRIVATE_KEY
mask_status SBM_SSH_PRIVATE_KEY_FILE

while IFS= read -r case_name; do
  run_case "$case_name"
done < <(split_cases)

echo "Proxy Node Deployment VPS verification matrix finished ($mode)"
