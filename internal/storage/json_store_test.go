package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewJSONStoreMigratesLegacyZashboardSettings(t *testing.T) {
	dataDir := t.TempDir()
	legacyJSON := `{
  "subscriptions": [],
  "manual_nodes": [],
  "filters": [],
  "rules": [],
  "rule_groups": [],
  "settings": {
    "singbox_path": "bin/sing-box",
    "config_path": "generated/config.json",
    "mixed_port": 2080,
    "tun_enabled": false,
    "lan_proxy_enabled": false,
    "lan_listen_ip": "0.0.0.0",
    "proxy_dns": "https://1.1.1.1/dns-query",
    "direct_dns": "https://dns.alidns.com/dns-query",
    "web_port": 9090,
    "clash_api_port": 9091,
    "clash_ui_path": "zashboard",
    "clash_api_secret": "",
    "final_outbound": "Proxy",
    "ruleset_base_url": "https://example.com",
    "auto_apply": true,
    "subscription_interval": 60,
    "github_proxy": ""
  }
}`

	if err := os.WriteFile(filepath.Join(dataDir, "data.json"), []byte(legacyJSON), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	store, err := NewJSONStore(dataDir)
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	settings := store.GetSettings()
	if !settings.ClashAPILanEnabled {
		t.Fatalf("ClashAPILanEnabled = false, want true")
	}
	if !settings.ClashUIEnabled {
		t.Fatalf("ClashUIEnabled = false, want true")
	}
	if strings.TrimSpace(settings.ClashAPISecret) == "" {
		t.Fatalf("ClashAPISecret = empty, want generated secret")
	}
}

func TestGetSubscriptionsReturnsDeepCopy(t *testing.T) {
	store, err := NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	autoUpdate := false
	sub := Subscription{
		ID:         "sub-1",
		Name:       "local",
		Content:    "original",
		AutoUpdate: &autoUpdate,
		Nodes: []Node{
			{
				Tag:   "node-1",
				Extra: map[string]interface{}{"tls": map[string]interface{}{"enabled": true}},
			},
		},
	}
	if err := store.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription() error = %v", err)
	}

	subs := store.GetSubscriptions()
	subs[0].Content = ""
	subs[0].Nodes[0].Extra["tls"].(map[string]interface{})["enabled"] = false
	*subs[0].AutoUpdate = true

	got := store.GetSubscription("sub-1")
	if got == nil {
		t.Fatal("GetSubscription() = nil")
	}
	if got.Content != "original" {
		t.Fatalf("Content = %q, want original", got.Content)
	}
	if got.Nodes[0].Extra["tls"].(map[string]interface{})["enabled"] != true {
		t.Fatalf("nested Extra was mutated through returned slice")
	}
	if *got.AutoUpdate {
		t.Fatalf("AutoUpdate was mutated through returned pointer")
	}
}

func TestGetSubscriptionReturnsDeepCopy(t *testing.T) {
	store, err := NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}

	sub := Subscription{
		ID:      "sub-1",
		Name:    "local",
		Content: "original",
		Nodes: []Node{
			{Tag: "node-1", Extra: map[string]interface{}{"password": "secret"}},
		},
	}
	if err := store.AddSubscription(sub); err != nil {
		t.Fatalf("AddSubscription() error = %v", err)
	}

	got := store.GetSubscription("sub-1")
	got.Content = ""
	got.Nodes[0].Extra["password"] = "changed"

	again := store.GetSubscription("sub-1")
	if again.Content != "original" {
		t.Fatalf("Content = %q, want original", again.Content)
	}
	if again.Nodes[0].Extra["password"] != "secret" {
		t.Fatalf("password = %v, want secret", again.Nodes[0].Extra["password"])
	}
}
func TestJSONStorePreservesManualNodeSourceName(t *testing.T) {
	store, err := NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	if err := store.AddManualNode(ManualNode{
		ID: "manual-1",
		Node: Node{
			Tag:        "edge-a",
			Type:       "vless",
			Server:     "vpn.example.com",
			ServerPort: 443,
			SourceName: "自建节点",
		},
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddManualNode() error = %v", err)
	}

	nodes := store.GetAllNodes()
	if len(nodes) != 1 {
		t.Fatalf("GetAllNodes() count = %d, want 1", len(nodes))
	}
	if nodes[0].Source != "manual" || nodes[0].SourceName != "自建节点" {
		t.Fatalf("GetAllNodes() source metadata = %q/%q, want manual/自建节点", nodes[0].Source, nodes[0].SourceName)
	}

	groups := store.GetNodesGrouped()
	if len(groups) != 1 || len(groups[0].Nodes) != 1 {
		t.Fatalf("GetNodesGrouped() = %#v, want one manual group with one node", groups)
	}
	if groups[0].Source != "manual:自建节点" || groups[0].SourceName != "自建节点" {
		t.Fatalf("manual group metadata = %q/%q, want manual:自建节点/自建节点", groups[0].Source, groups[0].SourceName)
	}
	if groups[0].Nodes[0].SourceName != "自建节点" {
		t.Fatalf("grouped manual node source_name = %q, want 自建节点", groups[0].Nodes[0].SourceName)
	}
}

func TestJSONStoreGroupsManualNodesBySourceName(t *testing.T) {
	store, err := NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore() error = %v", err)
	}
	for _, node := range []ManualNode{
		{
			ID:      "manual-default",
			Enabled: true,
			Node: Node{
				Tag:        "manual-a",
				Type:       "socks",
				Server:     "127.0.0.1",
				ServerPort: 1080,
			},
		},
		{
			ID:      "manual-self-hosted",
			Enabled: true,
			Node: Node{
				Tag:        "self-hosted-a",
				Type:       "vless",
				Server:     "vpn.example.com",
				ServerPort: 443,
				SourceName: "自建节点",
			},
		},
	} {
		if err := store.AddManualNode(node); err != nil {
			t.Fatalf("AddManualNode() error = %v", err)
		}
	}

	groups := store.GetNodesGrouped()
	if !nodeGroupHasTag(groups, "manual", "手动添加", "manual-a") {
		t.Fatalf("default manual group missing node: %#v", groups)
	}
	if !nodeGroupHasTag(groups, "manual:自建节点", "自建节点", "self-hosted-a") {
		t.Fatalf("custom manual group missing node: %#v", groups)
	}
}

func nodeGroupHasTag(groups []NodeGroup, source, sourceName, tag string) bool {
	for _, group := range groups {
		if group.Source != source || group.SourceName != sourceName {
			continue
		}
		for _, node := range group.Nodes {
			if node.Tag == tag && node.Source == "manual" {
				return true
			}
		}
	}
	return false
}
