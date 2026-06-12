package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/structName/sing-box-manager-gui/internal/service"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestCheckTorDiagnosticsReturnsSeparatedSegmentAndFullChain(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID:      "chain-tor",
		Name:    "entry-tor-post",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag, "post"},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID:         "port-tor",
		Type:       "mixed",
		Listen:     "127.0.0.1",
		Port:       2081,
		Enabled:    true,
		UseTorExit: true,
		TorChainID: chain.ID,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	server := &Server{
		store:             store,
		torDiagnosticsSvc: service.NewTorDiagnosticsService(store),
	}
	router := gin.New()
	router.POST("/api/proxy-chains/:id/tor-diagnostics", server.checkTorDiagnostics)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/proxy-chains/chain-tor/tor-diagnostics", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data service.TorDiagnosticResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if response.Data.ChainID != chain.ID {
		t.Fatalf("chain_id = %q, want %q", response.Data.ChainID, chain.ID)
	}
	if response.Data.TorSegment.Method != "temporary_segment" {
		t.Fatalf("tor segment method = %q, want temporary_segment", response.Data.TorSegment.Method)
	}
	if response.Data.FullChain.Method != "main_inbound" {
		t.Fatalf("full chain method = %q, want main_inbound", response.Data.FullChain.Method)
	}
	if response.Data.TorSegment.Status == "" || response.Data.FullChain.Status == "" {
		t.Fatalf("diagnostics should include both statuses, got %#v", response.Data)
	}
}
