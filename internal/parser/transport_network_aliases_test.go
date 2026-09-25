package parser

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func transportOf(t *testing.T, extra map[string]interface{}) map[string]interface{} {
	t.Helper()
	tr, ok := extra["transport"].(map[string]interface{})
	if !ok {
		t.Fatalf("transport missing: %#v", extra)
	}
	return tr
}

func TestNormalizeTransportNetworkAliases(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"websocket", "ws"},
		{"WebSocket", "ws"},
		{"http2", "h2"},
		{"HTTP/2", "h2"},
		{"http/2", "h2"},
		{"gun", "grpc"},
		{"GUN", "grpc"},
		{"ws", "ws"},
		{"grpc", "grpc"},
		{"h2", "h2"},
		{"tcp", "tcp"},
		{"  websocket  ", "ws"},
	}
	for _, tc := range cases {
		if got := normalizeTransportNetwork(tc.in); got != tc.want {
			t.Fatalf("normalizeTransportNetwork(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestVLESSURLWebsocketAliasCopiesPathHost(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=websocket&path=/ray&host=cdn.example.com&security=tls&sni=cdn.example.com#vless-ws"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tr := transportOf(t, node.Extra)
	if tr["type"] != "ws" {
		t.Fatalf("transport.type = %#v, want ws", tr["type"])
	}
	if tr["path"] != "/ray" {
		t.Fatalf("transport.path = %#v, want /ray", tr["path"])
	}
	headers, _ := tr["headers"].(map[string]string)
	if headers["Host"] != "cdn.example.com" {
		t.Fatalf("transport.headers.Host = %#v, want cdn.example.com", headers["Host"])
	}
}

func TestVLESSURLHTTP2AliasCopiesPathHost(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=http2&path=/h2&host=h2.example.com&security=tls&sni=h2.example.com#vless-h2"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tr := transportOf(t, node.Extra)
	if tr["type"] != "h2" {
		t.Fatalf("transport.type = %#v, want h2", tr["type"])
	}
	if tr["path"] != "/h2" {
		t.Fatalf("transport.path = %#v, want /h2", tr["path"])
	}
	host, ok := tr["host"].([]string)
	if !ok || len(host) != 1 || host[0] != "h2.example.com" {
		t.Fatalf("transport.host = %#v, want [h2.example.com]", tr["host"])
	}
}

func TestVLESSURLGunAliasCopiesServiceName(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=gun&serviceName=GunService&security=tls&sni=edge.example.com#vless-gun"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tr := transportOf(t, node.Extra)
	if tr["type"] != "grpc" {
		t.Fatalf("transport.type = %#v, want grpc", tr["type"])
	}
	if tr["service_name"] != "GunService" {
		t.Fatalf("transport.service_name = %#v, want GunService", tr["service_name"])
	}
}

func TestTrojanURLWebsocketAndGunAliases(t *testing.T) {
	cases := []struct {
		name, raw, wantType, wantPath, wantService string
	}{
		{
			name:     "websocket",
			raw:      "trojan://secret@edge.example.com:443?type=websocket&path=/ws&host=cdn.example.com&security=tls&sni=cdn.example.com#t-ws",
			wantType: "ws",
			wantPath: "/ws",
		},
		{
			name:        "gun",
			raw:         "trojan://secret@edge.example.com:443?type=gun&serviceName=TrojanGun&security=tls&sni=edge.example.com#t-gun",
			wantType:    "grpc",
			wantService: "TrojanGun",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := ParseURL(tc.raw)
			if err != nil {
				t.Fatalf("ParseURL: %v", err)
			}
			tr := transportOf(t, node.Extra)
			if tr["type"] != tc.wantType {
				t.Fatalf("transport.type = %#v, want %s", tr["type"], tc.wantType)
			}
			if tc.wantPath != "" && tr["path"] != tc.wantPath {
				t.Fatalf("transport.path = %#v, want %s", tr["path"], tc.wantPath)
			}
			if tc.wantService != "" && tr["service_name"] != tc.wantService {
				t.Fatalf("transport.service_name = %#v, want %s", tr["service_name"], tc.wantService)
			}
		})
	}
}

func TestVMessURLNetAliases(t *testing.T) {
	mk := func(net, path, host, ps string) string {
		cfg := map[string]interface{}{
			"v": "2", "ps": ps, "add": "edge.example.com", "port": "443",
			"id": "11111111-1111-1111-1111-111111111111", "aid": "0", "scy": "auto",
			"net": net, "type": "none", "host": host, "path": path, "tls": "tls", "sni": "cdn.example.com",
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return "vmess://" + base64.StdEncoding.EncodeToString(b)
	}

	t.Run("websocket", func(t *testing.T) {
		node, err := ParseURL(mk("websocket", "/ray", "cdn.example.com", "vmess-ws"))
		if err != nil {
			t.Fatalf("ParseURL: %v", err)
		}
		tr := transportOf(t, node.Extra)
		if tr["type"] != "ws" {
			t.Fatalf("transport.type = %#v, want ws", tr["type"])
		}
		if tr["path"] != "/ray" {
			t.Fatalf("transport.path = %#v, want /ray", tr["path"])
		}
		headers, _ := tr["headers"].(map[string]string)
		if headers["Host"] != "cdn.example.com" {
			t.Fatalf("headers.Host = %#v, want cdn.example.com", headers["Host"])
		}
	})
	t.Run("http2", func(t *testing.T) {
		node, err := ParseURL(mk("http2", "/h2", "h2.example.com", "vmess-h2"))
		if err != nil {
			t.Fatalf("ParseURL: %v", err)
		}
		tr := transportOf(t, node.Extra)
		if tr["type"] != "h2" {
			t.Fatalf("transport.type = %#v, want h2", tr["type"])
		}
		host, ok := tr["host"].([]string)
		if !ok || len(host) != 1 || host[0] != "h2.example.com" {
			t.Fatalf("transport.host = %#v, want [h2.example.com]", tr["host"])
		}
	})
	t.Run("gun", func(t *testing.T) {
		node, err := ParseURL(mk("gun", "GunService", "", "vmess-gun"))
		if err != nil {
			t.Fatalf("ParseURL: %v", err)
		}
		tr := transportOf(t, node.Extra)
		if tr["type"] != "grpc" {
			t.Fatalf("transport.type = %#v, want grpc", tr["type"])
		}
		if tr["service_name"] != "GunService" {
			t.Fatalf("service_name = %#v, want GunService", tr["service_name"])
		}
	})
}

func TestClashYAMLTransportNetworkAliases(t *testing.T) {
	cases := []struct {
		name, yaml, wantType, wantPath, wantService string
		wantHost                                    string
	}{
		{
			name: "websocket",
			yaml: `
proxies:
  - name: clash-ws-alias
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    alterId: 0
    cipher: auto
    tls: true
    network: websocket
    ws-opts:
      path: /ray
      headers:
        Host: cdn.example.com
`,
			wantType: "ws",
			wantPath: "/ray",
			wantHost: "cdn.example.com",
		},
		{
			name: "http2",
			yaml: `
proxies:
  - name: clash-h2-alias
    type: vmess
    server: example.com
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    alterId: 0
    cipher: auto
    tls: true
    network: http2
    h2-opts:
      path: /h2
      host:
        - h2.example.com
`,
			wantType: "h2",
			wantPath: "/h2",
		},
		{
			name: "gun",
			yaml: `
proxies:
  - name: clash-gun-alias
    type: trojan
    server: example.com
    port: 443
    password: secret
    network: gun
    grpc-opts:
      grpc-service-name: ClashGun
`,
			wantType:    "grpc",
			wantService: "ClashGun",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := ParseClashYAML(tc.yaml)
			if err != nil {
				t.Fatalf("ParseClashYAML: %v", err)
			}
			if len(nodes) != 1 {
				t.Fatalf("node count = %d, want 1", len(nodes))
			}
			tr := transportOf(t, nodes[0].Extra)
			if tr["type"] != tc.wantType {
				t.Fatalf("transport.type = %#v, want %s", tr["type"], tc.wantType)
			}
			if tc.wantPath != "" && tr["path"] != tc.wantPath {
				t.Fatalf("transport.path = %#v, want %s", tr["path"], tc.wantPath)
			}
			if tc.wantService != "" && tr["service_name"] != tc.wantService {
				t.Fatalf("service_name = %#v, want %s", tr["service_name"], tc.wantService)
			}
			if tc.wantHost != "" {
				headers, _ := tr["headers"].(map[string]string)
				if headers["Host"] != tc.wantHost {
					t.Fatalf("headers.Host = %#v, want %s", headers["Host"], tc.wantHost)
				}
			}
		})
	}
}

func TestCanonicalTransportTypesUnchanged(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=ws&path=/ok&host=cdn.example.com&security=tls&sni=cdn.example.com#canon"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tr := transportOf(t, node.Extra)
	if tr["type"] != "ws" || tr["path"] != "/ok" {
		t.Fatalf("canonical ws broken: %#v", tr)
	}
}
