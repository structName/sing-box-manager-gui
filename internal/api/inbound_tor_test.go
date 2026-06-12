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
	"github.com/xiaobei/singbox-manager/internal/daemon"
	"github.com/xiaobei/singbox-manager/internal/storage"
)

func TestAddInboundPortRejectsMoreThanThreeActiveTorChains(t *testing.T) {
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

	for index, chain := range []storage.ProxyChain{
		{ID: "chain-1", Name: "tor-alpha", Enabled: true, Nodes: []string{"entry", storage.ChainTorNodeTag}},
		{ID: "chain-2", Name: "tor-beta", Enabled: true, Nodes: []string{"entry", storage.ChainTorNodeTag}},
		{ID: "chain-3", Name: "tor-gamma", Enabled: true, Nodes: []string{"entry", storage.ChainTorNodeTag}},
		{ID: "chain-4", Name: "tor-delta", Enabled: true, Nodes: []string{"entry", storage.ChainTorNodeTag}},
	} {
		if err := store.AddProxyChain(chain); err != nil {
			t.Fatalf("AddProxyChain() error = %v", err)
		}
		if index < 3 {
			if err := store.AddInboundPort(storage.InboundPort{
				ID:         chain.ID + "-port",
				Name:       chain.Name + " port",
				Type:       "mixed",
				Listen:     "127.0.0.1",
				Port:       32000 + index,
				Enabled:    true,
				UseTorExit: true,
				TorChainID: chain.ID,
			}); err != nil {
				t.Fatalf("AddInboundPort() error = %v", err)
			}
		}
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}
	router := gin.New()
	router.POST("/api/inbound-ports", server.addInboundPort)

	requestBody := storage.InboundPort{
		Name:       "fourth tor port",
		Type:       "mixed",
		Listen:     "127.0.0.1",
		Port:       32099,
		Enabled:    true,
		UseTorExit: true,
		TorChainID: "chain-4",
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/inbound-ports", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	for _, want := range []string{"最多启用 3 条 Tor 链路", "tor-alpha", "tor-beta", "tor-gamma"} {
		if !bytes.Contains(recorder.Body.Bytes(), []byte(want)) {
			t.Fatalf("expected error to include %q, got %s", want, recorder.Body.String())
		}
	}
	if saved := store.GetInboundPorts(); len(saved) != 3 {
		t.Fatalf("rejected inbound should not persist, got %d ports", len(saved))
	}
}
