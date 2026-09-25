package parser

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestVLESSURLWSEarlyDataQueryParams(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=ws&path=%2Fws&host=cdn.example.com&ed=2048&eh=Sec-WebSocket-Protocol" +
		"&security=tls&sni=cdn.example.com#vless-ed"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["type"] != "ws" {
		t.Fatalf("transport.type = %#v, want ws", transport["type"])
	}
	if transport["path"] != "/ws" {
		t.Fatalf("transport.path = %#v, want /ws", transport["path"])
	}
	if transport["max_early_data"] != 2048 {
		t.Fatalf("max_early_data = %#v, want 2048", transport["max_early_data"])
	}
	if transport["early_data_header_name"] != "Sec-WebSocket-Protocol" {
		t.Fatalf("early_data_header_name = %#v", transport["early_data_header_name"])
	}
}

func TestVLESSURLWSEarlyDataEmbeddedInPath(t *testing.T) {
	// Common v2rayN convention: path carries ?ed=N
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=ws&path=%2Fray%3Fed%3D2560&host=cdn.example.com" +
		"&security=tls&sni=cdn.example.com#path-ed"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["path"] != "/ray" {
		t.Fatalf("transport.path = %#v, want /ray (ed stripped)", transport["path"])
	}
	if transport["max_early_data"] != 2560 {
		t.Fatalf("max_early_data = %#v, want 2560", transport["max_early_data"])
	}
}

func TestTrojanURLWSEarlyDataQueryParams(t *testing.T) {
	raw := "trojan://secret@edge.example.com:443" +
		"?type=ws&path=%2Ft&host=cdn.example.com&ed=1024&eh=Sec-WebSocket-Protocol" +
		"&security=tls&sni=cdn.example.com#trojan-ed"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["max_early_data"] != 1024 {
		t.Fatalf("max_early_data = %#v, want 1024", transport["max_early_data"])
	}
	if transport["early_data_header_name"] != "Sec-WebSocket-Protocol" {
		t.Fatalf("early_data_header_name = %#v", transport["early_data_header_name"])
	}
}

func TestVMessURLWSEarlyDataInPath(t *testing.T) {
	cfg := map[string]interface{}{
		"v":    "2",
		"ps":   "vmess-ed",
		"add":  "edge.example.com",
		"port": 443,
		"id":   "11111111-1111-1111-1111-111111111111",
		"aid":  0,
		"scy":  "auto",
		"net":  "ws",
		"type": "none",
		"host": "cdn.example.com",
		"path": "/vmess?ed=2048",
		"tls":  "tls",
		"sni":  "cdn.example.com",
	}
	rawJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	raw := "vmess://" + base64.StdEncoding.EncodeToString(rawJSON)
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if transport["path"] != "/vmess" {
		t.Fatalf("transport.path = %#v, want /vmess", transport["path"])
	}
	if transport["max_early_data"] != 2048 {
		t.Fatalf("max_early_data = %#v, want 2048", transport["max_early_data"])
	}
}

func TestVLESSURLWSWithoutEarlyDataUnchanged(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=ws&path=%2Fws&host=cdn.example.com&security=tls&sni=cdn.example.com#plain-ws"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", node.Extra)
	}
	if _, ok := transport["max_early_data"]; ok {
		t.Fatalf("plain ws must not set max_early_data, got %#v", transport["max_early_data"])
	}
	if transport["path"] != "/ws" {
		t.Fatalf("path = %#v, want /ws", transport["path"])
	}
}
