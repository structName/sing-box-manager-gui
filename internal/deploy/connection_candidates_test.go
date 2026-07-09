package deploy

import (
	"encoding/json"
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestDiscoverConnectionCandidatesEmptyInventoryReturnsJSONEmptyArray(t *testing.T) {
	candidates := DiscoverConnectionCandidates(CandidateInventoryInput{})
	if candidates == nil {
		t.Fatal("empty inventory returned nil candidates, want empty slice")
	}
	data, err := json.Marshal(struct {
		Data []ConnectionCandidate `json:"data"`
	}{Data: candidates})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(data) != `{"data":[]}` {
		t.Fatalf("empty candidates JSON = %s, want data array", data)
	}
}

func TestDiscoverConnectionCandidatesIncludesManagedRoutesAndTunnels(t *testing.T) {
	candidates := DiscoverConnectionCandidates(CandidateInventoryInput{
		Settings: &storage.Settings{MixedPort: 2080},
		Nodes: []storage.Node{
			{Tag: "hk-1", Source: "manual", SourceName: "手动添加"},
			{Tag: "jp-1", Source: "sub-1", SourceName: "机场 A"},
		},
		ProxyChains: []storage.ProxyChain{
			{ID: "chain-1", Name: "香港链路", Enabled: true},
			{ID: "chain-2", Name: "停用链路", Enabled: false},
		},
		InboundPorts: []storage.InboundPort{
			{ID: "port-1", Name: "香港隧道", Type: "mixed", Listen: "127.0.0.1", Port: 2081, Outbound: "hk-1", Enabled: true},
			{ID: "port-chain", Name: "链路隧道", Type: "socks", Listen: "127.0.0.1", Port: 2083, Outbound: "香港链路", Enabled: true},
			{ID: "port-2", Name: "HTTP 入站", Type: "http", Listen: "127.0.0.1", Port: 2082, Outbound: "hk-1", Enabled: true},
		},
		ServiceRunning: true,
	})

	byID := candidatesByID(candidates)

	node := byID["node:hk-1"]
	if node.Kind != CandidateKindNode || !node.Available || node.RequiresTemporaryEntrypoint || node.LocalEndpoint != "127.0.0.1:2081" || node.UnavailableReason != "" {
		t.Fatalf("node candidate = %#v", node)
	}
	if node.Source != "manual" || node.SourceName != "手动添加" {
		t.Fatalf("node source metadata = %#v", node)
	}
	nodeWithoutTunnel := byID["node:jp-1"]
	if nodeWithoutTunnel.Kind != CandidateKindNode || !nodeWithoutTunnel.Available || !nodeWithoutTunnel.RequiresTemporaryEntrypoint || nodeWithoutTunnel.LocalEndpoint != "" {
		t.Fatalf("node without reusable tunnel = %#v", nodeWithoutTunnel)
	}

	chain := byID["chain:chain-1"]
	if chain.Kind != CandidateKindProxyChain || !chain.Available || chain.RequiresTemporaryEntrypoint || chain.LocalEndpoint != "127.0.0.1:2083" || chain.Outbound != "香港链路" || chain.UnavailableReason != "" {
		t.Fatalf("chain candidate = %#v", chain)
	}

	disabledChain := byID["chain:chain-2"]
	if disabledChain.Available || disabledChain.UnavailableReason == "" {
		t.Fatalf("disabled chain candidate = %#v", disabledChain)
	}

	tunnel := byID["inbound:port-1"]
	if tunnel.Kind != CandidateKindInboundTunnel || !tunnel.Available || tunnel.RequiresTemporaryEntrypoint {
		t.Fatalf("tunnel candidate = %#v", tunnel)
	}
	if tunnel.LocalEndpoint != "127.0.0.1:2081" || tunnel.Outbound != "hk-1" {
		t.Fatalf("tunnel endpoint/outbound = %#v", tunnel)
	}
	if _, exists := byID["inbound:port-2"]; exists {
		t.Fatalf("HTTP inbound should not be reusable SSH tunnel: %#v", byID["inbound:port-2"])
	}

	if _, exists := byID["global:mixed"]; exists {
		t.Fatalf("legacy settings mixed port should not be advertised as a reusable tunnel: %#v", byID["global:mixed"])
	}
}

func TestDiscoverConnectionCandidatesMarksServiceDependentTunnelsUnavailable(t *testing.T) {
	candidates := DiscoverConnectionCandidates(CandidateInventoryInput{
		Settings: &storage.Settings{MixedPort: 2080},
		InboundPorts: []storage.InboundPort{
			{ID: "port-1", Name: "链路隧道", Type: "socks", Listen: "0.0.0.0", Port: 2081, Outbound: "Proxy", Enabled: true},
			{ID: "port-2", Name: "停用隧道", Type: "mixed", Listen: "::", Port: 2082, Outbound: "Proxy", Enabled: false},
		},
		ServiceRunning: false,
	})

	byID := candidatesByID(candidates)

	tunnel := byID["inbound:port-1"]
	if tunnel.Available || tunnel.UnavailableReason != "sing-box 服务未运行" {
		t.Fatalf("running-required tunnel = %#v", tunnel)
	}
	if tunnel.LocalEndpoint != "127.0.0.1:2081" {
		t.Fatalf("wildcard listen should be local endpoint, got %#v", tunnel)
	}

	disabled := byID["inbound:port-2"]
	if disabled.Available || disabled.UnavailableReason != "入站端口未启用" {
		t.Fatalf("disabled tunnel = %#v", disabled)
	}

	if _, exists := byID["global:mixed"]; exists {
		t.Fatalf("legacy settings mixed port should not be advertised as a reusable tunnel: %#v", byID["global:mixed"])
	}
}

func candidatesByID(candidates []ConnectionCandidate) map[string]ConnectionCandidate {
	byID := make(map[string]ConnectionCandidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	return byID
}
