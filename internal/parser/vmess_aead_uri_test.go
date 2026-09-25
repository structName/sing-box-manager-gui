package parser

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestVMessAEADURI_BasicTLSWS(t *testing.T) {
	raw := "vmess://44efe52b-e143-46b5-a9e7-aadbfd77eb9c@qv2ray.net:6939" +
		"?type=ws&security=tls&host=qv2ray.net&path=%2Fsomewhere&sni=qv2ray.net&fp=chrome#VMessWebSocketTLS"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Type != "vmess" {
		t.Fatalf("type = %q, want vmess", node.Type)
	}
	if node.Tag != "VMessWebSocketTLS" {
		t.Fatalf("tag = %q", node.Tag)
	}
	if node.Server != "qv2ray.net" || node.ServerPort != 6939 {
		t.Fatalf("server = %s:%d", node.Server, node.ServerPort)
	}
	if node.Extra["uuid"] != "44efe52b-e143-46b5-a9e7-aadbfd77eb9c" {
		t.Fatalf("uuid = %v", node.Extra["uuid"])
	}
	if node.Extra["alter_id"] != 0 {
		t.Fatalf("alter_id = %v, want 0", node.Extra["alter_id"])
	}
	if node.Extra["security"] != "auto" {
		t.Fatalf("security = %v, want auto (encryption omitted)", node.Extra["security"])
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	if transport["type"] != "ws" || transport["path"] != "/somewhere" {
		t.Fatalf("transport = %#v", transport)
	}
	headers, _ := transport["headers"].(map[string]string)
	if headers["Host"] != "qv2ray.net" {
		t.Fatalf("Host = %v", headers["Host"])
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if tls["enabled"] != true || tls["server_name"] != "qv2ray.net" {
		t.Fatalf("tls = %#v", tls)
	}
	utls, _ := tls["utls"].(map[string]interface{})
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("fp = %#v", utls)
	}
}

func TestVMessAEADURI_EncryptionAndNoQuery(t *testing.T) {
	raw := "vmess://5dc94f3a-ecf0-42d8-ae27-722a68a6456c@qv2ray.net:35897?encryption=aes-128-gcm#VMessTCPAES"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Extra["security"] != "aes-128-gcm" {
		t.Fatalf("security = %v", node.Extra["security"])
	}
	if _, hasTransport := node.Extra["transport"]; hasTransport {
		t.Fatalf("plain tcp should omit transport, got %#v", node.Extra["transport"])
	}
	if _, hasTLS := node.Extra["tls"]; hasTLS {
		t.Fatalf("security=none default should omit tls")
	}
}

func TestVMessAEADURI_Reality(t *testing.T) {
	raw := "vmess://11111111-1111-4111-8111-111111111111@edge.example.com:443" +
		"?security=reality&pbk=pubKeyHere&sid=abcd&sni=www.cloudflare.com&fp=chrome&type=tcp#reality"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	reality, _ := tls["reality"].(map[string]interface{})
	if reality["public_key"] != "pubKeyHere" || reality["short_id"] != "abcd" {
		t.Fatalf("reality = %#v", reality)
	}
	if tls["server_name"] != "www.cloudflare.com" {
		t.Fatalf("sni = %v", tls["server_name"])
	}
}

func TestVMessAEADURI_HTTPUpgradeAndInsecure(t *testing.T) {
	raw := "vmess://11111111-1111-4111-8111-111111111111@edge.example.com:443" +
		"?type=httpupgrade&path=%2Fup&host=cdn.example.com&security=tls&skip-cert-verify=1#hu"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	if transport["type"] != "httpupgrade" || transport["path"] != "/up" || transport["host"] != "cdn.example.com" {
		t.Fatalf("transport = %#v", transport)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if tls["insecure"] != true {
		t.Fatalf("insecure = %v", tls["insecure"])
	}
}

func TestVMessBase64JSONStillWorks(t *testing.T) {
	payload := map[string]interface{}{
		"v":    "2",
		"ps":   "json-node",
		"add":  "json.example.com",
		"port": 443,
		"id":   "22222222-2222-4222-8222-222222222222",
		"aid":  0,
		"scy":  "auto",
		"net":  "ws",
		"path": "/json",
		"host": "json.example.com",
		"tls":  "tls",
		"sni":  "json.example.com",
	}
	rawJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw := "vmess://" + base64.StdEncoding.EncodeToString(rawJSON) + "#frag"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Tag != "frag" {
		t.Fatalf("tag = %q (fragment should win)", node.Tag)
	}
	if node.Server != "json.example.com" {
		t.Fatalf("server = %q", node.Server)
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	if transport["type"] != "ws" || transport["path"] != "/json" {
		t.Fatalf("transport = %#v", transport)
	}
}

func TestVMessAEADURI_RejectsGarbage(t *testing.T) {
	_, err := ParseURL("vmess://not-a-valid-anything")
	if err == nil {
		t.Fatal("expected error for garbage vmess URL")
	}
}
