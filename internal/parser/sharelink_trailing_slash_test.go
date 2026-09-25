package parser

import (
	"testing"
)

// Share-link generators (tuic.sh, 3x-ui, etc.) commonly emit:
//   scheme://userinfo@host:port/?key=val#name
// parseURLParams keeps the trailing "/" in addressPart; without stripping it,
// parseServerInfo treats "443/" as the port and import fails for TUIC/HY2/
// Trojan/VLESS/AnyTLS alike.
func TestShareLinkTrailingSlashBeforeQuery(t *testing.T) {
	cases := []struct {
		name       string
		rawURL     string
		wantServer string
		wantPort   int
	}{
		{
			name:       "tuic",
			rawURL:     "tuic://11111111-1111-1111-1111-111111111111:pass@1.2.3.4:443/?congestion_control=bbr&alpn=h3&sni=www.example.com#n",
			wantServer: "1.2.3.4",
			wantPort:   443,
		},
		{
			name:       "hy2",
			rawURL:     "hysteria2://secret@hy.example.com:443/?sni=www.example.com&insecure=1#n",
			wantServer: "hy.example.com",
			wantPort:   443,
		},
		{
			name:       "trojan",
			rawURL:     "trojan://pass@t.example.com:443/?sni=www.example.com&allowInsecure=1#n",
			wantServer: "t.example.com",
			wantPort:   443,
		},
		{
			name:       "vless",
			rawURL:     "vless://11111111-1111-1111-1111-111111111111@v.example.com:443/?encryption=none&security=tls&sni=www.example.com#n",
			wantServer: "v.example.com",
			wantPort:   443,
		},
		{
			name:       "anytls",
			rawURL:     "anytls://pass@a.example.com:443/?sni=www.example.com#n",
			wantServer: "a.example.com",
			wantPort:   443,
		},
		{
			name:       "tuic-ipv6",
			rawURL:     "tuic://11111111-1111-1111-1111-111111111111:pass@[2001:db8::1]:8443/?alpn=h3#n",
			wantServer: "2001:db8::1",
			wantPort:   8443,
		},
		{
			name:       "no-slash-still-ok",
			rawURL:     "tuic://11111111-1111-1111-1111-111111111111:pass@1.2.3.4:8443?alpn=h3#n",
			wantServer: "1.2.3.4",
			wantPort:   8443,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := ParseURL(tc.rawURL)
			if err != nil {
				t.Fatalf("ParseURL: %v", err)
			}
			if node.Server != tc.wantServer {
				t.Fatalf("Server = %q, want %q", node.Server, tc.wantServer)
			}
			if node.ServerPort != tc.wantPort {
				t.Fatalf("ServerPort = %d, want %d", node.ServerPort, tc.wantPort)
			}
		})
	}
}
