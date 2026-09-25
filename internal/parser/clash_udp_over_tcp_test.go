package parser

import "testing"

func TestClashYAMLMapsShadowsocksUDPOverTCP(t *testing.T) {
	nodes, err := ParseClashYAML(`
proxies:
  - name: ss-uot
    type: ss
    server: 192.0.2.10
    port: 8388
    cipher: aes-128-gcm
    password: secret
    udp-over-tcp: true
    udp-over-tcp-version: 2
`)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(nodes))
	}
	uot, ok := nodes[0].Extra["udp_over_tcp"].(map[string]interface{})
	if !ok {
		t.Fatalf("udp_over_tcp type = %T, want map", nodes[0].Extra["udp_over_tcp"])
	}
	if uot["enabled"] != true {
		t.Fatalf("enabled = %v, want true", uot["enabled"])
	}
	if uot["version"] != 2 {
		t.Fatalf("version = %v, want 2", uot["version"])
	}
}

func TestClashYAMLMapsSocks5UDPOverTCPWithoutVersion(t *testing.T) {
	nodes, err := ParseClashYAML(`
proxies:
  - name: socks-uot
    type: socks5
    server: 192.0.2.11
    port: 1080
    username: u
    password: p
    udp-over-tcp: true
`)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(nodes))
	}
	uot, ok := nodes[0].Extra["udp_over_tcp"].(map[string]interface{})
	if !ok {
		t.Fatalf("udp_over_tcp type = %T, want map", nodes[0].Extra["udp_over_tcp"])
	}
	if uot["enabled"] != true {
		t.Fatalf("enabled = %v, want true", uot["enabled"])
	}
	if _, hasVersion := uot["version"]; hasVersion {
		t.Fatalf("version should be omitted when unset, got %#v", uot["version"])
	}
}

func TestClashYAMLOmitsUDPOverTCPWhenFalse(t *testing.T) {
	nodes, err := ParseClashYAML(`
proxies:
  - name: ss-plain
    type: ss
    server: 192.0.2.12
    port: 8388
    cipher: aes-128-gcm
    password: secret
`)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(nodes))
	}
	if _, ok := nodes[0].Extra["udp_over_tcp"]; ok {
		t.Fatalf("udp_over_tcp should be absent, got %#v", nodes[0].Extra["udp_over_tcp"])
	}
}

func TestClashYAMLMapsSocks4UDPOverTCP(t *testing.T) {
	nodes, err := ParseClashYAML(`
proxies:
  - name: socks4-uot
    type: socks4
    server: 192.0.2.13
    port: 1080
    udp-over-tcp: true
    udp-over-tcp-version: 1
`)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(nodes))
	}
	uot, ok := nodes[0].Extra["udp_over_tcp"].(map[string]interface{})
	if !ok {
		t.Fatalf("udp_over_tcp type = %T, want map", nodes[0].Extra["udp_over_tcp"])
	}
	if uot["enabled"] != true || uot["version"] != 1 {
		t.Fatalf("uot = %#v, want enabled + version 1", uot)
	}
}
