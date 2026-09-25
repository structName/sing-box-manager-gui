package parser

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func encodeVmessURL(cfg map[string]interface{}) string {
	raw, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(raw)
}

func TestVmessParserTCPHTTPHeaderObfuscation(t *testing.T) {
	url := encodeVmessURL(map[string]interface{}{
		"v":    "2",
		"ps":   "http-obfs",
		"add":  "1.2.3.4",
		"port": 443,
		"id":   "11111111-1111-1111-1111-111111111111",
		"aid":  0,
		"scy":  "auto",
		"net":  "tcp",
		"type": "http",
		"host": "www.example.com,cdn.example.com",
		"path": "/",
		"tls":  "",
	})

	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if node.Type != "vmess" {
		t.Fatalf("type = %q, want vmess", node.Type)
	}

	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing transport: %#v", node.Extra)
	}
	if got, _ := transport["type"].(string); got != "http" {
		t.Fatalf("transport.type = %v, want http", transport["type"])
	}
	if got, _ := transport["path"].(string); got != "/" {
		t.Fatalf("transport.path = %v, want /", transport["path"])
	}
	hosts, ok := transport["host"].([]string)
	if !ok {
		t.Fatalf("transport.host type = %T, want []string", transport["host"])
	}
	if len(hosts) != 2 || hosts[0] != "www.example.com" || hosts[1] != "cdn.example.com" {
		t.Fatalf("transport.host = %#v, want [www.example.com cdn.example.com]", hosts)
	}
}

func TestVmessParserPlainTCPHasNoTransport(t *testing.T) {
	url := encodeVmessURL(map[string]interface{}{
		"v":    "2",
		"ps":   "plain-tcp",
		"add":  "1.2.3.4",
		"port": 10086,
		"id":   "11111111-1111-1111-1111-111111111111",
		"aid":  0,
		"scy":  "auto",
		"net":  "tcp",
		"type": "none",
		"tls":  "",
	})

	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if _, ok := node.Extra["transport"]; ok {
		t.Fatalf("plain tcp should not set transport, got %#v", node.Extra["transport"])
	}
}

func TestVmessParserWSStillUsesHeadersHost(t *testing.T) {
	url := encodeVmessURL(map[string]interface{}{
		"v":    "2",
		"ps":   "ws-node",
		"add":  "1.2.3.4",
		"port": 443,
		"id":   "11111111-1111-1111-1111-111111111111",
		"aid":  0,
		"scy":  "auto",
		"net":  "ws",
		"type": "",
		"host": "ws.example.com",
		"path": "/ray",
		"tls":  "tls",
		"sni":  "ws.example.com",
	})

	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	transport, ok := node.Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing transport: %#v", node.Extra)
	}
	if got, _ := transport["type"].(string); got != "ws" {
		t.Fatalf("transport.type = %v, want ws", transport["type"])
	}
	headers, ok := transport["headers"].(map[string]string)
	if !ok || headers["Host"] != "ws.example.com" {
		t.Fatalf("transport.headers = %#v, want Host=ws.example.com", transport["headers"])
	}
}
