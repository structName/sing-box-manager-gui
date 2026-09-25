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

func TestParseClashYAMLTuicHeartbeatInterval(t *testing.T) {
	yaml := `
proxies:
  - name: tuic-hb
    type: tuic
    server: tuic.example.com
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    password: secret
    heartbeat-interval: 10000
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	// Clash Meta uses milliseconds; store as duration so sing-box accepts it.
	if got, _ := nodes[0].Extra["heartbeat"].(string); got != "10000ms" {
		t.Fatalf("heartbeat = %v, want 10000ms", nodes[0].Extra["heartbeat"])
	}
}

func TestTuicURLBareHeartbeatNormalizedByBuilder(t *testing.T) {
	// Share links often omit the unit (heartbeat=10). Parser keeps the raw
	// value; builder must add "s" before emit — covered in builder tests, but
	// assert the URL path still stores the bare string so we do not regress.
	node, err := ParseURL("tuic://11111111-1111-4111-8111-111111111111:secret@tuic.example.com:443?heartbeat=10#t")
	if err != nil {
		t.Fatalf("ParseURL error: %v", err)
	}
	if got, _ := node.Extra["heartbeat"].(string); got != "10" {
		t.Fatalf("parser heartbeat = %v, want raw \"10\" (builder normalizes)", node.Extra["heartbeat"])
	}
}
