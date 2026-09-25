package parser

import (
	"testing"
)

func TestVLESSURLHTTPUpgradeKeepsPathAndHost(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=httpupgrade&path=%2Fhu&host=cdn.example.com&security=tls&sni=cdn.example.com#hu-node"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	if transport == nil {
		t.Fatal("expected transport")
	}
	if transport["type"] != "httpupgrade" {
		t.Fatalf("type=%v", transport["type"])
	}
	if transport["path"] != "/hu" {
		t.Fatalf("path=%v want /hu", transport["path"])
	}
	if transport["host"] != "cdn.example.com" {
		t.Fatalf("host=%v want cdn.example.com", transport["host"])
	}
}

func TestTrojanURLHTTPUpgradeKeepsPathAndHost(t *testing.T) {
	raw := "trojan://secret@edge.example.com:443" +
		"?type=httpupgrade&path=%2Fhu&host=cdn.example.com&security=tls&sni=cdn.example.com#hu-trojan"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	if transport == nil {
		t.Fatal("expected transport")
	}
	if transport["type"] != "httpupgrade" {
		t.Fatalf("type=%v", transport["type"])
	}
	if transport["path"] != "/hu" {
		t.Fatalf("path=%v want /hu", transport["path"])
	}
	if transport["host"] != "cdn.example.com" {
		t.Fatalf("host=%v want cdn.example.com", transport["host"])
	}
}

func TestVMessURLHTTPUpgradeKeepsPathAndHost(t *testing.T) {
	// {"v":"2","ps":"hu","add":"edge.example.com","port":"443","id":"11111111-1111-1111-1111-111111111111","aid":"0","net":"httpupgrade","type":"none","host":"cdn.example.com","path":"/hu","tls":"tls","sni":"cdn.example.com"}
	raw := "vmess://eyJ2IjoiMiIsInBzIjoiaHUiLCJhZGQiOiJlZGdlLmV4YW1wbGUuY29tIiwicG9ydCI6IjQ0MyIsImlkIjoiMTExMTExMTEtMTExMS0xMTExLTExMTEtMTExMTExMTExMTExIiwiYWlkIjoiMCIsIm5ldCI6Imh0dHB1cGdyYWRlIiwidHlwZSI6Im5vbmUiLCJob3N0IjoiY2RuLmV4YW1wbGUuY29tIiwicGF0aCI6Ii9odSIsInRscyI6InRscyIsInNuaSI6ImNkbi5leGFtcGxlLmNvbSJ9"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	if transport == nil {
		t.Fatal("expected transport")
	}
	if transport["type"] != "httpupgrade" {
		t.Fatalf("type=%v", transport["type"])
	}
	if transport["path"] != "/hu" {
		t.Fatalf("path=%v want /hu", transport["path"])
	}
	if transport["host"] != "cdn.example.com" {
		t.Fatalf("host=%v want cdn.example.com", transport["host"])
	}
}

func TestClashYAMLHTTPUpgradeFromNetworkAndWSOpts(t *testing.T) {
	yaml := `
proxies:
  - name: clash-hu
    type: vless
    server: edge.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: httpupgrade
    tls: true
    servername: cdn.example.com
    ws-opts:
      path: /hu
      headers:
        Host: cdn.example.com
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len=%d", len(nodes))
	}
	transport, _ := nodes[0].Extra["transport"].(map[string]interface{})
	if transport == nil {
		t.Fatal("expected transport")
	}
	if transport["type"] != "httpupgrade" {
		t.Fatalf("type=%v", transport["type"])
	}
	if transport["path"] != "/hu" {
		t.Fatalf("path=%v want /hu", transport["path"])
	}
	headers, _ := transport["headers"].(map[string]string)
	if headers == nil || headers["Host"] != "cdn.example.com" {
		// also accept map[string]interface{}
		if hi, ok := transport["headers"].(map[string]interface{}); ok {
			if hi["Host"] != "cdn.example.com" {
				t.Fatalf("headers=%v", transport["headers"])
			}
		} else if headers["Host"] != "cdn.example.com" {
			t.Fatalf("headers=%v", transport["headers"])
		}
	}
}

func TestClashYAMLV2rayHTTPUpgradeFlagPromotesWS(t *testing.T) {
	yaml := `
proxies:
  - name: clash-ws-hu
    type: vmess
    server: edge.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    network: ws
    tls: true
    servername: cdn.example.com
    ws-opts:
      path: /hu
      v2ray-http-upgrade: true
      headers:
        Host: cdn.example.com
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len=%d", len(nodes))
	}
	transport, _ := nodes[0].Extra["transport"].(map[string]interface{})
	if transport == nil {
		t.Fatal("expected transport")
	}
	if transport["type"] != "httpupgrade" {
		t.Fatalf("type=%v want httpupgrade (v2ray-http-upgrade flag)", transport["type"])
	}
	if transport["path"] != "/hu" {
		t.Fatalf("path=%v", transport["path"])
	}
}
