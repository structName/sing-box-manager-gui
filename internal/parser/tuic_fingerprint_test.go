package parser

import "testing"

func TestTUICURLParsesFpQuery(t *testing.T) {
	node, err := ParseURL("tuic://11111111-1111-4111-8111-111111111111:secret@tuic.example.com:443" +
		"?sni=tuic.example.com&alpn=h3&fp=chrome#tuic-fp")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Type != "tuic" {
		t.Fatalf("type = %q, want tuic", node.Type)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls type = %T, want map", node.Extra["tls"])
	}
	utls, ok := tls["utls"].(map[string]interface{})
	if !ok {
		t.Fatalf("utls missing — fp= should enable uTLS; tls=%#v", tls)
	}
	if got := utls["fingerprint"]; got != "chrome" {
		t.Fatalf("fingerprint = %v, want chrome", got)
	}
	if got := utls["enabled"]; got != true {
		t.Fatalf("utls.enabled = %v, want true", got)
	}
}

func TestTUICURLAcceptsFingerprintAlias(t *testing.T) {
	node, err := ParseURL("tuic://11111111-1111-4111-8111-111111111111:secret@tuic.example.com:443" +
		"?sni=tuic.example.com&fingerprint=firefox#tuic-fp-alias")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	utls, _ := tls["utls"].(map[string]interface{})
	if utls == nil {
		t.Fatalf("utls missing — fingerprint= should enable uTLS; tls=%#v", tls)
	}
	if got := utls["fingerprint"]; got != "firefox" {
		t.Fatalf("fingerprint = %v, want firefox from fingerprint=", got)
	}
}

func TestTUICURLFpPrefersOverFingerprintAlias(t *testing.T) {
	node, err := ParseURL("tuic://11111111-1111-4111-8111-111111111111:secret@tuic.example.com:443" +
		"?sni=tuic.example.com&fp=chrome&fingerprint=firefox#tuic-fp-pref")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	utls, _ := tls["utls"].(map[string]interface{})
	if got := utls["fingerprint"]; got != "chrome" {
		t.Fatalf("fingerprint = %v, want chrome from fp= preference", got)
	}
}

func TestTUICURLOmitsUTLSWhenFingerprintAbsent(t *testing.T) {
	node, err := ParseURL("tuic://11111111-1111-4111-8111-111111111111:secret@tuic.example.com:443" +
		"?sni=tuic.example.com&alpn=h3#tuic-no-fp")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if _, ok := tls["utls"]; ok {
		t.Fatalf("utls unexpectedly set when fp/fingerprint omitted: %#v", tls["utls"])
	}
}
