package parser

import "testing"

// Share links with an unencoded "@" inside the password must still split on the
// final "@" before host:port (same as TUIC/SOCKS/SIP002 SS). Using Index made
// Trojan/AnyTLS/HY2 treat the suffix of the password as part of the hostname.

func TestTrojanShareLinkPasswordWithAtSign(t *testing.T) {
	node, err := ParseURL("trojan://p@ss@host.example.com:443?sni=host.example.com#at-pw")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Server != "host.example.com" {
		t.Fatalf("server = %q, want host.example.com", node.Server)
	}
	if node.ServerPort != 443 {
		t.Fatalf("port = %d, want 443", node.ServerPort)
	}
	if got, _ := node.Extra["password"].(string); got != "p@ss" {
		t.Fatalf("password = %q, want p@ss", got)
	}
}

func TestAnyTLSShareLinkPasswordWithAtSign(t *testing.T) {
	node, err := ParseURL("anytls://p@ss@host.example.com:443?sni=host.example.com#at-pw")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Server != "host.example.com" {
		t.Fatalf("server = %q, want host.example.com", node.Server)
	}
	if got, _ := node.Extra["password"].(string); got != "p@ss" {
		t.Fatalf("password = %q, want p@ss", got)
	}
}

func TestHysteria2ShareLinkPasswordWithAtSign(t *testing.T) {
	node, err := ParseURL("hysteria2://p@ss@host.example.com:443?sni=host.example.com#at-pw")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Server != "host.example.com" {
		t.Fatalf("server = %q, want host.example.com", node.Server)
	}
	if got, _ := node.Extra["password"].(string); got != "p@ss" {
		t.Fatalf("password = %q, want p@ss", got)
	}
}

func TestVLESSShareLinkStillParsesUUID(t *testing.T) {
	// UUID never contains "@"; LastIndex must remain equivalent to Index.
	node, err := ParseURL("vless://11111111-1111-4111-8111-111111111111@host.example.com:443?security=tls&sni=host.example.com#uuid")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Server != "host.example.com" {
		t.Fatalf("server = %q", node.Server)
	}
	if got, _ := node.Extra["uuid"].(string); got != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("uuid = %q", got)
	}
}

func TestTrojanShareLinkPasswordWithAtAndIPv6Host(t *testing.T) {
	node, err := ParseURL("trojan://p@ss@[2001:db8::1]:8443?sni=host.example.com#v6")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Server != "2001:db8::1" {
		t.Fatalf("server = %q, want 2001:db8::1", node.Server)
	}
	if node.ServerPort != 8443 {
		t.Fatalf("port = %d, want 8443", node.ServerPort)
	}
	if got, _ := node.Extra["password"].(string); got != "p@ss" {
		t.Fatalf("password = %q, want p@ss", got)
	}
}
