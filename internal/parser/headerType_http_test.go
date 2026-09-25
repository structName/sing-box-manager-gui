package parser

import (
	"testing"
)

func TestVLESSURLTCPHeaderTypeHTTP(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=tcp&headerType=http&host=www.example.com&path=/&security=tls&sni=www.example.com#vless-http"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Type != "vless" {
		t.Fatalf("type = %q, want vless", node.Type)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["type"] != "http" {
		t.Fatalf("transport.type = %#v, want http", transport["type"])
	}
	if transport["path"] != "/" {
		t.Fatalf("transport.path = %#v, want /", transport["path"])
	}
	host, ok := transport["host"].([]string)
	if !ok || len(host) != 1 || host[0] != "www.example.com" {
		t.Fatalf("transport.host = %#v, want [www.example.com]", transport["host"])
	}
}

func TestVLESSURLHeaderTypeUnderscoreAlias(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=tcp&header_type=http&host=cdn.example.com&security=none#alias"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["type"] != "http" {
		t.Fatalf("transport.type = %#v, want http", transport["type"])
	}
}

func TestTrojanURLTCPHeaderTypeHTTP(t *testing.T) {
	raw := "trojan://secret@edge.example.com:443" +
		"?type=tcp&headerType=http&host=www.example.com&path=/http&security=tls&sni=www.example.com#trojan-http"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Type != "trojan" {
		t.Fatalf("type = %q, want trojan", node.Type)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["type"] != "http" {
		t.Fatalf("transport.type = %#v, want http", transport["type"])
	}
	if transport["path"] != "/http" {
		t.Fatalf("transport.path = %#v, want /http", transport["path"])
	}
}

func TestVLESSURLPlainTCPHasNoTransport(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=tcp&security=tls&sni=edge.example.com#plain"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if _, ok := node.Extra["transport"]; ok {
		t.Fatalf("plain tcp should omit transport, got %#v", node.Extra["transport"])
	}
}

func TestVLESSURLTypeHTTPStillWorks(t *testing.T) {
	// type=http (transport) must remain distinct from type=tcp&headerType=http
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=http&host=www.example.com&path=/h&security=tls&sni=www.example.com#http-transport"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["type"] != "http" {
		t.Fatalf("transport.type = %#v, want http", transport["type"])
	}
	if node.Tag != "http-transport" {
		t.Fatalf("tag = %q, want http-transport", node.Tag)
	}
}
