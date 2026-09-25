package parser

import "testing"

func TestTrojanURLDefaultsServerNameWhenSNIAndHostOmitted(t *testing.T) {
	node, err := ParseURL("trojan://secret@trojan.example.com:443#t")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T", node.Extra["tls"])
	}
	if tls["server_name"] != "trojan.example.com" {
		t.Fatalf("server_name = %v, want trojan.example.com (default to server when sni/host omitted)", tls["server_name"])
	}
}

func TestTrojanURLKeepsExplicitSNI(t *testing.T) {
	node, err := ParseURL("trojan://secret@trojan.example.com:443?sni=edge.example.com#t")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "edge.example.com" {
		t.Fatalf("server_name = %v, want edge.example.com", tls["server_name"])
	}
}

func TestTrojanURLHostParamBeatsServerDefault(t *testing.T) {
	node, err := ParseURL("trojan://secret@trojan.example.com:443?host=cdn.example.com#t")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "cdn.example.com" {
		t.Fatalf("server_name = %v, want cdn.example.com", tls["server_name"])
	}
}

func TestVlessURLDefaultsServerNameWhenSNIAndHostOmitted(t *testing.T) {
	node, err := ParseURL("vless://11111111-1111-4111-8111-111111111111@vless.example.com:443?security=tls#v")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T", node.Extra["tls"])
	}
	if tls["server_name"] != "vless.example.com" {
		t.Fatalf("server_name = %v, want vless.example.com", tls["server_name"])
	}
}

func TestVlessRealityURLDefaultsServerNameWhenSNIAndHostOmitted(t *testing.T) {
	node, err := ParseURL("vless://11111111-1111-4111-8111-111111111111@vless.example.com:443?security=reality&pbk=abc&sid=01#v")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T", node.Extra["tls"])
	}
	if tls["server_name"] != "vless.example.com" {
		t.Fatalf("server_name = %v, want vless.example.com", tls["server_name"])
	}
	reality, ok := tls["reality"].(map[string]interface{})
	if !ok || reality["enabled"] != true {
		t.Fatalf("reality = %#v, want enabled", tls["reality"])
	}
}

func TestVlessURLKeepsExplicitSNI(t *testing.T) {
	node, err := ParseURL("vless://11111111-1111-4111-8111-111111111111@vless.example.com:443?security=tls&sni=edge.example.com#v")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "edge.example.com" {
		t.Fatalf("server_name = %v, want edge.example.com", tls["server_name"])
	}
}

func TestVlessURLSecurityNoneSkipsTLS(t *testing.T) {
	node, err := ParseURL("vless://11111111-1111-4111-8111-111111111111@vless.example.com:443#v")
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}
	if _, ok := node.Extra["tls"]; ok {
		t.Fatalf("tls unexpectedly set when security defaults to none: %#v", node.Extra["tls"])
	}
}
