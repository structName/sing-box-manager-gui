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

func TestNodeToMihomoProxyTuicRelayOpts(t *testing.T) {
	node := &models.Node{
		Tag:        "tuic-01",
		Type:       "tuic",
		Server:     "1.2.3.4",
		ServerPort: 443,
		Extra: models.JSONMap{
			"uuid":                 "11111111-2222-3333-4444-555555555555",
			"password":             "secret",
			"congestion_control":   "bbr",
			"udp_relay_mode":       "quic",
			"zero_rtt_handshake":   true,
			"heartbeat":            "10s",
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": "tuic.example.com",
				"insecure":    true,
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy tuic error: %v", err)
	}
	if proxy["type"] != "tuic" {
		t.Fatalf("type = %v, want tuic", proxy["type"])
	}
	if proxy["udp-relay-mode"] != "quic" {
		t.Fatalf("udp-relay-mode = %v, want quic", proxy["udp-relay-mode"])
	}
	if proxy["reduce-rtt"] != true {
		t.Fatalf("reduce-rtt = %v, want true", proxy["reduce-rtt"])
	}
	if proxy["heartbeat-interval"] != 10000 {
		t.Fatalf("heartbeat-interval = %v, want 10000", proxy["heartbeat-interval"])
	}
	if proxy["congestion-controller"] != "bbr" {
		t.Fatalf("congestion-controller = %v, want bbr", proxy["congestion-controller"])
	}
	if proxy["sni"] != "tuic.example.com" {
		t.Fatalf("sni = %v, want tuic.example.com", proxy["sni"])
	}
}

func TestTuicHeartbeatIntervalMs(t *testing.T) {
	cases := []struct {
		name string
		raw  interface{}
		want int
		ok   bool
	}{
		{"duration string", "10s", 10000, true},
		{"ms suffix", "5000ms", 5000, true},
		{"bare seconds string", "15", 15000, true},
		{"small int seconds", 10, 10000, true},
		{"large int already ms", 10000, 10000, true},
		{"empty", "", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := tuicHeartbeatIntervalMs(tc.raw)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("tuicHeartbeatIntervalMs(%v) = (%d, %v), want (%d, %v)", tc.raw, got, ok, tc.want, tc.ok)
			}
		})
	}
}
