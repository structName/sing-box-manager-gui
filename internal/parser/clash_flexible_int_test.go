package parser

import "testing"

func TestParseClashYAMLAcceptsQuotedNumericFields(t *testing.T) {
	nodes, err := ParseClashYAML(`
proxies:
  - name: quoted-ss
    type: ss
    server: 192.0.2.10
    port: "8388"
    cipher: aes-128-gcm
    password: secret
  - name: quoted-vmess
    type: vmess
    server: 192.0.2.11
    port: "443"
    uuid: 11111111-1111-4111-8111-111111111111
    alterId: "2"
    cipher: auto
    network: ws
    ws-opts:
      path: /ws
      max-early-data: "2048"
      early-data-header-name: Sec-WebSocket-Protocol
  - name: quoted-anytls
    type: anytls
    server: 192.0.2.12
    port: 8443
    password: secret
    idle-session-check-interval: "30"
    idle-session-timeout: "45"
    min-idle-session: "5"
  - name: int-ss
    type: ss
    server: 192.0.2.13
    port: 8389
    cipher: aes-128-gcm
    password: secret
`)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 4 {
		t.Fatalf("node count = %d, want 4", len(nodes))
	}

	if nodes[0].Tag != "quoted-ss" || nodes[0].ServerPort != 8388 {
		t.Fatalf("quoted-ss = tag=%q port=%d", nodes[0].Tag, nodes[0].ServerPort)
	}

	if nodes[1].Tag != "quoted-vmess" || nodes[1].ServerPort != 443 {
		t.Fatalf("quoted-vmess = tag=%q port=%d", nodes[1].Tag, nodes[1].ServerPort)
	}
	if got := nodes[1].Extra["alter_id"]; got != 2 {
		t.Fatalf("alter_id = %v (%T), want 2", got, got)
	}
	transport, ok := nodes[1].Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", nodes[1].Extra)
	}
	if got := transport["max_early_data"]; got != 2048 {
		t.Fatalf("max_early_data = %v (%T), want 2048", got, got)
	}

	if nodes[2].Tag != "quoted-anytls" {
		t.Fatalf("quoted-anytls tag = %q", nodes[2].Tag)
	}
	if got := nodes[2].Extra["idle_session_check_interval"]; got != "30s" {
		t.Fatalf("idle_session_check_interval = %v, want 30s", got)
	}
	if got := nodes[2].Extra["idle_session_timeout"]; got != "45s" {
		t.Fatalf("idle_session_timeout = %v, want 45s", got)
	}
	if got := nodes[2].Extra["min_idle_session"]; got != 5 {
		t.Fatalf("min_idle_session = %v (%T), want 5", got, got)
	}

	if nodes[3].Tag != "int-ss" || nodes[3].ServerPort != 8389 {
		t.Fatalf("int-ss = tag=%q port=%d", nodes[3].Tag, nodes[3].ServerPort)
	}
}

func TestParseClashYAMLStillSkipsNonNumericPort(t *testing.T) {
	nodes, err := ParseClashYAML(`
proxies:
  - name: usable
    type: socks5
    server: 127.0.0.1
    port: 1080
  - name: bad-port
    type: ss
    server: 192.0.2.1
    port: not-a-number
    cipher: aes-128-gcm
    password: secret
`)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 1 || nodes[0].Tag != "usable" {
		t.Fatalf("nodes = %#v, want only usable", nodes)
	}
}
