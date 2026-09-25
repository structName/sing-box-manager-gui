package parser

import "testing"

func TestTrojanURLAcceptsPasswordQueryParam(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "password query without userinfo",
			url:  "trojan://trojan.example.com:443?password=secret&sni=trojan.example.com#pwd",
			want: "secret",
		},
		{
			name: "empty userinfo falls back to password query",
			url:  "trojan://@trojan.example.com:443?password=empty-userinfo&sni=trojan.example.com#eu",
			want: "empty-userinfo",
		},
		{
			name: "userinfo still preferred over query",
			url:  "trojan://userinfo-secret@trojan.example.com:443?password=query-secret#ui",
			want: "userinfo-secret",
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
			if node.Type != "trojan" {
				t.Fatalf("type = %q, want trojan", node.Type)
			}
			if node.Server != "trojan.example.com" || node.ServerPort != 443 {
				t.Fatalf("server = %s:%d, want trojan.example.com:443", node.Server, node.ServerPort)
			}
			tls, _ := node.Extra["tls"].(map[string]interface{})
			if tls == nil || tls["enabled"] != true {
				t.Fatalf("expected TLS enabled, got %#v", node.Extra["tls"])
			}
		})
	}
}

func TestTrojanURLStillRequiresPassword(t *testing.T) {
	_, err := ParseURL("trojan://trojan.example.com:443?sni=trojan.example.com#nopw")
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if err.Error() != "缺少认证密码" {
		t.Fatalf("error = %v, want 缺少认证密码", err)
	}
}

func TestAnyTLSURLAcceptsPasswordQueryParam(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "password query without userinfo",
			url:  "anytls://any.example.com:443?password=secret&sni=any.example.com#pwd",
			want: "secret",
		},
		{
			name: "empty userinfo falls back to password query",
			url:  "anytls://@any.example.com:443?password=empty-userinfo&sni=any.example.com#eu",
			want: "empty-userinfo",
		},
		{
			name: "userinfo still preferred over query",
			url:  "anytls://userinfo-secret@any.example.com:443?password=query-secret#ui",
			want: "userinfo-secret",
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
			if node.Type != "anytls" {
				t.Fatalf("type = %q, want anytls", node.Type)
			}
			if node.Server != "any.example.com" || node.ServerPort != 443 {
				t.Fatalf("server = %s:%d, want any.example.com:443", node.Server, node.ServerPort)
			}
		})
	}
}

func TestAnyTLSURLStillRequiresPassword(t *testing.T) {
	_, err := ParseURL("anytls://any.example.com:443?sni=any.example.com#nopw")
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if err.Error() != "缺少认证密码" {
		t.Fatalf("error = %v, want 缺少认证密码", err)
	}
}
