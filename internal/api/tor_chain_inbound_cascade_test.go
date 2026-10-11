package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/structName/sing-box-manager-gui/internal/daemon"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func setupTorChainCascadeServer(t *testing.T) (*Server, *storage.JSONStore, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	torPath := filepath.Join(dataDir, "tor")
	if err := os.WriteFile(torPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	settings := store.GetSettings()
	settings.AutoApply = false
	settings.TorEnabled = true
	settings.TorExecutablePath = torPath
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID:      "entry-node",
		Enabled: true,
		Node: storage.Node{
			Tag:        "entry",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}
	return server, store, dataDir
}

func TestDeleteTorChainDisablesTorBoundInbound(t *testing.T) {
	server, store, _ := setupTorChainCascadeServer(t)

	chain := storage.ProxyChain{
		ID:      "tor-chain-1",
		Name:    "entry-to-tor",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	// Mirrors InboundPorts UI: UseTorExit stores outbound="" + tor_chain_id.
	port := storage.InboundPort{
		ID:         "tor-port-1",
		Name:       "tor mixed",
		Type:       "mixed",
		Listen:     "127.0.0.1",
		Port:       33100,
		Enabled:    true,
		Outbound:   "",
		UseTorExit: true,
		TorChainID: chain.ID,
	}
	if err := store.AddInboundPort(port); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	// Unrelated inbound must stay enabled.
	other := storage.InboundPort{
		ID:       "plain-port",
		Name:     "plain",
		Type:     "mixed",
		Listen:   "127.0.0.1",
		Port:     33101,
		Enabled:  true,
		Outbound: "Proxy",
	}
	if err := store.AddInboundPort(other); err != nil {
		t.Fatalf("AddInboundPort() other error = %v", err)
	}

	router := gin.New()
	router.DELETE("/api/proxy-chains/:id", server.deleteProxyChain)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/proxy-chains/"+chain.ID, nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	got := store.GetInboundPort(port.ID)
	if got == nil {
		t.Fatal("expected inbound port to remain stored")
	}
	if got.Enabled {
		t.Fatalf("expected Tor-bound inbound to be disabled after chain delete, got %#v", got)
	}
	if got.UseTorExit || got.TorChainID != "" {
		t.Fatalf("expected Tor refs cleared after chain delete, got UseTorExit=%v TorChainID=%q", got.UseTorExit, got.TorChainID)
	}

	still := store.GetInboundPort(other.ID)
	if still == nil || !still.Enabled {
		t.Fatalf("unrelated inbound should stay enabled, got %#v", still)
	}
	if store.GetProxyChain(chain.ID) != nil {
		t.Fatal("expected chain to be deleted")
	}
}

func TestDisableTorChainDisablesTorBoundInbound(t *testing.T) {
	server, store, _ := setupTorChainCascadeServer(t)

	chain := storage.ProxyChain{
		ID:      "tor-chain-2",
		Name:    "entry-to-tor-2",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID:         "tor-port-2",
		Name:       "tor mixed 2",
		Type:       "mixed",
		Listen:     "127.0.0.1",
		Port:       33200,
		Enabled:    true,
		Outbound:   "",
		UseTorExit: true,
		TorChainID: chain.ID,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	router := gin.New()
	router.PUT("/api/proxy-chains/:id", server.updateProxyChain)

	body, err := json.Marshal(storage.ProxyChain{
		Name:    chain.Name,
		Nodes:   chain.Nodes,
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/proxy-chains/"+chain.ID, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	got := store.GetInboundPort("tor-port-2")
	if got == nil || got.Enabled {
		t.Fatalf("expected Tor-bound inbound disabled after chain disable, got %#v", got)
	}
	// Disable path keeps Tor refs (same as Outbound name kept for non-Tor) so
	// re-enabling the chain can re-enable the inbound without re-picking.
	if !got.UseTorExit || got.TorChainID != chain.ID {
		t.Fatalf("expected Tor refs preserved on disable, got %#v", got)
	}

	updated := store.GetProxyChain(chain.ID)
	if updated == nil || updated.Enabled {
		t.Fatalf("expected chain disabled, got %#v", updated)
	}
}

func TestDeleteNonTorChainStillDisablesOutboundBoundInbound(t *testing.T) {
	server, store, _ := setupTorChainCascadeServer(t)

	chain := storage.ProxyChain{
		ID:      "plain-chain",
		Name:    "hop-a-b",
		Enabled: true,
		Nodes:   []string{"entry", "entry"},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID:       "bound-port",
		Name:     "bound",
		Type:     "mixed",
		Listen:   "127.0.0.1",
		Port:     33300,
		Enabled:  true,
		Outbound: chain.Name,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	router := gin.New()
	router.DELETE("/api/proxy-chains/:id", server.deleteProxyChain)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/proxy-chains/"+chain.ID, nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	got := store.GetInboundPort("bound-port")
	if got == nil || got.Enabled {
		t.Fatalf("expected outbound-bound inbound disabled, got %#v", got)
	}
}
