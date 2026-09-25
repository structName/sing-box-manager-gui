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

func newFilterCascadeTestServer(t *testing.T) (*Server, *storage.JSONStore, *gin.Engine) {
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

func TestUpdateFilterRetargetsInboundOutbound(t *testing.T) {
	_, store, router := newFilterCascadeTestServer(t)

	if err := store.AddFilter(storage.Filter{
		ID: "flt-1", Name: "HK-Auto", Mode: "urltest", Enabled: true,
	}); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-1", Name: "via-filter", Type: "mixed",
		Listen: "127.0.0.1", Port: 18081,
		Outbound: "HK-Auto", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-2", Name: "other", Type: "mixed",
		Listen: "127.0.0.1", Port: 18082,
		Outbound: "Proxy", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort(other) error = %v", err)
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

	ports := store.GetInboundPorts()
	byID := map[string]storage.InboundPort{}
	for _, p := range ports {
		byID[p.ID] = p
	}
	if byID["port-1"].Outbound != "HK-Select" {
		t.Fatalf("port-1 Outbound = %q, want HK-Select", byID["port-1"].Outbound)
	}
	if !byID["port-1"].Enabled {
		t.Fatalf("port-1 should stay enabled after rename")
	}
	if byID["port-2"].Outbound != "Proxy" || !byID["port-2"].Enabled {
		t.Fatalf("unrelated inbound must stay untouched: %+v", byID["port-2"])
	}
}

func TestDisableFilterDisablesInboundOutbound(t *testing.T) {
	_, store, router := newFilterCascadeTestServer(t)

	if err := store.AddFilter(storage.Filter{
		ID: "flt-1", Name: "JP-Auto", Mode: "urltest", Enabled: true,
	}); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-1", Name: "via-filter", Type: "mixed",
		Listen: "127.0.0.1", Port: 18083,
		Outbound: "JP-Auto", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
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

	ports := store.GetInboundPorts()
	if len(ports) != 1 {
		t.Fatalf("ports len = %d, want 1", len(ports))
	}
	if ports[0].Enabled {
		t.Fatalf("inbound bound to disabled filter must be disabled")
	}
	if ports[0].Outbound != "JP-Auto" {
		t.Fatalf("Outbound = %q, want JP-Auto (disable keeps name)", ports[0].Outbound)
	}
}

func TestDeleteFilterDisablesInboundOutbound(t *testing.T) {
	_, store, router := newFilterCascadeTestServer(t)

	if err := store.AddFilter(storage.Filter{
		ID: "flt-1", Name: "US-Auto", Mode: "urltest", Enabled: true,
	}); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-1", Name: "via-filter", Type: "mixed",
		Listen: "127.0.0.1", Port: 18084,
		Outbound: "US-Auto", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-2", Name: "other", Type: "mixed",
		Listen: "127.0.0.1", Port: 18085,
		Outbound: "DIRECT", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort(other) error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/filters/flt-1", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	if store.GetFilter("flt-1") != nil {
		t.Fatalf("filter should be deleted")
	}

	ports := store.GetInboundPorts()
	byID := map[string]storage.InboundPort{}
	for _, p := range ports {
		byID[p.ID] = p
	}
	if byID["port-1"].Enabled {
		t.Fatalf("inbound bound to deleted filter must be disabled")
	}
	if !byID["port-2"].Enabled || byID["port-2"].Outbound != "DIRECT" {
		t.Fatalf("unrelated inbound must stay enabled: %+v", byID["port-2"])
	}
}

func TestRenameAndDisableFilterCascadesInOneUpdate(t *testing.T) {
	_, store, router := newFilterCascadeTestServer(t)

	if err := store.AddFilter(storage.Filter{
		ID: "flt-1", Name: "OldName", Mode: "urltest", Enabled: true,
	}); err != nil {
		t.Fatalf("AddFilter() error = %v", err)
	}
	if err := store.AddInboundPort(storage.InboundPort{
		ID: "port-1", Name: "via-filter", Type: "mixed",
		Listen: "127.0.0.1", Port: 18086,
		Outbound: "OldName", Enabled: true,
	}); err != nil {
		t.Fatalf("AddInboundPort() error = %v", err)
	}

	body, err := json.Marshal(storage.Filter{
		Name: "NewName", Mode: "urltest", Enabled: false,
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

	ports := store.GetInboundPorts()
	if len(ports) != 1 {
		t.Fatalf("ports len = %d, want 1", len(ports))
	}
	if ports[0].Outbound != "NewName" {
		t.Fatalf("Outbound = %q, want NewName after rename-before-disable", ports[0].Outbound)
	}
	if ports[0].Enabled {
		t.Fatalf("inbound must be disabled after rename+disable")
	}
}
