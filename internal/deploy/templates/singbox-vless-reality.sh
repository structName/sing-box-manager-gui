#!/usr/bin/env bash
set -euo pipefail

# Standalone environment contract:
#   SBM_NODE_NAME                  Optional generated node tag, default deployed-vless-reality
#   SBM_NODE_SERVER                Required public server address written to the generated node
#   SBM_PROXY_PORT                 Optional VLESS Reality listen port, default 443
#   SBM_UUID                       Required generated VLESS user UUID
#   SBM_REALITY_PRIVATE_KEY        Required Reality private key for the remote server config
#   SBM_REALITY_PUBLIC_KEY         Required Reality public key written to the generated node
#   SBM_REALITY_SHORT_ID           Optional Reality short id, default 0123456789abcdef
#   SBM_REALITY_SERVER_NAME        Optional Reality handshake/server_name, default www.microsoft.com
#   SBM_SINGBOX_VERSION            Optional pinned sing-box version, default 1.13.13
#   SBM_SINGBOX_ARCHIVE            Optional local archive path uploaded by sing-box-manager
#   SBM_SINGBOX_DOWNLOAD_BASE_URL  Optional official release base URL
#   SBM_SINGBOX_SHA256_AMD64       Required expected checksum for linux/amd64
#   SBM_SINGBOX_SHA256_ARM64       Required expected checksum for linux/arm64
node_name="${SBM_NODE_NAME:-deployed-vless-reality}"
node_server="${SBM_NODE_SERVER:?SBM_NODE_SERVER is required}"
proxy_port="${SBM_PROXY_PORT:-443}"
uuid="${SBM_UUID:?SBM_UUID is required}"
reality_private_key="${SBM_REALITY_PRIVATE_KEY:?SBM_REALITY_PRIVATE_KEY is required}"
reality_public_key="${SBM_REALITY_PUBLIC_KEY:?SBM_REALITY_PUBLIC_KEY is required}"
short_id="${SBM_REALITY_SHORT_ID:-0123456789abcdef}"
server_name="${SBM_REALITY_SERVER_NAME:-www.microsoft.com}"
singbox_version="${SBM_SINGBOX_VERSION:-1.13.13}"
download_base_url="${SBM_SINGBOX_DOWNLOAD_BASE_URL:-https://github.com/SagerNet/sing-box/releases/download}"

run_root() {
  if [ "$(id -u)" = "0" ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
    sudo -n "$@"
  else
    printf 'SBM_RESULT {"status":"failed","message":"root or passwordless sudo is required"}\n'
    exit 1
  fi
}

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

if ! printf '%s' "$proxy_port" | grep -Eq '^[0-9]+$' || [ "$proxy_port" -lt 1 ] || [ "$proxy_port" -gt 65535 ]; then
  printf 'SBM_RESULT {"status":"failed","message":"SBM_PROXY_PORT must be in 1-65535"}\n'
  exit 1
fi

os_name="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch_name="$(uname -m)"
case "$arch_name" in
  x86_64) arch_name="amd64" ;;
  aarch64|arm64) arch_name="arm64" ;;
esac
if [ "$os_name" != "linux" ] || { [ "$arch_name" != "amd64" ] && [ "$arch_name" != "arm64" ]; }; then
  printf 'SBM_RESULT {"status":"failed","message":"unsupported target","os":"%s","arch":"%s"}\n' "$os_name" "$arch_name"
  exit 1
fi
expected_sha256=""
case "$arch_name" in
  amd64) expected_sha256="${SBM_SINGBOX_SHA256_AMD64:-}" ;;
  arm64) expected_sha256="${SBM_SINGBOX_SHA256_ARM64:-}" ;;
esac
if [ -z "$expected_sha256" ]; then
  printf 'SBM_RESULT {"status":"failed","message":"sing-box archive checksum metadata missing","arch":"%s"}\n' "$arch_name"
  exit 1
fi

printf 'SBM_PROGRESS {"step":"preflight","status":"running","message":"Checking target service manager and proxy port"}\n'
if ! command -v systemctl >/dev/null 2>&1; then
  printf 'SBM_RESULT {"status":"failed","message":"systemd is required"}\n'
  exit 1
fi
if command -v ss >/dev/null 2>&1 && ss -ltn | awk '{print $4}' | grep -Eq "[:.]${proxy_port}$"; then
  printf 'SBM_RESULT {"status":"failed","message":"target proxy port is already listening","proxy_port":%s,"port_listening":true}\n' "$proxy_port"
  exit 1
fi
printf 'SBM_PROGRESS {"step":"preflight","status":"success"}\n'

work_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$work_dir"
}
trap cleanup EXIT

printf 'SBM_PROGRESS {"step":"install_singbox","status":"running","message":"Installing sing-box"}\n'
archive="${work_dir}/sing-box.tar.gz"
if [ -n "${SBM_SINGBOX_ARCHIVE:-}" ]; then
  cp "$SBM_SINGBOX_ARCHIVE" "$archive"
else
  archive_name="sing-box-${singbox_version}-linux-${arch_name}.tar.gz"
  url="${download_base_url}/v${singbox_version}/${archive_name}"
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$archive"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$archive" "$url"
  else
    printf 'SBM_RESULT {"status":"failed","message":"curl or wget is required"}\n'
    exit 1
  fi
fi
if ! command -v sha256sum >/dev/null 2>&1; then
  printf 'SBM_RESULT {"status":"failed","message":"sha256sum is required"}\n'
  exit 1
fi
actual_sha256="$(sha256sum "$archive" | awk '{print $1}')"
if [ "$actual_sha256" != "$expected_sha256" ]; then
  printf 'SBM_RESULT {"status":"failed","message":"sing-box archive checksum mismatch","expected_sha256":"%s","actual_sha256":"%s"}\n' "$expected_sha256" "$actual_sha256"
  exit 1
fi
tar -xzf "$archive" -C "$work_dir"
singbox_bin="$(find "$work_dir" -type f -name sing-box -perm -111 | head -n 1)"
if [ -z "$singbox_bin" ]; then
  printf 'SBM_RESULT {"status":"failed","message":"sing-box binary missing from archive"}\n'
  exit 1
fi
run_root install -m 0755 "$singbox_bin" /usr/local/bin/sing-box
printf 'SBM_PROGRESS {"step":"install_singbox","status":"success"}\n'

printf 'SBM_PROGRESS {"step":"render_config","status":"running","message":"Rendering sing-box VLESS Reality config"}\n'
config_path="${work_dir}/config.json"
escaped_server_name_config="$(json_escape "$server_name")"
cat >"$config_path" <<JSON
{
  "log": {
    "level": "info",
    "timestamp": true
  },
  "inbounds": [
    {
      "type": "vless",
      "tag": "vless-reality-in",
      "listen": "::",
      "listen_port": ${proxy_port},
      "users": [
        {
          "uuid": "${uuid}",
          "flow": "xtls-rprx-vision"
        }
      ],
      "tls": {
        "enabled": true,
        "server_name": "${escaped_server_name_config}",
        "reality": {
          "enabled": true,
          "handshake": {
            "server": "${escaped_server_name_config}",
            "server_port": 443
          },
          "private_key": "${reality_private_key}",
          "short_id": ["${short_id}"]
        }
      }
    }
  ],
  "outbounds": [
    {"type": "direct", "tag": "direct"}
  ]
}
JSON
run_root install -d -m 0755 /etc/sing-box
run_root install -m 0600 "$config_path" /etc/sing-box/config.json
printf 'SBM_PROGRESS {"step":"render_config","status":"success"}\n'

printf 'SBM_PROGRESS {"step":"validate_config","status":"running","message":"Validating sing-box config"}\n'
run_root /usr/local/bin/sing-box check -c /etc/sing-box/config.json
printf 'SBM_PROGRESS {"step":"validate_config","status":"success"}\n'

printf 'SBM_PROGRESS {"step":"enable_service","status":"running","message":"Enabling sing-box service"}\n'
service_path="${work_dir}/sing-box.service"
cat >"$service_path" <<'UNIT'
[Unit]
Description=sing-box service
Documentation=https://sing-box.sagernet.org
After=network.target nss-lookup.target

[Service]
ExecStart=/usr/local/bin/sing-box run -c /etc/sing-box/config.json
Restart=on-failure
RestartSec=10s
LimitNOFILE=infinity

[Install]
WantedBy=multi-user.target
UNIT
run_root install -m 0644 "$service_path" /etc/systemd/system/sing-box.service
run_root systemctl daemon-reload
run_root systemctl enable sing-box
if run_root systemctl is-active --quiet sing-box; then
  run_root systemctl restart sing-box
else
  run_root systemctl start sing-box
fi
printf 'SBM_PROGRESS {"step":"enable_service","status":"success"}\n'

printf 'SBM_PROGRESS {"step":"verify_service","status":"running","message":"Verifying sing-box service and listen port"}\n'
run_root systemctl is-active --quiet sing-box
port_listening=false
for _ in 1 2 3 4 5; do
  if command -v ss >/dev/null 2>&1 && ss -ltn | awk '{print $4}' | grep -Eq "[:.]${proxy_port}$"; then
    port_listening=true
    break
  fi
  sleep 1
done
if [ "$port_listening" != true ]; then
  printf 'SBM_RESULT {"status":"failed","message":"sing-box service started but proxy port is not listening","proxy_port":%s,"service_active":true,"port_listening":false}\n' "$proxy_port"
  exit 1
fi
printf 'SBM_PROGRESS {"step":"verify_service","status":"success"}\n'

escaped_node_name="$(json_escape "$node_name")"
escaped_node_server="$(json_escape "$node_server")"
escaped_uuid="$(json_escape "$uuid")"
escaped_server_name="$(json_escape "$server_name")"
escaped_public_key="$(json_escape "$reality_public_key")"
escaped_short_id="$(json_escape "$short_id")"
printf 'SBM_NODE_BEGIN\n'
printf '{"type":"vless","tag":"%s","server":"%s","server_port":%s,"extra":{"uuid":"%s","flow":"xtls-rprx-vision","tls":{"enabled":true,"server_name":"%s","utls":{"enabled":true,"fingerprint":"chrome"},"reality":{"enabled":true,"public_key":"%s","short_id":"%s"}}}}\n' \
  "$escaped_node_name" "$escaped_node_server" "$proxy_port" "$escaped_uuid" "$escaped_server_name" "$escaped_public_key" "$escaped_short_id"
printf 'SBM_NODE_END\n'
printf 'SBM_RESULT {"status":"success","runtime":"sing-box","runtime_version":"%s","systemd":true,"config_path":"/etc/sing-box/config.json","service":"sing-box","service_active":true,"port_listening":true}\n' "$singbox_version"
