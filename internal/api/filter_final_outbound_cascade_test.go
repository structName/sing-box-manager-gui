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

func newFilterFinalOutboundTestServer(t *testing.T) (*Server, *storage.JSONStore, *gin.Engine) {
	t.Helper()
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
	router.PUT("/api/filters/:id", server.updateFilter)
	router.DELETE("/api/filters/:id", server.deleteFilter)
	return server, store, router
}

func TestUpdateFilterRetargetsFinalOutbound(t *testing.T) {
	_, store, router := newFilterFinalOutboundTestServer(t)

	settings := store.GetSettings()
	settings.FinalOutbound = "HK-Auto"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddFilter(storage.Filter{
		ID: "flt-1", Name: "HK-Auto", Mode: "urltest", Enabled: true,
	}); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}

	body, err := json.Marshal(storage.Filter{
		Name: "HK-Select", Mode: "selector", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/filters/flt-1", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := store.GetSettings().FinalOutbound; got != "HK-Select" {
		t.Fatalf("FinalOutbound = %q, want HK-Select", got)
	}
}

func TestDisableFilterResetsFinalOutbound(t *testing.T) {
	_, store, router := newFilterFinalOutboundTestServer(t)

	settings := store.GetSettings()
	settings.FinalOutbound = "JP-Auto"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddFilter(storage.Filter{
		ID: "flt-1", Name: "JP-Auto", Mode: "urltest", Enabled: true,
	}); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}

	body, err := json.Marshal(storage.Filter{
		Name: "JP-Auto", Mode: "urltest", Enabled: false,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/filters/flt-1", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := store.GetSettings().FinalOutbound; got != "Proxy" {
		t.Fatalf("FinalOutbound = %q, want Proxy", got)
	}
}

func TestDeleteFilterResetsFinalOutbound(t *testing.T) {
	_, store, router := newFilterFinalOutboundTestServer(t)

	settings := store.GetSettings()
	settings.FinalOutbound = "US-Auto"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddFilter(storage.Filter{
		ID: "flt-1", Name: "US-Auto", Mode: "urltest", Enabled: true,
	}); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/filters/flt-1", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := store.GetSettings().FinalOutbound; got != "Proxy" {
		t.Fatalf("FinalOutbound = %q, want Proxy", got)
	}
}

func TestUpdateFilterLeavesUnrelatedFinalOutbound(t *testing.T) {
	_, store, router := newFilterFinalOutboundTestServer(t)

	settings := store.GetSettings()
	settings.FinalOutbound = "Proxy"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddFilter(storage.Filter{
		ID: "flt-1", Name: "EU-Auto", Mode: "urltest", Enabled: true,
	}); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}

	body, err := json.Marshal(storage.Filter{
		Name: "EU-Select", Mode: "selector", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/filters/flt-1", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}
	if got := store.GetSettings().FinalOutbound; got != "Proxy" {
		t.Fatalf("FinalOutbound = %q, want Proxy (unrelated)", got)
	}
}
