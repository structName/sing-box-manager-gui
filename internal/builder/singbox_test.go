package builder

import (
	"os"
	"path/filepath"
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
		rules: []storage.Rule{
			{
				ID:       "rule-1",
				Enabled:  true,
				Priority: 1,
				RuleType: "domain",
				Values:   []string{"example.com"},
				Outbound: "DIRECT",
			},
		},
	}

	route, err := builder.buildRoute()
	if err != nil {
		t.Fatalf("buildRoute returned error: %v", err)
	}

	inboundIndex := -1
	customRuleIndex := -1
	for index, rule := range route.Rules {
		if outbound, _ := rule["outbound"].(string); outbound == "chain-a" {
			if inbound, ok := rule["inbound"].([]string); ok && len(inbound) == 1 && inbound[0] == "custom-port-1" {
				inboundIndex = index
			}
		}
		if outbound, _ := rule["outbound"].(string); outbound == "DIRECT" {
			if domains, ok := rule["domain"].([]string); ok && len(domains) == 1 && domains[0] == "example.com" {
				customRuleIndex = index
			}
		}
	}

	if inboundIndex == -1 {
		t.Fatal("custom inbound route rule not found")
	}
	if customRuleIndex == -1 {
		t.Fatal("custom domain route rule not found")
	}
	if inboundIndex >= customRuleIndex {
		t.Fatalf("custom inbound rule index = %d, want before custom rule index = %d", inboundIndex, customRuleIndex)
	}
}

func TestBuildRouteUsesBundledRuleSetsAsLocalFiles(t *testing.T) {
	dataDir := t.TempDir()
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			RuleSetBaseURL: "https://example.com/rules",
		},
		dataDir: dataDir,
		ruleGroups: []storage.RuleGroup{
			{
				ID:        "group-1",
				Enabled:   true,
				SiteRules: []string{"openai"},
				IPRules:   []string{"private"},
			},
		},
	}

	route, err := builder.buildRoute()
	if err != nil {
		t.Fatalf("buildRoute returned error: %v", err)
	}

	if len(route.RuleSet) != 2 {
		t.Fatalf("rule set count = %d, want 2", len(route.RuleSet))
	}

	for _, ruleSet := range route.RuleSet {
		if ruleSet.Type != "local" {
			t.Fatalf("rule set %s type = %s, want local", ruleSet.Tag, ruleSet.Type)
		}
		if ruleSet.Path == "" {
			t.Fatalf("rule set %s path is empty", ruleSet.Tag)
		}
		if !filepath.IsAbs(ruleSet.Path) {
			t.Fatalf("rule set %s path = %s, want absolute path", ruleSet.Tag, ruleSet.Path)
		}
		if _, err := os.Stat(ruleSet.Path); err != nil {
			t.Fatalf("rule set %s path stat error: %v", ruleSet.Tag, err)
		}
	}
}

func TestBuildRouteFallsBackToRemoteRuleSets(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{
			RuleSetBaseURL: "https://example.com/rules",
			GithubProxy:    "https://mirror.example/",
		},
		rules: []storage.Rule{
			{
				ID:       "rule-1",
				Enabled:  true,
				RuleType: "geosite",
				Values:   []string{"custom-service"},
			},
		},
	}

	route, err := builder.buildRoute()
	if err != nil {
		t.Fatalf("buildRoute returned error: %v", err)
	}

	if len(route.RuleSet) != 1 {
		t.Fatalf("rule set count = %d, want 1", len(route.RuleSet))
	}

	ruleSet := route.RuleSet[0]
	if ruleSet.Type != "remote" {
		t.Fatalf("rule set type = %s, want remote", ruleSet.Type)
	}
	wantURL := "https://mirror.example/https://example.com/rules/geosite-custom-service.srs"
	if ruleSet.URL != wantURL {
		t.Fatalf("rule set url = %s, want %s", ruleSet.URL, wantURL)
	}
}
