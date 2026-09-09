package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/structName/sing-box-manager-gui/internal/daemon"
	"github.com/structName/sing-box-manager-gui/internal/service"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestUpdateManualNodeRetargetsProxyChainTags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	settings.AutoApply = false
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
		ID: "chain-1", Name: "ab-chain", Enabled: true,
		Nodes: []string{"A", "B"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "A", CopyTag: storage.GenerateChainNodeCopyTag("ab-chain", "A"), Source: "manual"},
			{OriginalTag: "B", CopyTag: storage.GenerateChainNodeCopyTag("ab-chain", "B"), Source: "manual"},
		},
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
		chainSyncSvc:   service.NewChainSyncService(store),
	}

	router := gin.New()
	router.PUT("/api/manual-nodes/:id", server.updateManualNode)

	body, err := json.Marshal(storage.ManualNode{
		Enabled: true,
		Node:    storage.Node{Tag: "A2", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/manual-nodes/node-a", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	saved := store.GetProxyChain("chain-1")
	if saved == nil {
		t.Fatal("chain missing")
	}
	if len(saved.Nodes) != 2 || saved.Nodes[0] != "A2" || saved.Nodes[1] != "B" {
		t.Fatalf("nodes = %#v, want [A2 B]", saved.Nodes)
	}
	wantCopy := storage.GenerateChainNodeCopyTag("ab-chain", "A2")
	if saved.ChainNodes[0].OriginalTag != "A2" || saved.ChainNodes[0].CopyTag != wantCopy {
		t.Fatalf("ChainNodes[0] = %#v, want OriginalTag=A2 CopyTag=%s", saved.ChainNodes[0], wantCopy)
	}
}

func TestDeleteManualNodePrunesProxyChainTags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	settings.AutoApply = false
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
		ID: "chain-1", Name: "ab-chain", Enabled: true,
		Nodes: []string{"A", "B"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "A", CopyTag: "ab-chain-A", Source: "manual"},
			{OriginalTag: "B", CopyTag: "ab-chain-B", Source: "manual"},
		},
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
		chainSyncSvc:   service.NewChainSyncService(store),
	}

	router := gin.New()
	router.DELETE("/api/manual-nodes/:id", server.deleteManualNode)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/manual-nodes/node-a", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	saved := store.GetProxyChain("chain-1")
	if saved == nil {
		t.Fatal("chain missing")
	}
	if len(saved.Nodes) != 1 || saved.Nodes[0] != "B" {
		t.Fatalf("nodes = %#v, want [B]", saved.Nodes)
	}
	if len(saved.ChainNodes) != 1 || saved.ChainNodes[0].OriginalTag != "B" {
		t.Fatalf("chain nodes = %#v, want only B", saved.ChainNodes)
	}
}

func TestUpdateManualNodeDisableDoesNotPruneChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	settings.AutoApply = false
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
		ID: "chain-1", Name: "ab-chain", Enabled: true,
		Nodes: []string{"A", "B"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "A", CopyTag: "ab-chain-A", Source: "manual"},
			{OriginalTag: "B", CopyTag: "ab-chain-B", Source: "manual"},
		},
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
		chainSyncSvc:   service.NewChainSyncService(store),
	}

	router := gin.New()
	router.PUT("/api/manual-nodes/:id", server.updateManualNode)

	body, err := json.Marshal(storage.ManualNode{
		Enabled: false,
		Node:    storage.Node{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/manual-nodes/node-a", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	saved := store.GetProxyChain("chain-1")
	if len(saved.Nodes) != 2 || saved.Nodes[0] != "A" || saved.Nodes[1] != "B" {
		t.Fatalf("disable must keep chain refs, got %#v", saved.Nodes)
	}
}
