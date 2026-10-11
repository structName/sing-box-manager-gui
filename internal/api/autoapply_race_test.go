package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/structName/sing-box-manager-gui/internal/daemon"
	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func installStubSingBox(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sing-box")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(sing-box stub) error = %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestSaveConfigFileAtomicReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	server := &Server{}

	const writers = 16
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func(i int) {
			defer wg.Done()
			payload, _ := json.Marshal(map[string]int{"n": i})
			if err := server.saveConfigFile(path, string(payload)); err != nil {
				t.Errorf("saveConfigFile() error = %v", err)
			}
		}(i)
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var decoded map[string]int
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("final config is not valid JSON: %v\n%s", err, data)
	}
	if _, ok := decoded["n"]; !ok {
		t.Fatalf("decoded = %#v, want key n", decoded)
	}
}

func TestAutoApplyConfigSerializesConcurrentCalls(t *testing.T) {
	installStubSingBox(t)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	settings := store.GetSettings()
	settings.AutoApply = true
	settings.ConfigPath = "generated/config.json"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID:      "node-a",
		Enabled: true,
		Node: storage.Node{
			Tag:        "A",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	pm := daemon.NewProcessManager("sing-box", filepath.Join(dataDir, "generated", "config.json"), dataDir)
	server := &Server{
		store:          store,
		processManager: pm,
		baseDir:        dataDir,
	}

	var (
		wg       sync.WaitGroup
		failures atomic.Int32
	)

	const goroutines = 12
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if err := server.autoApplyConfig(); err != nil {
				failures.Add(1)
				t.Errorf("autoApplyConfig() error = %v", err)
			}
		}()
	}
	wg.Wait()

	if failures.Load() != 0 {
		t.Fatalf("autoApplyConfig failures = %d", failures.Load())
	}

	configPath := filepath.Join(dataDir, "generated", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", configPath, err)
	}
	if !json.Valid(data) {
		t.Fatalf("config is not valid JSON after concurrent apply:\n%s", data)
	}
}

func TestBuildAndSaveCurrentConfigHoldsApplyLockAcrossSave(t *testing.T) {
	installStubSingBox(t)

	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	settings := store.GetSettings()
	settings.ConfigPath = "generated/config.json"
	if err := store.UpdateSettings(settings); err != nil {
		t.Fatalf("UpdateSettings() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}

	started := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.applyMu.Lock()
		close(started)
		<-release
		server.applyMu.Unlock()
	}()
	<-started

	done := make(chan error, 1)
	go func() {
		done <- server.buildAndSaveCurrentConfig()
	}()

	select {
	case err := <-done:
		t.Fatalf("buildAndSaveCurrentConfig returned while applyMu held: %v", err)
	case <-time.After(50 * time.Millisecond):
		// expected: blocked on applyMu
	}

	close(release)
	wg.Wait()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("buildAndSaveCurrentConfig() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("buildAndSaveCurrentConfig did not complete after unlock")
	}
}
