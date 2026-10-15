# SOCKS / mihomo boundaries

Operator-facing notes for closed [#23](https://github.com/structName/sing-box-manager-gui/issues/23) / v0.2.22 and docs issue [#45](https://github.com/structName/sing-box-manager-gui/issues/45). Product mapping lives in `internal/speedtest` (mihomo adapter) and `internal/builder` (`normalizeSocksOutbound`).

## Matrix caveats

| Variant | Speed-test (mihomo) | Runtime (sing-box outbound) | Notes |
|---------|---------------------|-----------------------------|-------|
| SOCKS5 (`version` 5 / default) | `type: socks5` | `type: socks`, `version: "5"` | Full TCP; UDP ASSOCIATE when the remote supports it |
| SOCKS4 (`version` 4) | `type: socks4` | `type: socks`, `version: "4"` | **TCP-only** — no UDP ASSOCIATE |
| SOCKS4a (`socks4a://` URL or `version` 4a) | `type: socks4` | `type: socks`, `version: "4"` (aliases normalized) | Same TCP-only boundary as SOCKS4 |
| UDP-over-TCP (UoT) | `udp-over-tcp: true` **only if** `Extra.udp_over_tcp` is enabled | sing-box `udp_over_tcp` from Extra when present | Not implied by SOCKS type alone; typically a SOCKS5 Extra |

`socks4://` / `socks4a://` URL parse both store `Extra.version = "4"`. Builder aliases `socks4` / `socks4a` → version `"4"`. Mihomo mapping accepts Extra versions `"4"` and `"4a"` as `socks4`.

## Regression gate (keep referenced)

Minimum Socks → mihomo mapping coverage (already on main from #23):

```bash
go test ./internal/speedtest -count=1 -run 'TestNodeToMihomoProxySocks'
```

Named make target + CI job for the broader Socks/rename cascade ([#63](https://github.com/structName/sing-box-manager-gui/issues/63), open [PR #78](https://github.com/structName/sing-box-manager-gui/pull/78) — do not duplicate that job here):

```bash
make test-socks-rename-gate
```

Until #78 is on `main`, use the `go test … -run 'TestNodeToMihomoProxySocks'` command above. Related release-gate thread: [#29](https://github.com/structName/sing-box-manager-gui/issues/29).

## Optional live check (no VPS)

Unit mapping is enough for day-to-day. If you want a cheap local smoke (optional):

1. Point a manual SOCKS5 node at a local SOCKS5 listener (`127.0.0.1`).
2. Leave `Extra.udp_over_tcp` unset → speed-test adapter must **not** set `udp-over-tcp`.
3. Set `Extra.udp_over_tcp: {enabled: true}` (or URL `?uot=1`) → adapter must set `udp-over-tcp: true`.
4. For SOCKS4/4a, treat UDP / UoT results as out of scope (TCP-only); skip rather than treating as a product regression.

If the bundled mihomo build lacks UoT for a given SOCKS hop, document the skip — do not invent new protocol work unless `go test ./internal/speedtest -run Socks` itself regresses.
