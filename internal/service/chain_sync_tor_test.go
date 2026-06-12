package service

import (
	"testing"

	"github.com/xiaobei/singbox-manager/internal/storage"
)

func TestSyncChainNodesPreservesTorAndPrunesMissingOrdinaryNodes(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID:      "entry-node",
		Enabled: true,
		Node: storage.Node{
			Tag:        "entry",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID:      "chain-1",
		Name:    "entry-tor-missing",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag, "missing"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "entry", CopyTag: "entry-tor-missing-entry", Source: "manual"},
			{OriginalTag: storage.ChainTorNodeTag, CopyTag: storage.ChainTorDisplayName, Source: storage.ChainTorNodeSource},
			{OriginalTag: "missing", CopyTag: "entry-tor-missing-missing", Source: "manual"},
		},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.SyncChainNodes(); err != nil {
		t.Fatalf("SyncChainNodes() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	if saved == nil {
		t.Fatal("chain should still exist")
	}
	wantNodes := []string{"entry", storage.ChainTorNodeTag}
	if len(saved.Nodes) != len(wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", saved.Nodes, wantNodes)
	}
	for index, want := range wantNodes {
		if saved.Nodes[index] != want {
			t.Fatalf("nodes = %#v, want %#v", saved.Nodes, wantNodes)
		}
	}
	if len(saved.ChainNodes) != 2 {
		t.Fatalf("chain nodes = %#v, want entry and Tor", saved.ChainNodes)
	}
	torNode := saved.ChainNodes[1]
	if torNode.OriginalTag != storage.ChainTorNodeTag {
		t.Fatalf("Tor node was not preserved: %#v", saved.ChainNodes)
	}
	if torNode.CopyTag != storage.ChainTorDisplayName || torNode.Source != storage.ChainTorNodeSource {
		t.Fatalf("Tor metadata changed unexpectedly: %#v", torNode)
	}
}

func TestSyncChainNodesForSubscriptionPreservesTorInNodeTorNodeChain(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	sub := storage.Subscription{
		ID:      "sub-1",
		Name:    "test-sub",
		Enabled: true,
		Nodes: []storage.Node{
			{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1080, Source: "sub-1"},
		},
	}
	if err := store.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription() error = %v", err)
	}
	if err := store.AddManualNode(storage.ManualNode{
		ID:      "post-node",
		Enabled: true,
		Node: storage.Node{
			Tag:        "post",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1081,
		},
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID:      "chain-1",
		Name:    "entry-tor-post",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag, "post"},
		ChainNodes: []storage.ChainNode{
			{OriginalTag: "entry", CopyTag: "entry-tor-post-entry", Source: "sub-1"},
			{OriginalTag: storage.ChainTorNodeTag, CopyTag: storage.ChainTorDisplayName, Source: storage.ChainTorNodeSource},
			{OriginalTag: "post", CopyTag: "entry-tor-post-post", Source: "manual"},
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
	wantNodes := []string{"entry", storage.ChainTorNodeTag, "post"}
	if len(saved.Nodes) != len(wantNodes) {
		t.Fatalf("nodes = %#v, want %#v", saved.Nodes, wantNodes)
	}
	for index, want := range wantNodes {
		if saved.Nodes[index] != want {
			t.Fatalf("nodes = %#v, want %#v", saved.Nodes, wantNodes)
		}
	}
	if got := saved.ChainNodes[1]; got.OriginalTag != storage.ChainTorNodeTag || got.CopyTag != storage.ChainTorDisplayName || got.Source != storage.ChainTorNodeSource {
		t.Fatalf("Tor metadata not preserved: %#v", got)
	}
}

func TestRegenerateChainNodesKeepsTorMetadataStable(t *testing.T) {
	store, err := storage.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	if err := store.AddManualNode(storage.ManualNode{
		ID:      "entry-node",
		Enabled: true,
		Node: storage.Node{
			Tag:        "entry",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	chain := storage.ProxyChain{
		ID:      "chain-1",
		Name:    "renamed-entry-tor",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag},
	}
	if err := store.AddProxyChain(chain); err != nil {
		t.Fatalf("AddProxyChain() error = %v", err)
	}

	svc := NewChainSyncService(store)
	if err := svc.RegenerateChainNodes("chain-1"); err != nil {
		t.Fatalf("RegenerateChainNodes() error = %v", err)
	}

	saved := store.GetProxyChain("chain-1")
	if saved == nil {
		t.Fatal("chain should still exist")
	}
	if len(saved.ChainNodes) != 2 {
		t.Fatalf("chain nodes = %#v, want entry and Tor", saved.ChainNodes)
	}
	torNode := saved.ChainNodes[1]
	if torNode.OriginalTag != storage.ChainTorNodeTag {
		t.Fatalf("Tor node missing after regenerate: %#v", saved.ChainNodes)
	}
	if torNode.CopyTag != storage.ChainTorDisplayName || torNode.Source != storage.ChainTorNodeSource {
		t.Fatalf("Tor metadata unstable after regenerate: %#v", torNode)
	}
}
