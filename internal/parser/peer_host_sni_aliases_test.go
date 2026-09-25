package parser

import "testing"

func TestHysteria2PeerAsSNI(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@1.2.3.4:443?peer=cdn.example.com#hy2-peer")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if tls == nil {
		t.Fatal("tls missing")
	}
	if got, _ := tls["server_name"].(string); got != "cdn.example.com" {
		t.Fatalf("server_name = %q, want cdn.example.com from peer=", got)
	}
}

func TestHysteriaSchemePeerAsSNI(t *testing.T) {
	// hysteria:// is routed to the HY2 parser; v1-style peer= must still set SNI.
	node, err := ParseURL("hysteria://1.2.3.4:443?auth=secret&peer=sni.example.com&alpn=h3#hy1-peer")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if got, _ := tls["server_name"].(string); got != "sni.example.com" {
		t.Fatalf("server_name = %q, want sni.example.com from peer=", got)
	}
}

func TestHysteria2SNIPrefersOverPeer(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@1.2.3.4:443?sni=explicit.example.com&peer=ignored.example.com#pref")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if got, _ := tls["server_name"].(string); got != "explicit.example.com" {
		t.Fatalf("server_name = %q, want explicit sni=", got)
	}
}

func TestAnyTLSHostAsSNI(t *testing.T) {
	node, err := ParseURL("anytls://pass@any.example.com:443?host=cdn.example.com#any-host")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if got, _ := tls["server_name"].(string); got != "cdn.example.com" {
		t.Fatalf("server_name = %q, want cdn.example.com from host=", got)
	}
}

func TestTuicPeerAsSNI(t *testing.T) {
	node, err := ParseURL("tuic://11111111-1111-4111-8111-111111111111:pass@tuic.example.com:443?peer=cdn.example.com#tuic-peer")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if got, _ := tls["server_name"].(string); got != "cdn.example.com" {
		t.Fatalf("server_name = %q, want cdn.example.com from peer=", got)
	}
}
