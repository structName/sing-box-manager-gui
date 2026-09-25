package service

import (
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestSyncChainNodesForSubscriptionPrunesEmptySourceStaleHop(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	// Subscription after refresh: stale-node removed, keep-node remains.
	sub := storage.Subscription{
		ID:      "sub-1",
		Name:    "test-sub",
		Enabled: true,
		Nodes: []storage.Node{
			{Tag: "keep-node", Type: "socks", Server: "127.0.0.1", ServerPort: 1081, Source: "sub-1"},
		},
	}
	if err := store.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID:      "chain-1",
		Name:    "mixed-chain",
		Enabled: true,
		Nodes:   []string{"stale-node", "keep-node"},
		ChainNodes: []storage.ChainNode{
			// Historical empty Source — tag no longer exists anywhere.
			{OriginalTag: "stale-node", CopyTag: "mixed-chain-stale-node", Source: ""},
			{OriginalTag: "keep-node", CopyTag: "mixed-chain-keep-node", Source: "sub-1"},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.SyncChainNodesForSubscription("sub-1"); err != nil {
		t.Fatalf("SyncChainNodesForSubscription() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	if saved == nil {
		t.Fatal("chain should still exist")
	}
	if len(saved.Nodes) != 1 || saved.Nodes[0] != "keep-node" {
		t.Fatalf("nodes = %#v, want [keep-node]", saved.Nodes)
	}
	if len(saved.ChainNodes) != 1 || saved.ChainNodes[0].OriginalTag != "keep-node" {
		t.Fatalf("chain nodes = %#v, want only keep-node", saved.ChainNodes)
	}
}

func TestSyncChainNodesForSubscriptionKeepsOtherSubSource(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	sub1 := storage.Subscription{
		ID: "sub-1", Name: "sub-one", Enabled: true,
		Nodes: []storage.Node{
			{Tag: "from-sub1", Type: "socks", Server: "127.0.0.1", ServerPort: 1080, Source: "sub-1"},
		},
	}
	sub2 := storage.Subscription{
		ID: "sub-2", Name: "sub-two", Enabled: true,
		Nodes: []storage.Node{
			{Tag: "from-sub2", Type: "socks", Server: "127.0.0.1", ServerPort: 1081, Source: "sub-2"},
		},
	}
	if err := store.AddSubscription(sub1); err != nil {
		t.Fatalf("AddSubscription(sub-1) error = %v", err)
	}
	if err := store.AddSubscription(sub2); err != nil {
		t.Fatalf("AddSubscription(sub-2) error = %v", err)
	}

	chain := storage.ProxyChain{
		ID: "chain-1", Name: "cross-sub", Enabled: true,
		Nodes: []string{"from-sub1", "from-sub2"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "from-sub1", CopyTag: "cross-sub-from-sub1", Source: "sub-1"},
			{OriginalTag: "from-sub2", CopyTag: "cross-sub-from-sub2", Source: "sub-2"},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	// Refresh sub-1 only — sub-2 hop must be preserved regardless of sub-1's node set.
	svc := NewChainSyncService(store)
	if err := svc.SyncChainNodesForSubscription("sub-1"); err != nil {
		t.Fatalf("SyncChainNodesForSubscription() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	wantNodes := []string{"from-sub1", "from-sub2"}
	if len(saved.Nodes) != 2 || saved.Nodes[0] != wantNodes[0] || saved.Nodes[1] != wantNodes[1] {
		t.Fatalf("nodes = %#v, want %#v", saved.Nodes, wantNodes)
	}
	if saved.ChainNodes[1].Source != "sub-2" {
		t.Fatalf("other-sub Source mutated: %#v", saved.ChainNodes[1])
	}
}

func TestSyncChainNodesForSubscriptionBackfillsEmptySource(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	sub := storage.Subscription{
		ID: "sub-1", Name: "test-sub", Enabled: true,
		Nodes: []storage.Node{
			{Tag: "alive", Type: "socks", Server: "127.0.0.1", ServerPort: 1080, Source: "sub-1"},
		},
	}
	if err := store.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription() error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "manual-b", Enabled: true,
		Node: storage.Node{Tag: "manual-b", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID: "chain-1", Name: "backfill-chain", Enabled: true,
		Nodes: []string{"alive", "manual-b"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "alive", CopyTag: "backfill-chain-alive", Source: ""},
			{OriginalTag: "manual-b", CopyTag: "backfill-chain-manual-b", Source: ""},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.SyncChainNodesForSubscription("sub-1"); err != nil {
		t.Fatalf("SyncChainNodesForSubscription() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	if len(saved.ChainNodes) != 2 {
		t.Fatalf("chain nodes = %#v, want 2", saved.ChainNodes)
	}
	if saved.ChainNodes[0].Source != "sub-1" {
		t.Fatalf("alive Source = %q, want sub-1", saved.ChainNodes[0].Source)
	}
	if saved.ChainNodes[1].Source != "manual" {
		t.Fatalf("manual-b Source = %q, want manual", saved.ChainNodes[1].Source)
	}
}

func TestSyncChainNodesForSubscriptionKeepsDisabledManualEmptySource(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	sub := storage.Subscription{
		ID: "sub-1", Name: "test-sub", Enabled: true,
		Nodes: []storage.Node{
			{Tag: "sub-hop", Type: "socks", Server: "127.0.0.1", ServerPort: 1080, Source: "sub-1"},
		},
	}
	if err := store.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription() error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID: "manual-a", Enabled: false, // disabled — must not be pruned (#23)
		Node: storage.Node{Tag: "manual-a", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID: "chain-1", Name: "disabled-manual-chain", Enabled: true,
		Nodes: []string{"sub-hop", "manual-a"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "sub-hop", CopyTag: "disabled-manual-chain-sub-hop", Source: "sub-1"},
			{OriginalTag: "manual-a", CopyTag: "disabled-manual-chain-manual-a", Source: ""},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.SyncChainNodesForSubscription("sub-1"); err != nil {
		t.Fatalf("SyncChainNodesForSubscription() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	wantNodes := []string{"sub-hop", "manual-a"}
	if len(saved.Nodes) != 2 || saved.Nodes[0] != wantNodes[0] || saved.Nodes[1] != wantNodes[1] {
		t.Fatalf("nodes = %#v, want %#v (disabled manual must be kept)", saved.Nodes, wantNodes)
	}
	if saved.ChainNodes[1].Source != "manual" {
		t.Fatalf("disabled manual Source = %q, want manual (backfilled)", saved.ChainNodes[1].Source)
	}
}
