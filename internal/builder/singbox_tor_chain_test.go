package builder

import (
	"path/filepath"
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestBuildConfigGeneratesTorChainOnlyForExplicitInbound(t *testing.T) {
	chain := storage.ProxyChain{
		ID:      "tor-chain-1",
		Name:    "entry-to-tor",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag},
	}
	unusedChain := storage.ProxyChain{
		ID:      "unused-tor-chain",
		Name:    "unused-tor",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag},
	}
	dataDir := t.TempDir()

	builder := &ConfigBuilder{
		settings: &storage.Settings{
			FinalOutbound:     "Proxy",
			TorEnabled:        true,
			TorExecutablePath: "/usr/bin/tor",
		},
		nodes: []storage.Node{
			{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
		},
		inboundPorts: []storage.InboundPort{
			{
				ID:         "port-1",
				Type:       "mixed",
				Listen:     "127.0.0.1",
				Port:       2081,
				Enabled:    true,
				UseTorExit: true,
				TorChainID: "tor-chain-1",
			},
		},
		proxyChains: []storage.ProxyChain{chain, unusedChain},
		dataDir:     dataDir,
	}

	config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	outboundMap := make(map[string]Outbound, len(config.Outbounds))
	for _, outbound := range config.Outbounds {
		tag, _ := outbound["tag"].(string)
		if tag != "" {
			outboundMap[tag] = outbound
		}
	}

	if _, exists := outboundMap["unused-tor"]; exists {
		t.Fatal("unused Tor chain selector should not be generated")
	}

	torTag := storage.GenerateChainTorOutboundTag(chain.ID)
	torOutbound, exists := outboundMap[torTag]
	if !exists {
		t.Fatalf("Tor outbound %q not generated", torTag)
	}
	if got := torOutbound["type"]; got != "tor" {
		t.Fatalf("Tor outbound type = %v, want tor", got)
	}
	if got := torOutbound["executable_path"]; got != "/usr/bin/tor" {
		t.Fatalf("Tor executable_path = %v", got)
	}
	if got := torOutbound["data_directory"]; got != filepath.Join(dataDir, "tor", chain.ID) {
		t.Fatalf("Tor data_directory = %v", got)
	}
	if got := torOutbound["detour"]; got != storage.GenerateChainNodeCopyTag(chain.Name, "entry") {
		t.Fatalf("Tor detour = %v", got)
	}
	torrc, ok := torOutbound["torrc"].(map[string]interface{})
	if !ok {
		t.Fatalf("Tor torrc should be map[string]interface{}, got %T", torOutbound["torrc"])
	}
	if got := torrc["ClientOnly"]; got != 1 {
		t.Fatalf("Tor torrc ClientOnly = %v", got)
	}

	selector, exists := outboundMap[chain.Name]
	if !exists {
		t.Fatalf("Tor chain selector %q not generated", chain.Name)
	}
	selectorOutbounds, ok := selector["outbounds"].([]string)
	if !ok || len(selectorOutbounds) != 1 || selectorOutbounds[0] != torTag {
		t.Fatalf("Tor chain selector outbounds = %#v", selector["outbounds"])
	}

	for _, outboundName := range []string{"Proxy", "Final"} {
		selector, exists := outboundMap[outboundName]
		if !exists {
			t.Fatalf("%s selector not generated", outboundName)
		}
		members, ok := selector["outbounds"].([]string)
		if !ok {
			t.Fatalf("%s outbounds should be []string, got %T", outboundName, selector["outbounds"])
		}
		for _, member := range members {
			if member == chain.Name || member == unusedChain.Name {
				t.Fatalf("%s selector should not include Tor chain %q: %#v", outboundName, member, members)
			}
		}
	}

	foundInboundRule := false
	for _, rule := range config.Route.Rules {
		if outbound, _ := rule["outbound"].(string); outbound != chain.Name {
			continue
		}
		inbounds, _ := rule["inbound"].([]string)
		if len(inbounds) == 1 && inbounds[0] == "custom-port-1" {
			foundInboundRule = true
			break
		}
	}
	if !foundInboundRule {
		t.Fatalf("custom inbound route should point to Tor chain selector %q", chain.Name)
	}
}

func TestBuildConfigReusesOneTorChainForMultipleInbounds(t *testing.T) {
	chain := storage.ProxyChain{
		ID:      "tor-chain-1",
		Name:    "shared-tor",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag},
	}

	builder := &ConfigBuilder{
		settings: &storage.Settings{
			FinalOutbound:     "Proxy",
			TorEnabled:        true,
			TorExecutablePath: "/usr/bin/tor",
		},
		nodes: []storage.Node{
			{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
		},
		inboundPorts: []storage.InboundPort{
			{ID: "port-1", Type: "mixed", Listen: "127.0.0.1", Port: 2081, Enabled: true, UseTorExit: true, TorChainID: chain.ID},
			{ID: "port-2", Type: "mixed", Listen: "127.0.0.1", Port: 2082, Enabled: true, UseTorExit: true, TorChainID: chain.ID},
		},
		proxyChains: []storage.ProxyChain{chain},
	}

	config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	torTag := storage.GenerateChainTorOutboundTag(chain.ID)
	torOutboundCount := 0
	selectorCount := 0
	for _, outbound := range config.Outbounds {
		if outbound["tag"] == torTag {
			torOutboundCount++
		}
		if outbound["tag"] == chain.Name {
			selectorCount++
		}
	}
	if torOutboundCount != 1 {
		t.Fatalf("Tor outbound count = %d, want 1", torOutboundCount)
	}
	if selectorCount != 1 {
		t.Fatalf("Tor chain selector count = %d, want 1", selectorCount)
	}

	routedInbounds := map[string]bool{}
	for _, rule := range config.Route.Rules {
		if outbound, _ := rule["outbound"].(string); outbound != chain.Name {
			continue
		}
		inbounds, _ := rule["inbound"].([]string)
		if len(inbounds) == 1 {
			routedInbounds[inbounds[0]] = true
		}
	}
	for _, inbound := range []string{"custom-port-1", "custom-port-2"} {
		if !routedInbounds[inbound] {
			t.Fatalf("inbound %q should route to shared Tor chain, got %#v", inbound, routedInbounds)
		}
	}
}

func TestBuildRouteRejectsStaleTorChainInbound(t *testing.T) {
	builder := &ConfigBuilder{
		settings: &storage.Settings{FinalOutbound: "Proxy"},
		inboundPorts: []storage.InboundPort{
			{
				ID:         "port-1",
				Type:       "mixed",
				Listen:     "127.0.0.1",
				Port:       2081,
				Enabled:    true,
				UseTorExit: true,
				TorChainID: "deleted-chain",
			},
		},
		proxyChains: []storage.ProxyChain{
			{
				ID:      "deleted-chain",
				Name:    "disabled-tor",
				Enabled: false,
				Nodes:   []string{"entry", storage.ChainTorNodeTag},
			},
		},
	}

	route := builder.buildRoute()
	for _, rule := range route.Rules {
		inbounds, _ := rule["inbound"].([]string)
		if len(inbounds) == 1 && inbounds[0] == "custom-port-1" {
			if outbound, _ := rule["outbound"].(string); outbound != "REJECT" {
				t.Fatalf("stale Tor inbound outbound = %q, want REJECT", outbound)
			}
			return
		}
	}
	t.Fatalf("stale Tor inbound should get explicit REJECT rule, got %#v", route.Rules)
}

func TestBuildConfigSupportsCountryAutoBeforeTor(t *testing.T) {
	countryTag := storage.MakeChainCountryNodeTag("HK")
	chain := storage.ProxyChain{
		ID:      "tor-chain-country",
		Name:    "hk-auto-to-tor",
		Enabled: true,
		Nodes:   []string{countryTag, storage.ChainTorNodeTag},
	}

	builder := &ConfigBuilder{
		settings: &storage.Settings{
			FinalOutbound:     "Proxy",
			TorEnabled:        true,
			TorExecutablePath: "/usr/bin/tor",
		},
		nodes: []storage.Node{
			{Tag: "hk-a", Type: "socks", Server: "127.0.0.1", ServerPort: 1081, Country: "HK"},
			{Tag: "hk-b", Type: "socks", Server: "127.0.0.1", ServerPort: 1082, Country: "HK"},
			{Tag: "jp-a", Type: "socks", Server: "127.0.0.1", ServerPort: 1083, Country: "JP"},
		},
		inboundPorts: []storage.InboundPort{
			{ID: "port-1", Type: "mixed", Listen: "127.0.0.1", Port: 2081, Enabled: true, UseTorExit: true, TorChainID: chain.ID},
		},
		proxyChains: []storage.ProxyChain{chain},
	}

	config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	outboundMap := outboundsByTag(config.Outbounds)
	groupCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, countryTag)
	groupOutbound := outboundMap[groupCopyTag]
	if groupOutbound == nil {
		t.Fatalf("country auto group %q not generated", groupCopyTag)
	}
	if got := groupOutbound["type"]; got != "urltest" {
		t.Fatalf("country auto group type = %v, want urltest", got)
	}
	members, ok := groupOutbound["outbounds"].([]string)
	if !ok || len(members) != 2 {
		t.Fatalf("country auto group members = %#v", groupOutbound["outbounds"])
	}
	for _, candidate := range members {
		outbound := outboundMap[candidate]
		if outbound == nil {
			t.Fatalf("country candidate %q not generated", candidate)
		}
		if got := outbound["detour"]; got != nil {
			t.Fatalf("first country candidate should not detour before Tor, got %v", got)
		}
	}

	torTag := storage.GenerateChainTorOutboundTag(chain.ID)
	if got := outboundMap[torTag]["detour"]; got != groupCopyTag {
		t.Fatalf("Tor detour = %v, want country auto group %q", got, groupCopyTag)
	}
}

func TestBuildConfigSupportsAllNodeAutoBeforeTor(t *testing.T) {
	chain := storage.ProxyChain{
		ID:      "tor-chain-auto",
		Name:    "auto-to-tor",
		Enabled: true,
		Nodes:   []string{storage.ChainAutoNodeTag, storage.ChainTorNodeTag},
	}

	builder := &ConfigBuilder{
		settings: &storage.Settings{
			FinalOutbound:     "Proxy",
			TorEnabled:        true,
			TorExecutablePath: "/usr/bin/tor",
		},
		nodes: []storage.Node{
			{Tag: "node-a", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
			{Tag: "node-b", Type: "socks", Server: "127.0.0.1", ServerPort: 1082},
		},
		inboundPorts: []storage.InboundPort{
			{ID: "port-1", Type: "mixed", Listen: "127.0.0.1", Port: 2081, Enabled: true, UseTorExit: true, TorChainID: chain.ID},
		},
		proxyChains: []storage.ProxyChain{chain},
	}

	config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	outboundMap := outboundsByTag(config.Outbounds)
	groupCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, storage.ChainAutoNodeTag)
	groupOutbound := outboundMap[groupCopyTag]
	if groupOutbound == nil {
		t.Fatalf("Auto entry group %q not generated", groupCopyTag)
	}
	if got := groupOutbound["type"]; got != "urltest" {
		t.Fatalf("Auto entry group type = %v, want urltest", got)
	}
	members, ok := groupOutbound["outbounds"].([]string)
	if !ok || len(members) != 2 {
		t.Fatalf("Auto entry group members = %#v", groupOutbound["outbounds"])
	}
	for _, candidate := range members {
		outbound := outboundMap[candidate]
		if outbound == nil {
			t.Fatalf("Auto candidate %q not generated", candidate)
		}
	}

	torTag := storage.GenerateChainTorOutboundTag(chain.ID)
	if got := outboundMap[torTag]["detour"]; got != groupCopyTag {
		t.Fatalf("Tor detour = %v, want Auto entry group %q", got, groupCopyTag)
	}
}

func TestBuildConfigSupportsNodeTorNodeChain(t *testing.T) {
	chain := storage.ProxyChain{
		ID:      "tor-chain-post-node",
		Name:    "entry-tor-post",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag, "post"},
	}

	builder := &ConfigBuilder{
		settings: &storage.Settings{
			FinalOutbound:     "Proxy",
			TorEnabled:        true,
			TorExecutablePath: "/usr/bin/tor",
		},
		nodes: []storage.Node{
			{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
			{Tag: "post", Type: "socks", Server: "127.0.0.1", ServerPort: 1082},
		},
		inboundPorts: []storage.InboundPort{
			{ID: "port-1", Type: "mixed", Listen: "127.0.0.1", Port: 2081, Enabled: true, UseTorExit: true, TorChainID: chain.ID},
		},
		proxyChains: []storage.ProxyChain{chain},
	}

	config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	outboundMap := outboundsByTag(config.Outbounds)
	entryCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, "entry")
	torTag := storage.GenerateChainTorOutboundTag(chain.ID)
	postCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, "post")

	if got := outboundMap[torTag]["detour"]; got != entryCopyTag {
		t.Fatalf("Tor detour = %v, want entry copy %q", got, entryCopyTag)
	}
	postOutbound := outboundMap[postCopyTag]
	if postOutbound == nil {
		t.Fatalf("post-Tor outbound %q not generated", postCopyTag)
	}
	if got := postOutbound["detour"]; got != torTag {
		t.Fatalf("post-Tor outbound detour = %v, want Tor %q", got, torTag)
	}
	selector := outboundMap[chain.Name]
	if selector == nil {
		t.Fatalf("chain selector %q not generated", chain.Name)
	}
	selectorOutbounds, ok := selector["outbounds"].([]string)
	if !ok || len(selectorOutbounds) != 1 || selectorOutbounds[0] != postCopyTag {
		t.Fatalf("chain selector should exit via post node %q, got %#v", postCopyTag, selector["outbounds"])
	}
}

func TestBuildConfigSupportsAutomaticPostTorSelection(t *testing.T) {
	chain := storage.ProxyChain{
		ID:      "tor-chain-post-auto",
		Name:    "entry-tor-auto",
		Enabled: true,
		Nodes:   []string{"entry", storage.ChainTorNodeTag, storage.ChainAutoNodeTag},
	}

	builder := &ConfigBuilder{
		settings: &storage.Settings{
			FinalOutbound:     "Proxy",
			TorEnabled:        true,
			TorExecutablePath: "/usr/bin/tor",
		},
		nodes: []storage.Node{
			{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
			{Tag: "post-a", Type: "socks", Server: "127.0.0.1", ServerPort: 1082},
			{Tag: "post-b", Type: "socks", Server: "127.0.0.1", ServerPort: 1083},
		},
		inboundPorts: []storage.InboundPort{
			{ID: "port-1", Type: "mixed", Listen: "127.0.0.1", Port: 2081, Enabled: true, UseTorExit: true, TorChainID: chain.ID},
		},
		proxyChains: []storage.ProxyChain{chain},
	}

	config, err := builder.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	outboundMap := outboundsByTag(config.Outbounds)
	torTag := storage.GenerateChainTorOutboundTag(chain.ID)
	autoGroupTag := storage.GenerateChainNodeCopyTag(chain.Name, storage.ChainAutoNodeTag)
	autoGroup := outboundMap[autoGroupTag]
	if autoGroup == nil {
		t.Fatalf("post-Tor auto group %q not generated", autoGroupTag)
	}
	members, ok := autoGroup["outbounds"].([]string)
	if !ok || len(members) != 3 {
		t.Fatalf("post-Tor auto group members = %#v", autoGroup["outbounds"])
	}
	for _, candidate := range members {
		outbound := outboundMap[candidate]
		if outbound == nil {
			t.Fatalf("post-Tor auto candidate %q not generated", candidate)
		}
		if got := outbound["detour"]; got != torTag {
			t.Fatalf("post-Tor auto candidate detour = %v, want Tor %q", got, torTag)
		}
	}

	selector := outboundMap[chain.Name]
	if selector == nil {
		t.Fatalf("chain selector %q not generated", chain.Name)
	}
	selectorOutbounds, ok := selector["outbounds"].([]string)
	if !ok || len(selectorOutbounds) != 1 || selectorOutbounds[0] != autoGroupTag {
		t.Fatalf("chain selector should exit via auto group %q, got %#v", autoGroupTag, selector["outbounds"])
	}
}

func outboundsByTag(outbounds []Outbound) map[string]Outbound {
	outboundMap := make(map[string]Outbound, len(outbounds))
	for _, outbound := range outbounds {
		tag, _ := outbound["tag"].(string)
		if tag != "" {
			outboundMap[tag] = outbound
		}
	}
	return outboundMap
}
