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
