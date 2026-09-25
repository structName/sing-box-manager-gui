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

func TestNodeToMihomoProxyHysteria2PortsAlpnHopInterval(t *testing.T) {
	node := &models.Node{
		Tag:        "hy2-hop",
		Type:       "hysteria2",
		Server:     "hy2.example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"password":     "secret",
			"ports":        "20000-50000",
			"hop_interval": "30s",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "hy2.example.com",
				"alpn":        []interface{}{"h3"},
				"insecure":    true,
			},
			"obfs": map[string]interface{}{
				"type":     "salamander",
				"password": "obfs-secret",
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy error: %v", err)
	}
	if proxy["type"] != "hysteria2" {
		t.Fatalf("type = %v, want hysteria2", proxy["type"])
	}
	if proxy["ports"] != "20000-50000" {
		t.Fatalf("ports = %v, want 20000-50000", proxy["ports"])
	}
	if proxy["hop-interval"] != 30 {
		t.Fatalf("hop-interval = %v, want 30", proxy["hop-interval"])
	}
	if proxy["sni"] != "hy2.example.com" {
		t.Fatalf("sni = %v", proxy["sni"])
	}
	if proxy["skip-cert-verify"] != true {
		t.Fatalf("skip-cert-verify = %v", proxy["skip-cert-verify"])
	}
	alpn, ok := proxy["alpn"].([]string)
	if !ok || len(alpn) != 1 || alpn[0] != "h3" {
		t.Fatalf("alpn = %#v, want [h3]", proxy["alpn"])
	}
	if proxy["obfs"] != "salamander" || proxy["obfs-password"] != "obfs-secret" {
		t.Fatalf("obfs = %v / %v", proxy["obfs"], proxy["obfs-password"])
	}
}

func TestNodeToMihomoProxyHysteria2ServerPortsList(t *testing.T) {
	node := &models.Node{
		Tag:        "hy2-sp",
		Type:       "hy2",
		Server:     "hy2.example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"password":     "secret",
			"server_ports": []string{"1000-2000", "3000"},
			"hop_interval": 10,
		},
	}
	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy error: %v", err)
	}
	if proxy["ports"] != "1000-2000,3000" {
		t.Fatalf("ports = %v, want 1000-2000,3000", proxy["ports"])
	}
	if proxy["hop-interval"] != 10 {
		t.Fatalf("hop-interval = %v, want 10", proxy["hop-interval"])
	}
}
