# Socks mihomo + rename-cascade CI gate

Regression guard for closed [#23](https://github.com/structName/sing-box-manager-gui/issues/23) / v0.2.22 (issue [#63](https://github.com/structName/sing-box-manager-gui/issues/63)).

## Why

Socks → mihomo mapping and manual-node rename/delete cascade were green at merge, but there was no **named** package gate. Easy to drop coverage when touching `tester` / `chain_sync` / `router`.

## Minimum package / `-run` set

| Package | Focus |
|---------|--------|
| `./internal/speedtest` | `TestNodeToMihomoProxySocks*` — Socks4/5/default/UoT mihomo mapping |
| `./internal/service` | `TestRetargetNodeTag*`, prune helpers — Retarget / Chain prune |
| `./internal/api` | `TestUpdateManualNode*`, `TestDeleteManualNode*` — cascade (+ `web/dist` embed) |

## How to run

```bash
make test-socks-rename-gate
```

CI job `socks-rename-gate` runs the same target without a full frontend build. It calls `scripts/ensure-web-dist-stub.sh` so `//go:embed` succeeds (composes with [#27](https://github.com/structName/sing-box-manager-gui/issues/27) / PR #68; does not fight a committed stub).

Related: #27, #29, #45.
