package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestProcessManager(t *testing.T) *ProcessManager {
	t.Helper()

	dataDir := t.TempDir()
	binPath := filepath.Join(dataDir, "sing-box")
	configPath := filepath.Join(dataDir, "config.json")

	script := `#!/bin/sh
trap 'exit 0' TERM INT
while true; do
  sleep 1 &
  wait $!
done
`
	if err := os.WriteFile(binPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake sing-box: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"inbounds":[]}`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
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

func TestProcessManagerDesiredStateTracksManualStartStop(t *testing.T) {
	pm := newTestProcessManager(t)

	if pm.ShouldBeRunning() {
		t.Fatal("new process manager should not have desired running state")
	}

	if err := pm.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = pm.Stop() })

	if !pm.ShouldBeRunning() {
		t.Fatal("start should persist desired running state")
	}

	if err := pm.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if !pm.ShouldBeRunning() {
		t.Fatal("restart should keep desired running state")
	}

	if err := pm.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if pm.ShouldBeRunning() {
		t.Fatal("manual stop should clear desired running state")
	}
}

func TestProcessManagerRestoreDesiredStateStartsSingBox(t *testing.T) {
	pm := newTestProcessManager(t)

	if err := pm.setDesiredRunning(true); err != nil {
		t.Fatalf("set desired running: %v", err)
	}

	restored, err := pm.RestoreDesiredState()
	if err != nil {
		t.Fatalf("restore desired state: %v", err)
	}
	t.Cleanup(func() { _ = pm.Stop() })

	if !restored {
		t.Fatal("expected restore to start sing-box")
	}
	if !pm.IsRunning() {
		t.Fatal("sing-box should be running after restore")
	}
	if !pm.ShouldBeRunning() {
		t.Fatal("restore should keep desired running state")
	}
}

func TestProcessManagerRestoreDesiredStateSkipsWhenNotDesired(t *testing.T) {
	pm := newTestProcessManager(t)

	restored, err := pm.RestoreDesiredState()
	if err != nil {
		t.Fatalf("restore desired state: %v", err)
	}
	if restored {
		t.Fatal("restore should skip when desired running state is absent")
	}
	if pm.IsRunning() {
		t.Fatal("sing-box should not be running")
	}
}

func TestProcessManagerDesiredStateMonitorRestartsUnexpectedExit(t *testing.T) {
	pm := newTestProcessManager(t)

	if err := pm.setDesiredRunning(true); err != nil {
		t.Fatalf("set desired running: %v", err)
	}
	pm.StartDesiredStateMonitor(10 * time.Millisecond)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if pm.IsRunning() {
			t.Cleanup(func() { _ = pm.Stop() })
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("monitor did not start sing-box from desired running state")
}
