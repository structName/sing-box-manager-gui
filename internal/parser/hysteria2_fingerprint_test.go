package parser

import "testing"

func TestHysteria2URLParsesFingerprintAndAlpn(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@example.com:443?sni=example.com&alpn=h3&fp=chrome#hy2-fp")
	if err != nil {
		t.Fatalf("ParseURL returned error: %v", err)
	}
	if node.Type != "hysteria2" {
		t.Fatalf("type = %q, want hysteria2", node.Type)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T, want map", node.Extra["tls"])
	}
	alpn, ok := tls["alpn"].([]string)
	if !ok || len(alpn) != 1 || alpn[0] != "h3" {
		t.Fatalf("alpn = %#v, want [h3]", tls["alpn"])
	}
	utls, ok := tls["utls"].(map[string]interface{})
	if !ok {
		t.Fatalf("utls type = %T, want map", tls["utls"])
	}
	if got := utls["fingerprint"]; got != "chrome" {
		t.Fatalf("fingerprint = %v, want chrome", got)
	}
	if got := utls["enabled"]; got != true {
		t.Fatalf("utls.enabled = %v, want true", got)
	}
}

func TestHysteria2URLAcceptsFingerprintAlias(t *testing.T) {
	node, err := ParseURL("hy2://secret@1.2.3.4:8443?fingerprint=firefox&alpn=h3#alias")
	if err != nil {
		t.Fatalf("ParseURL returned error: %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	utls := tls["utls"].(map[string]interface{})
	if got := utls["fingerprint"]; got != "firefox" {
		t.Fatalf("fingerprint = %v, want firefox", got)
	}
}

func TestHysteria2URLOmitsUtlsWhenFingerprintAbsent(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@example.com:443?sni=example.com#no-fp")
	if err != nil {
		t.Fatalf("ParseURL returned error: %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	if _, ok := tls["utls"]; ok {
		t.Fatalf("utls unexpectedly set: %#v", tls["utls"])
	}
}
