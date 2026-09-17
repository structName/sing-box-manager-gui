package service

import (
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestRetargetNodeTagRewritesChainNodesAndCopyTag(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
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
		t.Fatalf("AddManualNode(A) error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID:      "node-b",
		Enabled: true,
		Node: storage.Node{
			Tag:        "B",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1081,
		},
	}); err != nil {
		t.Fatalf("AddManualNode(B) error = %v", err)
	}

	chain := storage.ProxyChain{
		ID:      "chain-1",
		Name:    "ab-chain",
		Enabled: true,
		Nodes:   []string{"A", "B"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "A", CopyTag: storage.GenerateChainNodeCopyTag("ab-chain", "A"), Source: "manual"},
			{OriginalTag: "B", CopyTag: storage.GenerateChainNodeCopyTag("ab-chain", "B"), Source: "manual"},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	// Simulate rename A -> A2 in storage first (API does Update then Retarget)
	if err := store.UpdateManualNode(storage.ManualNode{
		ID:      "node-a",
		Enabled: true,
		Node: storage.Node{
			Tag:        "A2",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
	}); err != nil {
		t.Fatalf("UpdateManualNode() error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.RetargetNodeTag("A", "A2"); err != nil {
		t.Fatalf("RetargetNodeTag() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	if saved == nil {
		t.Fatal("chain should still exist")
	}
	wantNodes := []string{"A2", "B"}
	if len(saved.Nodes) != len(wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", saved.Nodes, wantNodes)
	}
	for i, want := range wantNodes {
		if saved.Nodes[i] != want {
			t.Fatalf("nodes = %#v, want %#v", saved.Nodes, wantNodes)
		}
	}
	if len(saved.ChainNodes) != 2 {
		t.Fatalf("chain nodes = %#v, want 2 entries", saved.ChainNodes)
	}
	if saved.ChainNodes[0].OriginalTag != "A2" {
		t.Fatalf("ChainNodes[0].OriginalTag = %q, want A2", saved.ChainNodes[0].OriginalTag)
	}
	wantCopy := storage.GenerateChainNodeCopyTag("ab-chain", "A2")
	if saved.ChainNodes[0].CopyTag != wantCopy {
		t.Fatalf("ChainNodes[0].CopyTag = %q, want %q", saved.ChainNodes[0].CopyTag, wantCopy)
	}
	if saved.ChainNodes[1].OriginalTag != "B" {
		t.Fatalf("ChainNodes[1] should remain B: %#v", saved.ChainNodes[1])
	}
}

func TestRetargetNodeTagSkipsSpecialTags(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID:      "entry-node",
		Enabled: true,
		Node:    storage.Node{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID:      "chain-1",
		Name:    "entry-tor",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "entry", CopyTag: "entry-tor-entry", Source: "manual"},
			{OriginalTag: storage.ChainTorNodeTag, CopyTag: storage.ChainTorDisplayName, Source: storage.ChainTorNodeSource},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.RetargetNodeTag(storage.ChainTorNodeTag, "not-tor"); err != nil {
		t.Fatalf("RetargetNodeTag(special) error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	if saved.Nodes[1] != storage.ChainTorNodeTag {
		t.Fatalf("Tor tag should not be retargeted: %#v", saved.Nodes)
	}
}

func TestSyncChainNodesPrunesDeletedManualNode(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-a", Enabled: true,
		Node: storage.Node{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	}); err != nil {
		t.Fatalf("AddManualNode(A) error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-b", Enabled: true,
		Node: storage.Node{Tag: "B", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
	}); err != nil {
		t.Fatalf("AddManualNode(B) error = %v", err)
	}

	chain := storage.ProxyChain{
		ID: "chain-1", Name: "ab-chain", Enabled: true,
		Nodes: []string{"A", "B"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "A", CopyTag: "ab-chain-A", Source: "manual"},
			{OriginalTag: "B", CopyTag: "ab-chain-B", Source: "manual"},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	if err := store.DeleteManualNode("node-a"); err != nil {
		t.Fatalf("DeleteManualNode() error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.SyncChainNodes(); err != nil {
		t.Fatalf("SyncChainNodes() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	if saved == nil {
		t.Fatal("chain should still exist")
	}
	if len(saved.Nodes) != 1 || saved.Nodes[0] != "B" {
		t.Fatalf("nodes = %#v, want [B]", saved.Nodes)
	}
	if len(saved.ChainNodes) != 1 || saved.ChainNodes[0].OriginalTag != "B" {
		t.Fatalf("chain nodes = %#v, want only B", saved.ChainNodes)
	}
}

func TestSyncChainNodesKeepsDisabledManualNodeRefs(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-a", Enabled: true,
		Node: storage.Node{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	}); err != nil {
		t.Fatalf("AddManualNode(A) error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "node-b", Enabled: true,
		Node: storage.Node{Tag: "B", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
	}); err != nil {
		t.Fatalf("AddManualNode(B) error = %v", err)
	}

	chain := storage.ProxyChain{
		ID: "chain-1", Name: "ab-chain", Enabled: true,
		Nodes: []string{"A", "B"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "A", CopyTag: "ab-chain-A", Source: "manual"},
			{OriginalTag: "B", CopyTag: "ab-chain-B", Source: "manual"},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	// Disable A — must NOT prune chain refs on Sync
	if err := store.UpdateManualNode(storage.ManualNode{
		ID: "node-a", Enabled: false,
		Node: storage.Node{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
	}); err != nil {
		t.Fatalf("UpdateManualNode(disable) error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.SyncChainNodes(); err != nil {
		t.Fatalf("SyncChainNodes() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	wantNodes := []string{"A", "B"}
	if len(saved.Nodes) != 2 || saved.Nodes[0] != "A" || saved.Nodes[1] != "B" {
		t.Fatalf("nodes = %#v, want %#v (disabled manual must be kept)", saved.Nodes, wantNodes)
	}
}

func TestRemoveNodeTagPrunesFromChain(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID: "chain-1", Name: "ab-chain", Enabled: true,
		Nodes: []string{"A", "B", storage.ChainTorNodeTag},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "A", CopyTag: "ab-chain-A", Source: "manual"},
			{OriginalTag: "B", CopyTag: "ab-chain-B", Source: "manual"},
			{OriginalTag: storage.ChainTorNodeTag, CopyTag: storage.ChainTorDisplayName, Source: storage.ChainTorNodeSource},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.RemoveNodeTag("A"); err != nil {
		t.Fatalf("RemoveNodeTag() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	wantNodes := []string{"B", storage.ChainTorNodeTag}
	if len(saved.Nodes) != len(wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", saved.Nodes, wantNodes)
	}
	for i, want := range wantNodes {
		if saved.Nodes[i] != want {
			t.Fatalf("nodes = %#v, want %#v", saved.Nodes, wantNodes)
		}
	}
}
