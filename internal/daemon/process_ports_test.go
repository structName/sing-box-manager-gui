package daemon

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEnsureInboundPortsAvailableDetectsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	configPath := filepath.Join(t.TempDir(), "config.json")
	config := `{"inbounds":[{"type":"mixed","listen":"127.0.0.1","listen_port":` + strconv.Itoa(port) + `},{"type":"tun"}]}`
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	pm := &ProcessManager{configPath: configPath}
	err = pm.ensureInboundPortsAvailable()
	if err == nil {
		t.Fatal("ensureInboundPortsAvailable error = nil, want occupied port error")
	}
	if !strings.Contains(err.Error(), strconv.Itoa(port)) {
		t.Fatalf("ensureInboundPortsAvailable error = %q, want port %d included", err.Error(), port)
	}
}
