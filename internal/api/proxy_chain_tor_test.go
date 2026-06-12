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

func TestAddProxyChainPersistsNodeToTorChain(t *testing.T) {
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

	router := gin.New()
	router.POST("/api/proxy-chains", server.addProxyChain)

	requestBody := storage.ProxyChain{
		Name:    "entry-to-tor",
		Nodes:   []string{"entry", storage.ChainTorNodeTag},
		Enabled: true,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/proxy-chains", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data storage.ProxyChain `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(response.Data.Nodes) != 2 || response.Data.Nodes[1] != storage.ChainTorNodeTag {
		t.Fatalf("expected Tor sentinel in response nodes, got %#v", response.Data.Nodes)
	}
	if len(response.Data.ChainNodes) != 2 {
		t.Fatalf("expected two chain nodes, got %#v", response.Data.ChainNodes)
	}
	torNode := response.Data.ChainNodes[1]
	if torNode.OriginalTag != storage.ChainTorNodeTag {
		t.Fatalf("expected Tor chain node original tag, got %#v", torNode)
	}
	if torNode.CopyTag != storage.ChainTorDisplayName {
		t.Fatalf("expected Tor display copy tag, got %#v", torNode)
	}
	if torNode.Source != storage.ChainTorNodeSource {
		t.Fatalf("expected Tor source marker, got %#v", torNode)
	}

	saved := store.GetProxyChains()
	if len(saved) != 1 {
		t.Fatalf("expected one saved chain, got %d", len(saved))
	}
	if saved[0].Nodes[1] != storage.ChainTorNodeTag {
		t.Fatalf("expected saved Tor sentinel, got %#v", saved[0].Nodes)
	}
}

func TestAddProxyChainAcceptsPostTorNode(t *testing.T) {
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

	for _, nodeTag := range []string{"entry", "post"} {
		if err := store.AddManualNode(storage.ManualNode{
			ID:      nodeTag + "-node",
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

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}

	router := gin.New()
	router.POST("/api/proxy-chains", server.addProxyChain)

	requestBody := storage.ProxyChain{
		Name:    "entry-tor-post",
		Nodes:   []string{"entry", storage.ChainTorNodeTag, "post"},
		Enabled: true,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/proxy-chains", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data storage.ProxyChain `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got := response.Data.Nodes; len(got) != 3 || got[0] != "entry" || got[1] != storage.ChainTorNodeTag || got[2] != "post" {
		t.Fatalf("expected node -> Tor -> post nodes, got %#v", got)
	}
	if len(response.Data.ChainNodes) != 3 {
		t.Fatalf("expected three chain nodes, got %#v", response.Data.ChainNodes)
	}
	if response.Data.ChainNodes[2].OriginalTag != "post" {
		t.Fatalf("expected post chain node metadata, got %#v", response.Data.ChainNodes[2])
	}
}

func TestAddProxyChainRejectsInvalidTorPlacement(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		nodes       []string
		wantMessage string
	}{
		{
			name:        "tor first",
			nodes:       []string{storage.ChainTorNodeTag, "entry"},
			wantMessage: "不能作为第一个",
		},
		{
			name:        "multiple tor nodes",
			nodes:       []string{"entry", storage.ChainTorNodeTag, storage.ChainTorNodeTag},
			wantMessage: "只能包含一个",
		},
		{
			name:        "missing tor runtime",
			nodes:       []string{"entry", storage.ChainTorNodeTag},
			wantMessage: "设置",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dataDir := t.TempDir()
			store, err := storage.NewJSONStore(dataDir)
			if err != nil {
				t.Fatalf("NewJSONStore() error = %v", err)
			}
			settings := store.GetSettings()
			settings.AutoApply = false
			if tt.name != "missing tor runtime" {
				torPath := filepath.Join(dataDir, "tor")
				if err := os.WriteFile(torPath, []byte("#!/bin/sh\n"), 0755); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
				settings.TorEnabled = true
				settings.TorExecutablePath = torPath
			}
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
			router := gin.New()
			router.POST("/api/proxy-chains", server.addProxyChain)

			requestBody := storage.ProxyChain{
				Name:    "invalid-tor-chain",
				Nodes:   tt.nodes,
				Enabled: true,
			}
			body, err := json.Marshal(requestBody)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/proxy-chains", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
			}
			if !bytes.Contains(recorder.Body.Bytes(), []byte(tt.wantMessage)) {
				t.Fatalf("expected error containing %q, got %s", tt.wantMessage, recorder.Body.String())
			}
			if saved := store.GetProxyChains(); len(saved) != 0 {
				t.Fatalf("invalid Tor chain should not persist, got %#v", saved)
			}
		})
	}
}

func TestUpdateProxyChainRejectsInvalidTorPlacementWithoutChangingSavedChain(t *testing.T) {
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

	chain := storage.ProxyChain{
		ID:      "chain-1",
		Name:    "plain-chain",
		Nodes:   []string{"entry", "exit"},
		Enabled: true,
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}
	router := gin.New()
	router.PUT("/api/proxy-chains/:id", server.updateProxyChain)

	updateBody := storage.ProxyChain{
		Name:    "plain-chain",
		Nodes:   []string{storage.ChainTorNodeTag, "entry"},
		Enabled: true,
	}
	body, err := json.Marshal(updateBody)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/proxy-chains/chain-1", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if saved := store.GetProxyChain("chain-1"); saved == nil || saved.Nodes[0] != "entry" || saved.Nodes[1] != "exit" {
		t.Fatalf("invalid update should not change saved chain, got %#v", saved)
	}
}

func TestAddProxyChainRejectsRegularAutoEntry(t *testing.T) {
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

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}
	router := gin.New()
	router.POST("/api/proxy-chains", server.addProxyChain)

	requestBody := storage.ProxyChain{
		Name:    "unsupported-auto-chain",
		Nodes:   []string{storage.ChainAutoNodeTag, "entry"},
		Enabled: true,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/proxy-chains", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("Auto 自动选择只能用于 Tor 链路")) {
		t.Fatalf("expected unsupported auto error, got %s", recorder.Body.String())
	}
	if saved := store.GetProxyChains(); len(saved) != 0 {
		t.Fatalf("regular auto chain should not persist, got %#v", saved)
	}
}
