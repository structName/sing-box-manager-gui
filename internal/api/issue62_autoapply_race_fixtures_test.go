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

// Issue #62 regression fixtures for #50 (concurrent autoApply race).
// Contracts match #64: applyMu serializes rebuild+save+restart; saveConfigFile is atomic.
// Companion CopyTag (#48) fixtures: storage/issue62_* and proxy_chain_copytag_test.go.
// (#49 portion of #62 is covered by PR #66.)

func TestIssue62_AutoApplyRaceFixtures(t *testing.T) {
	t.Run("atomic_save_survives_parallel_writers", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.json")
		server := &Server{}

		const writers = 24
		var wg sync.WaitGroup
		wg.Add(writers)
		for i := 0; i < writers; i++ {
			go func(i int) {
				defer wg.Done()
				payload, err := json.Marshal(map[string]any{"n": i, "pad": i * 7})
				if err != nil {
					t.Errorf("Marshal: %v", err)
					return
				}
				if err := server.saveConfigFile(path, string(payload)); err != nil {
					t.Errorf("saveConfigFile: %v", err)
				}
			}(i)
		}
		wg.Wait()

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if !json.Valid(data) {
			t.Fatalf("#50: torn write left invalid JSON:\n%s", data)
		}
	})

	t.Run("concurrent_auto_apply_no_failures", func(t *testing.T) {
		installStubSingBox(t)

		dataDir := t.TempDir()
		store, err := storage.NewJSONStore(dataDir)
		if err != nil {
			t.Fatalf("NewJSONStore: %v", err)
		}
		settings := store.GetSettings()
		settings.AutoApply = true
		settings.ConfigPath = "generated/config.json"
		if err := store.UpdateSettings(settings); err != nil {
			t.Fatalf("UpdateSettings: %v", err)
		}
		if err := store.AddManualNode(storage.ManualNode{
			ID: "node-a", Enabled: true,
			Node: storage.Node{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
		}); err != nil {
			t.Fatalf("AddManualNode: %v", err)
		}

		server := &Server{
			store:          store,
			processManager: daemon.NewProcessManager("sing-box", filepath.Join(dataDir, "generated", "config.json"), dataDir),
			baseDir:        dataDir,
		}

		var failures atomic.Int32
		const n = 16
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				if err := server.autoApplyConfig(); err != nil {
					failures.Add(1)
					t.Errorf("autoApplyConfig: %v", err)
				}
			}()
		}
		wg.Wait()
		if failures.Load() != 0 {
			t.Fatalf("#50: autoApplyConfig failures = %d", failures.Load())
		}
		data, err := os.ReadFile(filepath.Join(dataDir, "generated", "config.json"))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if !json.Valid(data) {
			t.Fatalf("#50: config not valid JSON after concurrent apply")
		}
	})

	t.Run("build_and_save_blocks_on_apply_mu", func(t *testing.T) {
		installStubSingBox(t)

		dataDir := t.TempDir()
		store, err := storage.NewJSONStore(dataDir)
		if err != nil {
			t.Fatalf("NewJSONStore: %v", err)
		}
		settings := store.GetSettings()
		settings.ConfigPath = "generated/config.json"
		if err := store.UpdateSettings(settings); err != nil {
			t.Fatalf("UpdateSettings: %v", err)
		}
		server := &Server{
			store:          store,
			processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
			baseDir:        dataDir,
		}

		started := make(chan struct{})
		release := make(chan struct{})
		var holder sync.WaitGroup
		holder.Add(1)
		go func() {
			defer holder.Done()
			server.applyMu.Lock()
			close(started)
			<-release
			server.applyMu.Unlock()
		}()
		<-started

		done := make(chan error, 1)
		go func() { done <- server.buildAndSaveCurrentConfig() }()

		select {
		case err := <-done:
			t.Fatalf("#50: buildAndSaveCurrentConfig returned while applyMu held: %v", err)
		case <-time.After(50 * time.Millisecond):
		}

		close(release)
		holder.Wait()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("buildAndSaveCurrentConfig: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("buildAndSaveCurrentConfig did not complete after unlock")
		}
	})
}
