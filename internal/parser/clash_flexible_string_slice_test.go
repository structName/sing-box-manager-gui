package parser

import "testing"

func TestClashALPNAcceptsStringAndList(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want []string
	}{
		{
			name: "scalar",
			yaml: `
proxies:
  - name: hy2-alpn-str
    type: hysteria2
    server: hy2.example.com
    port: 443
    password: secret
    alpn: h3
`,
			want: []string{"h3"},
		},
		{
			name: "comma-joined",
			yaml: `
proxies:
  - name: hy2-alpn-csv
    type: hysteria2
    server: hy2.example.com
    port: 443
    password: secret
    alpn: "h3, h2"
`,
			want: []string{"h3", "h2"},
		},
		{
			name: "list",
			yaml: `
proxies:
  - name: hy2-alpn-list
    type: hysteria2
    server: hy2.example.com
    port: 443
    password: secret
    alpn:
      - h3
      - h2
`,
			want: []string{"h3", "h2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nodes, err := ParseClashYAML(tc.yaml)
			if err != nil {
				t.Fatalf("ParseClashYAML: %v", err)
			}
			if len(nodes) != 1 {
				t.Fatalf("len(nodes)=%d, want 1 (scalar alpn must not drop proxy)", len(nodes))
			}
			tls, _ := nodes[0].Extra["tls"].(map[string]interface{})
			if tls == nil {
				t.Fatal("expected tls")
			}
			alpn, ok := tls["alpn"].([]string)
			if !ok {
				t.Fatalf("alpn type=%T value=%v", tls["alpn"], tls["alpn"])
			}
			if len(alpn) != len(tc.want) {
				t.Fatalf("alpn=%v want %v", alpn, tc.want)
			}
			for i := range tc.want {
				if alpn[i] != tc.want[i] {
					t.Fatalf("alpn=%v want %v", alpn, tc.want)
				}
			}
		})
	}
}

func TestClashH2HostAcceptsString(t *testing.T) {
	yaml := `
proxies:
  - name: vless-h2
    type: vless
    server: edge.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    network: h2
    tls: true
    h2-opts:
      path: /
      host: cdn.example.com
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes)=%d, want 1 (scalar h2 host must not drop proxy)", len(nodes))
	}
	transport, _ := nodes[0].Extra["transport"].(map[string]interface{})
	if transport == nil {
		t.Fatal("expected transport")
	}
	host, ok := transport["host"].([]string)
	if !ok || len(host) != 1 || host[0] != "cdn.example.com" {
		t.Fatalf("host=%v (%T), want [cdn.example.com]", transport["host"], transport["host"])
	}
}

func TestClashHTTPOptsPathAndHeadersAcceptStrings(t *testing.T) {
	yaml := `
proxies:
  - name: vmess-http
    type: vmess
    server: edge.example.com
    port: 80
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    network: http
    http-opts:
      method: GET
      path: /api
      headers:
        Host: cdn.example.com
        Connection: keep-alive
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes)=%d, want 1 (scalar http path/headers must not drop proxy)", len(nodes))
	}
	transport, _ := nodes[0].Extra["transport"].(map[string]interface{})
	if transport == nil {
		t.Fatal("expected transport")
	}
	if transport["path"] != "/api" {
		t.Fatalf("path=%v want /api", transport["path"])
	}
	if transport["method"] != "GET" {
		t.Fatalf("method=%v want GET", transport["method"])
	}
	headers, ok := transport["headers"].(map[string][]string)
	if !ok {
		t.Fatalf("headers type=%T value=%v", transport["headers"], transport["headers"])
	}
	if got := headers["Host"]; len(got) != 1 || got[0] != "cdn.example.com" {
		t.Fatalf("Host=%v", got)
	}
	if got := headers["Connection"]; len(got) != 1 || got[0] != "keep-alive" {
		t.Fatalf("Connection=%v", got)
	}
}

func TestClashHTTPOptsListFormsStillWork(t *testing.T) {
	yaml := `
proxies:
  - name: vmess-http-list
    type: vmess
    server: edge.example.com
    port: 80
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    network: http
    http-opts:
      path:
        - /api
      headers:
        Host:
          - cdn.example.com
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes)=%d, want 1", len(nodes))
	}
	transport, _ := nodes[0].Extra["transport"].(map[string]interface{})
	if transport["path"] != "/api" {
		t.Fatalf("path=%v", transport["path"])
	}
	headers, _ := transport["headers"].(map[string][]string)
	if got := headers["Host"]; len(got) != 1 || got[0] != "cdn.example.com" {
		t.Fatalf("Host=%v", got)
	}
}
