#!/usr/bin/env bash
set -euo pipefail

install_base_packages="${SBM_SECURITY_INSTALL_BASE_PACKAGES:-false}"
base_packages="${SBM_SECURITY_BASE_PACKAGES:-curl ca-certificates tar gzip unzip}"
firewall_mode="${SBM_SECURITY_FIREWALL_MODE:-inspect_only}"

case "$install_base_packages" in
  true|false) ;;
  *)
    printf 'SBM_RESULT {"status":"failed","message":"SBM_SECURITY_INSTALL_BASE_PACKAGES must be true or false"}\n'
    exit 1
    ;;
esac

if [ "$firewall_mode" != "inspect_only" ]; then
  printf 'SBM_RESULT {"status":"failed","message":"SBM_SECURITY_FIREWALL_MODE only supports inspect_only"}\n'
  exit 1
fi

package_manager="unknown"
for candidate in apt dnf yum apk pacman; do
  if command -v "$candidate" >/dev/null 2>&1; then
    package_manager="$candidate"
    break
  fi
done

firewall="none"
for candidate in ufw firewall-cmd nft iptables; do
  if command -v "$candidate" >/dev/null 2>&1; then
    firewall="$candidate"
    break
  fi
done

privilege_mode="unprivileged"
run_cmd=""
if [ "$(id -u)" = "0" ]; then
  privilege_mode="root"
elif command -v sudo >/dev/null 2>&1 && sudo -n true >/dev/null 2>&1; then
  privilege_mode="passwordless_sudo"
  run_cmd="sudo -n"
fi

printf 'SBM_PROGRESS {"step":"security_inspect","status":"success","message":"Security baseline inspected"}\n'

packages_installed=false
if [ "$install_base_packages" = "true" ]; then
  if [ "$package_manager" = "unknown" ]; then
    printf 'SBM_RESULT {"status":"failed","message":"no supported package manager found","package_manager":"unknown"}\n'
    exit 1
  fi
  if [ "$privilege_mode" = "unprivileged" ]; then
    printf 'SBM_RESULT {"status":"failed","message":"root or passwordless sudo is required to install base packages","privilege_mode":"unprivileged"}\n'
    exit 1
  fi

  printf 'SBM_PROGRESS {"step":"base_packages","status":"running","message":"Installing allowed base packages"}\n'
  case "$package_manager" in
    apt)
      ${run_cmd} apt-get update
      ${run_cmd} DEBIAN_FRONTEND=noninteractive apt-get install -y $base_packages
      ;;
    dnf)
      ${run_cmd} dnf install -y $base_packages
      ;;
    yum)
      ${run_cmd} yum install -y $base_packages
      ;;
    apk)
      ${run_cmd} apk add --no-cache $base_packages
      ;;
    pacman)
      ${run_cmd} pacman -Sy --noconfirm --needed $base_packages
      ;;
  esac
  packages_installed=true
  printf 'SBM_PROGRESS {"step":"base_packages","status":"success","message":"Allowed base packages installed"}\n'
else
  printf 'SBM_PROGRESS {"step":"base_packages","status":"skipped","message":"Base package installation disabled"}\n'
fi

printf 'SBM_PROGRESS {"step":"firewall_inspect","status":"success","message":"Firewall state inspected without changes"}\n'
printf 'SBM_RESULT {"status":"success","package_manager":"%s","firewall":"%s","firewall_mode":"inspect_only","privilege_mode":"%s","base_packages_installed":%s,"service_restarted":false,"ssh_policy_changed":false}\n' \
  "$package_manager" "$firewall" "$privilege_mode" "$packages_installed"
