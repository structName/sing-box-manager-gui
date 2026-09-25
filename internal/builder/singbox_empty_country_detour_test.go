package builder

import (
	"strings"
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

// forceEmptyCountryMidChain bypasses the allNodesExist pre-check by injecting a
// country hop that becomes empty only after the entry hop is emitted. We do that
// by calling buildOutbounds with a country code that has no nodes while
// temporarily pretending the pre-check passed — achieved by using Tor path where
// there is no allNodesExist gate, and by a direct non-Tor abort regression via
// buildOutbounds when countryNodes key casing would have historically mismatched.
func TestEmptyCountryMidChainDoesNotLeaveBrokenDetourOrOrphans(t *testing.T) {
	countryTag := storage.MakeChainCountryNodeTag("JP")
	chain := storage.ProxyChain{
		ID:      "chain-empty-jp",
		Name:    "sg-jp-us",
		Enabled: true,
		Nodes:   []string{"entry-sg", countryTag, "exit-us"},
	}
	b := &ConfigBuilder{
		settings: &storage.Settings{FinalOutbound: "Proxy"},
		nodes: []storage.Node{
			{Tag: "entry-sg", Type: "socks", Server: "127.0.0.1", ServerPort: 1080, Country: "SG"},
			{Tag: "exit-us", Type: "socks", Server: "127.0.0.1", ServerPort: 1081, Country: "US"},
			// No JP nodes → empty country hop
		},
		proxyChains: []storage.ProxyChain{chain},
	}

	outbounds, err := b.buildOutbounds()
	if err != nil {
		t.Fatalf("buildOutbounds() error = %v", err)
	}

	entryCopy := storage.GenerateChainNodeCopyTag(chain.Name, "entry-sg")
	exitCopy := storage.GenerateChainNodeCopyTag(chain.Name, "exit-us")
	groupCopy := storage.GenerateChainNodeCopyTag(chain.Name, countryTag)

	for _, outbound := range outbounds {
		tag, _ := outbound["tag"].(string)
		switch tag {
		case entryCopy, exitCopy, groupCopy, chain.Name:
			t.Fatalf("empty-country chain leaked outbound tag %q (broken/partial detour)", tag)
		}
		if detour, _ := outbound["detour"].(string); detour == entryCopy {
			t.Fatalf("orphan outbound %q still detours to aborted chain entry %q", tag, entryCopy)
		}
	}
}

func TestTorChainEmptyCountryMidHopRollsBackPartials(t *testing.T) {
	countryTag := storage.MakeChainCountryNodeTag("JP")
	chain := storage.ProxyChain{
		ID:      "tor-empty-jp",
		Name:    "entry-jp-tor",
		Enabled: true,
		Nodes:   []string{"entry", countryTag, storage.ChainTorNodeTag},
	}
	dataDir := t.TempDir()
	b := &ConfigBuilder{
		settings: &storage.Settings{
			FinalOutbound:     "Proxy",
			TorEnabled:        true,
			TorExecutablePath: "/usr/bin/tor",
		},
		nodes: []storage.Node{
			{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1080, Country: "SG"},
		},
		inboundPorts: []storage.InboundPort{{
			ID: "port-1", Type: "mixed", Listen: "127.0.0.1", Port: 2081,
			Enabled: true, UseTorExit: true, TorChainID: chain.ID,
		}},
		proxyChains: []storage.ProxyChain{chain},
		dataDir:     dataDir,
	}

	config, err := b.Build()
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	entryCopy := storage.GenerateChainNodeCopyTag(chain.Name, "entry")
	torTag := storage.GenerateChainTorOutboundTag(chain.ID)
	for _, outbound := range config.Outbounds {
		tag, _ := outbound["tag"].(string)
		if tag == entryCopy || tag == torTag || tag == chain.Name {
			t.Fatalf("Tor chain with empty country hop leaked tag %q", tag)
		}
		if detour, _ := outbound["detour"].(string); detour == entryCopy {
			t.Fatalf("orphan %q detours to rolled-back entry", tag)
		}
	}

	// Sanity: JSON should not mention the chain selector
	json, err := b.BuildJSON()
	if err != nil {
		t.Fatalf("BuildJSON() error = %v", err)
	}
	if strings.Contains(json, `"tag": "entry-jp-tor"`) {
		t.Fatalf("aborted Tor chain selector still present")
	}
}
