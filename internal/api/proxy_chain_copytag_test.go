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

func TestAddProxyChainRejectsDuplicateName(t *testing.T) {
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

	for _, tag := range []string{"A", "B"} {
		if err := store.AddManualNode(storage.ManualNode{
			ID:      "node-" + tag,
			Enabled: true,
			Node: storage.Node{
				Tag:        tag,
				Type:       "socks",
				Server:     "127.0.0.1",
				ServerPort: 1080,
			},
		}); err != nil {
			t.Fatalf("AddManualNode(%s) error = %v", tag, err)
		}
	}

	if err := store.AddProxyChain(storage.ProxyChain{
		ID:      "existing",
		Name:    "shared-name",
		Enabled: true,
		Nodes:   []string{"A", "B"},
	}); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	server := &Server{
		store:          store,
		processManager: daemon.NewProcessManager("sing-box", "config.json", dataDir),
		baseDir:        dataDir,
	}
	router := gin.New()
	router.POST("/api/proxy-chains", server.addProxyChain)

	body, err := json.Marshal(storage.ProxyChain{
		Name:    "shared-name",
		Nodes:   []string{"A", "B"},
		Enabled: true,
	})
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
}

func TestGenerateChainNodesUsesHopIndexedCopyTags(t *testing.T) {
	dataDir := t.TempDir()
	store, err := storage.NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	for _, tag := range []string{"A", "B"} {
		if err := store.AddManualNode(storage.ManualNode{
			ID:      "node-" + tag,
			Enabled: true,
			Node: storage.Node{
				Tag:        tag,
				Type:       "socks",
				Server:     "127.0.0.1",
				ServerPort: 1080,
			},
		}); err != nil {
			t.Fatalf("AddManualNode(%s) error = %v", tag, err)
		}
	}

	server := &Server{store: store, baseDir: dataDir}
	nodes := server.generateChainNodes("aba", []string{"A", "B", "A"})
	if len(nodes) != 3 {
		t.Fatalf("len = %d, want 3", len(nodes))
	}
	want := []string{
		storage.GenerateChainNodeCopyTag("aba", "A", 0),
		storage.GenerateChainNodeCopyTag("aba", "B", 1),
		storage.GenerateChainNodeCopyTag("aba", "A", 2),
	}
	for i, w := range want {
		if nodes[i].CopyTag != w {
			t.Fatalf("nodes[%d].CopyTag = %q, want %q", i, nodes[i].CopyTag, w)
		}
	}
	if nodes[0].CopyTag == nodes[2].CopyTag {
		t.Fatal("repeated hop A must get distinct CopyTags")
	}
}
