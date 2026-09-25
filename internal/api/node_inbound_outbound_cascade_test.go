package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/structName/sing-box-manager-gui/internal/daemon"
	"github.com/structName/sing-box-manager-gui/internal/service"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestUpdateManualNodeRetargetsInboundOutbound(t *testing.T) {
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
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-1", Name: "direct-a", Type: "mixed",
		Listen: "127.0.0.1", Port: 18080,
		Outbound: "A", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
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

	ports := store.GetInboundPorts()
	if len(ports) != 1 {
		t.Fatalf("ports len = %d, want 1", len(ports))
	}
	if ports[0].Outbound != "A2" {
		t.Fatalf("Outbound = %q, want A2 (rename must retarget inbound)", ports[0].Outbound)
	}
	if !ports[0].Enabled {
		t.Fatal("inbound should stay enabled after rename retarget")
	}
}

func TestDeleteManualNodeDisablesInboundOutbound(t *testing.T) {
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
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-1", Name: "direct-a", Type: "mixed",
		Listen: "127.0.0.1", Port: 18081,
		Outbound: "A", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}
	// Unrelated inbound must stay enabled
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-2", Name: "direct", Type: "mixed",
		Listen: "127.0.0.1", Port: 18082,
		Outbound: "DIRECT", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort(DIRECT) error = %v", err)
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

	byID := map[string]storage.InboundPort{}
	for _, p := range store.GetInboundPorts() {
		byID[p.ID] = p
	}
	if p, ok := byID["port-1"]; !ok || p.Enabled {
		t.Fatalf("port-1 = %#v, want Enabled=false after node delete", byID["port-1"])
	}
	if p, ok := byID["port-2"]; !ok || !p.Enabled || p.Outbound != "DIRECT" {
		t.Fatalf("port-2 = %#v, want untouched Enabled DIRECT", byID["port-2"])
	}
}

func TestDeleteSubscriptionPrunesChainAndDisablesInbound(t *testing.T) {
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

	subID := "sub-1"
	if err := store.AddSubscription(storage.Subscription{
		ID: subID, Name: "airport", Enabled: true, URL: "https://example.com/sub",
		UpdatedAt: time.Now(), NodeCount: 2,
		Nodes: []storage.Node{
			{Tag: "hk-1", Type: "vmess", Server: "1.1.1.1", ServerPort: 443},
			{Tag: "jp-1", Type: "vmess", Server: "2.2.2.2", ServerPort: 443},
		},
	}); err != nil {
		t.Fatalf("AddSubscription() error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-b", Enabled: true,
		Node: storage.Node{Tag: "B", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
	}); err != nil {
		t.Fatalf("AddManualNode(B) error = %v", err)
	}
	if err := store.AddProxyChain(storage.ProxyChain{
		ID: "chain-1", Name: "hk-b", Enabled: true,
		Nodes: []string{"hk-1", "B"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "hk-1", CopyTag: storage.GenerateChainNodeCopyTag("hk-b", "hk-1"), Source: subID},
			{OriginalTag: "B", CopyTag: storage.GenerateChainNodeCopyTag("hk-b", "B"), Source: "manual"},
		},
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-hk", Name: "hk-direct", Type: "mixed",
		Listen: "127.0.0.1", Port: 18083,
		Outbound: "hk-1", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort(hk) error = %v", err)
	}

	chainSync := service.NewChainSyncService(store)
	subSvc := service.NewSubscriptionService(store, chainSync)
	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
		chainSyncSvc:   chainSync,
		subService:     subSvc,
	}

	router := gin.New()
	router.DELETE("/api/subscriptions/:id", server.deleteSubscription)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/subscriptions/"+subID, nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	if store.GetSubscription(subID) != nil {
		t.Fatal("subscription should be deleted")
	}

	chain := store.GetProxyChain("chain-1")
	if chain == nil {
		t.Fatal("chain missing")
	}
	if len(chain.Nodes) != 1 || chain.Nodes[0] != "B" {
		t.Fatalf("chain.Nodes = %#v, want [B] after subscription delete prune", chain.Nodes)
	}
	if len(chain.ChainNodes) != 1 || chain.ChainNodes[0].OriginalTag != "B" {
		t.Fatalf("chain.ChainNodes = %#v, want only B", chain.ChainNodes)
	}

	ports := store.GetInboundPorts()
	if len(ports) != 1 {
		t.Fatalf("ports len = %d, want 1", len(ports))
	}
	if ports[0].Enabled {
		t.Fatalf("inbound bound to deleted sub node must be disabled, got %#v", ports[0])
	}
	if ports[0].Outbound != "hk-1" {
		t.Fatalf("Outbound should remain hk-1 (disabled, not cleared), got %q", ports[0].Outbound)
	}
}
