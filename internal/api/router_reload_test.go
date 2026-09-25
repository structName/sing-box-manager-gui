package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/structName/sing-box-manager-gui/internal/daemon"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestReloadServiceRebuildsConfigBeforeSIGHUP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	markerPath := filepath.Join(dataDir, "hup.marker")
	readyPath := markerPath + ".ready"
	binPath := filepath.Join(dataDir, "sing-box")
	configPath := filepath.Join(dataDir, "generated", "config.json")

	script := `#!/bin/sh
case "$1" in
  check|version) exit 0 ;;
esac
MARKER="` + markerPath + `"
READY="` + readyPath + `"
trap 'printf reloaded >> "$MARKER"' HUP
trap 'exit 0' TERM INT
printf ready > "$READY"
while true; do
  sleep 1 &
  wait $! || true
done
`
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(binPath, []byte(script), 0755); err != nil {
		t.Fatalf("write fake sing-box: %v", err)
	}
	if err := os.WriteFile(markerPath, nil, 0644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(`{"stale":true}`), 0644); err != nil {
		t.Fatalf("write stale config: %v", err)
	}

	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	settings := store.GetSettings()
	settings.AutoApply = false
	settings.ConfigPath = "generated/config.json"
	settings.ClashUIEnabled = false
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	pm := daemon.NewProcessManager(binPath, configPath, dataDir)
	if err := pm.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() { _ = pm.Stop() })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(readyPath); err == nil && strings.Contains(string(data), "ready") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if data, err := os.ReadFile(readyPath); err != nil || !strings.Contains(string(data), "ready") {
		t.Fatalf("fake sing-box never became ready for SIGHUP: %v %q", err, data)
	}

	server := &Server{
		store:          store,
		processManager: pm,
		baseDir:        dataDir,
	}

	router := gin.New()
	router.POST("/api/service/reload", server.reloadService)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/service/reload", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("reload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	fresh, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config after reload: %v", err)
	}
	if strings.Contains(string(fresh), `"stale":true`) {
		t.Fatal("reloadService left stale config on disk; expected rebuild before SIGHUP")
	}
	if !strings.Contains(string(fresh), "inbounds") && !strings.Contains(string(fresh), "outbounds") {
		t.Fatalf("rebuilt config looks unexpected: %s", fresh)
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(markerPath)
		if err == nil && strings.Contains(string(data), "reloaded") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	data, _ := os.ReadFile(markerPath)
	t.Fatalf("timed out waiting for SIGHUP after reload, marker=%q", data)
}

func TestRebuildConfigThenReloadSkipsActionOnBuildError(t *testing.T) {
	buildErr := errors.New("build failed")
	reloadCalled := false

	err := rebuildConfigAndRestart(func() error {
		return buildErr
	}, func() error {
		reloadCalled = true
		return nil
	})
	if !errors.Is(err, buildErr) {
		t.Fatalf("expected build error %v, got %v", buildErr, err)
	}
	if reloadCalled {
		t.Fatal("Reload should not run when rebuild fails")
	}
}
