package service

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// listenAcceptClose reserves a port and accepts+closes connections for TCP hop stubs.
func listenAcceptClose(t *testing.T) (int, net.Listener) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, ln
}

func setupChainHealthFixture(t *testing.T, clashPort int, nodeTags []string, portOverrides map[string]int) (*storage.JSONStore, *HealthCheckService, map[string]int) {
	t.Helper()

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	settings.ClashAPIPort = clashPort
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	ports := make(map[string]int, len(nodeTags))
	for i, tag := range nodeTags {
		if storage.IsChainCountryNodeTag(tag) {
			continue
		}
		port, ok := portOverrides[tag]
		if !ok {
			port = freeTCPPort(t)
		}
		ports[tag] = port
		if err := store.AddManualNode(storage.ManualNode{
			ID:      fmt.Sprintf("mn-%d", i),
			Enabled: true,
			Node: storage.Node{
				Tag:        tag,
				Type:       "shadowsocks",
				Server:     "127.0.0.1",
				ServerPort: port,
				Extra: map[string]interface{}{
					"method":   "aes-256-gcm",
					"password": "test",
				},
			},
		}); err != nil {
			t.Fatalf("AddManualNode(%s) error = %v", tag, err)
		}
	}

	if err := store.AddProxyChain(storage.ProxyChain{
		ID:      "chain-1",
		Name:    "us-chain",
		Enabled: true,
		Nodes:   nodeTags,
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	return store, NewHealthCheckService(store), ports
}

func TestCheckChain_ExitOnlyMihomoCannotMarkHealthy(t *testing.T) {
	t.Parallel()

	entryPort, _ := listenAcceptClose(t)
	_, svc, _ := setupChainHealthFixture(t, 9090, []string{"entry-node", "exit-node"}, map[string]int{"entry-node": entryPort})

	svc.clashDelayFn = func(port int, proxyName, testURL string, timeout time.Duration) (int, error) {
		return 0, fmt.Errorf("clash path unavailable")
	}
	svc.mihomoDelayFn = func(node storage.Node, testURL string, timeout time.Duration) (int, error) {
		if node.Tag != "exit-node" {
			t.Fatalf("mihomo probed unexpected node %q", node.Tag)
		}
		return 42, nil
	}

	status, err := svc.CheckChain("chain-1")
	if err != nil {
		t.Fatalf("CheckChain() error = %v", err)
	}
	if status.Status == "healthy" {
		t.Fatalf("Status = healthy; exit-only mihomo must not mark full chain healthy")
	}
	if status.Status != "degraded" {
		t.Fatalf("Status = %q, want degraded", status.Status)
	}
	if status.ProbeMode != storage.ProbeModeExitDirect {
		t.Fatalf("ProbeMode = %q, want %q", status.ProbeMode, storage.ProbeModeExitDirect)
	}
	if status.DegradedReason == "" {
		t.Fatal("DegradedReason should explain exit-direct fallback")
	}
	if status.Latency != 42 {
		t.Fatalf("Latency = %d, want 42", status.Latency)
	}
}

func TestCheckChain_ClashSuccessUsesChainProbeMode(t *testing.T) {
	t.Parallel()

	entryPort, _ := listenAcceptClose(t)
	_, svc, _ := setupChainHealthFixture(t, 9090, []string{"entry-node", "exit-node"}, map[string]int{"entry-node": entryPort})

	svc.clashDelayFn = func(port int, proxyName, testURL string, timeout time.Duration) (int, error) {
		wantTag := storage.GenerateChainNodeCopyTag("us-chain", "exit-node")
		if proxyName != wantTag {
			t.Fatalf("Clash proxyName = %q, want %q", proxyName, wantTag)
		}
		return 88, nil
	}
	svc.mihomoDelayFn = func(node storage.Node, testURL string, timeout time.Duration) (int, error) {
		t.Fatal("mihomo should not be called when Clash succeeds")
		return 0, nil
	}

	status, err := svc.CheckChain("chain-1")
	if err != nil {
		t.Fatalf("CheckChain() error = %v", err)
	}
	if status.Status != "healthy" {
		t.Fatalf("Status = %q, want healthy (probe_mode=chain)", status.Status)
	}
	if status.ProbeMode != storage.ProbeModeChain {
		t.Fatalf("ProbeMode = %q, want %q", status.ProbeMode, storage.ProbeModeChain)
	}
	if status.DegradedReason != "" {
		t.Fatalf("DegradedReason = %q, want empty", status.DegradedReason)
	}
	if status.Latency != 88 {
		t.Fatalf("Latency = %d, want 88", status.Latency)
	}
}

func TestCheckChain_ClashDisabledMihomoIsDegraded(t *testing.T) {
	t.Parallel()

	_, svc, _ := setupChainHealthFixture(t, 0, []string{"exit-only"}, nil)
	svc.mihomoDelayFn = func(node storage.Node, testURL string, timeout time.Duration) (int, error) {
		return 15, nil
	}

	status, err := svc.CheckChain("chain-1")
	if err != nil {
		t.Fatalf("CheckChain() error = %v", err)
	}
	if status.Status == "healthy" {
		t.Fatal("Clash disabled + mihomo success must not report healthy")
	}
	if status.Status != "degraded" {
		t.Fatalf("Status = %q, want degraded", status.Status)
	}
	if status.ProbeMode != storage.ProbeModeExitDirect {
		t.Fatalf("ProbeMode = %q, want %q", status.ProbeMode, storage.ProbeModeExitDirect)
	}
}

func TestFindProxyPort_NoSilentWrongInbound(t *testing.T) {
	t.Parallel()

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "1", Type: "mixed", Listen: "127.0.0.1", Port: 7890, Outbound: "jp-chain", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	svc := NewHealthCheckService(store)
	got, err := svc.findProxyPort("us-chain")
	if err == nil {
		t.Fatalf("findProxyPort() = %q, want error for unbound chain", got)
	}
	if !strings.Contains(err.Error(), "未绑定入站") {
		t.Fatalf("error = %q, want 未绑定入站 message", err)
	}
}

func TestCheckChainSpeed_RequiresBoundInbound(t *testing.T) {
	t.Parallel()

	store, svc, _ := setupChainHealthFixture(t, 0, []string{"exit-node"}, nil)

	// Wrong inbound only (different outbound) — must not sample it.
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "wrong", Type: "mixed", Listen: "127.0.0.1", Port: freeTCPPort(t), Outbound: "other-chain", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	_, err := svc.CheckChainSpeed("chain-1")
	if err == nil {
		t.Fatal("CheckChainSpeed() expected error when chain inbound unbound")
	}
	if !strings.Contains(err.Error(), "未绑定入站") {
		t.Fatalf("error = %q, want 未绑定入站", err)
	}
	if cached := svc.GetCachedSpeedResult("chain-1"); cached != nil {
		t.Fatalf("should not cache wrong-inbound / exit-direct speed sample: %+v", cached)
	}
}

func TestCheckChainSpeed_BoundInboundSetsChainProbeMode(t *testing.T) {
	t.Parallel()

	store, svc, _ := setupChainHealthFixture(t, 0, []string{"exit-node"}, nil)
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "bound", Type: "socks", Listen: "127.0.0.1", Port: freeTCPPort(t), Outbound: "us-chain", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	// No live SOCKS on that port — expect proxy path failure, not unbound inbound.
	_, err := svc.CheckChainSpeed("chain-1")
	if err == nil {
		t.Fatal("expected speed test error without live SOCKS server")
	}
	if strings.Contains(err.Error(), "未绑定入站") {
		t.Fatalf("unexpected unbound error: %v", err)
	}
}
