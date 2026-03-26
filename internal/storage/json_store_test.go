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
	if !settings.ClashUIEnabled {
		t.Fatalf("ClashUIEnabled = false, want true")
	}
	if strings.TrimSpace(settings.ClashAPISecret) == "" {
		t.Fatalf("ClashAPISecret = empty, want generated secret")
	}
}
