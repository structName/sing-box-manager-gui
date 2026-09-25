package builder

import (
	"strings"
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestBuildValidatedJSONReportsSkippedChainForMissingHop(t *testing.T) {
	b := NewConfigBuilder(
		&storage.Settings{FinalOutbound: "Proxy"},
		[]storage.Node{
			{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
			{Tag: "exit", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
		},
		nil,
		nil,
		[]storage.ProxyChain{{
			ID:      "chain-missing",
			Name:    "broken-chain",
			Enabled: true,
			Nodes:   []string{"entry", "gone-node", "exit"},
		}},
	)

	result, err := b.BuildValidatedJSON(nil)
	if err != nil {
		t.Fatalf("BuildValidatedJSON() error = %v", err)
	}
	if len(result.SkippedChains) != 1 {
		t.Fatalf("SkippedChains = %#v, want 1", result.SkippedChains)
	}
	got := result.SkippedChains[0]
	if got.ID != "chain-missing" || got.Name != "broken-chain" {
		t.Fatalf("skipped chain identity = %#v", got)
	}
	if !strings.Contains(got.Reason, "gone-node") {
		t.Fatalf("reason = %q, want mention of gone-node", got.Reason)
	}
	if strings.Contains(result.JSON, `"tag": "broken-chain"`) {
		t.Fatalf("skipped chain selector should not appear in JSON")
	}
}

func TestBuildValidatedJSONReportsSkippedChainForEmptyCountryHop(t *testing.T) {
	countryTag := storage.MakeChainCountryNodeTag("JP")
	b := NewConfigBuilder(
		&storage.Settings{FinalOutbound: "Proxy"},
		[]storage.Node{
			{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1080, Country: "SG"},
			{Tag: "exit", Type: "socks", Server: "127.0.0.1", ServerPort: 1081, Country: "US"},
		},
		nil,
		nil,
		[]storage.ProxyChain{{
			ID:      "chain-empty-country",
			Name:    "jp-mid",
			Enabled: true,
			Nodes:   []string{"entry", countryTag, "exit"},
		}},
	)

	result, err := b.BuildValidatedJSON(nil)
	if err != nil {
		t.Fatalf("BuildValidatedJSON() error = %v", err)
	}
	if len(result.SkippedChains) != 1 {
		t.Fatalf("SkippedChains = %#v, want 1", result.SkippedChains)
	}
	if !strings.Contains(result.SkippedChains[0].Reason, "no JP candidates") {
		t.Fatalf("reason = %q", result.SkippedChains[0].Reason)
	}
	if strings.Contains(result.JSON, `"tag": "jp-mid"`) {
		t.Fatalf("empty-country chain should not emit selector")
	}
}

func TestBuildValidatedJSONKeepsHealthyChainOutOfSkippedList(t *testing.T) {
	b := NewConfigBuilder(
		&storage.Settings{FinalOutbound: "Proxy"},
		[]storage.Node{
			{Tag: "entry", Type: "socks", Server: "127.0.0.1", ServerPort: 1080},
			{Tag: "exit", Type: "socks", Server: "127.0.0.1", ServerPort: 1081},
		},
		nil,
		nil,
		[]storage.ProxyChain{{
			ID:      "chain-ok",
			Name:    "ok-chain",
			Enabled: true,
			Nodes:   []string{"entry", "exit"},
		}},
	)

	result, err := b.BuildValidatedJSON(nil)
	if err != nil {
		t.Fatalf("BuildValidatedJSON() error = %v", err)
	}
	if len(result.SkippedChains) != 0 {
		t.Fatalf("SkippedChains = %#v, want none", result.SkippedChains)
	}
	if !strings.Contains(result.JSON, `"tag": "ok-chain"`) {
		t.Fatalf("healthy chain selector missing from JSON")
	}
}
