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

func TestNodeToMihomoProxyConvertsAnyTLSDurationsToSeconds(t *testing.T) {
	node := &models.Node{
		Tag:        "anytls-01",
		Type:       "anytls",
		Server:     "example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"password":                    "secret",
			"idle_session_check_interval": "30s",
			"idle_session_timeout":        "45s",
			"min_idle_session":            float64(5),
			"tls": map[string]interface{}{
				"server_name": "example.com",
				"utls": map[string]interface{}{
					"fingerprint": "chrome",
				},
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy returned error: %v", err)
	}

	if got := proxy["idle-session-check-interval"]; got != 30 {
		t.Fatalf("idle-session-check-interval = %v, want 30", got)
	}
	if got := proxy["idle-session-timeout"]; got != 45 {
		t.Fatalf("idle-session-timeout = %v, want 45", got)
	}
	if got := proxy["min-idle-session"]; got != 5 {
		t.Fatalf("min-idle-session = %v, want 5", got)
	}
	if got := proxy["client-fingerprint"]; got != "chrome" {
		t.Fatalf("client-fingerprint = %v, want chrome", got)
	}
}
