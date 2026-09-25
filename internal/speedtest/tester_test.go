package speedtest

import (
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/database/models"
)

func TestNodeToMihomoProxyIncludesClashStyleObfsPlugin(t *testing.T) {
	node := &models.Node{
		Tag:        "hk-01",
		Type:       "shadowsocks",
		Server:     "example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"method":   "aes-128-gcm",
			"password": "secret",
			"plugin":   "obfs",
			"plugin_opts": map[string]interface{}{
				"mode": "http",
				"host": "cdn.example.com",
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy returned error: %v", err)
	}

	if got := proxy["plugin"]; got != "obfs" {
		t.Fatalf("plugin = %v, want obfs", got)
	}

	opts, ok := proxy["plugin-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("plugin-opts type = %T, want map[string]interface{}", proxy["plugin-opts"])
	}
	if got := opts["mode"]; got != "http" {
		t.Fatalf("plugin-opts.mode = %v, want http", got)
	}
	if got := opts["host"]; got != "cdn.example.com" {
		t.Fatalf("plugin-opts.host = %v, want cdn.example.com", got)
	}
}

func TestNodeToMihomoProxyParsesSingBoxStyleObfsPlugin(t *testing.T) {
	node := &models.Node{
		Tag:        "hk-01",
		Type:       "shadowsocks",
		Server:     "example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"method":      "aes-128-gcm",
			"password":    "secret",
			"plugin":      "obfs-local",
			"plugin_opts": "obfs=http;obfs-host=cdn.example.com",
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy returned error: %v", err)
	}

	if got := proxy["plugin"]; got != "obfs" {
		t.Fatalf("plugin = %v, want obfs", got)
	}

	opts, ok := proxy["plugin-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("plugin-opts type = %T, want map[string]interface{}", proxy["plugin-opts"])
	}
	if got := opts["mode"]; got != "http" {
		t.Fatalf("plugin-opts.mode = %v, want http", got)
	}
	if got := opts["host"]; got != "cdn.example.com" {
		t.Fatalf("plugin-opts.host = %v, want cdn.example.com", got)
	}
}

func TestNodeToMihomoProxyConvertsAnyTLSDurationsToSeconds(t *testing.T) {
	node := &models.Node{
		Tag:        "anytls-01",
		Type:       "anytls",
		Server:     "example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"password":                    "secret",
			"idle_session_check_interval": "30s",
			"idle_session_timeout":        "45s",
			"min_idle_session":            float64(5),
			"tls": map[string]interface{}{
				"server_name": "example.com",
				"utls": map[string]interface{}{
					"fingerprint": "chrome",
				},
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy returned error: %v", err)
	}

	if got := proxy["idle-session-check-interval"]; got != 30 {
		t.Fatalf("idle-session-check-interval = %v, want 30", got)
	}
	if got := proxy["idle-session-timeout"]; got != 45 {
		t.Fatalf("idle-session-timeout = %v, want 45", got)
	}
	if got := proxy["min-idle-session"]; got != 5 {
		t.Fatalf("min-idle-session = %v, want 5", got)
	}
	if got := proxy["client-fingerprint"]; got != "chrome" {
		t.Fatalf("client-fingerprint = %v, want chrome", got)
	}
}

func TestNodeToMihomoProxySocks5(t *testing.T) {
	node := &models.Node{
		Tag:        "socks5-node",
		Type:       "socks",
		Server:     "127.0.0.1",
		ServerPort: 1080,
		Extra: models.JSONMap{
			"version":  "5",
			"username": "user",
			"password": "pass",
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy socks5 error: %v", err)
	}
	if proxy["type"] != "socks5" {
		t.Fatalf("type = %v, want socks5", proxy["type"])
	}
	if proxy["username"] != "user" || proxy["password"] != "pass" {
		t.Fatalf("auth = %v/%v, want user/pass", proxy["username"], proxy["password"])
	}
}

func TestNodeToMihomoProxySocks4(t *testing.T) {
	node := &models.Node{
		Tag:        "socks4-node",
		Type:       "socks",
		Server:     "127.0.0.1",
		ServerPort: 1080,
		Extra: models.JSONMap{
			"version": "4",
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy socks4 error: %v", err)
	}
	if proxy["type"] != "socks4" {
		t.Fatalf("type = %v, want socks4", proxy["type"])
	}
}

func TestNodeToMihomoProxySocks5URLAlias(t *testing.T) {
	node := &models.Node{
		Tag:        "socks5-alias",
		Type:       "socks5",
		Server:     "example.com",
		ServerPort: 1080,
		Extra:      models.JSONMap{},
	}
	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy socks5 alias error: %v", err)
	}
	if proxy["type"] != "socks5" {
		t.Fatalf("type = %v, want socks5", proxy["type"])
	}
}


func TestNodeToMihomoProxySocksDefaultVersion(t *testing.T) {
	node := &models.Node{
		Tag:        "socks-default",
		Type:       "socks",
		Server:     "127.0.0.1",
		ServerPort: 1080,
		Extra:      models.JSONMap{},
	}
	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy default socks error: %v", err)
	}
	if proxy["type"] != "socks5" {
		t.Fatalf("type = %v, want socks5 when version omitted", proxy["type"])
	}
}

func TestNodeToMihomoProxySocksUDPOverTCP(t *testing.T) {
	node := &models.Node{
		Tag:        "socks-uot",
		Type:       "socks",
		Server:     "127.0.0.1",
		ServerPort: 1080,
		Extra: models.JSONMap{
			"version": "5",
			"udp_over_tcp": map[string]interface{}{
				"enabled": true,
			},
		},
	}
	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy uot error: %v", err)
	}
	if proxy["udp-over-tcp"] != true {
		t.Fatalf("udp-over-tcp = %v, want true", proxy["udp-over-tcp"])
	}
}

func TestNodeToMihomoProxyTrojanRealityOpts(t *testing.T) {
	node := &models.Node{
		Tag:        "trojan-reality",
		Type:       "trojan",
		Server:     "1.2.3.4",
		ServerPort: 443,
		Extra: models.JSONMap{
			"password": "secret",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "www.microsoft.com",
				"alpn":        []string{"h2", "http/1.1"},
				"reality": map[string]interface{}{
					"enabled":    true,
					"public_key": "abcdefghijklmnopqrstuvwxyz123456",
					"short_id":   "0123456789abcdef",
				},
				"utls": map[string]interface{}{
					"enabled":     true,
					"fingerprint": "firefox",
				},
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy trojan reality error: %v", err)
	}
	if proxy["type"] != "trojan" {
		t.Fatalf("type = %v, want trojan", proxy["type"])
	}
	if proxy["tls"] != true {
		t.Fatalf("tls = %v, want true", proxy["tls"])
	}
	if proxy["sni"] != "www.microsoft.com" {
		t.Fatalf("sni = %v, want www.microsoft.com", proxy["sni"])
	}
	if proxy["client-fingerprint"] != "firefox" {
		t.Fatalf("client-fingerprint = %v, want firefox", proxy["client-fingerprint"])
	}
	realityOpts, ok := proxy["reality-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("reality-opts missing or wrong type: %#v", proxy["reality-opts"])
	}
	if realityOpts["public-key"] != "abcdefghijklmnopqrstuvwxyz123456" {
		t.Fatalf("public-key = %v", realityOpts["public-key"])
	}
	if realityOpts["short-id"] != "0123456789abcdef" {
		t.Fatalf("short-id = %v", realityOpts["short-id"])
	}
	alpn, ok := proxy["alpn"].([]string)
	if !ok || len(alpn) != 2 || alpn[0] != "h2" || alpn[1] != "http/1.1" {
		t.Fatalf("alpn = %#v, want [h2 http/1.1]", proxy["alpn"])
	}
}

func TestNodeToMihomoProxyTrojanRealityDefaultsSNIAndFingerprint(t *testing.T) {
	node := &models.Node{
		Tag:        "trojan-reality-defaults",
		Type:       "trojan",
		Server:     "edge.example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"password": "secret",
			"tls": map[string]interface{}{
				"enabled": true,
				"reality": map[string]interface{}{
					"enabled":    true,
					"public_key": "pk",
					"short_id":   "abcd",
				},
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	if proxy["sni"] != "edge.example.com" {
		t.Fatalf("sni = %v, want edge.example.com (fallback to server)", proxy["sni"])
	}
	if proxy["client-fingerprint"] != "chrome" {
		t.Fatalf("client-fingerprint = %v, want chrome default", proxy["client-fingerprint"])
	}
	realityOpts, ok := proxy["reality-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("reality-opts missing: %#v", proxy["reality-opts"])
	}
	if realityOpts["public-key"] != "pk" || realityOpts["short-id"] != "abcd" {
		t.Fatalf("reality-opts = %#v", realityOpts)
	}
}

func TestNodeToMihomoProxyTrojanPlainTLSFingerprintAndAlpn(t *testing.T) {
	node := &models.Node{
		Tag:        "trojan-tls-fp",
		Type:       "trojan",
		Server:     "1.2.3.4",
		ServerPort: 443,
		Extra: models.JSONMap{
			"password": "secret",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "cdn.example.com",
				"alpn":        []interface{}{"http/1.1"},
				"utls": map[string]interface{}{
					"enabled":     true,
					"fingerprint": "chrome",
				},
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	if _, has := proxy["reality-opts"]; has {
		t.Fatalf("unexpected reality-opts on plain TLS: %#v", proxy["reality-opts"])
	}
	if proxy["client-fingerprint"] != "chrome" {
		t.Fatalf("client-fingerprint = %v, want chrome", proxy["client-fingerprint"])
	}
	alpn, ok := proxy["alpn"].([]string)
	if !ok || len(alpn) != 1 || alpn[0] != "http/1.1" {
		t.Fatalf("alpn = %#v, want [http/1.1]", proxy["alpn"])
	}
}
