package parser

import "testing"

func TestParseClashYAMLAddsTLSForProtocolsThatRequireIt(t *testing.T) {
	tests := []struct {
		name      string
		proxyYAML string
		wantType  string
	}{
		{
			name: "hysteria2 without explicit tls flag",
			proxyYAML: `
  - name: hy2-node
    type: hysteria2
    server: 192.0.2.1
    port: 443
    password: secret
    sni: edge.example.com
    skip-cert-verify: true
`,
			wantType: "hysteria2",
		},
		{
			name: "tuic without explicit tls flag",
			proxyYAML: `
  - name: tuic-node
    type: tuic
    server: 192.0.2.2
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    password: secret
    sni: tuic.example.com
    skip-cert-verify: true
`,
			wantType: "tuic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes, err := ParseClashYAML("proxies:\n" + tt.proxyYAML)
			if err != nil {
				t.Fatalf("ParseClashYAML() error = %v", err)
			}
			if len(nodes) != 1 {
				t.Fatalf("node count = %d, want 1", len(nodes))
			}
			if nodes[0].Type != tt.wantType {
				t.Fatalf("node type = %q, want %q", nodes[0].Type, tt.wantType)
			}
			tls, ok := nodes[0].Extra["tls"].(map[string]interface{})
			if !ok {
				t.Fatalf("tls type = %T, want map[string]interface{}", nodes[0].Extra["tls"])
			}
			if tls["enabled"] != true || tls["insecure"] != true {
				t.Fatalf("tls flags = %#v, want enabled and insecure", tls)
			}
			if tls["server_name"] == nil {
				t.Fatalf("tls server_name missing: %#v", tls)
			}
		})
	}
}

func TestParseClashYAMLSkipsOnlyMalformedProxy(t *testing.T) {
	nodes, err := ParseClashYAML(`
proxies:
  - name: usable-trojan
    type: trojan
    server: trojan.example.com
    port: 443
    password: secret
  - name: malformed-vmess
    type: vmess
    server: vmess.example.com
    port: not-a-number
    uuid: 11111111-1111-4111-8111-111111111111
  - name: usable-socks
    type: socks5
    server: 127.0.0.1
    port: 1080
`)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("node count = %d, want 2 usable nodes", len(nodes))
	}
	if nodes[0].Tag != "usable-trojan" || nodes[1].Tag != "usable-socks" {
		t.Fatalf("parsed node tags = %q, %q", nodes[0].Tag, nodes[1].Tag)
	}
}

func TestClashYAMLMapsHysteria2PortsAndHopInterval(t *testing.T) {
	yaml := `
proxies:
  - name: hy2-hop
    type: hysteria2
    server: hy2.example.com
    port: 443
    password: secret
    sni: hy2.example.com
    ports: 20000-50000
    hop-interval: 15
    up: 100 Mbps
    down: 200
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	n := nodes[0]
	if n.Type != "hysteria2" {
		t.Fatalf("type = %q, want hysteria2", n.Type)
	}
	if got, _ := n.Extra["ports"].(string); got != "20000-50000" {
		t.Fatalf("ports = %v, want 20000-50000", n.Extra["ports"])
	}
	if got := n.Extra["hop_interval"]; got != 15 {
		t.Fatalf("hop_interval = %v (%T), want 15", got, got)
	}
	if got, _ := n.Extra["up"].(string); got != "100 Mbps" {
		t.Fatalf("up = %v, want 100 Mbps", n.Extra["up"])
	}
	if got, _ := n.Extra["down"].(string); got != "200" {
		t.Fatalf("down = %v, want 200", n.Extra["down"])
	}
}
