package parser

import "testing"

func TestHysteria2URLDefaultsServerNameWhenSNIOmitted(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@hy2.example.com:443#hy2")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T", node.Extra["tls"])
	}
	if tls["server_name"] != "hy2.example.com" {
		t.Fatalf("server_name = %v, want hy2.example.com (default to server when sni omitted)", tls["server_name"])
	}
}

func TestHysteria2URLKeepsExplicitSNI(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@hy2.example.com:443?sni=edge.example.com#hy2")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "edge.example.com" {
		t.Fatalf("server_name = %v, want edge.example.com", tls["server_name"])
	}
}

func TestAnyTLSURLDefaultsServerNameWhenSNIOmitted(t *testing.T) {
	node, err := ParseURL("anytls://secret@any.example.com:443#any")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T", node.Extra["tls"])
	}
	if tls["server_name"] != "any.example.com" {
		t.Fatalf("server_name = %v, want any.example.com", tls["server_name"])
	}
}

func TestTuicURLDefaultsServerNameWhenSNIOmitted(t *testing.T) {
	node, err := ParseURL("tuic://11111111-1111-4111-8111-111111111111:pass@tuic.example.com:443#tuic")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T", node.Extra["tls"])
	}
	if tls["server_name"] != "tuic.example.com" {
		t.Fatalf("server_name = %v, want tuic.example.com", tls["server_name"])
	}
	if tls["disable_sni"] != nil {
		t.Fatalf("disable_sni unexpectedly set: %#v", tls)
	}
}

func TestTuicURLDisableSNISkipsServerName(t *testing.T) {
	node, err := ParseURL("tuic://11111111-1111-4111-8111-111111111111:pass@tuic.example.com:443?disable-sni=1#tuic")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	if tls["disable_sni"] != true {
		t.Fatalf("disable_sni = %v, want true", tls["disable_sni"])
	}
	if tls["server_name"] != nil {
		t.Fatalf("server_name = %v, want unset when disable-sni", tls["server_name"])
	}
}
