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

func TestNodeToMihomoProxyVmessHTTPUpgradeTransportOpts(t *testing.T) {
	node := &models.Node{
		Tag:        "vmess-hu",
		Type:       "vmess",
		Server:     "edge.example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"uuid":     "11111111-1111-1111-1111-111111111111",
			"alter_id": 0,
			"security": "auto",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "cdn.example.com",
			},
			"transport": map[string]interface{}{
				"type": "httpupgrade",
				"path": "/hu",
				"host": "cdn.example.com",
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	if proxy["network"] != "ws" {
		t.Fatalf("network = %v, want ws (httpupgrade remapped)", proxy["network"])
	}
	opts, ok := proxy["ws-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("ws-opts missing: %#v", proxy["ws-opts"])
	}
	if opts["path"] != "/hu" {
		t.Fatalf("path = %v, want /hu", opts["path"])
	}
	if opts["v2ray-http-upgrade"] != true {
		t.Fatalf("v2ray-http-upgrade = %v, want true", opts["v2ray-http-upgrade"])
	}
	headers, ok := opts["headers"].(map[string]string)
	if !ok || headers["Host"] != "cdn.example.com" {
		t.Fatalf("headers = %#v, want Host=cdn.example.com", opts["headers"])
	}
}

func TestNodeToMihomoProxyVlessHTTPUpgradeClashHeaders(t *testing.T) {
	node := &models.Node{
		Tag:        "vless-hu",
		Type:       "vless",
		Server:     "edge.example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"uuid": "22222222-2222-2222-2222-222222222222",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "cdn.example.com",
			},
			"transport": map[string]interface{}{
				"type": "http_upgrade",
				"path": "/upgrade",
				"headers": map[string]string{
					"Host": "cdn.example.com",
				},
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	if proxy["network"] != "ws" {
		t.Fatalf("network = %v, want ws", proxy["network"])
	}
	opts, ok := proxy["ws-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("ws-opts missing: %#v", proxy)
	}
	if opts["v2ray-http-upgrade"] != true {
		t.Fatalf("v2ray-http-upgrade = %v", opts["v2ray-http-upgrade"])
	}
	if opts["path"] != "/upgrade" {
		t.Fatalf("path = %v", opts["path"])
	}
	headers, ok := opts["headers"].(map[string]string)
	if !ok || headers["Host"] != "cdn.example.com" {
		t.Fatalf("headers = %#v", opts["headers"])
	}
}

func TestNodeToMihomoProxyTrojanHTTPUpgradeTransportOpts(t *testing.T) {
	node := &models.Node{
		Tag:        "trojan-hu",
		Type:       "trojan",
		Server:     "edge.example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"password": "secret",
			"tls": map[string]interface{}{
				"server_name": "cdn.example.com",
			},
			"transport": map[string]interface{}{
				"type": "httpupgrade",
				"path": "/trojan-hu",
				"host": "cdn.example.com",
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	if proxy["network"] != "ws" {
		t.Fatalf("network = %v, want ws", proxy["network"])
	}
	opts, ok := proxy["ws-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("ws-opts missing: %#v", proxy)
	}
	if opts["path"] != "/trojan-hu" {
		t.Fatalf("path = %v", opts["path"])
	}
	if opts["v2ray-http-upgrade"] != true {
		t.Fatalf("v2ray-http-upgrade = %v", opts["v2ray-http-upgrade"])
	}
	headers, ok := opts["headers"].(map[string]string)
	if !ok || headers["Host"] != "cdn.example.com" {
		t.Fatalf("headers = %#v", opts["headers"])
	}
}

func TestNodeToMihomoProxyTrojanWSStillNetworkOnly(t *testing.T) {
	// WS/gRPC/HTTP/H2 opts are owned by other open digs; do not invent them here.
	node := &models.Node{
		Tag:        "trojan-ws",
		Type:       "trojan",
		Server:     "edge.example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"password": "secret",
			"transport": map[string]interface{}{
				"type": "ws",
				"path": "/ws",
				"headers": map[string]string{
					"Host": "ws.example.com",
				},
			},
		},
	}
	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	if proxy["network"] != "ws" {
		t.Fatalf("network = %v, want ws", proxy["network"])
	}
	if _, ok := proxy["ws-opts"]; ok {
		t.Fatalf("ws-opts left to other dig; must not invent here, got %#v", proxy["ws-opts"])
	}
}
