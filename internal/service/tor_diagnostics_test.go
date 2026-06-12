package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

type fakeTorDiagnosticProcess struct {
	stopped bool
}

func TestTorDiagnosticHTTPCheckerRejectsNonTorExit(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"IsTor":false,"IP":"198.51.100.10"}`))
	}))
	defer target.Close()

	err := validateTorProjectResponse(target.URL, []byte(`{"IsTor":false,"IP":"198.51.100.10"}`))
	if err == nil {
		t.Fatal("validateTorProjectResponse error = nil, want non-Tor exit error")
	}
	if !strings.Contains(err.Error(), "IsTor=false") {
		t.Fatalf("error should mention IsTor=false, got %q", err.Error())
	}

	if err := validateTorProjectResponse(target.URL, []byte(`{"IsTor":true,"IP":"203.0.113.10"}`)); err != nil {
		t.Fatalf("validateTorProjectResponse true response error = %v", err)
	}
}

func TestTorDiagnosticHTTPCheckerReadsHealthyResponseBody(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"IsTor":true,"IP":"203.0.113.10"}`)),
	}

	if err := validateTorHTTPResponse("https://check.torproject.org/api/ip", response); err != nil {
		t.Fatalf("validateTorHTTPResponse healthy response error = %v", err)
	}
}

func (p *fakeTorDiagnosticProcess) Stop() error {
	p.stopped = true
	return nil
}

func TestTorDiagnosticsUsesBoundInboundForFullChain(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID:      "tor-chain-1",
		Name:    "entry-tor-post",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag, "post"},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID:         "port-1",
		Name:       "Tor bound port",
		Type:       "mixed",
		Listen:     "0.0.0.0",
		Port:       2081,
		Enabled:    true,
		UseTorExit: true,
		TorChainID: chain.ID,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	var fullChainProxyAddr string
	var segmentChainID string
	svc := NewTorDiagnosticsService(store)
	svc.fullChainChecker = func(ctx context.Context, proxyAddr string, targetURL string) (TorDiagnosticCheck, error) {
		fullChainProxyAddr = proxyAddr
		return TorDiagnosticCheck{Status: "healthy", Method: "main_inbound", Latency: 12}, nil
	}
	svc.segmentChecker = func(ctx context.Context, chain storage.ProxyChain) (TorDiagnosticCheck, error) {
		segmentChainID = chain.ID
		return TorDiagnosticCheck{Status: "healthy", Method: "temporary_segment", Latency: 7}, nil
	}

	result, err := svc.CheckTorChain(context.Background(), chain.ID)
	if err != nil {
		t.Fatalf("CheckTorChain() error = %v", err)
	}

	if result.ChainID != chain.ID {
		t.Fatalf("result chain ID = %q, want %q", result.ChainID, chain.ID)
	}
	if segmentChainID != chain.ID {
		t.Fatalf("segment checker chain ID = %q, want %q", segmentChainID, chain.ID)
	}
	if fullChainProxyAddr != "127.0.0.1:2081" {
		t.Fatalf("full-chain proxy addr = %q, want 127.0.0.1:2081", fullChainProxyAddr)
	}
	if result.TorSegment.Status != "healthy" || result.FullChain.Status != "healthy" {
		t.Fatalf("unexpected diagnostic result: %#v", result)
	}
	if result.FullChain.Method != "main_inbound" {
		t.Fatalf("full-chain method = %q, want main_inbound", result.FullChain.Method)
	}
	if time.Since(result.CheckedAt) > time.Minute {
		t.Fatalf("checked_at should be recent, got %s", result.CheckedAt)
	}
}

func TestTorDiagnosticsSerializesTemporarySegmentChecks(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID:      "tor-chain-1",
		Name:    "entry-to-tor",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID:         "port-1",
		Type:       "mixed",
		Listen:     "127.0.0.1",
		Port:       2081,
		Enabled:    true,
		UseTorExit: true,
		TorChainID: chain.ID,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	svc := NewTorDiagnosticsService(store)
	svc.fullChainChecker = func(ctx context.Context, proxyAddr string, targetURL string) (TorDiagnosticCheck, error) {
		return TorDiagnosticCheck{Status: "healthy", Method: "main_inbound"}, nil
	}
	svc.segmentChecker = func(ctx context.Context, chain storage.ProxyChain) (TorDiagnosticCheck, error) {
		close(entered)
		<-release
		return TorDiagnosticCheck{Status: "healthy", Method: "temporary_segment"}, nil
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = svc.CheckTorChain(context.Background(), chain.ID)
	}()

	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first segment check did not start")
	}

	second, err := svc.CheckTorChain(context.Background(), chain.ID)
	if err != nil {
		t.Fatalf("second CheckTorChain() error = %v", err)
	}
	if second.TorSegment.Status != "unhealthy" {
		t.Fatalf("second segment status = %q, want unhealthy", second.TorSegment.Status)
	}
	if !strings.Contains(second.TorSegment.Error, "正在运行") {
		t.Fatalf("second segment error should mention running diagnostic, got %q", second.TorSegment.Error)
	}

	close(release)
	wg.Wait()
}

func TestTorDiagnosticsTemporarySegmentUsesTruncatedChainAndCleansUp(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	settings.SingBoxPath = "/usr/bin/sing-box"
	settings.TorEnabled = true
	settings.TorExecutablePath = "/usr/bin/tor"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	for _, nodeTag := range []string{"entry", "post"} {
		if err := store.AddManualNode(storage.ManualNode{
			ID:      nodeTag,
			Enabled: true,
			Node: storage.Node{
				Tag:        nodeTag,
				Type:       "socks",
				Server:     "127.0.0.1",
				ServerPort: 1080,
			},
		}); err != nil {
			t.Fatalf("AddManualNode() error = %v", err)
		}
	}

	chain := storage.ProxyChain{
		ID:      "tor-chain-1",
		Name:    "entry-tor-post",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag, "post"},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID:         "port-1",
		Type:       "mixed",
		Listen:     "127.0.0.1",
		Port:       2081,
		Enabled:    true,
		UseTorExit: true,
		TorChainID: chain.ID,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	var launchedConfigPath string
	var launchedWorkDir string
	fakeProcess := &fakeTorDiagnosticProcess{}

	svc := NewTorDiagnosticsService(store)
	svc.segmentProcessLauncher = func(ctx context.Context, executablePath, configPath, workDir string) (torDiagnosticProcess, error) {
		launchedConfigPath = configPath
		launchedWorkDir = workDir
		return fakeProcess, nil
	}
	svc.proxyWaiter = func(ctx context.Context, proxyAddr string) error {
		return nil
	}
	svc.fullChainChecker = func(ctx context.Context, proxyAddr string, targetURL string) (TorDiagnosticCheck, error) {
		if !strings.HasPrefix(proxyAddr, "127.0.0.1:") {
			t.Fatalf("segment proxy addr = %q, want localhost", proxyAddr)
		}
		return TorDiagnosticCheck{Status: "healthy"}, nil
	}

	result, err := svc.CheckTorChain(context.Background(), chain.ID)
	if err != nil {
		t.Fatalf("CheckTorChain() error = %v", err)
	}
	if result.TorSegment.Status != "healthy" {
		t.Fatalf("Tor segment status = %q, want healthy", result.TorSegment.Status)
	}
	if result.TorSegment.Method != "temporary_segment" {
		t.Fatalf("Tor segment method = %q, want temporary_segment", result.TorSegment.Method)
	}
	if !fakeProcess.stopped {
		t.Fatal("temporary diagnostic process was not stopped")
	}
	if _, err := os.Stat(launchedWorkDir); !os.IsNotExist(err) {
		t.Fatalf("temporary work dir should be removed, stat err = %v", err)
	}

	configBytes, err := os.ReadFile(launchedConfigPath)
	if !os.IsNotExist(err) {
		t.Fatalf("temporary config should be removed with work dir, read err = %v, bytes = %d", err, len(configBytes))
	}

	// Regenerate directly to assert the launched config shape before cleanup would have
	// excluded post-Tor hops. The launcher path above verifies this is the config path
	// handed to the process.
	configPath, workDir, _, cleanup, err := svc.writeTemporarySegmentConfig(chain)
	if err != nil {
		t.Fatalf("writeTemporarySegmentConfig() error = %v", err)
	}
	defer cleanup()

	configBytes, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if workDir == "" {
		t.Fatal("temporary work dir should not be empty")
	}

	var config struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(configBytes, &config); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	for _, outbound := range config.Outbounds {
		tag, _ := outbound["tag"].(string)
		if tag == storage.GenerateChainNodeCopyTag(chain.Name, "post") {
			t.Fatalf("temporary Tor segment config should exclude post-Tor chain copy, got tag %q", tag)
		}
	}
}
