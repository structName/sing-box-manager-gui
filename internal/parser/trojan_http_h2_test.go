package parser

import "testing"

func TestTrojanURLTypeHTTPPathHost(t *testing.T) {
	raw := "trojan://secret@edge.example.com:443" +
		"?type=http&host=www.example.com&path=/http&security=tls&sni=www.example.com#trojan-http"
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
	host, ok := transport["host"].([]string)
	if !ok || len(host) != 1 || host[0] != "www.example.com" {
		t.Fatalf("transport.host = %#v, want [www.example.com]", transport["host"])
	}
}

func TestTrojanURLTypeH2PathHost(t *testing.T) {
	raw := "trojan://secret@edge.example.com:443" +
		"?type=h2&host=cdn.example.com,www.example.com&path=/h2&security=tls&sni=cdn.example.com#trojan-h2"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["type"] != "h2" {
		t.Fatalf("transport.type = %#v, want h2", transport["type"])
	}
	if transport["path"] != "/h2" {
		t.Fatalf("transport.path = %#v, want /h2", transport["path"])
	}
	host, ok := transport["host"].([]string)
	if !ok || len(host) != 2 || host[0] != "cdn.example.com" || host[1] != "www.example.com" {
		t.Fatalf("transport.host = %#v, want [cdn.example.com www.example.com]", transport["host"])
	}
}

func TestTrojanURLTypeWSStillUsesHostHeader(t *testing.T) {
	// Regression: ws must keep Host header mapping, not transport.host.
	raw := "trojan://secret@edge.example.com:443" +
		"?type=ws&host=ws.example.com&path=/ws&security=tls#trojan-ws"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["path"] != "/ws" {
		t.Fatalf("transport.path = %#v, want /ws", transport["path"])
	}
	headers, ok := transport["headers"].(map[string]string)
	if !ok || headers["Host"] != "ws.example.com" {
		t.Fatalf("transport.headers = %#v, want Host=ws.example.com", transport["headers"])
	}
	if _, hasHost := transport["host"]; hasHost {
		t.Fatalf("ws transport must not set host field, got %#v", transport["host"])
	}
}
