# Windows beta vs mainline feature alignment

**Audience:** operators and new contributors choosing between mainline `v0.2.x` and Windows beta `v0.3.x-windows-beta`.

**Compared refs (evidence base):**

| Line | Latest tag used here | Tip commit (short) | Notes |
|------|----------------------|--------------------|-------|
| Mainline | `v0.2.22` (draft release; equals `origin/main` at doc time) | `cc71a06` — merge of [#23](https://github.com/structName/sing-box-manager-gui/pull/23) | GitHub Releases publish Linux/macOS/Windows **amd64** artifacts from this line |
| Windows beta | `v0.3.2-windows-beta` | `cd687d4` (2025-12-25 HKT) | Tags only — **no** GitHub Release assets for `v0.3.*-windows-beta` |

Fork point: merge-base(`v0.2.22`, `v0.3.2-windows-beta`) = `74abd48` (*feat: update version*, 2025-12-24). Windows beta then added three commits (`c6c8786` Windows service/process support → theme CI fix → subscription UI fix). Mainline continued with Tor, AnyTLS, deployment, SOCKS/mihomo, #23 rename cascade, default-port change, SSR kernel embeds, etc.

## Quick answer

> **Does Windows beta include [#23](https://github.com/structName/sing-box-manager-gui/issues/23) SOCKS mihomo mapping / node-rename cascade?**  
> **No.** `10bd982` / PR #23 is an ancestor of `v0.2.22` / `main`, **not** of `v0.3.2-windows-beta`.

## Feature × mainline × latest windows-beta

| Feature | Mainline `v0.2.22` / `main` | Latest Windows beta `v0.3.2-windows-beta` | Evidence |
|---------|----------------------------|-------------------------------------------|----------|
| **SOCKS protocol** (parser / URL / Clash) | Yes (`internal/parser/socks.go`, README protocols table) | **No** (no `socks.go`; README lists SS/VMess/VLESS/Trojan/Hysteria2/TUIC only) | `git merge-base --is-ancestor f6a1814 v0.3.2-windows-beta` → false |
| **SOCKS mihomo mapping** (speedtest/health version map) | Yes (PR [#23](https://github.com/structName/sing-box-manager-gui/pull/23) / `10bd982`) | **No** | Same ancestry check; `git grep mihomo` on beta → 0 hits |
| **Node-rename / delete cascade into chains** (#23) | Yes (`ChainSyncService.RetargetNodeTag`, API cascade messages) | **No** | `10bd982` not on beta |
| **AnyTLS** (parser, web form, mihomo map) | Yes (`internal/parser/anytls.go` + follow-ups) | **No** | `3779b79` / `5c88694` / `e26995a` not on beta |
| **Tor proxy chains** (`UseTorExit` / Tor node tag) | Yes (`e429cc3`) | **No** | Ancestry + `git grep UseTorExit` on beta → 0 |
| **Proxy node deployment workflow** | Yes (PR [#20](https://github.com/structName/sing-box-manager-gui/pull/20) / `197e198`, `internal/deploy/*`, `docs/proxy-node-deployment-verification.md`) | **No** | No `internal/deploy` / `internal/api/deployment*.go` on beta |
| **SSR protocol + SSR-enabled kernel embeds** | Yes (`shadowsocksr` parser; `bundled_asset_*` incl. `bundled_asset_windows_amd64.go`) | **No** SSR parser; **no** `internal/kernel/bundled_asset_*` on beta tip | Tree listing per tag |
| **Windows Task Scheduler / registry service manager** | **No** on this tip (`internal/daemon/windows.go` absent; `process.go` still Unix `pgrep` / `SIGHUP`) | **Yes** (`internal/daemon/windows.go`, `process_windows.go` from `c6c8786`) | Tree listing; commit `c6c8786` |
| **Windows amd64 binary in CI/Release matrix** | Yes (`build.sh` / `.github/workflows/release.yml` build `windows/amd64`; e.g. `sbm-windows-amd64.exe` on `v0.2.21`) | Beta tags have **no** published Release; Windows support landed as source/CI changes on the beta branch | Releases API + workflows |
| **Default web port** | `19090` (avoid Prometheus `9090`; `91fc5ba`) | `9090` | `cmd/sbm/main.go` per tag |
| **Default data dir** | `~/.singbox-manager` | `~/.singbox-manager` (same flag default) | `cmd/sbm/main.go` |

Legend: **Yes/No** = present in that tag’s tree / ancestry. Prefer this table over guessing from the `v0.3` semver alone — the Windows beta line is **not** a superset of `v0.2.x`.

## Service / path differences (discoverable)

| Concern | Mainline (Linux/macOS focus on tip) | Windows beta `v0.3.2` |
|---------|-------------------------------------|------------------------|
| Service integration | Linux: systemd unit `singbox-manager.service` (`/etc/systemd/system` or `~/.config/systemd/user`); macOS: launchd | Windows: Task Scheduler task name **`SingBoxManager`**; task XML at **`%USERPROFILE%\.singbox-manager\task.xml`**; optional HKCU `...\Run` value **`SingBoxManager`** |
| Hot reload | `SIGHUP` to sing-box where supported | **No SIGHUP**; reload path expects **process restart** (`process_windows.go`) |
| Process discovery / stop | `pgrep` / `SIGTERM` (Unix `process.go` on mainline tip) | `tasklist` / gopsutil; stop via `Kill` |
| Kernel binary layout | Bundled per-OS assets under `internal/kernel/` (incl. Windows amd64 zip embed on mainline) | Beta tip has **no** `bundled_asset_*` files — expect different/missing embed behavior vs current mainline |
| Published install story | GitHub Releases for `v0.2.x` (Windows exe on mainline releases) | `v0.3.*-windows-beta` tags exist; **no** matching Release pages/assets as of this doc |

A dedicated Windows acceptance host is still warranted before treating either line as “Windows production”: mainline ships a Windows **build** without the beta service manager; beta has the service manager but lacks post-`74abd48` product features.

## Tracking policy (unknown / needs decision)

**Status: unknown — needs maintainer decision.** Honest options under discussion (not chosen here):

1. **Track main:** after each `v0.2.x` (or `main`) release, merge/rebase into a Windows line and cut a new `v0.3.x-windows-beta` (or fold Windows daemon support into mainline and retire the parallel tags).
2. **Frozen API:** keep `v0.3.2-windows-beta` as a historical Windows spike; operators who need Tor / deployment / AnyTLS / #23 must use mainline builds (and accept incomplete Windows service integration until ported).

Until that decision is recorded, **do not** install `v0.3.x-windows-beta` expecting Linux/`v0.2.22` mainline behavior, and **do not** assume a mainline `sbm-windows-amd64.exe` includes the Task Scheduler installer from the beta branch.

## How this was verified

```bash
git fetch --tags
git merge-base --is-ancestor 10bd982 v0.2.22          # true  (#23)
git merge-base --is-ancestor 10bd982 v0.3.2-windows-beta  # false
git merge-base --is-ancestor e429cc3 v0.3.2-windows-beta  # false (Tor)
git merge-base --is-ancestor 197e198 v0.3.2-windows-beta  # false (deployment)
git merge-base --is-ancestor 3779b79 v0.3.2-windows-beta  # false (AnyTLS)
git merge-base --is-ancestor c6c8786 v0.2.22              # false (Windows service mgr)
```

Update this doc when a new `v0.2.x` or `v0.3.*-windows-beta` tag is cut, or when the tracking policy is decided.
