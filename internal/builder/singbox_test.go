package builder

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

type testOutboundValidationError struct {
	index   int
	message string
}

func (e *testOutboundValidationError) Error() string {
	return e.message
}

func (e *testOutboundValidationError) OutboundIndex() (int, bool) {
	return e.index, true
}

func TestBuildValidatedJSONSkipsNodeRejectedBySingBox(t *testing.T) {
	settings := storage.DefaultSettings()
	b := NewConfigBuilder(settings, []storage.Node{
		{
			Tag:        "usable-node",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
			Extra:      map[string]interface{}{"version": "5"},
		},
		{
			Tag:        "future-node",
			Type:       "future-protocol",
			Server:     "192.0.2.10",
			ServerPort: 443,
		},
	}, nil, nil, nil)

	validationCalls := 0
	result, err := b.BuildValidatedJSON(func(configJSON string) error {
		validationCalls++
		return rejectFutureProtocol(configJSON)
	})
	if err != nil {
		t.Fatalf("BuildValidatedJSON() error = %v", err)
	}
	if validationCalls != 2 {
		t.Fatalf("validation calls = %d, want 2", validationCalls)
	}
	if len(result.SkippedNodes) != 1 {
		t.Fatalf("skipped node count = %d, want 1", len(result.SkippedNodes))
	}
	if got := result.SkippedNodes[0]; got.Tag != "future-node" || got.Type != "future-protocol" || !strings.Contains(got.Reason, "unsupported outbound type") {
		t.Fatalf("skipped node = %#v", got)
	}

	var config SingBoxConfig
	if err := json.Unmarshal([]byte(result.JSON), &config); err != nil {
		t.Fatalf("decode validated config: %v", err)
	}
	if outboundByTag(config.Outbounds, "usable-node") == nil {
		t.Fatal("usable node missing from validated config")
	}
	if outboundByTag(config.Outbounds, "future-node") != nil {
		t.Fatal("rejected node remains in validated config")
	}
	auto := outboundByTag(config.Outbounds, "Auto")
	if auto == nil || !stringListContains(auto["outbounds"], "usable-node") || stringListContains(auto["outbounds"], "future-node") {
		t.Fatalf("Auto group not rebuilt after exclusion: %#v", auto)
	}
}

func TestBuildValidatedJSONDoesNotSwallowGlobalConfigErrors(t *testing.T) {
	b := NewConfigBuilder(storage.DefaultSettings(), nil, nil, nil, nil)

	result, err := b.BuildValidatedJSON(func(string) error {
		return fmt.Errorf("FATAL initialize inbound[0]: listen tcp: address already in use")
	})
	if err == nil {
		t.Fatal("BuildValidatedJSON() error = nil, want global config error")
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

func TestBuildValidatedJSONSkipsRejectedNodeUsedByProxyChain(t *testing.T) {
	b := NewConfigBuilder(storage.DefaultSettings(), []storage.Node{
		{
			Tag:        "relay-node",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
			Extra:      map[string]interface{}{"version": "5"},
		},
		{
			Tag:        "future-node",
			Type:       "future-protocol",
			Server:     "192.0.2.20",
			ServerPort: 443,
		},
	}, nil, nil, []storage.ProxyChain{
		{
			ID:      "chain-1",
			Name:    "test-chain",
			Nodes:   []string{"relay-node", "future-node"},
			Enabled: true,
		},
	})

	result, err := b.BuildValidatedJSON(rejectFutureProtocol)
	if err != nil {
		t.Fatalf("BuildValidatedJSON() error = %v", err)
	}
	if len(result.SkippedNodes) != 1 || result.SkippedNodes[0].Tag != "future-node" {
		t.Fatalf("skipped nodes = %#v", result.SkippedNodes)
	}

	var config SingBoxConfig
	if err := json.Unmarshal([]byte(result.JSON), &config); err != nil {
		t.Fatal(err)
	}
	if outboundByTag(config.Outbounds, "test-chain") != nil {
		t.Fatal("chain containing rejected node remains in validated config")
	}
	if outboundByTag(config.Outbounds, "relay-node") == nil {
		t.Fatal("unrelated usable node was removed with chain")
	}
}

func TestBuildValidatedJSONSkipsLocallyRejectedNode(t *testing.T) {
	b := NewConfigBuilder(storage.DefaultSettings(), []storage.Node{
		{
			Tag:        "usable-node",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
			Extra:      map[string]interface{}{"version": "5"},
		},
		{
			Tag:        "bad-plugin",
			Type:       "shadowsocks",
			Server:     "192.0.2.30",
			ServerPort: 443,
			Extra: map[string]interface{}{
				"method":      "aes-128-gcm",
				"password":    "secret",
				"plugin":      "shadowtls",
				"plugin_opts": "version=3",
			},
		},
	}, nil, nil, nil)

	validationCalls := 0
	result, err := b.BuildValidatedJSON(func(string) error {
		validationCalls++
		return nil
	})
	if err != nil {
		t.Fatalf("BuildValidatedJSON() error = %v", err)
	}
	if validationCalls != 1 {
		t.Fatalf("validation calls = %d, want 1", validationCalls)
	}
	if len(result.SkippedNodes) != 1 || result.SkippedNodes[0].Tag != "bad-plugin" {
		t.Fatalf("skipped nodes = %#v", result.SkippedNodes)
	}
	if !strings.Contains(result.SkippedNodes[0].Reason, "shadowtls") || !strings.Contains(result.SkippedNodes[0].Reason, "不受 sing-box 支持") {
		t.Fatalf("skip reason = %q", result.SkippedNodes[0].Reason)
	}
}

func TestBuildValidatedJSONRewritesRouteThatTargetsRejectedNode(t *testing.T) {
	b := NewConfigBuilder(storage.DefaultSettings(), []storage.Node{
		{
			Tag:        "usable-node",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
			Extra:      map[string]interface{}{"version": "5"},
		},
		{
			Tag:        "future-node",
			Type:       "future-protocol",
			Server:     "192.0.2.40",
			ServerPort: 443,
		},
	}, nil, []storage.InboundPort{
		{
			ID:       "port-1",
			Type:     "mixed",
			Listen:   "127.0.0.1",
			Port:     2081,
			Outbound: "future-node",
			Enabled:  true,
		},
	}, nil)

	result, err := b.BuildValidatedJSON(func(configJSON string) error {
		if err := rejectFutureProtocol(configJSON); err != nil {
			return err
		}
		var config SingBoxConfig
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			return err
		}
		for index, rule := range config.Route.Rules {
			if rule["outbound"] == "future-node" {
				return fmt.Errorf("FATAL route.rules[%d].outbound: outbound not found: future-node", index)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("BuildValidatedJSON() error = %v", err)
	}

	var config SingBoxConfig
	if err := json.Unmarshal([]byte(result.JSON), &config); err != nil {
		t.Fatal(err)
	}
	for _, rule := range config.Route.Rules {
		if inbound, ok := rule["inbound"].([]interface{}); ok && len(inbound) == 1 && inbound[0] == "custom-port-1" {
			if rule["outbound"] != "Proxy" {
				t.Fatalf("custom inbound outbound = %v, want Proxy", rule["outbound"])
			}
			return
		}
	}
	t.Fatal("custom inbound route rule missing")
}

func TestBuildValidatedJSONPreservesUnrelatedGlobalRouteError(t *testing.T) {
	b := NewConfigBuilder(storage.DefaultSettings(), []storage.Node{
		{
			Tag:        "usable-node",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
			Extra:      map[string]interface{}{"version": "5"},
		},
		{
			Tag:        "future-node",
			Type:       "future-protocol",
			Server:     "192.0.2.41",
			ServerPort: 443,
		},
	}, nil, []storage.InboundPort{
		{
			ID:       "port-1",
			Type:     "mixed",
			Listen:   "127.0.0.1",
			Port:     2081,
			Outbound: "unrelated-typo",
			Enabled:  true,
		},
	}, nil)

	result, err := b.BuildValidatedJSON(func(configJSON string) error {
		if err := rejectFutureProtocol(configJSON); err != nil {
			return err
		}
		var config SingBoxConfig
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			return err
		}
		for index, rule := range config.Route.Rules {
			if rule["outbound"] == "unrelated-typo" {
				return fmt.Errorf("FATAL route.rules[%d].outbound: outbound not found: unrelated-typo", index)
			}
		}
		return nil
	})
	if err == nil {
		t.Fatal("BuildValidatedJSON() error = nil, want unrelated global route error")
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}

func rejectFutureProtocol(configJSON string) error {
	var config SingBoxConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return err
	}
	for index, outbound := range config.Outbounds {
		if outbound["type"] == "future-protocol" {
			return &testOutboundValidationError{
				index:   index,
				message: fmt.Sprintf("FATAL initialize outbound[%d]: unsupported outbound type", index),
			}
		}
	}
	return nil
}

func stringListContains(raw interface{}, want string) bool {
	switch values := raw.(type) {
	case []interface{}:
		for _, value := range values {
			if value == want {
				return true
			}
		}
	case []string:
		for _, value := range values {
			if value == want {
				return true
			}
		}
	}
	return false
}

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


func TestAppendHostOverrideRouteRulesUserBeatsSystem(t *testing.T) {
	systemHosts := map[string][]string{
		"example.internal": {"10.0.0.1", "10.0.0.2"},
		"only-system.test": {"10.0.0.9"},
	}
	userHosts := []storage.HostEntry{
		{ID: "u1", Domain: "example.internal", IPs: []string{"192.0.2.10", "192.0.2.11"}, Enabled: true},
		{ID: "u2", Domain: "only-user.test", IPs: []string{"198.51.100.7"}, Enabled: true},
		{ID: "u3", Domain: "disabled.test", IPs: []string{"203.0.113.1"}, Enabled: false},
	}

	rules := appendHostOverrideRouteRules(nil, systemHosts, userHosts)

	ipByDomain := map[string]string{}
	counts := map[string]int{}
	for _, rule := range rules {
		domains, _ := rule["domain"].([]string)
		if len(domains) != 1 {
			t.Fatalf("unexpected domains %#v", rule["domain"])
		}
		domain := domains[0]
		counts[domain]++
		ip, _ := rule["override_address"].(string)
		ipByDomain[domain] = ip
		if outbound, _ := rule["outbound"].(string); outbound != "DIRECT" {
			t.Fatalf("outbound for %s = %q, want DIRECT", domain, outbound)
		}
	}

	if counts["example.internal"] != 1 {
		t.Fatalf("example.internal rule count = %d, want 1 (no duplicate system rule)", counts["example.internal"])
	}
	if ipByDomain["example.internal"] != "192.0.2.10" {
		t.Fatalf("example.internal override = %q, want user IP 192.0.2.10", ipByDomain["example.internal"])
	}
	if ipByDomain["only-user.test"] != "198.51.100.7" {
		t.Fatalf("only-user.test override = %q, want 198.51.100.7", ipByDomain["only-user.test"])
	}
	if ipByDomain["only-system.test"] != "10.0.0.9" {
		t.Fatalf("only-system.test override = %q, want 10.0.0.9", ipByDomain["only-system.test"])
	}
	if _, ok := ipByDomain["disabled.test"]; ok {
		t.Fatal("disabled user host must not emit a route rule")
	}

	// User rules must appear before remaining system rules so first-match cannot resurrect system IP.
	var exampleIdx, onlySystemIdx = -1, -1
	for i, rule := range rules {
		domains, _ := rule["domain"].([]string)
		if len(domains) == 0 {
			continue
		}
		switch domains[0] {
		case "example.internal":
			exampleIdx = i
		case "only-system.test":
			onlySystemIdx = i
		}
	}
	if exampleIdx < 0 || onlySystemIdx < 0 {
		t.Fatalf("missing expected rules: example=%d system=%d", exampleIdx, onlySystemIdx)
	}
	if exampleIdx > onlySystemIdx {
		t.Fatalf("user override rule index %d should precede leftover system rule index %d", exampleIdx, onlySystemIdx)
	}
}

func TestBuildRouteHonorsUserHostOverSystemHost(t *testing.T) {
	// Pick a domain present in this machine's /etc/hosts when available; otherwise
	// synthesize by only asserting the user rule is present exactly once with user IP.
	systemHosts := ParseSystemHosts()
	var domain string
	for d := range systemHosts {
		domain = d
		break
	}
	if domain == "" {
		domain = "sbm-hosts-override.test"
	}
	wantIP := "192.0.2.55"
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			Hosts: []storage.HostEntry{
				{ID: "override", Domain: domain, IPs: []string{wantIP}, Enabled: true},
			},
		},
	}

	route := builder.buildRoute()
	matches := 0
	for _, rule := range route.Rules {
		domains, _ := rule["domain"].([]string)
		if len(domains) != 1 || domains[0] != domain {
			continue
		}
		matches++
		if ip, _ := rule["override_address"].(string); ip != wantIP {
			t.Fatalf("override_address = %q, want user IP %q", ip, wantIP)
		}
	}
	if matches != 1 {
		t.Fatalf("route rules for %q = %d, want exactly 1", domain, matches)
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


func TestNodeToOutboundNormalizesSocks(t *testing.T) {
	builder := &ConfigBuilder{}

	t.Run("defaults version and canonical type", func(t *testing.T) {
		outbound, err := builder.nodeToOutbound(storage.Node{
			Tag:        "socks-manual",
			Type:       "socks5",
			Server:     "127.0.0.1",
			ServerPort: 1080,
			Extra: map[string]interface{}{
				"username": "u",
				"password": "p",
			},
		})
		if err != nil {
			t.Fatalf("nodeToOutbound error: %v", err)
		}
		if outbound["type"] != "socks" {
			t.Fatalf("type = %v, want socks", outbound["type"])
		}
		if outbound["version"] != "5" {
			t.Fatalf("version = %v, want 5", outbound["version"])
		}
		if outbound["username"] != "u" || outbound["password"] != "p" {
			t.Fatalf("auth not preserved: %v/%v", outbound["username"], outbound["password"])
		}
	})

	t.Run("maps socks4 alias version", func(t *testing.T) {
		outbound, err := builder.nodeToOutbound(storage.Node{
			Tag:        "socks4-node",
			Type:       "socks",
			Server:     "127.0.0.1",
			ServerPort: 1080,
			Extra: map[string]interface{}{
				"version": "socks4",
			},
		})
		if err != nil {
			t.Fatalf("nodeToOutbound error: %v", err)
		}
		if outbound["version"] != "4" {
			t.Fatalf("version = %v, want 4", outbound["version"])
		}
	})
}

func TestProxyChainDetourMixesSocksSSAndVLESS(t *testing.T) {
	settings := storage.DefaultSettings()
	nodes := []storage.Node{
		{
			Tag:        "jp-socks",
			Type:       "socks5", // alias must normalize to socks + version 5 in chain copies
			Server:     "147.79.59.130",
			ServerPort: 21080,
			Extra: map[string]interface{}{
				"username": "sbm_test",
				"password": "secret",
			},
		},
		{
			Tag:        "hk-socks",
			Type:       "socks",
			Server:     "163.53.18.90",
			ServerPort: 21080,
			Extra: map[string]interface{}{
				"username": "sbm_test",
				"password": "secret",
			},
		},
		{
			Tag:        "jp-ss",
			Type:       "shadowsocks",
			Server:     "147.79.59.130",
			ServerPort: 21081,
			Extra: map[string]interface{}{
				"method":   "aes-128-gcm",
				"password": "ss-secret",
			},
		},
		{
			Tag:        "hk-ss",
			Type:       "shadowsocks",
			Server:     "163.53.18.90",
			ServerPort: 21081,
			Extra: map[string]interface{}{
				"method":   "aes-128-gcm",
				"password": "ss-secret",
			},
		},
		{
			Tag:        "jp-vless",
			Type:       "vless",
			Server:     "147.79.59.130",
			ServerPort: 21083,
			Extra: map[string]interface{}{
				"uuid": "29a7533a-e541-406b-a455-1de847a1ff34",
				"tls": map[string]interface{}{
					"enabled":     true,
					"server_name": "www.cloudflare.com",
					"insecure":    true,
				},
			},
		},
	}

	chains := []storage.ProxyChain{
		{ID: "c1", Name: "socks-jp-hk", Nodes: []string{"jp-socks", "hk-socks"}, Enabled: true},
		{ID: "c2", Name: "ss-jp-socks-hk", Nodes: []string{"jp-ss", "hk-socks"}, Enabled: true},
		{ID: "c3", Name: "vless-jp-ss-hk", Nodes: []string{"jp-vless", "hk-ss"}, Enabled: true},
	}

	builder := NewConfigBuilder(settings, nodes, nil, nil, chains)
	builder.SetDataDir(t.TempDir())

	configJSON, err := builder.BuildJSON()
	if err != nil {
		t.Fatalf("BuildJSON() error = %v", err)
	}
	var config SingBoxConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		t.Fatalf("decode: %v\n%s", err, configJSON)
	}

	cases := []struct {
		chainName string
		entryTag  string
		exitTag   string
		exitType  string
	}{
		{"socks-jp-hk", "jp-socks", "hk-socks", "socks"},
		{"ss-jp-socks-hk", "jp-ss", "hk-socks", "socks"},
		{"vless-jp-ss-hk", "jp-vless", "hk-ss", "shadowsocks"},
	}

	for _, tc := range cases {
		entryCopy := storage.GenerateChainNodeCopyTag(tc.chainName, tc.entryTag)
		exitCopy := storage.GenerateChainNodeCopyTag(tc.chainName, tc.exitTag)
		entry := outboundByTag(config.Outbounds, entryCopy)
		exit := outboundByTag(config.Outbounds, exitCopy)
		if entry == nil {
			t.Fatalf("%s: missing entry copy %q in %#v", tc.chainName, entryCopy, outboundTags(config.Outbounds))
		}
		if exit == nil {
			t.Fatalf("%s: missing exit copy %q", tc.chainName, exitCopy)
		}
		if _, hasDetour := entry["detour"]; hasDetour {
			t.Fatalf("%s: entry %q must not have detour: %#v", tc.chainName, entryCopy, entry)
		}
		if exit["detour"] != entryCopy {
			t.Fatalf("%s: exit detour = %v, want %q", tc.chainName, exit["detour"], entryCopy)
		}
		if exit["type"] != tc.exitType {
			t.Fatalf("%s: exit type = %v, want %q", tc.chainName, exit["type"], tc.exitType)
		}
		selector := outboundByTag(config.Outbounds, tc.chainName)
		if selector == nil {
			t.Fatalf("%s: missing chain selector outbound", tc.chainName)
		}
		outs, _ := selector["outbounds"].([]interface{})
		if len(outs) == 0 || outs[0] != exitCopy {
			t.Fatalf("%s: selector outbounds = %#v, want first %q", tc.chainName, outs, exitCopy)
		}
	}

	// SOCKS5 alias on entry must normalize inside chain copy
	jpSocksCopy := outboundByTag(config.Outbounds, storage.GenerateChainNodeCopyTag("socks-jp-hk", "jp-socks"))
	if jpSocksCopy["type"] != "socks" || jpSocksCopy["version"] != "5" {
		t.Fatalf("socks5 alias not normalized in chain copy: %#v", jpSocksCopy)
	}

	singBoxPath := os.Getenv("SING_BOX_PATH")
	if singBoxPath == "" {
		if _, err := os.Stat("/workspace/sbm-test/bin/sing-box"); err == nil {
			singBoxPath = "/workspace/sbm-test/bin/sing-box"
		}
	}
	if singBoxPath == "" {
		t.Log("SING_BOX_PATH unset; skip sing-box check")
		return
	}
	configPath := filepath.Join(t.TempDir(), "chain-mix.json")
	if err := os.WriteFile(configPath, []byte(configJSON), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	output, err := exec.Command(singBoxPath, "check", "-c", configPath).CombinedOutput()
	if err != nil {
		t.Fatalf("sing-box check failed: %v\n%s", err, output)
	}
}
