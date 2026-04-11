package builder

import (
	"testing"

	"github.com/xiaobei/singbox-manager/internal/storage"
)

func TestNodeToOutboundNormalizesSimpleObfsPlugin(t *testing.T) {
	builder := &ConfigBuilder{}
	node := storage.Node{
		Tag:        "test-node",
		Type:       "shadowsocks",
		Server:     "example.com",
		ServerPort: 443,
		Extra: map[string]interface{}{
			"method":   "aes-128-gcm",
			"password": "secret",
			"plugin":   "obfs",
			"plugin_opts": map[string]interface{}{
				"host": "cdn.example.com",
				"mode": "http",
			},
		},
	}

	outbound, err := builder.nodeToOutbound(node)
	if err != nil {
		t.Fatalf("nodeToOutbound returned error: %v", err)
	}

	if got := outbound["plugin"]; got != "obfs-local" {
		t.Fatalf("plugin = %v, want obfs-local", got)
	}

	if got := outbound["plugin_opts"]; got != "obfs=http;obfs-host=cdn.example.com" {
		t.Fatalf("plugin_opts = %v, want obfs=http;obfs-host=cdn.example.com", got)
	}
}

func TestNodeToOutboundRejectsUnsupportedShadowsocksPlugin(t *testing.T) {
	builder := &ConfigBuilder{}
	node := storage.Node{
		Tag:        "test-node",
		Type:       "shadowsocks",
		Server:     "example.com",
		ServerPort: 443,
		Extra: map[string]interface{}{
			"method":   "aes-128-gcm",
			"password": "secret",
			"plugin":   "shadowtls",
		},
	}

	_, err := builder.nodeToOutbound(node)
	if err == nil {
		t.Fatal("nodeToOutbound error = nil, want unsupported plugin error")
	}
}

func TestBuildRoutePrioritizesCustomInboundOutbound(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{},
		inboundPorts: []storage.InboundPort{
			{
				ID:       "port-1",
				Enabled:  true,
				Outbound: "chain-a",
			},
		},
	}

	route := builder.buildRoute()
	inboundIndex := -1
	for index, rule := range route.Rules {
		if outbound, _ := rule["outbound"].(string); outbound == "chain-a" {
			if inbound, ok := rule["inbound"].([]string); ok && len(inbound) == 1 && inbound[0] == "custom-port-1" {
				inboundIndex = index
			}
		}
	}

	if inboundIndex == -1 {
		t.Fatal("custom inbound route rule not found")
	}
}

func TestBuildRouteDoesNotGenerateRuleSets(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{},
	}

	route := builder.buildRoute()

	if len(route.Rules) < 2 {
		t.Fatalf("route rule count = %d, want at least 2 base rules", len(route.Rules))
	}

	if action, _ := route.Rules[0]["action"].(string); action != "sniff" {
		t.Fatalf("first route action = %s, want sniff", action)
	}
	if action, _ := route.Rules[1]["action"].(string); action != "hijack-dns" {
		t.Fatalf("second route action = %s, want hijack-dns", action)
	}
}

func TestBuildExperimentalIncludesClashAPISecret(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: true,
			ClashUIPath:    "zashboard",
			ClashAPISecret: "test-secret",
		},
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	if experimental.ClashAPI.Secret != "test-secret" {
		t.Fatalf("secret = %q, want %q", experimental.ClashAPI.Secret, "test-secret")
	}
}

func TestBuildExperimentalDisablesExternalUIWhenZashboardClosed(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: false,
			ClashUIPath:    "zashboard",
			ClashAPISecret: "test-secret",
		},
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	if experimental.ClashAPI.ExternalUI != "" {
		t.Fatalf("external ui = %q, want empty", experimental.ClashAPI.ExternalUI)
	}
	if experimental.ClashAPI.ExternalUIDownloadURL != "" {
		t.Fatalf("external ui download url = %q, want empty", experimental.ClashAPI.ExternalUIDownloadURL)
	}
}

func TestBuildExperimentalUsesEmbeddedExternalUIByDefault(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: true,
			ClashUIPath:    "zashboard",
		},
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	if experimental.ClashAPI.ExternalUI != "zashboard" {
		t.Fatalf("external ui = %q, want zashboard", experimental.ClashAPI.ExternalUI)
	}
	if experimental.ClashAPI.ExternalUIDownloadURL != "" {
		t.Fatalf("external ui download url = %q, want empty", experimental.ClashAPI.ExternalUIDownloadURL)
	}
}

func TestBuildExperimentalFallsBackToEmbeddedUIPathWhenEmpty(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: true,
			ClashUIPath:    "   ",
		},
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	if experimental.ClashAPI.ExternalUI != "zashboard" {
		t.Fatalf("external ui = %q, want zashboard", experimental.ClashAPI.ExternalUI)
	}
	if experimental.ClashAPI.ExternalUIDownloadURL != "" {
		t.Fatalf("external ui download url = %q, want empty", experimental.ClashAPI.ExternalUIDownloadURL)
	}
}

func TestBuildExperimentalUsesDefaultExternalUIDownloadURLForCustomUIPath(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: true,
			ClashUIPath:    "custom-ui",
		},
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	if experimental.ClashAPI.ExternalUIDownloadURL != defaultZashboardExternalUIDownloadURL {
		t.Fatalf("external ui download url = %q, want %q", experimental.ClashAPI.ExternalUIDownloadURL, defaultZashboardExternalUIDownloadURL)
	}
}

func TestBuildExperimentalUsesGithubProxyForCustomExternalUIDownloadURL(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: true,
			ClashUIPath:    "custom-ui",
			GithubProxy:    "https://ghproxy.com",
		},
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	expected := "https://ghproxy.com/" + defaultZashboardExternalUIDownloadURL
	if experimental.ClashAPI.ExternalUIDownloadURL != expected {
		t.Fatalf("external ui download url = %q, want %q", experimental.ClashAPI.ExternalUIDownloadURL, expected)
	}
}
