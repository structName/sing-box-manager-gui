package builder

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
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

func TestNodeToOutboundNormalizesAnyTLSDurationAndTLS(t *testing.T) {
	builder := &ConfigBuilder{}
	node := storage.Node{
		Tag:        "anytls-node",
		Type:       "anytls",
		Server:     "example.com",
		ServerPort: 443,
		Extra: map[string]interface{}{
			"password":                    "secret",
			"idle_session_check_interval": 30,
			"idle_session_timeout":        float64(45),
			"tls": map[string]interface{}{
				"enabled": false,
			},
		},
	}

	outbound, err := builder.nodeToOutbound(node)
	if err != nil {
		t.Fatalf("nodeToOutbound returned error: %v", err)
	}

	if got := outbound["idle_session_check_interval"]; got != "30s" {
		t.Fatalf("idle_session_check_interval = %v, want 30s", got)
	}
	if got := outbound["idle_session_timeout"]; got != "45s" {
		t.Fatalf("idle_session_timeout = %v, want 45s", got)
	}
	tls, ok := outbound["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T, want map[string]interface{}", outbound["tls"])
	}
	if got := tls["enabled"]; got != true {
		t.Fatalf("tls.enabled = %v, want true", got)
	}
}

func TestNodeToOutboundPreservesVLESSRealityFields(t *testing.T) {
	builder := &ConfigBuilder{}
	node := storage.Node{
		Tag:        "edge-a-imported",
		Type:       "vless",
		Server:     "vpn.example.com",
		ServerPort: 443,
		Extra: map[string]interface{}{
			"uuid": "generated-uuid",
			"flow": "xtls-rprx-vision",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "www.microsoft.com",
				"reality": map[string]interface{}{
					"enabled":    true,
					"public_key": "generated-public",
					"short_id":   "0123456789abcdef",
				},
			},
			"node_origin":       "deployed_self_hosted",
			"entry_method":      "deployment_import",
			"deployment_run_id": "run-1",
		},
	}

	outbound, err := builder.nodeToOutbound(node)
	if err != nil {
		t.Fatalf("nodeToOutbound returned error: %v", err)
	}

	if outbound["type"] != "vless" || outbound["server"] != "vpn.example.com" || outbound["server_port"] != 443 {
		t.Fatalf("unexpected VLESS outbound base fields: %#v", outbound)
	}
	if outbound["uuid"] != "generated-uuid" || outbound["flow"] != "xtls-rprx-vision" {
		t.Fatalf("VLESS auth fields not preserved: %#v", outbound)
	}
	tls, ok := outbound["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T, want map[string]interface{}", outbound["tls"])
	}
	if tls["enabled"] != true || tls["server_name"] != "www.microsoft.com" {
		t.Fatalf("VLESS TLS fields not preserved: %#v", tls)
	}
	reality, ok := tls["reality"].(map[string]interface{})
	if !ok {
		t.Fatalf("reality type = %T, want map[string]interface{}", tls["reality"])
	}
	if reality["enabled"] != true || reality["public_key"] != "generated-public" || reality["short_id"] != "0123456789abcdef" {
		t.Fatalf("Reality fields not preserved: %#v", reality)
	}
	utls, ok := tls["utls"].(map[string]interface{})
	if !ok || utls["enabled"] != true || utls["fingerprint"] != "chrome" {
		t.Fatalf("Reality uTLS default not added: %#v", tls["utls"])
	}
}

func TestBuildJSONWithDeploymentImportedVLESSRealityNodePassesSingBoxCheckWhenAvailable(t *testing.T) {
	singBoxPath := os.Getenv("SBM_SING_BOX_CHECK_BIN")
	if singBoxPath == "" {
		t.Skip("set SBM_SING_BOX_CHECK_BIN to run sing-box config validation")
	}

	settings := storage.DefaultSettings()
	builder := NewConfigBuilder(settings, []storage.Node{
		{
			Tag:        "edge-a-imported",
			Type:       "vless",
			Server:     "vpn.example.com",
			ServerPort: 443,
			Extra: map[string]interface{}{
				"uuid": "11111111-1111-4111-8111-111111111111",
				"flow": "xtls-rprx-vision",
				"tls": map[string]interface{}{
					"enabled":     true,
					"server_name": "www.microsoft.com",
					"reality": map[string]interface{}{
						"enabled":    true,
						"public_key": "gUL70jxK5gzi-stwsJKexC8HLM9zK3UI8mHgK24iVFo",
						"short_id":   "0123456789abcdef",
					},
				},
				"node_origin":       "deployed_self_hosted",
				"entry_method":      "deployment_import",
				"deployment_run_id": "run-1",
			},
		},
	}, nil, nil, nil)
	builder.SetDataDir(t.TempDir())

	configJSON, err := builder.BuildJSON()
	if err != nil {
		t.Fatalf("BuildJSON() error = %v", err)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	output, err := exec.Command(singBoxPath, "check", "-c", configPath).CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box check failed: %v\n%s\nconfig:\n%s", err, output, configJSON)
	}
}

func TestDeploymentImportedVLESSRealityNodeCanBeUsedInProxyChain(t *testing.T) {
	settings := storage.DefaultSettings()
	imported := storage.Node{
		Tag:        "edge-a-imported",
		Type:       "vless",
		Server:     "vpn.example.com",
		ServerPort: 443,
		Extra: map[string]interface{}{
			"uuid": "11111111-1111-4111-8111-111111111111",
			"flow": "xtls-rprx-vision",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "www.microsoft.com",
				"reality": map[string]interface{}{
					"enabled":    true,
					"public_key": "gUL70jxK5gzi-stwsJKexC8HLM9zK3UI8mHgK24iVFo",
					"short_id":   "0123456789abcdef",
				},
			},
			"node_origin":       "deployed_self_hosted",
			"entry_method":      "deployment_import",
			"deployment_run_id": "run-1",
		},
	}
	builder := NewConfigBuilder(settings, []storage.Node{
		{
			Tag:        "relay-a",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
		},
		imported,
	}, nil, nil, []storage.ProxyChain{
		{
			ID:      "chain-1",
			Name:    "self-hosted-chain",
			Nodes:   []string{"relay-a", "edge-a-imported"},
			Enabled: true,
		},
	})
	builder.SetDataDir(t.TempDir())

	configJSON, err := builder.BuildJSON()
	if err != nil {
		t.Fatalf("BuildJSON() error = %v", err)
	}
	var config SingBoxConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		t.Fatalf("decode config: %v\n%s", err, configJSON)
	}

	relayCopyTag := storage.GenerateChainNodeCopyTag("self-hosted-chain", "relay-a")
	importedCopyTag := storage.GenerateChainNodeCopyTag("self-hosted-chain", "edge-a-imported")
	chainOutbound := outboundByTag(config.Outbounds, importedCopyTag)
	if chainOutbound == nil {
		t.Fatalf("chain copy outbound %q missing from %#v", importedCopyTag, outboundTags(config.Outbounds))
	}
	if chainOutbound["type"] != "vless" || chainOutbound["server"] != "vpn.example.com" || chainOutbound["detour"] != relayCopyTag {
		t.Fatalf("imported chain outbound lost base fields or detour: %#v", chainOutbound)
	}
	if chainOutbound["uuid"] != "11111111-1111-4111-8111-111111111111" || chainOutbound["flow"] != "xtls-rprx-vision" {
		t.Fatalf("imported chain outbound lost VLESS auth fields: %#v", chainOutbound)
	}
	tls, ok := chainOutbound["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("chain outbound tls type = %T", chainOutbound["tls"])
	}
	reality, ok := tls["reality"].(map[string]interface{})
	if !ok || reality["public_key"] != "gUL70jxK5gzi-stwsJKexC8HLM9zK3UI8mHgK24iVFo" || reality["short_id"] != "0123456789abcdef" {
		t.Fatalf("chain outbound lost Reality fields: %#v", tls)
	}
}

func outboundByTag(outbounds []Outbound, tag string) Outbound {
	for _, outbound := range outbounds {
		if outbound["tag"] == tag {
			return outbound
		}
	}
	return nil
}

func outboundTags(outbounds []Outbound) []interface{} {
	tags := make([]interface{}, 0, len(outbounds))
	for _, outbound := range outbounds {
		tags = append(tags, outbound["tag"])
	}
	return tags
}

func TestNodeToOutboundNormalizesLegacyVLESSRealityShape(t *testing.T) {
	builder := &ConfigBuilder{}
	node := storage.Node{
		Tag:        "legacy-reality",
		Type:       "vless",
		Server:     "vpn.example.com",
		ServerPort: 443,
		Extra: map[string]interface{}{
			"uuid":        "legacy-uuid",
			"flow":        "xtls-rprx-vision",
			"tls":         true,
			"server_name": "www.microsoft.com",
			"security":    "reality",
			"reality": map[string]interface{}{
				"public_key": "legacy-public",
				"short_id":   "0123456789abcdef",
			},
		},
	}

	outbound, err := builder.nodeToOutbound(node)
	if err != nil {
		t.Fatalf("nodeToOutbound returned error: %v", err)
	}

	tls, ok := outbound["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T, want map[string]interface{}", outbound["tls"])
	}
	if tls["enabled"] != true || tls["server_name"] != "www.microsoft.com" {
		t.Fatalf("legacy TLS fields not normalized: %#v", tls)
	}
	reality, ok := tls["reality"].(map[string]interface{})
	if !ok {
		t.Fatalf("reality type = %T, want map[string]interface{}", tls["reality"])
	}
	if reality["enabled"] != true || reality["public_key"] != "legacy-public" || reality["short_id"] != "0123456789abcdef" {
		t.Fatalf("legacy Reality fields not normalized: %#v", reality)
	}
	utls, ok := tls["utls"].(map[string]interface{})
	if !ok || utls["enabled"] != true || utls["fingerprint"] != "chrome" {
		t.Fatalf("legacy Reality uTLS default not added: %#v", tls["utls"])
	}
	if _, exists := outbound["server_name"]; exists {
		t.Fatalf("legacy server_name should move under tls: %#v", outbound)
	}
	if _, exists := outbound["reality"]; exists {
		t.Fatalf("legacy reality should move under tls: %#v", outbound)
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
		dataDir: "/test/data",
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	if experimental.ClashAPI.Secret != "test-secret" {
		t.Fatalf("secret = %q, want %q", experimental.ClashAPI.Secret, "test-secret")
	}
	if experimental.ClashAPI.ExternalController != "127.0.0.1:9091" {
		t.Fatalf("external controller = %q, want %q", experimental.ClashAPI.ExternalController, "127.0.0.1:9091")
	}
}

func TestBuildExperimentalEnablesLANClashAPIController(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:       9091,
			ClashAPILanEnabled: true,
			ClashUIEnabled:     true,
			ClashUIPath:        "zashboard",
		},
		dataDir: "/test/data",
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	if experimental.ClashAPI.ExternalController != "0.0.0.0:9091" {
		t.Fatalf("external controller = %q, want %q", experimental.ClashAPI.ExternalController, "0.0.0.0:9091")
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
		dataDir: "/test/data",
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
	dataDir := "/test/data"
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: true,
			ClashUIPath:    "zashboard",
		},
		dataDir: dataDir,
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	expectedPath := filepath.Join(dataDir, "zashboard")
	if experimental.ClashAPI.ExternalUI != expectedPath {
		t.Fatalf("external ui = %q, want %q", experimental.ClashAPI.ExternalUI, expectedPath)
	}
	if experimental.ClashAPI.ExternalUIDownloadURL != "" {
		t.Fatalf("external ui download url = %q, want empty", experimental.ClashAPI.ExternalUIDownloadURL)
	}
}

func TestBuildExperimentalFallsBackToEmbeddedUIPathWhenEmpty(t *testing.T) {
	dataDir := "/test/data"
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: true,
			ClashUIPath:    "   ",
		},
		dataDir: dataDir,
	}

	experimental := builder.buildExperimental()
	if experimental.ClashAPI == nil {
		t.Fatal("clash api config = nil")
	}
	expectedPath := filepath.Join(dataDir, "zashboard")
	if experimental.ClashAPI.ExternalUI != expectedPath {
		t.Fatalf("external ui = %q, want %q", experimental.ClashAPI.ExternalUI, expectedPath)
	}
	if experimental.ClashAPI.ExternalUIDownloadURL != "" {
		t.Fatalf("external ui download url = %q, want empty", experimental.ClashAPI.ExternalUIDownloadURL)
	}
}

func TestBuildExperimentalUsesDefaultExternalUIDownloadURLForCustomUIPath(t *testing.T) {
	dataDir := "/test/data"
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: true,
			ClashUIPath:    "custom-ui",
		},
		dataDir: dataDir,
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
	dataDir := "/test/data"
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			ClashAPIPort:   9091,
			ClashUIEnabled: true,
			ClashUIPath:    "custom-ui",
			GithubProxy:    "https://ghproxy.com",
		},
		dataDir: dataDir,
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
