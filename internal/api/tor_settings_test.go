package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xiaobei/singbox-manager/internal/daemon"
	"github.com/xiaobei/singbox-manager/internal/storage"
)

func TestUpdateSettingsRejectsMissingTorExecutablePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}

	router := gin.New()
	router.PUT("/api/settings", server.updateSettings)

	settings := storage.DefaultSettings()
	settings.AutoApply = false
	settings.TorEnabled = true
	settings.TorExecutablePath = "/path/that/does/not/exist/tor"

	body, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "Tor") || !strings.Contains(recorder.Body.String(), "不存在") {
		t.Fatalf("expected clear Tor missing-path error, got %s", recorder.Body.String())
	}

	saved := store.GetSettings()
	if saved.TorEnabled {
		t.Fatal("invalid Tor settings should not be persisted")
	}
	if saved.TorExecutablePath != "" {
		t.Fatalf("invalid Tor path should not be persisted, got %q", saved.TorExecutablePath)
	}
}

func TestUpdateSettingsAllowsDisabledTorWithStaleExecutablePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}

	router := gin.New()
	router.PUT("/api/settings", server.updateSettings)

	settings := storage.DefaultSettings()
	settings.AutoApply = false
	settings.TorEnabled = false
	settings.TorExecutablePath = "/path/that/used/to/exist/tor"

	body, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	saved := store.GetSettings()
	if saved.TorEnabled {
		t.Fatal("Tor should remain disabled")
	}
	if saved.TorExecutablePath != settings.TorExecutablePath {
		t.Fatalf("disabled Tor path should be retained, got %q", saved.TorExecutablePath)
	}
}

func TestValidateTorExecutableReportsExecutableStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	server := &Server{store: store, baseDir: dataDir}
	router := gin.New()
	router.POST("/api/tor/validate", server.validateTorExecutable)

	torPath := filepath.Join(dataDir, "tor")
	if err := os.WriteFile(torPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	body, err := json.Marshal(gin.H{"path": torPath})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tor/validate", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			Valid bool   `json:"valid"`
			Path  string `json:"path"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !response.Data.Valid {
		t.Fatalf("expected executable path to be valid, got %s", recorder.Body.String())
	}
	if response.Data.Path != torPath {
		t.Fatalf("expected path %q, got %q", torPath, response.Data.Path)
	}
}

func TestDetectTorExecutablePersistsSingleDetectedPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	torPath := filepath.Join(dataDir, "bin", "tor")
	if err := os.MkdirAll(filepath.Dir(torPath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(torPath, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	server := &Server{
		store:             store,
		baseDir:           dataDir,
		torDetectionPaths: []string{torPath},
		processManager:    daemon.NewProcessManager("sing-box", "config.json", dataDir),
	}
	router := gin.New()
	router.POST("/api/tor/detect", server.detectTorExecutable)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tor/detect", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			State     string   `json:"state"`
			Paths     []string `json:"paths"`
			Persisted bool     `json:"persisted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response.Data.State != "single" || !response.Data.Persisted {
		t.Fatalf("expected single persisted detection, got %s", recorder.Body.String())
	}
	if len(response.Data.Paths) != 1 || response.Data.Paths[0] != torPath {
		t.Fatalf("expected detected path %q, got %#v", torPath, response.Data.Paths)
	}

	saved := store.GetSettings()
	if saved.TorExecutablePath != torPath {
		t.Fatalf("expected detected path to persist, got %q", saved.TorExecutablePath)
	}
}

func TestDetectTorExecutableReturnsManualStateWhenNoPathExists(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	server := &Server{
		store:             store,
		baseDir:           dataDir,
		torDetectionPaths: []string{filepath.Join(dataDir, "missing-tor")},
	}
	router := gin.New()
	router.POST("/api/tor/detect", server.detectTorExecutable)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tor/detect", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			State     string   `json:"state"`
			Paths     []string `json:"paths"`
			Persisted bool     `json:"persisted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response.Data.State != "manual" || response.Data.Persisted || len(response.Data.Paths) != 0 {
		t.Fatalf("expected manual state without persistence, got %s", recorder.Body.String())
	}
}

func TestDetectTorExecutableReturnsMultipleChoicesWithoutPersisting(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	firstPath := filepath.Join(dataDir, "first", "tor")
	secondPath := filepath.Join(dataDir, "second", "tor")
	for _, torPath := range []string{firstPath, secondPath} {
		if err := os.MkdirAll(filepath.Dir(torPath), 0755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(torPath, []byte("#!/bin/sh\n"), 0755); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	server := &Server{
		store:             store,
		baseDir:           dataDir,
		torDetectionPaths: []string{firstPath, secondPath},
	}
	router := gin.New()
	router.POST("/api/tor/detect", server.detectTorExecutable)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/tor/detect", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data struct {
			State     string   `json:"state"`
			Paths     []string `json:"paths"`
			Persisted bool     `json:"persisted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if response.Data.State != "multiple" || response.Data.Persisted {
		t.Fatalf("expected multiple state without persistence, got %s", recorder.Body.String())
	}
	if len(response.Data.Paths) != 2 {
		t.Fatalf("expected two detected paths, got %#v", response.Data.Paths)
	}

	saved := store.GetSettings()
	if saved.TorExecutablePath != "" {
		t.Fatalf("multiple choices should not persist automatically, got %q", saved.TorExecutablePath)
	}
}
