package speedtest

import (
	"testing"

	"github.com/xiaobei/singbox-manager/internal/database/models"
)

func TestNodeToMihomoProxyIncludesClashStyleObfsPlugin(t *testing.T) {
	node := &models.Node{
		Tag:        "hk-01",
		Type:       "shadowsocks",
		Server:     "example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"method":   "aes-128-gcm",
			"password": "secret",
			"plugin":   "obfs",
			"plugin_opts": map[string]interface{}{
				"mode": "http",
				"host": "cdn.example.com",
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy returned error: %v", err)
	}

	if got := proxy["plugin"]; got != "obfs" {
		t.Fatalf("plugin = %v, want obfs", got)
	}

	opts, ok := proxy["plugin-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("plugin-opts type = %T, want map[string]interface{}", proxy["plugin-opts"])
	}
	if got := opts["mode"]; got != "http" {
		t.Fatalf("plugin-opts.mode = %v, want http", got)
	}
	if got := opts["host"]; got != "cdn.example.com" {
		t.Fatalf("plugin-opts.host = %v, want cdn.example.com", got)
	}
}

func TestNodeToMihomoProxyParsesSingBoxStyleObfsPlugin(t *testing.T) {
	node := &models.Node{
		Tag:        "hk-01",
		Type:       "shadowsocks",
		Server:     "example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"method":      "aes-128-gcm",
			"password":    "secret",
			"plugin":      "obfs-local",
			"plugin_opts": "obfs=http;obfs-host=cdn.example.com",
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy returned error: %v", err)
	}

	if got := proxy["plugin"]; got != "obfs" {
		t.Fatalf("plugin = %v, want obfs", got)
	}

	opts, ok := proxy["plugin-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("plugin-opts type = %T, want map[string]interface{}", proxy["plugin-opts"])
	}
	if got := opts["mode"]; got != "http" {
		t.Fatalf("plugin-opts.mode = %v, want http", got)
	}
	if got := opts["host"]; got != "cdn.example.com" {
		t.Fatalf("plugin-opts.host = %v, want cdn.example.com", got)
	}
}
