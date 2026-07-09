#!/usr/bin/env bash
set -euo pipefail

proxy_port="${SBM_PROXY_PORT:-443}"
os_name="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch_name="$(uname -m)"
kernel_name="$(uname -r 2>/dev/null || true)"
case "$arch_name" in
  x86_64) arch_name="amd64" ;;
  aarch64|arm64) arch_name="arm64" ;;
esac

package_manager="unknown"
for candidate in apt dnf yum apk pacman; do
  if command -v "$candidate" >/dev/null 2>&1; then
    package_manager="$candidate"
    break
  fi
done

firewall="none"
for candidate in ufw firewalld nft iptables; do
  if command -v "$candidate" >/dev/null 2>&1; then
    firewall="$candidate"
    break
  fi
done

privilege_mode="unprivileged"
if [ "$(id -u)" = "0" ]; then
  privilege_mode="root"
elif command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
  privilege_mode="passwordless_sudo"
fi

systemd=false
if command -v systemctl >/dev/null 2>&1; then
  systemd=true
fi

port_listening=false
if command -v ss >/dev/null 2>&1 && ss -ltn | awk '{print $4}' | grep -Eq "[:.]${proxy_port}$"; then
  port_listening=true
fi

printf 'SBM_PROGRESS {"step":"probe_system","status":"success","message":"System probe completed"}\n'
printf 'SBM_RESULT {"status":"success","os":"%s","arch":"%s","kernel":"%s","package_manager":"%s","firewall":"%s","privilege_mode":"%s","systemd":%s,"proxy_port":%s,"port_listening":%s}\n' \
  "$os_name" "$arch_name" "$kernel_name" "$package_manager" "$firewall" "$privilege_mode" "$systemd" "$proxy_port" "$port_listening"
