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
