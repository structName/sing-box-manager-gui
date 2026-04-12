package builder

import (
	"testing"

	"github.com/xiaobei/singbox-manager/internal/storage"
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

	entryCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, "entry-sg")
	if got := outboundMap[entryCopyTag]["tag"]; got != entryCopyTag {
		t.Fatalf("entry copy not found, got: %v", got)
	}

	groupCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, chainCountryTag)
	groupOutbound, ok := outboundMap[groupCopyTag]
	if !ok {
		t.Fatalf("country group copy %q not found", groupCopyTag)
	}
	if got := groupOutbound["type"]; got != "urltest" {
		t.Fatalf("country group copy should be urltest, got %v", got)
	}

	candidateA := storage.GenerateChainCountryCandidateCopyTag(chain.Name, chainCountryTag, "hk-a")
	candidateB := storage.GenerateChainCountryCandidateCopyTag(chain.Name, chainCountryTag, "hk-b")

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

	exitCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, "exit-us")
	exitOutbound, ok := outboundMap[exitCopyTag]
	if !ok {
		t.Fatalf("exit copy %q not found", exitCopyTag)
	}
	if got := exitOutbound["detour"]; got != groupCopyTag {
		t.Fatalf("exit copy should detour to %q, got %v", groupCopyTag, got)
	}
}
