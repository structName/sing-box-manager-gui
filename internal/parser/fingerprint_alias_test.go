package parser

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestVLESSFingerprintQueryAlias(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=tcp&security=tls&sni=cdn.example.com&fingerprint=firefox#fp-alias"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if tls == nil {
		t.Fatal("tls missing")
	}
	utls, _ := tls["utls"].(map[string]interface{})
	if utls == nil {
		t.Fatal("utls missing — fingerprint= should enable uTLS on plain TLS")
	}
	if got, _ := utls["fingerprint"].(string); got != "firefox" {
		t.Fatalf("fingerprint = %q, want firefox from fingerprint=", got)
	}
}

func TestVLESSRealityFingerprintAliasOverridesDefaultChrome(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=tcp&security=reality&pbk=abc&sid=01&fingerprint=safari&sni=cdn.example.com#reality-fp"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	utls, _ := tls["utls"].(map[string]interface{})
	if got, _ := utls["fingerprint"].(string); got != "safari" {
		t.Fatalf("fingerprint = %q, want safari (not default chrome)", got)
	}
}

func TestVLESSFpPrefersOverFingerprintAlias(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?security=tls&sni=cdn.example.com&fp=chrome&fingerprint=firefox#pref"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	utls, _ := tls["utls"].(map[string]interface{})
	if got, _ := utls["fingerprint"].(string); got != "chrome" {
		t.Fatalf("fingerprint = %q, want chrome from fp= preference", got)
	}
}

func TestTrojanFingerprintQueryAlias(t *testing.T) {
	raw := "trojan://secret@edge.example.com:443?sni=cdn.example.com&fingerprint=ios#t"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	utls, _ := tls["utls"].(map[string]interface{})
	if got, _ := utls["fingerprint"].(string); got != "ios" {
		t.Fatalf("fingerprint = %q, want ios", got)
	}
}

func TestAnyTLSFingerprintQueryAlias(t *testing.T) {
	raw := "anytls://secret@edge.example.com:443?sni=cdn.example.com&fingerprint=edge#a"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	utls, _ := tls["utls"].(map[string]interface{})
	if got, _ := utls["fingerprint"].(string); got != "edge" {
		t.Fatalf("fingerprint = %q, want edge", got)
	}
}

func TestVMessJSONFingerprintAlias(t *testing.T) {
	cfg := map[string]interface{}{
		"v":           "2",
		"ps":          "vmess-fp",
		"add":         "edge.example.com",
		"port":        "443",
		"id":          "11111111-1111-1111-1111-111111111111",
		"aid":         "0",
		"net":         "tcp",
		"tls":         "tls",
		"sni":         "cdn.example.com",
		"fingerprint": "android",
	}
	rawJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw := "vmess://" + base64.StdEncoding.EncodeToString(rawJSON)
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	utls, _ := tls["utls"].(map[string]interface{})
	if got, _ := utls["fingerprint"].(string); got != "android" {
		t.Fatalf("fingerprint = %q, want android from JSON fingerprint", got)
	}
}
