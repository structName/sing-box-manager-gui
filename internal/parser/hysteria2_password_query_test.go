package parser

import "testing"

func TestHysteria2URLAcceptsPasswordQueryParam(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "password query (Clash Meta / NekoBox style)",
			url:  "hysteria2://hy2.example.com:443?password=secret&sni=hy2.example.com#pwd",
			want: "secret",
		},
		{
			name: "auth query still works",
			url:  "hysteria2://hy2.example.com:443?auth=from-auth&sni=hy2.example.com#auth",
			want: "from-auth",
		},
		{
			name: "auth preferred when both present",
			url:  "hysteria2://hy2.example.com:443?auth=from-auth&password=from-password#both",
			want: "from-auth",
		},
		{
			name: "hy2 scheme + password query",
			url:  "hy2://hy2.example.com:443?password=hy2-secret#hy2",
			want: "hy2-secret",
		},
		{
			name: "userinfo still preferred over query",
			url:  "hysteria2://userinfo-secret@hy2.example.com:443?password=query-secret#ui",
			want: "userinfo-secret",
		},
		{
			name: "empty userinfo falls back to password query",
			url:  "hysteria2://@hy2.example.com:443?password=empty-userinfo#eu",
			want: "empty-userinfo",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := ParseURL(tc.url)
			if err != nil {
				t.Fatalf("ParseURL error: %v", err)
			}
			got, _ := node.Extra["password"].(string)
			if got != tc.want {
				t.Fatalf("password = %q, want %q", got, tc.want)
			}
			if node.Type != "hysteria2" {
				t.Fatalf("type = %q, want hysteria2", node.Type)
			}
			if node.Server != "hy2.example.com" || node.ServerPort != 443 {
				t.Fatalf("server = %s:%d, want hy2.example.com:443", node.Server, node.ServerPort)
			}
		})
	}
}

func TestHysteria2URLStillRequiresPassword(t *testing.T) {
	_, err := ParseURL("hysteria2://hy2.example.com:443?sni=hy2.example.com#nopw")
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if err.Error() != "缺少认证密码" {
		t.Fatalf("error = %v, want 缺少认证密码", err)
	}
}
