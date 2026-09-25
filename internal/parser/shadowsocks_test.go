package parser

import (
	"testing"
)

func TestShadowsocksParser_SIP002PluginQuery(t *testing.T) {
	cases := []struct {
		name       string
		url        string
		wantTag    string
		wantServer string
		wantPort   int
		wantPlugin string
		wantOpts   string
	}{
		{
			name:       "slash-before-query obfs-local",
			url:        "ss://YWVzLTEyOC1nY206dGVzdA@192.168.100.1:8888/?plugin=obfs-local%3Bobfs%3Dhttp#Example",
			wantTag:    "Example",
			wantServer: "192.168.100.1",
			wantPort:   8888,
			wantPlugin: "obfs-local",
			wantOpts:   "obfs=http",
		},
		{
			name:       "no-slash-before-query with host",
			url:        "ss://YWVzLTEyOC1nY206dGVzdA@192.168.100.1:8888?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dcdn.example.com#Example",
			wantTag:    "Example",
			wantServer: "192.168.100.1",
			wantPort:   8888,
			wantPlugin: "obfs-local",
			wantOpts:   "obfs=http;obfs-host=cdn.example.com",
		},
		{
			name:       "v2ray-plugin",
			url:        "ss://YWVzLTEyOC1nY206dGVzdA@example.com:8388/?plugin=v2ray-plugin%3Bmode%3Dwebsocket%3Bhost%3Dexample.com#vp",
			wantTag:    "vp",
			wantServer: "example.com",
			wantPort:   8388,
			wantPlugin: "v2ray-plugin",
			wantOpts:   "mode=websocket;host=example.com",
		},
		{
			name:       "plugin name only",
			url:        "ss://YWVzLTEyOC1nY206dGVzdA@192.168.100.1:8888?plugin=obfs-local#p",
			wantTag:    "p",
			wantServer: "192.168.100.1",
			wantPort:   8888,
			wantPlugin: "obfs-local",
			wantOpts:   "",
		},
		{
			name:       "no plugin still works",
			url:        "ss://YWVzLTEyOC1nY206dGVzdA@192.168.100.1:8888#nplugin",
			wantTag:    "nplugin",
			wantServer: "192.168.100.1",
			wantPort:   8888,
			wantPlugin: "",
			wantOpts:   "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := ParseURL(tc.url)
			if err != nil {
				t.Fatalf("ParseURL error: %v", err)
			}
			if node.Tag != tc.wantTag {
				t.Fatalf("tag = %q, want %q", node.Tag, tc.wantTag)
			}
			if node.Type != "shadowsocks" {
				t.Fatalf("type = %q, want shadowsocks", node.Type)
			}
			if node.Server != tc.wantServer || node.ServerPort != tc.wantPort {
				t.Fatalf("server = %s:%d, want %s:%d", node.Server, node.ServerPort, tc.wantServer, tc.wantPort)
			}
			gotPlugin, _ := node.Extra["plugin"].(string)
			if gotPlugin != tc.wantPlugin {
				t.Fatalf("plugin = %q, want %q", gotPlugin, tc.wantPlugin)
			}
			gotOpts, _ := node.Extra["plugin_opts"].(string)
			if gotOpts != tc.wantOpts {
				t.Fatalf("plugin_opts = %q, want %q", gotOpts, tc.wantOpts)
			}
			if tc.wantPlugin == "" {
				if _, ok := node.Extra["plugin"]; ok {
					t.Fatalf("plugin key should be absent: %#v", node.Extra)
				}
				if _, ok := node.Extra["plugin_opts"]; ok {
					t.Fatalf("plugin_opts key should be absent: %#v", node.Extra)
				}
			}
			if tc.wantOpts == "" && tc.wantPlugin != "" {
				if _, ok := node.Extra["plugin_opts"]; ok {
					t.Fatalf("plugin_opts should be absent when empty: %#v", node.Extra)
				}
			}
			method, _ := node.Extra["method"].(string)
			password, _ := node.Extra["password"].(string)
			if method != "aes-128-gcm" || password != "test" {
				t.Fatalf("method/password = %q/%q, want aes-128-gcm/test", method, password)
			}
		})
	}
}

func TestShadowsocksParser_LegacyStillWorks(t *testing.T) {
	// ss://BASE64(aes-128-gcm:test@192.168.100.1:8888)#legacy
	node, err := ParseURL("ss://YWVzLTEyOC1nY206dGVzdEAxOTIuMTY4LjEwMC4xOjg4ODg#legacy")
	if err != nil {
		t.Fatalf("ParseURL error: %v", err)
	}
	if node.Server != "192.168.100.1" || node.ServerPort != 8888 {
		t.Fatalf("server = %s:%d", node.Server, node.ServerPort)
	}
	if node.Tag != "legacy" {
		t.Fatalf("tag = %q", node.Tag)
	}
}
