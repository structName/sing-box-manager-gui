package deploy

import (
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

const (
	CandidateKindNode          = "node"
	CandidateKindProxyChain    = "proxy_chain"
	CandidateKindInboundTunnel = "inbound_tunnel"
)

type ConnectionCandidate struct {
	ID                          string `json:"id"`
	Kind                        string `json:"kind"`
	DisplayName                 string `json:"display_name"`
	Source                      string `json:"source,omitempty"`
	SourceName                  string `json:"source_name,omitempty"`
	NodeTag                     string `json:"node_tag,omitempty"`
	ChainID                     string `json:"chain_id,omitempty"`
	Outbound                    string `json:"outbound,omitempty"`
	LocalEndpoint               string `json:"local_endpoint,omitempty"`
	RequiresServiceRunning      bool   `json:"requires_service_running"`
	RequiresTemporaryEntrypoint bool   `json:"requires_temporary_entrypoint"`
	Available                   bool   `json:"available"`
	UnavailableReason           string `json:"unavailable_reason,omitempty"`
}

type CandidateInventoryInput struct {
	Settings       *storage.Settings
	Nodes          []storage.Node
	ProxyChains    []storage.ProxyChain
	InboundPorts   []storage.InboundPort
	ServiceRunning bool
}

func DiscoverConnectionCandidates(input CandidateInventoryInput) []ConnectionCandidate {
	candidates := []ConnectionCandidate{}

	for _, node := range input.Nodes {
		tag := strings.TrimSpace(node.Tag)
		if tag == "" {
			continue
		}
		localEndpoint := reusableInboundEndpoint(input.InboundPorts, input.ServiceRunning, func(port storage.InboundPort) bool {
			return !port.UseTorExit && strings.TrimSpace(port.Outbound) == tag
		})
		candidate := ConnectionCandidate{
			ID:          "node:" + tag,
			Kind:        CandidateKindNode,
			DisplayName: tag,
			Source:      node.Source,
			SourceName:  node.SourceName,
			NodeTag:     tag,
			Outbound:    tag,
			Available:   true,
		}
		if localEndpoint != "" {
			candidate.LocalEndpoint = localEndpoint
		} else {
			candidate.RequiresTemporaryEntrypoint = true
		}
		candidates = append(candidates, candidate)
	}

	for _, chain := range input.ProxyChains {
		if strings.TrimSpace(chain.ID) == "" || strings.TrimSpace(chain.Name) == "" {
			continue
		}
		localEndpoint := reusableInboundEndpoint(input.InboundPorts, input.ServiceRunning, func(port storage.InboundPort) bool {
			if port.UseTorExit {
				return strings.TrimSpace(port.TorChainID) == strings.TrimSpace(chain.ID)
			}
			return strings.TrimSpace(port.Outbound) == strings.TrimSpace(chain.Name)
		})
		candidate := ConnectionCandidate{
			ID:          "chain:" + chain.ID,
			Kind:        CandidateKindProxyChain,
			DisplayName: chain.Name,
			Source:      "proxy_chain",
			ChainID:     chain.ID,
			Outbound:    chain.Name,
			Available:   chain.Enabled,
		}
		if !chain.Enabled {
			candidate.UnavailableReason = "代理链路未启用"
		} else if localEndpoint != "" {
			candidate.LocalEndpoint = localEndpoint
		} else {
			candidate.RequiresTemporaryEntrypoint = true
		}
		candidates = append(candidates, candidate)
	}

	for _, port := range input.InboundPorts {
		if !isReusableTunnelInbound(port) {
			continue
		}
		candidate := ConnectionCandidate{
			ID:                     "inbound:" + port.ID,
			Kind:                   CandidateKindInboundTunnel,
			DisplayName:            tunnelDisplayName(port),
			Source:                 "inbound_port",
			Outbound:               inboundPortOutbound(port, input.ProxyChains),
			LocalEndpoint:          formatEndpoint(port.Listen, port.Port),
			RequiresServiceRunning: true,
			Available:              port.Enabled && input.ServiceRunning,
		}
		if !port.Enabled {
			candidate.UnavailableReason = "入站端口未启用"
		} else if !input.ServiceRunning {
			candidate.UnavailableReason = "sing-box 服务未运行"
		}
		candidates = append(candidates, candidate)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Kind != candidates[j].Kind {
			return candidates[i].Kind < candidates[j].Kind
		}
		if candidates[i].DisplayName != candidates[j].DisplayName {
			return candidates[i].DisplayName < candidates[j].DisplayName
		}
		return candidates[i].ID < candidates[j].ID
	})

	return candidates
}

func reusableInboundEndpoint(ports []storage.InboundPort, serviceRunning bool, matches func(storage.InboundPort) bool) string {
	if !serviceRunning {
		return ""
	}
	for _, port := range ports {
		if !port.Enabled || !isReusableTunnelInbound(port) || !matches(port) {
			continue
		}
		return formatEndpoint(port.Listen, port.Port)
	}
	return ""
}

func isReusableTunnelInbound(port storage.InboundPort) bool {
	switch strings.ToLower(strings.TrimSpace(port.Type)) {
	case "mixed", "socks":
		return strings.TrimSpace(port.ID) != "" && port.Port > 0
	default:
		return false
	}
}

func inboundPortOutbound(port storage.InboundPort, chains []storage.ProxyChain) string {
	if !port.UseTorExit {
		return strings.TrimSpace(port.Outbound)
	}
	for _, chain := range chains {
		if chain.ID == port.TorChainID {
			return chain.Name
		}
	}
	return strings.TrimSpace(port.TorChainID)
}

func tunnelDisplayName(port storage.InboundPort) string {
	name := strings.TrimSpace(port.Name)
	if name == "" {
		name = port.ID
	}
	return fmt.Sprintf("%s (%s)", name, strings.ToLower(strings.TrimSpace(port.Type)))
}

func formatEndpoint(listen string, port int) string {
	host := strings.TrimSpace(listen)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}
