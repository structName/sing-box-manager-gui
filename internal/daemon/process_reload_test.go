package daemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newReloadAwareProcessManager(t *testing.T, markerPath string) *ProcessManager {
	t.Helper()

	dataDir := t.TempDir()
	binPath := filepath.Join(dataDir, "sing-box")
	configPath := filepath.Join(dataDir, "config.json")
	readyPath := markerPath + ".ready"

	script := `#!/bin/sh
case "$1" in
  check|version) exit 0 ;;
esac
MARKER="` + markerPath + `"
READY="` + readyPath + `"
trap 'printf reloaded >> "$MARKER"' HUP
trap 'exit 0' TERM INT
printf ready > "$READY"
while true; do
  sleep 1 &
  wait $! || true
done
`
	if err := os.WriteFile(binPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake sing-box: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"inbounds":[]}`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(markerPath, nil, 0644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	return &ProcessManager{
		singboxPath: binPath,
		configPath:  configPath,
		dataDir:     dataDir,
		pidFile:     filepath.Join(dataDir, "singbox.pid"),
		maxLogs:     1000,
		logs:        make([]string, 0),
	}
}

func waitForFileContent(t *testing.T, path, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	data, _ := os.ReadFile(path)
	t.Fatalf("timed out waiting for %q in %s, got %q", want, path, data)
}

func TestReloadWithManagedCmdProcess(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "hup.marker")
	pm := newReloadAwareProcessManager(t, markerPath)

	if err := pm.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = pm.Stop() })
	waitForFileContent(t, markerPath+".ready", "ready", 2*time.Second)

	if err := pm.Reload(); err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	waitForFileContent(t, markerPath, "reloaded", 2*time.Second)
}

func TestReloadAfterPIDOnlyRecovery(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "hup.marker")
	pm := newReloadAwareProcessManager(t, markerPath)

	cmd := exec.Command(pm.singboxPath)
	cmd.Dir = pm.dataDir
	if err := cmd.Start(); err != nil {
		t.Fatalf("start external sing-box: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	waitForFileContent(t, markerPath+".ready", "ready", 2*time.Second)

	// Simulate manager restart: recovered process is tracked by PID only (cmd == nil).
	pm.mu.Lock()
	pm.running = true
	pm.pid = cmd.Process.Pid
	pm.cmd = nil
	pm.mu.Unlock()

	if err := pm.Reload(); err != nil {
		t.Fatalf("Reload() after PID-only recovery error = %v", err)
	}
	waitForFileContent(t, markerPath, "reloaded", 2*time.Second)
}

func TestReloadErrorsWhenNeitherCmdNorPID(t *testing.T) {
	pm := newTestProcessManager(t)

	if err := pm.Reload(); err == nil {
		t.Fatal("Reload() should error when process is not running")
	} else if !strings.Contains(err.Error(), "未运行") {
		t.Fatalf("Reload() error = %v, want 未运行", err)
	}

	pm.mu.Lock()
	pm.running = true
	pm.pid = 0
	pm.cmd = nil
	pm.mu.Unlock()

	if err := pm.Reload(); err == nil {
		t.Fatal("Reload() should error when running flag is set but neither cmd nor pid is available")
	} else if !strings.Contains(err.Error(), "未运行") {
		t.Fatalf("Reload() error = %v, want 未运行", err)
	}
}
