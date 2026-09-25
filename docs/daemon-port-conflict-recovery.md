# Daemon / sing-box port-conflict recovery

Ops runbook for diagnosing inbound or Web listen port conflicts without killing user-owned processes such as `sing-box-socks`.

Related: issue #28 · restore path #18 (`3cd7e40`) · stop restart loop on panel Web port conflict (`7fa453d`).

---

## What the panel does today

Two different listen layers can conflict. Behavior is intentional and **does not** terminate foreign processes.

| Layer | Typical port | Code path | On conflict |
|-------|--------------|-----------|-------------|
| **sbm Web UI** | `19090` (CLI `-port`) | `ExitCodeForRunError` → exit **98**; systemd `RestartPreventExitStatus=98` | Panel binary exits; systemd **does not** restart-loop |
| **Managed sing-box inbounds** | e.g. mixed/SOCKS `1080`, Clash API `19091` | `ProcessManager.ensureInboundPortsAvailable` before `Start` | Start fails with `入站端口被占用: …`; restore / desired-state monitor log and retry later — **never kill** the occupant |

Implications:

- A machine can show **systemd `singbox-manager` / panel healthy** while **panel-managed sing-box is not running**, because something else already holds an inbound port.
- Conversely, a user-owned binary named like `sing-box-socks` can keep serving `1080` while the panel’s managed process (or a separate `sing-box` unit) is **inactive**. The panel is not lying when it says its own process is down.

### Do not auto-kill foreign `sing-box-socks`

**Policy:** never automatically `kill` / `fuser -k` / `systemctl stop` a process that the panel did not start, including user-owned `sing-box-socks` (or any other SOCKS helper on the same ports).

Only the operator may free a port after confirming ownership. Automatic recovery must stay limited to: refuse start, surface the occupied endpoints, and avoid restart storms on the Web listen port.

---

## Symptom checklist

Common real-machine pattern from #28:

1. `ss` / `lsof` shows a **non-panel** process (often `sing-box-socks`) listening on an inbound such as **1080**.
2. `systemctl status sing-box` (or the panel’s managed sing-box) is **inactive / failed**.
3. Dashboard / start action looks like “service never came up”, or logs repeat `入站端口被占用` / auto-start failures.
4. Optionally, if the **Web** port itself is taken, `singbox-manager.service` exits with status **98** and stays down without a restart loop.

---

## Diagnose (copy-paste)

Replace `1080` / `19090` / `19091` with the ports from your profile / Settings.

### Who is listening?

```bash
# Preferred (iproute2)
ss -ltnp | grep -E ':(1080|19090|19091)\b'

# Or by process name
ss -ltnp | grep -E 'sing-box|sbm|socks'
```

```bash
# lsof (if installed)
sudo lsof -nP -iTCP:1080 -sTCP:LISTEN
sudo lsof -nP -iTCP:19090 -sTCP:LISTEN
```

```bash
# fuser — list only; do NOT add -k unless you intentionally reclaim the port
sudo fuser -v 1080/tcp
sudo fuser -v 19090/tcp
```

### Panel vs external occupancy

| Signal | Likely meaning |
|--------|----------------|
| Listener PID matches `~/.singbox-manager/singbox.pid` (or your `-data` dir `singbox.pid`) | **Panel-managed** sing-box |
| Listener is `sbm` on `-port` (default 19090) | **Panel Web** process |
| Listener is `sing-box-socks`, another user’s `sing-box`, Docker, or unknown binary; PID **≠** `singbox.pid` | **External** occupancy — do not auto-kill |
| `systemctl is-active singbox-manager` is active, but start still reports `入站端口被占用` | Panel is up; **inbound** ports are held elsewhere |

```bash
# Panel-managed PID (default data dir; adjust -data if customized)
cat ~/.singbox-manager/singbox.pid 2>/dev/null
ps -p "$(cat ~/.singbox-manager/singbox.pid 2>/dev/null)" -o pid,cmd 2>/dev/null

# systemd units (names vary by install)
systemctl status singbox-manager.service --no-pager
systemctl --user status singbox-manager.service --no-pager
# Separate classic unit, if present on the host — not the same as panel-managed process:
systemctl status sing-box.service --no-pager 2>/dev/null || true
```

```bash
# Recent panel / sing-box hints
journalctl -u singbox-manager.service -n 80 --no-pager
# or log files under the data dir
tail -n 80 ~/.singbox-manager/logs/sbm.log
tail -n 80 ~/.singbox-manager/logs/singbox.log
```

Look for `入站端口被占用`, repeated auto-start failures, or exit status **98** on the Web port.

---

## Recovery steps

1. **Identify the occupant** with `ss` / `lsof` / `fuser -v` (list only).
2. **Classify** panel-managed (`singbox.pid` / `sbm`) vs external (`sing-box-socks`, foreign `sing-box`, etc.).
3. **Choose one operator action** (never automated by the panel):
   - **Keep the external listener:** change the panel inbound / Clash API / Web ports so they no longer collide, then start from the UI; **or**
   - **Hand the port to the panel:** stop the external process yourself only after you confirm it is safe (example: stop your own `sing-box-socks` service), then start / restart managed sing-box from the panel.
4. **Verify:** `ss -ltnp` shows the expected PID; panel reports running; no new `入站端口被占用` lines.
5. If the **Web** port was the conflict: free or retarget `-port`, then `systemctl start singbox-manager` (or reinstall the unit). Exit **98** is expected until the bind succeeds.

### Optional reclaim (manual only)

Only when **you** decide the listener is disposable:

```bash
# Example — operator-driven; NOT what the panel should do automatically
# sudo fuser -k 1080/tcp
```

Prefer stopping the owning service/unit over raw `-k` when one exists.

---

## UI follow-up (deferred)

Settings → **后台服务** (`DaemonCard`) today reports **sbm systemd/launchd install state** only. It does **not** yet distinguish “panel-managed sing-box running” vs “inbound port held by an external process”. That needs API + ownership signals beyond a copy tweak; tracked as a follow-up to #28 rather than bundled here.

---

## Code anchors (keep docs honest)

- `internal/daemon/process_ports.go` — `ensureInboundPortsAvailable`
- `internal/daemon/process.go` — `Start`, `RestoreDesiredState`, `StartDesiredStateMonitor`
- `internal/daemon/run_error.go` — exit **98** for `EADDRINUSE` on the Web listen
- `internal/daemon/systemd.go` — `RestartPreventExitStatus=98`
