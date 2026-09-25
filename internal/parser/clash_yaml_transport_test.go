package parser

import "testing"

func TestClashYAMLInfersWSTransportWhenNetworkOmitted(t *testing.T) {
	yaml := `
proxies:
  - name: ws-omit-network
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    alterId: 0
    cipher: auto
    tls: true
    ws-opts:
      path: /ray
      headers:
        Host: cdn.example.com
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(nodes))
	}
	tr, ok := nodes[0].Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", nodes[0].Extra)
	}
	if tr["type"] != "ws" {
		t.Fatalf("transport.type = %v, want ws (inferred from ws-opts)", tr["type"])
	}
	if tr["path"] != "/ray" {
		t.Fatalf("transport.path = %v, want /ray", tr["path"])
	}
	headers, _ := tr["headers"].(map[string]string)
	if headers["Host"] != "cdn.example.com" {
		t.Fatalf("transport.headers.Host = %v, want cdn.example.com", headers["Host"])
	}
}

func TestClashYAMLInfersHTTPTransportWhenNetworkOmitted(t *testing.T) {
	yaml := `
proxies:
  - name: http-omit-network
    type: vmess
    server: example.com
    port: 80
    uuid: 11111111-1111-4111-8111-111111111111
    alterId: 0
    cipher: auto
    http-opts:
      method: GET
      path: ["/"]
      headers:
        Host: ["cdn.example.com"]
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(nodes))
	}
	tr, ok := nodes[0].Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", nodes[0].Extra)
	}
	if tr["type"] != "http" {
		t.Fatalf("transport.type = %v, want http (inferred from http-opts)", tr["type"])
	}
	if tr["path"] != "/" {
		t.Fatalf("transport.path = %v, want /", tr["path"])
	}
	if tr["method"] != "GET" {
		t.Fatalf("transport.method = %v, want GET", tr["method"])
	}
}

func TestClashYAMLInfersGrpcTransportWhenNetworkOmitted(t *testing.T) {
	yaml := `
proxies:
  - name: grpc-omit-network
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    tls: true
    grpc-opts:
      grpc-service-name: GunService
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("node count = %d, want 1", len(nodes))
	}
	tr, ok := nodes[0].Extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", nodes[0].Extra)
	}
	if tr["type"] != "grpc" {
		t.Fatalf("transport.type = %v, want grpc", tr["type"])
	}
	if tr["service_name"] != "GunService" {
		t.Fatalf("service_name = %v, want GunService", tr["service_name"])
	}
}

func TestClashYAMLKeepsExplicitNetworkWS(t *testing.T) {
	yaml := `
proxies:
  - name: ws-explicit
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    alterId: 0
    cipher: auto
    network: ws
    ws-opts:
      path: /explicit
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	tr := nodes[0].Extra["transport"].(map[string]interface{})
	if tr["type"] != "ws" || tr["path"] != "/explicit" {
		t.Fatalf("transport = %#v", tr)
	}
}

func TestClashYAMLPlainTCPHasNoTransport(t *testing.T) {
	yaml := `
proxies:
  - name: plain-tcp
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    alterId: 0
    cipher: auto
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML() error = %v", err)
	}
	if _, ok := nodes[0].Extra["transport"]; ok {
		t.Fatalf("plain tcp should not set transport: %#v", nodes[0].Extra["transport"])
	}
}
