package parser

import "testing"

func TestClashWSHostUsedAsSNIWhenOmitted(t *testing.T) {
	yaml := `
proxies:
  - name: cdn-ws
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    tls: true
    network: ws
    ws-opts:
      path: /ray
      headers:
        Host: www.cdn.example.com
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes=%d", len(nodes))
	}
	tls, _ := nodes[0].Extra["tls"].(map[string]interface{})
	if tls == nil {
		t.Fatal("tls missing")
	}
	if tls["server_name"] != "www.cdn.example.com" {
		t.Fatalf("server_name=%v, want www.cdn.example.com (from ws Host)", tls["server_name"])
	}
}

func TestClashWSHostLowercaseHeaderUsedAsSNI(t *testing.T) {
	yaml := `
proxies:
  - name: cdn-ws-lc
    type: trojan
    server: 5.6.7.8
    port: 443
    password: secret
    tls: true
    network: ws
    ws-opts:
      path: /
      headers:
        host: edge.example.com
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes=%d", len(nodes))
	}
	tls, _ := nodes[0].Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "edge.example.com" {
		t.Fatalf("server_name=%v, want edge.example.com", tls["server_name"])
	}
}

func TestClashH2HostUsedAsSNIWhenOmitted(t *testing.T) {
	yaml := `
proxies:
  - name: cdn-h2
    type: vless
    server: 9.9.9.9
    port: 443
    uuid: 22222222-2222-2222-2222-222222222222
    tls: true
    network: h2
    h2-opts:
      path: /
      host:
        - h2.cdn.example.com
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes=%d", len(nodes))
	}
	tls, _ := nodes[0].Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "h2.cdn.example.com" {
		t.Fatalf("server_name=%v, want h2.cdn.example.com (from h2 host)", tls["server_name"])
	}
}

func TestClashExplicitSNIPrefersOverWSHost(t *testing.T) {
	yaml := `
proxies:
  - name: explicit-sni
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    tls: true
    sni: real-sni.example.com
    network: ws
    ws-opts:
      path: /ray
      headers:
        Host: www.cdn.example.com
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatal(err)
	}
	tls, _ := nodes[0].Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "real-sni.example.com" {
		t.Fatalf("server_name=%v, want explicit sni", tls["server_name"])
	}
}

func TestClashNoTransportHostFallsBackToServer(t *testing.T) {
	yaml := `
proxies:
  - name: plain-tls
    type: trojan
    server: origin.example.com
    port: 443
    password: secret
    tls: true
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatal(err)
	}
	tls, _ := nodes[0].Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "origin.example.com" {
		t.Fatalf("server_name=%v, want server host fallback", tls["server_name"])
	}
}
