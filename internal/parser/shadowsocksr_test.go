package parser

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

// ssrBase64Encode encodes a string using URL-safe Base64 without padding,
// matching the SSR convention.
func ssrBase64Encode(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// buildSSRURL constructs a full ssr:// URL from parts.
// mainPart = server:port:protocol:method:obfs:base64(password)
// params are key=base64(value) pairs joined by &.
func buildSSRURL(server string, port int, protocol, method, obfs, password string, params map[string]string) string {
	passEnc := ssrBase64Encode(password)
	main := fmt.Sprintf("%s:%d:%s:%s:%s:%s", server, port, protocol, method, obfs, passEnc)

	if len(params) > 0 {
		var pairs []string
		for k, v := range params {
			pairs = append(pairs, fmt.Sprintf("%s=%s", k, ssrBase64Encode(v)))
		}
		main += "/?" + strings.Join(pairs, "&")
	}

	return "ssr://" + ssrBase64Encode(main)
}

// TestSSR_StandardWithAllParams verifies parsing a standard SSR URL that
// contains every optional parameter: obfsparam, protoparam, remarks, group.
func TestSSR_StandardWithAllParams(t *testing.T) {
	url := buildSSRURL(
		"example.com", 8388,
		"auth_aes128_sha1", "aes-256-cfb", "tls1.2_ticket_auth",
		"mypassword123",
		map[string]string{
			"obfsparam":  "cdn.example.com",
			"protoparam": "12345:userkey",
			"remarks":    "HK-Node-01",
			"group":      "MyGroup",
		},
	)

	parser := &ShadowsocksRParser{}
	node, err := parser.Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Server & port
	if node.Server != "example.com" {
		t.Errorf("server: want %q, got %q", "example.com", node.Server)
	}
	if node.ServerPort != 8388 {
		t.Errorf("port: want 8388, got %d", node.ServerPort)
	}

	// Type
	if node.Type != "shadowsocksr" {
		t.Errorf("type: want %q, got %q", "shadowsocksr", node.Type)
	}

	// Tag should come from remarks
	if node.Tag != "HK-Node-01" {
		t.Errorf("tag: want %q, got %q", "HK-Node-01", node.Tag)
	}

	// Extra fields
	assertExtra := func(key, want string) {
		t.Helper()
		got, ok := node.Extra[key]
		if !ok {
			t.Errorf("extra[%q] missing", key)
			return
		}
		if fmt.Sprintf("%v", got) != want {
			t.Errorf("extra[%q]: want %q, got %q", key, want, got)
		}
	}

	assertExtra("method", "aes-256-cfb")
	assertExtra("password", "mypassword123")
	assertExtra("protocol", "auth_aes128_sha1")
	assertExtra("protocol_param", "12345:userkey")
	assertExtra("obfs", "tls1.2_ticket_auth")
	assertExtra("obfs_param", "cdn.example.com")
}

// TestSSR_RequiredFieldsOnly verifies parsing an SSR URL that has no optional
// query parameters at all (no obfsparam, protoparam, remarks, group).
func TestSSR_RequiredFieldsOnly(t *testing.T) {
	// Build raw payload without /? query string
	passEnc := ssrBase64Encode("secret")
	main := fmt.Sprintf("1.2.3.4:443:origin:aes-128-ctr:plain:%s", passEnc)
	url := "ssr://" + ssrBase64Encode(main)

	parser := &ShadowsocksRParser{}
	node, err := parser.Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if node.Server != "1.2.3.4" {
		t.Errorf("server: want %q, got %q", "1.2.3.4", node.Server)
	}
	if node.ServerPort != 443 {
		t.Errorf("port: want 443, got %d", node.ServerPort)
	}

	// Tag should default to server:port when remarks is absent
	wantTag := "1.2.3.4:443"
	if node.Tag != wantTag {
		t.Errorf("tag: want %q, got %q", wantTag, node.Tag)
	}

	if node.Extra["password"] != "secret" {
		t.Errorf("password: want %q, got %v", "secret", node.Extra["password"])
	}
	if node.Extra["method"] != "aes-128-ctr" {
		t.Errorf("method: want %q, got %v", "aes-128-ctr", node.Extra["method"])
	}
	if node.Extra["protocol"] != "origin" {
		t.Errorf("protocol: want %q, got %v", "origin", node.Extra["protocol"])
	}
	if node.Extra["obfs"] != "plain" {
		t.Errorf("obfs: want %q, got %v", "plain", node.Extra["obfs"])
	}

	// No optional params should be set
	if _, ok := node.Extra["obfs_param"]; ok {
		t.Errorf("obfs_param should not be set, got %v", node.Extra["obfs_param"])
	}
	if _, ok := node.Extra["protocol_param"]; ok {
		t.Errorf("protocol_param should not be set, got %v", node.Extra["protocol_param"])
	}
}

// TestSSR_SpecialCharsChinese verifies that Chinese characters in remarks and
// passwords are handled correctly after double Base64 encoding.
func TestSSR_SpecialCharsChinese(t *testing.T) {
	url := buildSSRURL(
		"cn.server.net", 12345,
		"auth_chain_a", "chacha20-ietf", "http_simple",
		"p@ss中文密码!",
		map[string]string{
			"remarks": "香港节点-高速",
		},
	)

	parser := &ShadowsocksRParser{}
	node, err := parser.Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if node.Tag != "香港节点-高速" {
		t.Errorf("tag: want %q, got %q", "香港节点-高速", node.Tag)
	}
	if node.Extra["password"] != "p@ss中文密码!" {
		t.Errorf("password: want %q, got %v", "p@ss中文密码!", node.Extra["password"])
	}
}

// TestSSR_SpecialCharsEmoji verifies that emoji in remarks survive the
// Base64 encode/decode round-trip.
func TestSSR_SpecialCharsEmoji(t *testing.T) {
	url := buildSSRURL(
		"emoji.server.io", 9999,
		"origin", "rc4-md5", "plain",
		"hunter2",
		map[string]string{
			"remarks": "🚀 Fast Node 🌍",
		},
	)

	parser := &ShadowsocksRParser{}
	node, err := parser.Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if node.Tag != "🚀 Fast Node 🌍" {
		t.Errorf("tag: want %q, got %q", "🚀 Fast Node 🌍", node.Tag)
	}
}

// TestSSR_IPv6Address verifies that an IPv6 server address enclosed in
// brackets is parsed correctly.
func TestSSR_IPv6Address(t *testing.T) {
	// IPv6 SSR uses brackets: [::1]:port:proto:method:obfs:pass
	passEnc := ssrBase64Encode("ipv6pass")
	main := fmt.Sprintf("[::1]:8080:auth_sha1_v4:aes-256-cfb:tls1.2_ticket_auth:%s", passEnc)
	params := fmt.Sprintf("remarks=%s", ssrBase64Encode("IPv6-Node"))
	payload := main + "/?" + params
	url := "ssr://" + ssrBase64Encode(payload)

	parser := &ShadowsocksRParser{}
	node, err := parser.Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if node.Server != "::1" {
		t.Errorf("server: want %q, got %q", "::1", node.Server)
	}
	if node.ServerPort != 8080 {
		t.Errorf("port: want 8080, got %d", node.ServerPort)
	}
	if node.Tag != "IPv6-Node" {
		t.Errorf("tag: want %q, got %q", "IPv6-Node", node.Tag)
	}
	if node.Extra["protocol"] != "auth_sha1_v4" {
		t.Errorf("protocol: want %q, got %v", "auth_sha1_v4", node.Extra["protocol"])
	}
}

// TestSSR_IPv6FullAddress verifies a full (non-loopback) IPv6 address.
func TestSSR_IPv6FullAddress(t *testing.T) {
	passEnc := ssrBase64Encode("v6pass")
	main := fmt.Sprintf("[2001:db8::1]:443:origin:chacha20:plain:%s", passEnc)
	url := "ssr://" + ssrBase64Encode(main)

	parser := &ShadowsocksRParser{}
	node, err := parser.Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if node.Server != "2001:db8::1" {
		t.Errorf("server: want %q, got %q", "2001:db8::1", node.Server)
	}
	if node.ServerPort != 443 {
		t.Errorf("port: want 443, got %d", node.ServerPort)
	}
}

// TestSSR_DispatchViaParsURL verifies that the global ParseURL dispatcher
// correctly routes ssr:// URLs to ShadowsocksRParser.
func TestSSR_DispatchViaParsURL(t *testing.T) {
	url := buildSSRURL(
		"dispatch.test.com", 1234,
		"origin", "aes-256-cfb", "plain",
		"pw123",
		map[string]string{
			"remarks": "DispatchTest",
		},
	)

	node, err := ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL failed: %v", err)
	}

	if node.Type != "shadowsocksr" {
		t.Errorf("type: want %q, got %q", "shadowsocksr", node.Type)
	}
	if node.Server != "dispatch.test.com" {
		t.Errorf("server: want %q, got %q", "dispatch.test.com", node.Server)
	}
	if node.ServerPort != 1234 {
		t.Errorf("port: want 1234, got %d", node.ServerPort)
	}
	if node.Tag != "DispatchTest" {
		t.Errorf("tag: want %q, got %q", "DispatchTest", node.Tag)
	}
	if node.Extra["password"] != "pw123" {
		t.Errorf("password: want %q, got %v", "pw123", node.Extra["password"])
	}
}

// TestSSR_ProtocolMethod verifies the Protocol() method returns the correct
// protocol string.
func TestSSR_ProtocolMethod(t *testing.T) {
	p := &ShadowsocksRParser{}
	if got := p.Protocol(); got != "shadowsocksr" {
		t.Errorf("Protocol(): want %q, got %q", "shadowsocksr", got)
	}
}

// TestSSR_InvalidInputs verifies that malformed SSR URLs produce errors.
func TestSSR_InvalidInputs(t *testing.T) {
	parser := &ShadowsocksRParser{}

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "not_base64",
			url:  "ssr://!!!invalid-base64!!!",
		},
		{
			name: "too_few_fields",
			// Only 3 colon-separated fields after decode
			url: "ssr://" + ssrBase64Encode("server:80:origin"),
		},
		{
			name: "invalid_port",
			url:  "ssr://" + ssrBase64Encode("server:notaport:origin:aes-256-cfb:plain:" + ssrBase64Encode("pw")),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parser.Parse(tc.url)
			if err == nil {
				t.Errorf("expected error for input %q, got nil", tc.name)
			}
		})
	}
}

// TestSSR_Base64NoPadding verifies that SSR URLs with Base64 values that
// would normally require padding (length not a multiple of 4) are handled.
func TestSSR_Base64NoPadding(t *testing.T) {
	// "a" encodes to "YQ" (2 chars) in RawURLEncoding -- needs 2 padding chars normally
	// "ab" encodes to "YWI" (3 chars) -- needs 1 padding char normally
	// Both should decode fine through utils.DecodeBase64.
	url := buildSSRURL(
		"pad.test.com", 5555,
		"origin", "aes-128-cfb", "plain",
		"a", // short password to trigger non-padded base64
		map[string]string{
			"remarks": "ab", // another short string
		},
	)

	parser := &ShadowsocksRParser{}
	node, err := parser.Parse(url)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if node.Extra["password"] != "a" {
		t.Errorf("password: want %q, got %v", "a", node.Extra["password"])
	}
	if node.Tag != "ab" {
		t.Errorf("tag: want %q, got %q", "ab", node.Tag)
	}
}

// TestSSR_ClashYAMLIntegration verifies that Clash YAML with SSR proxies
// is correctly parsed into shadowsocksr nodes.
func TestSSR_ClashYAMLIntegration(t *testing.T) {
	yamlContent := `port: 7890
proxies:
  - {name: "台湾・01", server: 6kusjo.ps6ywnxy.com, port: 1070, type: ssr, cipher: chacha20-ietf, password: bxsnucrgk6hfish, protocol: auth_aes128_sha1, obfs: plain, protocol-param: "346320:HEQHnc", obfs-param: "11061346320.microsoft.com", tfo: false}
  - {name: "台湾・02", server: 6kusjo.ps6ywnxy.com, port: 1071, type: ssr, cipher: chacha20-ietf, password: bxsnucrgk6hfish, protocol: auth_aes128_sha1, obfs: plain, protocol-param: "346320:HEQHnc", obfs-param: "11061346320.microsoft.com", tfo: false}
`

	nodes, err := ParseClashYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseClashYAML failed: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	node := nodes[0]
	if node.Type != "shadowsocksr" {
		t.Errorf("type: want %q, got %q", "shadowsocksr", node.Type)
	}
	if node.Server != "6kusjo.ps6ywnxy.com" {
		t.Errorf("server: want %q, got %q", "6kusjo.ps6ywnxy.com", node.Server)
	}
	if node.ServerPort != 1070 {
		t.Errorf("port: want 1070, got %d", node.ServerPort)
	}
	if node.Extra["method"] != "chacha20-ietf" {
		t.Errorf("method: want %q, got %v", "chacha20-ietf", node.Extra["method"])
	}
	if node.Extra["password"] != "bxsnucrgk6hfish" {
		t.Errorf("password: want %q, got %v", "bxsnucrgk6hfish", node.Extra["password"])
	}
	if node.Extra["protocol"] != "auth_aes128_sha1" {
		t.Errorf("protocol: want %q, got %v", "auth_aes128_sha1", node.Extra["protocol"])
	}
	if node.Extra["protocol_param"] != "346320:HEQHnc" {
		t.Errorf("protocol_param: want %q, got %v", "346320:HEQHnc", node.Extra["protocol_param"])
	}
	if node.Extra["obfs"] != "plain" {
		t.Errorf("obfs: want %q, got %v", "plain", node.Extra["obfs"])
	}
	if node.Extra["obfs_param"] != "11061346320.microsoft.com" {
		t.Errorf("obfs_param: want %q, got %v", "11061346320.microsoft.com", node.Extra["obfs_param"])
	}
	if nodes[1].ServerPort != 1071 {
		t.Errorf("second node port: want 1071, got %d", nodes[1].ServerPort)
	}
}
