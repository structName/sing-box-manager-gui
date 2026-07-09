#!/usr/bin/env bash
set -euo pipefail

api_base="${SBM_API_BASE:-http://127.0.0.1:19090}"
password="${SBM_PASSWORD:-}"
timeout_seconds="${SBM_VERIFY_TIMEOUT_SECONDS:-900}"
poll_seconds="${SBM_VERIFY_POLL_SECONDS:-5}"

usage() {
  cat <<'USAGE'
Usage:
  SBM_PASSWORD=... SBM_SSH_HOST=... SBM_SSH_PASSWORD=... scripts/verify-deployment-vps.sh

Required environment:
  SBM_PASSWORD                 sing-box-manager admin password
  SBM_SSH_HOST                 target VPS SSH host

Common optional environment:
  SBM_API_BASE                 default http://127.0.0.1:19090
  SBM_SSH_PORT                 default 22
  SBM_SSH_USER                 default root
  SBM_SSH_AUTH_METHOD          password or private_key, default password
  SBM_SSH_PASSWORD             required when auth method is password
  SBM_SSH_PRIVATE_KEY_FILE     private key file when auth method is private_key
  SBM_SSH_PRIVATE_KEY          inline private key alternative
  SBM_SSH_PRIVATE_KEY_PASSPHRASE
  SBM_NODE_NAME                default deployed-vless-reality-<timestamp>
  SBM_NODE_SERVER              default SBM_SSH_HOST
  SBM_PROXY_PORT               default 443
  SBM_RUNTIME_SOURCE           remote or cache, default remote
  SBM_RUNTIME_ARCH             optional amd64 or arm64 expectation; inferred from SSH probe when omitted
  SBM_CONNECTION_MODE          direct, custom_proxy, or managed_proxy; default direct
  SBM_MANAGED_CANDIDATE_ID     optional for managed_proxy; defaults to first usable route
  SBM_PROXY_TYPE               socks5 or http_connect, default socks5 for custom_proxy
  SBM_PROXY_HOST               required for custom_proxy
  SBM_PROXY_PORT_LOCAL         required for custom_proxy
  SBM_PROXY_USERNAME
  SBM_PROXY_PASSWORD
  SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR true or false, default false
  SBM_DEBUG_PRESERVE_VERIFY_TMPDIR true or false, default false
  SBM_CANCEL_AFTER_CREATE      true to cancel immediately after creating the run, default false
  SBM_IMPORT_NODE              true to import the generated node after success, default false
  SBM_CHECK_EXPORTED_CONFIG    true to run sing-box check after importing, default false
  SBM_SING_BOX_CHECK_BIN       sing-box binary for exported config check

The script logs in, checks the runtime cache when SBM_RUNTIME_SOURCE=cache,
runs the deployment connection test through the selected Deployment Connection,
starts a real singbox-vless-reality deployment run, polls history until
completion, validates the generated-node review endpoint, and optionally imports
the generated node with manual-node provenance verification. When
SBM_CANCEL_AFTER_CREATE=true is set, it cancels immediately after run creation
and verifies the run reaches cancelled instead of validating generated-node import.
When
SBM_CHECK_EXPORTED_CONFIG=true is set, it also exports the generated client
configuration and validates it with sing-box check.
USAGE
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

require() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    echo "missing required environment: $name" >&2
    exit 2
  fi
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 2
  fi
}

require_cmd curl
require_cmd python3
require SBM_PASSWORD
require SBM_SSH_HOST

ssh_auth_method="${SBM_SSH_AUTH_METHOD:-password}"
case "$ssh_auth_method" in
  password)
    require SBM_SSH_PASSWORD
    ;;
  private_key)
    if [ -z "${SBM_SSH_PRIVATE_KEY:-}" ] && [ -z "${SBM_SSH_PRIVATE_KEY_FILE:-}" ]; then
      echo "private_key auth requires SBM_SSH_PRIVATE_KEY or SBM_SSH_PRIVATE_KEY_FILE" >&2
      exit 2
    fi
    ;;
  *)
    echo "SBM_SSH_AUTH_METHOD must be password or private_key" >&2
    exit 2
    ;;
esac
ssh_private_key_secret="${SBM_SSH_PRIVATE_KEY:-}"
if [ -z "$ssh_private_key_secret" ] && [ -n "${SBM_SSH_PRIVATE_KEY_FILE:-}" ]; then
  ssh_private_key_secret="$(cat "$SBM_SSH_PRIVATE_KEY_FILE")"
fi

connection_mode="${SBM_CONNECTION_MODE:-direct}"
case "$connection_mode" in
  direct)
    ;;
  custom_proxy)
    require SBM_PROXY_HOST
    require SBM_PROXY_PORT_LOCAL
    ;;
  managed_proxy)
    :
    ;;
  *)
    echo "SBM_CONNECTION_MODE must be direct, custom_proxy, or managed_proxy" >&2
    exit 2
    ;;
esac

check_exported_config="${SBM_CHECK_EXPORTED_CONFIG:-false}"
case "$check_exported_config" in
  true|false)
    ;;
  *)
    echo "SBM_CHECK_EXPORTED_CONFIG must be true or false" >&2
    exit 2
    ;;
esac
debug_preserve_remote_run_dir="${SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR:-false}"
case "$debug_preserve_remote_run_dir" in
  true|false)
    ;;
  *)
    echo "SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR must be true or false" >&2
    exit 2
    ;;
esac
debug_preserve_verify_tmpdir="${SBM_DEBUG_PRESERVE_VERIFY_TMPDIR:-false}"
case "$debug_preserve_verify_tmpdir" in
  true|false)
    ;;
  *)
    echo "SBM_DEBUG_PRESERVE_VERIFY_TMPDIR must be true or false" >&2
    exit 2
    ;;
esac
cancel_after_create="${SBM_CANCEL_AFTER_CREATE:-false}"
case "$cancel_after_create" in
  true|false)
    ;;
  *)
    echo "SBM_CANCEL_AFTER_CREATE must be true or false" >&2
    exit 2
    ;;
esac
if [ "$cancel_after_create" = "true" ] && [ "${SBM_IMPORT_NODE:-false}" = "true" ]; then
  echo "SBM_CANCEL_AFTER_CREATE=true cannot be combined with SBM_IMPORT_NODE=true" >&2
  exit 2
fi
if [ "$check_exported_config" = "true" ] && [ "${SBM_IMPORT_NODE:-false}" != "true" ]; then
  echo "SBM_CHECK_EXPORTED_CONFIG=true requires SBM_IMPORT_NODE=true" >&2
  exit 2
fi

sing_box_check_bin="${SBM_SING_BOX_CHECK_BIN:-}"
if [ "$check_exported_config" = "true" ]; then
  if [ -z "$sing_box_check_bin" ]; then
    if command -v sing-box >/dev/null 2>&1; then
      sing_box_check_bin="$(command -v sing-box)"
    elif [ -x "$HOME/.singbox-manager/bin/sing-box" ]; then
      sing_box_check_bin="$HOME/.singbox-manager/bin/sing-box"
    else
      echo "SBM_CHECK_EXPORTED_CONFIG=true requires SBM_SING_BOX_CHECK_BIN or a sing-box binary on PATH" >&2
      exit 2
    fi
  fi
  if [ ! -x "$sing_box_check_bin" ]; then
    echo "SBM_SING_BOX_CHECK_BIN is not executable: $sing_box_check_bin" >&2
    exit 2
  fi
fi

tmpdir="$(mktemp -d /tmp/sbm-vps-verify.XXXXXX)"
if [ "$debug_preserve_verify_tmpdir" = "true" ]; then
  echo "verifier tmpdir preserved: $tmpdir"
else
  trap 'rm -rf "$tmpdir"' EXIT
fi
cookie_jar="$tmpdir/cookies.txt"

response_body_message() {
  local file="$1"
  if [ "$debug_preserve_verify_tmpdir" = "true" ]; then
    echo "response body retained at $file" >&2
  else
    echo "response body not printed; set SBM_DEBUG_PRESERVE_VERIFY_TMPDIR=true to retain it" >&2
  fi
}

curl_json() {
  local method="$1"
  local path="$2"
  local body_file="${3:-}"
  local output_file="${4:-$tmpdir/response.json}"
  local status
  if [ -n "$body_file" ]; then
    status="$(curl -sS -o "$output_file" -w '%{http_code}' -b "$cookie_jar" -c "$cookie_jar" \
      -H 'Content-Type: application/json' -X "$method" --data-binary @"$body_file" \
      "${api_base%/}${path}")"
  else
    status="$(curl -sS -o "$output_file" -w '%{http_code}' -b "$cookie_jar" -c "$cookie_jar" \
      -X "$method" "${api_base%/}${path}")"
  fi
  if [ "$status" -lt 200 ] || [ "$status" -ge 300 ]; then
    echo "HTTP $status $method $path" >&2
    response_body_message "$output_file"
    return 1
  fi
}

json_get() {
  local file="$1"
  local expr="$2"
  python3 - "$file" "$expr" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    value = json.load(fh)
for part in sys.argv[2].split("."):
    if part == "":
        continue
    if isinstance(value, list):
        value = value[int(part)]
    else:
        value = value.get(part)
    if value is None:
        print("")
        sys.exit(0)
if isinstance(value, bool):
    print("true" if value else "false")
else:
    print(value)
PY
}

build_payload() {
  local kind="$1"
  python3 - "$kind" <<'PY'
import json
import os
import sys
import time

kind = sys.argv[1]
auth_method = os.environ.get("SBM_SSH_AUTH_METHOD", "password")
ssh = {
    "host": os.environ["SBM_SSH_HOST"],
    "port": int(os.environ.get("SBM_SSH_PORT", "22")),
    "user": os.environ.get("SBM_SSH_USER", "root"),
    "auth_method": auth_method,
}
if auth_method == "password":
    ssh["password"] = os.environ.get("SBM_SSH_PASSWORD", "")
else:
    key = os.environ.get("SBM_SSH_PRIVATE_KEY", "")
    key_file = os.environ.get("SBM_SSH_PRIVATE_KEY_FILE", "")
    if not key and key_file:
        with open(key_file, "r", encoding="utf-8") as fh:
            key = fh.read()
    ssh["private_key"] = key
    ssh["private_key_passphrase"] = os.environ.get("SBM_SSH_PRIVATE_KEY_PASSPHRASE", "")

mode = os.environ.get("SBM_CONNECTION_MODE", "direct")
connection = {"mode": mode}
if mode == "managed_proxy":
    connection["managed_candidate_id"] = os.environ.get("SBM_MANAGED_CANDIDATE_ID", "")
elif mode == "custom_proxy":
    connection.update({
        "proxy_type": os.environ.get("SBM_PROXY_TYPE", "socks5"),
        "proxy_host": os.environ.get("SBM_PROXY_HOST", ""),
        "proxy_port": int(os.environ.get("SBM_PROXY_PORT_LOCAL", "0")),
        "proxy_username": os.environ.get("SBM_PROXY_USERNAME", ""),
        "proxy_password": os.environ.get("SBM_PROXY_PASSWORD", ""),
    })

if kind == "connection-test":
    print(json.dumps({"ssh": ssh, "connection": connection}, separators=(",", ":")))
    sys.exit(0)

node_name = os.environ.get("SBM_NODE_NAME") or f"deployed-vless-reality-{int(time.time())}"
parameters = {
    "node_name": node_name,
    "node_server": os.environ.get("SBM_NODE_SERVER") or os.environ["SBM_SSH_HOST"],
    "proxy_port": int(os.environ.get("SBM_PROXY_PORT", "443")),
    "runtime_source": os.environ.get("SBM_RUNTIME_SOURCE", "remote"),
    "runtime_os": "linux",
    "runtime_arch": os.environ.get("SBM_RUNTIME_ARCH", "amd64"),
    "debug_preserve_remote_run_dir": os.environ.get("SBM_DEBUG_PRESERVE_REMOTE_RUN_DIR", "false").lower() == "true",
}
payload = {
    "template_name": "singbox-vless-reality",
    "dry_run": False,
    "ssh": ssh,
    "connection": connection,
    "parameters": parameters,
}
print(json.dumps(payload, separators=(",", ":")))
PY
}

assert_no_secret_leak() {
  local file="$1"
  shift
  local text
  local index=0
  text="$(cat "$file")"
  for secret in "$@"; do
    index=$((index + 1))
    if [ -n "$secret" ] && printf '%s' "$text" | grep -F -- "$secret" >/dev/null; then
      echo "secret leaked in API response at argument #$index" >&2
      exit 1
    fi
  done
}

assert_sensitive_history_fields_redacted() {
  local file="$1"
  python3 - "$file" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    payload = json.load(fh)

sensitive_exact = {"password", "secret", "token", "uuid", "key", "short_id"}
sensitive_suffixes = ("_password", "_secret", "_token", "_private_key", "_uuid", "_key", "_short_id")
violations = []

def is_sensitive_key(key):
    normalized = str(key).strip().lower()
    return normalized in sensitive_exact or any(normalized.endswith(suffix) for suffix in sensitive_suffixes)

def walk(value, path):
    if isinstance(value, dict):
        for key, child in value.items():
            child_path = f"{path}.{key}" if path else str(key)
            if is_sensitive_key(key) and child not in ("", None, "[REDACTED]"):
                violations.append(child_path)
            walk(child, child_path)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            walk(child, f"{path}[{index}]")

walk(payload, "")
if violations:
    raise SystemExit("sensitive history fields were not redacted: " + ", ".join(violations))
PY
}

validate_deployment_run_list_redaction() {
  curl_json GET "/api/deployments/runs?limit=20" "" "$tmpdir/run-list.json"
  assert_no_secret_leak "$tmpdir/run-list.json" \
    "${SBM_SSH_PASSWORD:-}" "$ssh_private_key_secret" "${SBM_SSH_PRIVATE_KEY_PASSPHRASE:-}" \
    "${SBM_PROXY_PASSWORD:-}" "${SBM_MANAGED_CANDIDATE_ID:-}"
  assert_sensitive_history_fields_redacted "$tmpdir/run-list.json"
}

echo "==> Authenticating against ${api_base%/}"
curl_json GET /api/auth/me "" "$tmpdir/auth.json"
bootstrapped="$(json_get "$tmpdir/auth.json" data.bootstrapped)"
authenticated="$(json_get "$tmpdir/auth.json" data.authenticated)"
if [ "$authenticated" != "true" ]; then
  if [ "$bootstrapped" != "true" ]; then
    python3 - "$password" >"$tmpdir/auth-body.json" <<'PY'
import json
import sys
password = sys.argv[1]
print(json.dumps({"password": password, "confirm_password": password}))
PY
    curl_json POST /api/auth/bootstrap "$tmpdir/auth-body.json" "$tmpdir/bootstrap.json"
  else
    python3 - "$password" >"$tmpdir/auth-body.json" <<'PY'
import json
import sys
print(json.dumps({"password": sys.argv[1]}))
PY
    curl_json POST /api/auth/login "$tmpdir/auth-body.json" "$tmpdir/login.json"
  fi
fi

echo "==> Checking deployment templates and connection candidates"
curl_json GET /api/deployments/templates "" "$tmpdir/templates.json"
runtime_version="$(python3 - "$tmpdir/templates.json" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    templates = json.load(fh).get("data", [])
template = next((item for item in templates if item.get("name") == "singbox-vless-reality"), None)
if not template:
    raise SystemExit("singbox-vless-reality template not found")
version = ((template.get("runtime") or {}).get("version") or "").strip()
if not version:
    raise SystemExit("singbox-vless-reality runtime version missing")
print(version)
PY
)"
curl_json GET /api/deployments/connection-candidates "" "$tmpdir/candidates.json"
if [ "$connection_mode" = "managed_proxy" ]; then
  selected_managed_candidate_id="$(python3 - "$tmpdir/candidates.json" "${SBM_MANAGED_CANDIDATE_ID:-}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    candidates = json.load(fh).get("data", [])
requested = sys.argv[2].strip()
if requested:
    match = next((item for item in candidates if item.get("id") == requested), None)
else:
    match = next((item for item in candidates if item.get("available") and (item.get("local_endpoint") or item.get("requires_temporary_entrypoint"))), None)
if not match:
    if requested:
        raise SystemExit("managed candidate not found")
    raise SystemExit("no usable managed candidate found")
if not match.get("available"):
    reason = str(match.get("unavailable_reason") or "").strip()
    if reason:
        raise SystemExit(f"managed candidate unavailable: {reason}")
    raise SystemExit("managed candidate unavailable")
route = match.get("kind") or "route"
source = match.get("source") or ""
entrypoint = "temporary-entrypoint" if match.get("requires_temporary_entrypoint") else "local-endpoint"
label = "/".join(part for part in (route, source, entrypoint) if part)
print(f"managed candidate selected: {label}", file=sys.stderr)
print(match.get("id") or "")
PY
)"
  export SBM_MANAGED_CANDIDATE_ID="$selected_managed_candidate_id"
fi

echo "==> Running SSH connection test through selected Deployment Connection"
build_payload connection-test >"$tmpdir/connection-test.json"
curl_json POST /api/deployments/connection-test "$tmpdir/connection-test.json" "$tmpdir/connection-test-response.json"
assert_no_secret_leak "$tmpdir/connection-test-response.json" \
  "${SBM_SSH_PASSWORD:-}" "$ssh_private_key_secret" "${SBM_SSH_PRIVATE_KEY_PASSPHRASE:-}" \
  "${SBM_PROXY_PASSWORD:-}" "${SBM_MANAGED_CANDIDATE_ID:-}"
if [ "$(json_get "$tmpdir/connection-test-response.json" data.ok)" != "true" ]; then
  connection_test_message="$(json_get "$tmpdir/connection-test-response.json" data.message)"
  if [ -n "$connection_test_message" ]; then
    echo "$connection_test_message" >&2
  else
    echo "SSH connection test failed" >&2
  fi
  response_body_message "$tmpdir/connection-test-response.json"
  exit 1
fi

runtime_arch="$(python3 - "$tmpdir/connection-test-response.json" <<'PY'
import json
import os
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    remote = ((json.load(fh).get("data") or {}).get("remote") or {})
system = (remote.get("uname_s") or "").strip().lower()
if system != "linux":
    raise SystemExit(f"target OS is not supported: {remote.get('uname_s') or 'unknown'}")

machine = (remote.get("uname_m") or "").strip()
mapping = {
    "x86_64": "amd64",
    "amd64": "amd64",
    "aarch64": "arm64",
    "arm64": "arm64",
}
requested = os.environ.get("SBM_RUNTIME_ARCH", "").strip()
detected = mapping.get(machine, "")
if detected not in {"amd64", "arm64"}:
    raise SystemExit(f"target architecture is not supported: {machine or requested or 'unknown'}")
if requested:
    if requested not in {"amd64", "arm64"}:
        raise SystemExit(f"SBM_RUNTIME_ARCH is not supported: {requested}")
    if requested != detected:
        raise SystemExit(f"SBM_RUNTIME_ARCH={requested} does not match target architecture {detected}")
arch = detected
print(arch)
PY
)"
export SBM_RUNTIME_ARCH="$runtime_arch"

if [ "${SBM_RUNTIME_SOURCE:-remote}" = "cache" ]; then
  echo "==> Checking runtime cache for sing-box ${runtime_version} linux/${SBM_RUNTIME_ARCH}"
  curl_json GET "/api/deployments/runtime-cache?runtime_name=sing-box&version=$runtime_version" "" "$tmpdir/runtime-cache.json"
  python3 - "$tmpdir/runtime-cache.json" "$SBM_RUNTIME_ARCH" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    archives = json.load(fh).get("data", [])
arch = sys.argv[2]
match = next((item for item in archives if item.get("os") == "linux" and item.get("arch") == arch), None)
if not match:
    raise SystemExit(f"runtime cache entry missing for linux/{arch}")
if match.get("status") != "valid":
    raise SystemExit(f"runtime cache for linux/{arch} is {match.get('status')}: {match}")
print(f"runtime cache: linux/{arch} valid")
PY
fi

echo "==> Creating real deployment run"
build_payload run >"$tmpdir/run-create.json"
curl_json POST /api/deployments/runs "$tmpdir/run-create.json" "$tmpdir/run-create-response.json"
assert_no_secret_leak "$tmpdir/run-create-response.json" \
  "${SBM_SSH_PASSWORD:-}" "$ssh_private_key_secret" "${SBM_SSH_PRIVATE_KEY_PASSPHRASE:-}" \
  "${SBM_PROXY_PASSWORD:-}" "${SBM_MANAGED_CANDIDATE_ID:-}"
assert_sensitive_history_fields_redacted "$tmpdir/run-create-response.json"
run_id="$(json_get "$tmpdir/run-create-response.json" data.id)"
if [ -z "$run_id" ]; then
  echo "deployment run id missing" >&2
  response_body_message "$tmpdir/run-create-response.json"
  exit 1
fi
echo "deployment run: $run_id"

if [ "$cancel_after_create" = "true" ]; then
  echo "==> Cancelling deployment run"
  curl_json POST "/api/deployments/runs/$run_id/cancel" "" "$tmpdir/run-cancel-response.json"
  assert_no_secret_leak "$tmpdir/run-cancel-response.json" \
    "${SBM_SSH_PASSWORD:-}" "$ssh_private_key_secret" "${SBM_SSH_PRIVATE_KEY_PASSPHRASE:-}" \
    "${SBM_PROXY_PASSWORD:-}" "${SBM_MANAGED_CANDIDATE_ID:-}"
  assert_sensitive_history_fields_redacted "$tmpdir/run-cancel-response.json"

  deadline=$((SECONDS + timeout_seconds))
  while true; do
    curl_json GET "/api/deployments/runs/$run_id" "" "$tmpdir/run-current.json"
    status="$(json_get "$tmpdir/run-current.json" data.status)"
    echo "status: $status"
    case "$status" in
      cancelled)
        break
        ;;
      success|failed)
        echo "deployment run reached $status before cancellation completed" >&2
        response_body_message "$tmpdir/run-current.json"
        exit 1
        ;;
    esac
    if [ "$SECONDS" -ge "$deadline" ]; then
      echo "deployment cancellation timed out after ${timeout_seconds}s" >&2
      response_body_message "$tmpdir/run-current.json"
      exit 1
    fi
    sleep "$poll_seconds"
  done
  assert_no_secret_leak "$tmpdir/run-current.json" \
    "${SBM_SSH_PASSWORD:-}" "$ssh_private_key_secret" "${SBM_SSH_PRIVATE_KEY_PASSPHRASE:-}" \
    "${SBM_PROXY_PASSWORD:-}" "${SBM_MANAGED_CANDIDATE_ID:-}"
  assert_sensitive_history_fields_redacted "$tmpdir/run-current.json"
  validate_deployment_run_list_redaction
  echo "cancellation validation passed: $run_id"
  exit 0
fi

deadline=$((SECONDS + timeout_seconds))
while true; do
  curl_json GET "/api/deployments/runs/$run_id" "" "$tmpdir/run-current.json"
  status="$(json_get "$tmpdir/run-current.json" data.status)"
  echo "status: $status"
  case "$status" in
    success|failed|cancelled)
      break
      ;;
  esac
  if [ "$SECONDS" -ge "$deadline" ]; then
    echo "deployment run timed out after ${timeout_seconds}s" >&2
    response_body_message "$tmpdir/run-current.json"
    exit 1
  fi
  sleep "$poll_seconds"
done

assert_no_secret_leak "$tmpdir/run-current.json" \
  "${SBM_SSH_PASSWORD:-}" "$ssh_private_key_secret" "${SBM_SSH_PRIVATE_KEY_PASSPHRASE:-}" \
  "${SBM_PROXY_PASSWORD:-}" "${SBM_MANAGED_CANDIDATE_ID:-}"
assert_sensitive_history_fields_redacted "$tmpdir/run-current.json"
validate_deployment_run_list_redaction

if [ "$status" != "success" ]; then
  echo "deployment run did not succeed: $status" >&2
  response_body_message "$tmpdir/run-current.json"
  exit 1
fi

python3 - "$tmpdir/run-current.json" <<'PY'
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    run = json.load(fh).get("data", {})
if not run.get("generated_node"):
    raise SystemExit("successful run did not expose a generated_node in history")
if (run.get("progress_markers") or {}).get("external_reachability", {}).get("status") != "success":
    raise SystemExit("external_reachability marker is not success")
if run.get("exit_code") != 0:
    raise SystemExit(f"exit_code is not 0: {run.get('exit_code')}")
print("run validation passed")
PY

if [ "$debug_preserve_remote_run_dir" = "true" ]; then
  python3 - "$tmpdir/run-current.json" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    markers = (json.load(fh).get("data") or {}).get("progress_markers") or {}
marker = markers.get("remote_run_dir") or {}
path = str(marker.get("path") or "")
if marker.get("status") != "preserved" or marker.get("preserved") is not True:
    raise SystemExit(f"remote_run_dir marker is not preserved: {marker}")
if not path.startswith("/tmp/sbm-deploy-"):
    raise SystemExit(f"remote_run_dir path is not a deployment temp dir: {path}")
print(f"remote run dir preserved: {path}")
PY
fi

echo "==> Validating generated-node review endpoint"
curl_json GET "/api/deployments/runs/$run_id/generated-node" "" "$tmpdir/generated-node.json"
assert_no_secret_leak "$tmpdir/generated-node.json" \
  "${SBM_SSH_PASSWORD:-}" "$ssh_private_key_secret" "${SBM_SSH_PRIVATE_KEY_PASSPHRASE:-}" \
  "${SBM_PROXY_PASSWORD:-}" "${SBM_MANAGED_CANDIDATE_ID:-}"
if [ "$(json_get "$tmpdir/generated-node.json" data.can_import)" != "true" ]; then
  echo "generated-node review cannot import" >&2
  response_body_message "$tmpdir/generated-node.json"
  exit 1
fi

if [ "${SBM_IMPORT_NODE:-false}" = "true" ]; then
  tag="$(json_get "$tmpdir/generated-node.json" data.default_tag)"
  python3 - "$tag" >"$tmpdir/import-node.json" <<'PY'
import json
import os
import sys
print(json.dumps({
    "tag": os.environ.get("SBM_IMPORT_TAG") or sys.argv[1],
    "source_name": os.environ.get("SBM_IMPORT_SOURCE_NAME", "自建部署"),
    "enabled": os.environ.get("SBM_IMPORT_ENABLED", "true").lower() == "true",
}))
PY
  echo "==> Importing generated node"
  curl_json POST "/api/deployments/runs/$run_id/import-node" "$tmpdir/import-node.json" "$tmpdir/import-response.json"
  assert_no_secret_leak "$tmpdir/import-response.json" \
    "${SBM_SSH_PASSWORD:-}" "$ssh_private_key_secret" "${SBM_SSH_PRIVATE_KEY_PASSPHRASE:-}" \
    "${SBM_PROXY_PASSWORD:-}" "${SBM_MANAGED_CANDIDATE_ID:-}"
  imported_node_id="$(json_get "$tmpdir/import-response.json" data.id)"
  imported_node_tag="$(json_get "$tmpdir/import-response.json" data.tag)"
  if [ -z "$imported_node_id" ] || [ -z "$imported_node_tag" ]; then
    echo "import response missing imported node id/tag" >&2
    response_body_message "$tmpdir/import-response.json"
    exit 1
  fi
  curl_json GET "/api/deployments/runs/$run_id/generated-node" "" "$tmpdir/generated-node-after-import.json"
  assert_no_secret_leak "$tmpdir/generated-node-after-import.json" \
    "${SBM_SSH_PASSWORD:-}" "$ssh_private_key_secret" "${SBM_SSH_PRIVATE_KEY_PASSPHRASE:-}" \
    "${SBM_PROXY_PASSWORD:-}" "${SBM_MANAGED_CANDIDATE_ID:-}"
  if [ "$(json_get "$tmpdir/generated-node-after-import.json" data.can_import)" != "false" ]; then
    echo "generated-node review remains importable after import" >&2
    response_body_message "$tmpdir/generated-node-after-import.json"
    exit 1
  fi
  review_imported_node_id="$(json_get "$tmpdir/generated-node-after-import.json" data.imported_node_id)"
  if [ "$review_imported_node_id" != "$imported_node_id" ]; then
    echo "generated-node review imported_node_id mismatch: $review_imported_node_id != $imported_node_id" >&2
    response_body_message "$tmpdir/generated-node-after-import.json"
    exit 1
  fi
  curl_json GET /api/manual-nodes "" "$tmpdir/manual-nodes.json"
  python3 - "$tmpdir/manual-nodes.json" "$imported_node_tag" "$run_id" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as fh:
    manual_nodes = json.load(fh).get("data", [])
tag = sys.argv[2]
run_id = sys.argv[3]
match = next((item for item in manual_nodes if (item.get("node") or {}).get("tag") == tag), None)
if not match:
    raise SystemExit(f"imported manual node not found: {tag}")
node = match.get("node") or {}
extra = node.get("extra") or {}
if node.get("source") != "manual":
    raise SystemExit("imported node source is not manual")
if extra.get("node_origin") != "deployed_self_hosted":
    raise SystemExit("imported node origin missing")
if extra.get("entry_method") != "deployment_import":
    raise SystemExit("imported node entry method missing")
if extra.get("deployment_run_id") != run_id:
    raise SystemExit("imported node run id mismatch")
print(f"imported manual node verified: {tag}")
PY

  if [ "$check_exported_config" = "true" ]; then
    echo "==> Exporting client config and running sing-box check"
    curl_json GET /api/config/export "" "$tmpdir/exported-config.json"
    "$sing_box_check_bin" check -c "$tmpdir/exported-config.json"
  fi
fi

echo "Proxy Node Deployment VPS verification passed: $run_id"
