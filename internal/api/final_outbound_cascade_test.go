package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/structName/sing-box-manager-gui/internal/daemon"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestUpdateProxyChainRetargetsFinalOutbound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	settings.AutoApply = false
	settings.FinalOutbound = "old-chain"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-a", Enabled: true,
		Node: storage.Node{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	}); err != nil {
		t.Fatalf("AddManualNode(A) error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-b", Enabled: true,
		Node: storage.Node{Tag: "B", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
	}); err != nil {
		t.Fatalf("AddManualNode(B) error = %v", err)
	}
	if err := store.AddProxyChain(storage.ProxyChain{
		ID: "chain-1", Name: "old-chain", Enabled: true,
		Nodes: []string{"A", "B"},
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}
	router := gin.New()
	router.PUT("/api/proxy-chains/:id", server.updateProxyChain)

	body, err := json.Marshal(storage.ProxyChain{
		Name: "new-chain", Enabled: true, Nodes: []string{"A", "B"},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/proxy-chains/chain-1", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := store.GetSettings().FinalOutbound; got != "new-chain" {
		t.Fatalf("FinalOutbound = %q, want new-chain", got)
	}
}

func TestDeleteProxyChainResetsFinalOutbound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	settings.AutoApply = false
	settings.FinalOutbound = "doomed-chain"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-a", Enabled: true,
		Node: storage.Node{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	}); err != nil {
		t.Fatalf("AddManualNode(A) error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-b", Enabled: true,
		Node: storage.Node{Tag: "B", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
	}); err != nil {
		t.Fatalf("AddManualNode(B) error = %v", err)
	}
	if err := store.AddProxyChain(storage.ProxyChain{
		ID: "chain-1", Name: "doomed-chain", Enabled: true,
		Nodes: []string{"A", "B"},
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}
	router := gin.New()
	router.DELETE("/api/proxy-chains/:id", server.deleteProxyChain)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/proxy-chains/chain-1", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := store.GetSettings().FinalOutbound; got != "Proxy" {
		t.Fatalf("FinalOutbound = %q, want Proxy", got)
	}
}

func TestDisableProxyChainResetsFinalOutbound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	settings.AutoApply = false
	settings.FinalOutbound = "pause-chain"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-a", Enabled: true,
		Node: storage.Node{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	}); err != nil {
		t.Fatalf("AddManualNode(A) error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-b", Enabled: true,
		Node: storage.Node{Tag: "B", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
	}); err != nil {
		t.Fatalf("AddManualNode(B) error = %v", err)
	}
	if err := store.AddProxyChain(storage.ProxyChain{
		ID: "chain-1", Name: "pause-chain", Enabled: true,
		Nodes: []string{"A", "B"},
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}
	router := gin.New()
	router.PUT("/api/proxy-chains/:id", server.updateProxyChain)

	body, err := json.Marshal(storage.ProxyChain{
		Name: "pause-chain", Enabled: false, Nodes: []string{"A", "B"},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/proxy-chains/chain-1", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := store.GetSettings().FinalOutbound; got != "Proxy" {
		t.Fatalf("FinalOutbound = %q, want Proxy", got)
	}
}

func TestDeleteProxyChainLeavesUnrelatedFinalOutbound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	settings.AutoApply = false
	settings.FinalOutbound = "DIRECT"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-a", Enabled: true,
		Node: storage.Node{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	}); err != nil {
		t.Fatalf("AddManualNode(A) error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-b", Enabled: true,
		Node: storage.Node{Tag: "B", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
	}); err != nil {
		t.Fatalf("AddManualNode(B) error = %v", err)
	}
	if err := store.AddProxyChain(storage.ProxyChain{
		ID: "chain-1", Name: "other-chain", Enabled: true,
		Nodes: []string{"A", "B"},
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}
	router := gin.New()
	router.DELETE("/api/proxy-chains/:id", server.deleteProxyChain)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/proxy-chains/chain-1", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := store.GetSettings().FinalOutbound; got != "DIRECT" {
		t.Fatalf("FinalOutbound = %q, want DIRECT", got)
	}
}
