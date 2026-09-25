package parser

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func vmessURL(t *testing.T, cfg map[string]interface{}) string {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(raw)
}

func TestVmessTLSStringTrueEnablesTLS(t *testing.T) {
	url := vmessURL(t, map[string]interface{}{
		"v": "2", "ps": "true-str", "add": "1.2.3.4", "port": 443,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0,
		"net": "tcp", "tls": "true", "sni": "cdn.example.com",
	})
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing: %#v", node.Extra)
	}
	if tls["enabled"] != true {
		t.Fatalf("enabled = %#v", tls["enabled"])
	}
	if tls["server_name"] != "cdn.example.com" {
		t.Fatalf("server_name = %#v", tls["server_name"])
	}
}

func TestVmessTLSBoolTrueEnablesTLS(t *testing.T) {
	// Boolean true previously failed JSON unmarshal into string tls field.
	url := vmessURL(t, map[string]interface{}{
		"v": "2", "ps": "true-bool", "add": "1.2.3.4", "port": 443,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0,
		"net": "tcp", "tls": true, "sni": "cdn.example.com",
	})
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing: %#v", node.Extra)
	}
	if tls["enabled"] != true || tls["server_name"] != "cdn.example.com" {
		t.Fatalf("tls = %#v", tls)
	}
}

func TestVmessTLSUppercaseEnablesTLS(t *testing.T) {
	url := vmessURL(t, map[string]interface{}{
		"v": "2", "ps": "TLS-upper", "add": "1.2.3.4", "port": 443,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0,
		"net": "ws", "path": "/", "host": "cdn.example.com",
		"tls": "TLS", "sni": "cdn.example.com",
	})
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing: %#v", node.Extra)
	}
	if tls["enabled"] != true {
		t.Fatalf("enabled = %#v", tls["enabled"])
	}
}

func TestVmessTLSCanonicalStillWorks(t *testing.T) {
	url := vmessURL(t, map[string]interface{}{
		"v": "2", "ps": "canonical", "add": "1.2.3.4", "port": 443,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0,
		"net": "tcp", "tls": "tls", "sni": "cdn.example.com", "fp": "chrome",
	})
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	utls := tls["utls"].(map[string]interface{})
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("fingerprint = %#v", utls["fingerprint"])
	}
}

func TestVmessTLSEmptyOmitsTLS(t *testing.T) {
	url := vmessURL(t, map[string]interface{}{
		"v": "2", "ps": "plain", "add": "1.2.3.4", "port": 80,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0,
		"net": "tcp", "tls": "",
	})
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if _, ok := node.Extra["tls"]; ok {
		t.Fatalf("tls should be omitted, got %#v", node.Extra["tls"])
	}
}

func TestVmessTLSFalseOmitsTLS(t *testing.T) {
	url := vmessURL(t, map[string]interface{}{
		"v": "2", "ps": "false-bool", "add": "1.2.3.4", "port": 80,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0,
		"net": "tcp", "tls": false,
	})
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if _, ok := node.Extra["tls"]; ok {
		t.Fatalf("tls should be omitted for false, got %#v", node.Extra["tls"])
	}
}

func TestVmessSkipCertVerifyStringTrue(t *testing.T) {
	url := vmessURL(t, map[string]interface{}{
		"v": "2", "ps": "skip-str", "add": "1.2.3.4", "port": 443,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0,
		"net": "tcp", "tls": "tls", "sni": "cdn.example.com",
		"skip-cert-verify": "true",
	})
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	if tls["insecure"] != true {
		t.Fatalf("insecure = %#v, want true", tls["insecure"])
	}
}

func TestVmessAllowInsecureAlias(t *testing.T) {
	url := vmessURL(t, map[string]interface{}{
		"v": "2", "ps": "allow", "add": "1.2.3.4", "port": 443,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0,
		"net": "tcp", "tls": "true", "sni": "cdn.example.com",
		"allowInsecure": 1,
	})
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	if tls["insecure"] != true {
		t.Fatalf("insecure = %#v, want true from allowInsecure", tls["insecure"])
	}
}

func TestVmessTLSRealityMapsPublicKey(t *testing.T) {
	url := vmessURL(t, map[string]interface{}{
		"v": "2", "ps": "reality", "add": "1.2.3.4", "port": 443,
		"id": "11111111-1111-1111-1111-111111111111", "aid": 0,
		"net": "tcp", "tls": "reality", "sni": "www.cloudflare.com",
		"pbk": "abcdefghijklmnopqrstuvwxyzABCDEFG", "sid": "abcd",
	})
	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls := node.Extra["tls"].(map[string]interface{})
	reality, ok := tls["reality"].(map[string]interface{})
	if !ok {
		t.Fatalf("reality missing: %#v", tls)
	}
	if reality["public_key"] != "abcdefghijklmnopqrstuvwxyzABCDEFG" {
		t.Fatalf("public_key = %#v", reality["public_key"])
	}
	if reality["short_id"] != "abcd" {
		t.Fatalf("short_id = %#v", reality["short_id"])
	}
	utls := tls["utls"].(map[string]interface{})
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("default fingerprint = %#v, want chrome", utls["fingerprint"])
	}
}
