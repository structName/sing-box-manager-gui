package builder

import (
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestBuildOutboundsSupportsCountryChainNode(t *testing.T) {
	chainCountryTag := storage.MakeChainCountryNodeTag("HK")
	chain := storage.ProxyChain{
		ID:      "chain-1",
		Name:    "smart-chain",
		Enabled: true,
		Nodes:   []string{"entry-sg", chainCountryTag, "exit-us"},
	}

	builder := &ConfigBuilder{
		settings: &storage.Settings{
			FinalOutbound: "Proxy",
		},
		nodes: []storage.Node{
			{Tag: "entry-sg", Type: "trojan", Server: "sg.example.com", ServerPort: 443, Country: "SG"},
			{Tag: "hk-a", Type: "trojan", Server: "hk-a.example.com", ServerPort: 443, Country: "HK"},
			{Tag: "hk-b", Type: "trojan", Server: "hk-b.example.com", ServerPort: 443, Country: "HK"},
			{Tag: "exit-us", Type: "trojan", Server: "us.example.com", ServerPort: 443, Country: "US"},
		},
		proxyChains: []storage.ProxyChain{chain},
	}

	outbounds, err := builder.buildOutbounds()
	if err != nil {
		t.Fatalf("buildOutbounds returned error: %v", err)
	}

	outboundMap := make(map[string]Outbound, len(outbounds))
	for _, outbound := range outbounds {
		tag, _ := outbound["tag"].(string)
		if tag != "" {
			outboundMap[tag] = outbound
		}
	}

	entryCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, "entry-sg", 0)
	if got := outboundMap[entryCopyTag]["tag"]; got != entryCopyTag {
		t.Fatalf("entry copy not found, got: %v", got)
	}

	groupCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, chainCountryTag, 1)
	groupOutbound, ok := outboundMap[groupCopyTag]
	if !ok {
		t.Fatalf("country group copy %q not found", groupCopyTag)
	}
	if got := groupOutbound["type"]; got != "urltest" {
		t.Fatalf("country group copy should be urltest, got %v", got)
	}

	candidateA := storage.GenerateChainCountryCandidateCopyTag(chain.Name, chainCountryTag, "hk-a", 1)
	candidateB := storage.GenerateChainCountryCandidateCopyTag(chain.Name, chainCountryTag, "hk-b", 1)

	for _, candidateTag := range []string{candidateA, candidateB} {
		outbound, exists := outboundMap[candidateTag]
		if !exists {
			t.Fatalf("candidate outbound %q not found", candidateTag)
		}
		if got := outbound["detour"]; got != entryCopyTag {
			t.Fatalf("candidate outbound %q should detour to %q, got %v", candidateTag, entryCopyTag, got)
		}
	}

	groupMembers, ok := groupOutbound["outbounds"].([]string)
	if !ok {
		t.Fatalf("country group outbounds should be []string, got %T", groupOutbound["outbounds"])
	}
	if len(groupMembers) != 2 || groupMembers[0] != candidateA || groupMembers[1] != candidateB {
		t.Fatalf("unexpected country group members: %#v", groupMembers)
	}

	exitCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, "exit-us", 2)
	exitOutbound, ok := outboundMap[exitCopyTag]
	if !ok {
		t.Fatalf("exit copy %q not found", exitCopyTag)
	}
	if got := exitOutbound["detour"]; got != groupCopyTag {
		t.Fatalf("exit copy should detour to %q, got %v", groupCopyTag, got)
	}
}

func TestBuildOutboundsRepeatedHopGetsDistinctCopyTagsAndDetours(t *testing.T) {
	chain := storage.ProxyChain{
		ID:      "chain-repeat",
		Name:    "aba-chain",
		Enabled: true,
		Nodes:   []string{"A", "B", "A"},
	}

	builder := &ConfigBuilder{
		settings: &storage.Settings{FinalOutbound: "Proxy"},
		nodes: []storage.Node{
			{Tag: "A", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
			{Tag: "B", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
		},
		proxyChains: []storage.ProxyChain{chain},
	}

	outbounds, err := builder.buildOutbounds()
	if err != nil {
		t.Fatalf("buildOutbounds() error = %v", err)
	}

	outboundMap := make(map[string]Outbound, len(outbounds))
	for _, outbound := range outbounds {
		tag, _ := outbound["tag"].(string)
		if tag != "" {
			outboundMap[tag] = outbound
		}
	}

	entryA := storage.GenerateChainNodeCopyTag(chain.Name, "A", 0)
	midB := storage.GenerateChainNodeCopyTag(chain.Name, "B", 1)
	exitA := storage.GenerateChainNodeCopyTag(chain.Name, "A", 2)

	if entryA == exitA {
		t.Fatal("entry and exit A copies must differ")
	}
	if outboundMap[entryA] == nil {
		t.Fatalf("missing entry copy %q", entryA)
	}
	if outboundMap[midB] == nil {
		t.Fatalf("missing middle copy %q", midB)
	}
	if outboundMap[exitA] == nil {
		t.Fatalf("missing exit copy %q", exitA)
	}

	if got := outboundMap[entryA]["detour"]; got != nil {
		t.Fatalf("entry A should have no detour, got %v", got)
	}
	if got := outboundMap[midB]["detour"]; got != entryA {
		t.Fatalf("B detour = %v, want %q", got, entryA)
	}
	if got := outboundMap[exitA]["detour"]; got != midB {
		t.Fatalf("exit A detour = %v, want %q", got, midB)
	}

	selector := outboundMap[chain.Name]
	if selector == nil {
		t.Fatalf("missing chain selector %q", chain.Name)
	}
	outs, ok := selector["outbounds"].([]string)
	if !ok || len(outs) != 1 || outs[0] != exitA {
		t.Fatalf("selector outbounds = %#v, want [%q]", selector["outbounds"], exitA)
	}
	if got := selector["default"]; got != exitA {
		t.Fatalf("selector default = %v, want %q", got, exitA)
	}
}
