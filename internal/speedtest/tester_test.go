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

func TestNodeToMihomoProxyVmessHTTPTransportOpts(t *testing.T) {
	node := &models.Node{
		Tag:        "vmess-http",
		Type:       "vmess",
		Server:     "1.2.3.4",
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
				"type":   "http",
				"method": "GET",
				"path":   "/api",
				"host":   []string{"cdn.example.com"},
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	if proxy["network"] != "http" {
		t.Fatalf("network = %v, want http", proxy["network"])
	}
	opts, ok := proxy["http-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("http-opts missing: %#v", proxy["http-opts"])
	}
	if opts["method"] != "GET" {
		t.Fatalf("method = %v, want GET", opts["method"])
	}
	paths, ok := opts["path"].([]string)
	if !ok || len(paths) != 1 || paths[0] != "/api" {
		t.Fatalf("path = %#v, want [/api]", opts["path"])
	}
	headers, ok := opts["headers"].(map[string][]string)
	if !ok || len(headers["Host"]) != 1 || headers["Host"][0] != "cdn.example.com" {
		t.Fatalf("headers = %#v, want Host=cdn.example.com", opts["headers"])
	}
}

func TestNodeToMihomoProxyVmessH2TransportOpts(t *testing.T) {
	node := &models.Node{
		Tag:        "vmess-h2",
		Type:       "vmess",
		Server:     "1.2.3.4",
		ServerPort: 443,
		Extra: models.JSONMap{
			"uuid":     "11111111-1111-1111-1111-111111111111",
			"alter_id": 0,
			"security": "auto",
			"transport": map[string]interface{}{
				"type": "h2",
				"path": "/h2",
				"host": []interface{}{"h2.example.com", "h2-alt.example.com"},
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	if proxy["network"] != "h2" {
		t.Fatalf("network = %v, want h2", proxy["network"])
	}
	opts, ok := proxy["h2-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("h2-opts missing: %#v", proxy["h2-opts"])
	}
	if opts["path"] != "/h2" {
		t.Fatalf("path = %v, want /h2", opts["path"])
	}
	host, ok := opts["host"].([]string)
	if !ok || len(host) != 2 || host[0] != "h2.example.com" || host[1] != "h2-alt.example.com" {
		t.Fatalf("host = %#v", opts["host"])
	}
}

func TestNodeToMihomoProxyVlessHTTPTransportClashHeaders(t *testing.T) {
	node := &models.Node{
		Tag:        "vless-http",
		Type:       "vless",
		Server:     "edge.example.com",
		ServerPort: 443,
		Extra: models.JSONMap{
			"uuid": "22222222-2222-2222-2222-222222222222",
			"transport": map[string]interface{}{
				"type": "http",
				"path": []interface{}{"/", "/api"},
				"headers": map[string]interface{}{
					"Host": []interface{}{"www.example.com"},
					"User-Agent": []interface{}{
						"Mozilla/5.0",
					},
				},
			},
		},
	}

	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	if proxy["network"] != "http" {
		t.Fatalf("network = %v, want http", proxy["network"])
	}
	opts, ok := proxy["http-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("http-opts missing: %#v", proxy["http-opts"])
	}
	paths, ok := opts["path"].([]string)
	if !ok || len(paths) != 2 || paths[0] != "/" || paths[1] != "/api" {
		t.Fatalf("path = %#v, want [/ /api]", opts["path"])
	}
	headers, ok := opts["headers"].(map[string][]string)
	if !ok {
		t.Fatalf("headers type = %T", opts["headers"])
	}
	if len(headers["Host"]) != 1 || headers["Host"][0] != "www.example.com" {
		t.Fatalf("Host = %#v", headers["Host"])
	}
	if len(headers["User-Agent"]) != 1 || headers["User-Agent"][0] != "Mozilla/5.0" {
		t.Fatalf("User-Agent = %#v", headers["User-Agent"])
	}
}


func TestNodeToMihomoProxyHTTPHeadersHostCanonicalLowercase(t *testing.T) {
	// Lowercase "host" header must become canonical "Host" and must not block
	// recognition (mihomo looks up Headers["Host"] exactly).
	node := &models.Node{
		Tag:        "vmess-http-host-lower",
		Type:       "vmess",
		Server:     "1.2.3.4",
		ServerPort: 443,
		Extra: models.JSONMap{
			"uuid":     "11111111-1111-1111-1111-111111111111",
			"alter_id": 0,
			"security": "auto",
			"transport": map[string]interface{}{
				"type": "http",
				"path": "/api",
				"headers": map[string][]string{
					"host": {"cdn.example.com"},
				},
			},
		},
	}
	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	opts, ok := proxy["http-opts"].(map[string]interface{})
	if !ok {
		t.Fatalf("http-opts missing: %#v", proxy)
	}
	headers, ok := opts["headers"].(map[string][]string)
	if !ok {
		t.Fatalf("headers type = %T", opts["headers"])
	}
	if _, bad := headers["host"]; bad {
		t.Fatalf("lowercase host key must be canonicalized away, got %#v", headers)
	}
	if len(headers["Host"]) != 1 || headers["Host"][0] != "cdn.example.com" {
		t.Fatalf("Host = %#v, want [cdn.example.com]", headers["Host"])
	}
}

func TestNodeToMihomoProxyHTTPHeadersHostBackfillDespiteLowercaseEmpty(t *testing.T) {
	// Empty lowercase "host" header must not block backfill from transport["host"].
	node := &models.Node{
		Tag:        "vmess-http-host-backfill",
		Type:       "vmess",
		Server:     "1.2.3.4",
		ServerPort: 443,
		Extra: models.JSONMap{
			"uuid":     "11111111-1111-1111-1111-111111111111",
			"alter_id": 0,
			"security": "auto",
			"transport": map[string]interface{}{
				"type": "http",
				"path": "/api",
				"host": []string{"cdn.example.com"},
				"headers": map[string][]string{
					"host": {""},
				},
			},
		},
	}
	proxy, err := nodeToMihomoProxy(node)
	if err != nil {
		t.Fatalf("nodeToMihomoProxy: %v", err)
	}
	opts := proxy["http-opts"].(map[string]interface{})
	headers := opts["headers"].(map[string][]string)
	if _, bad := headers["host"]; bad {
		t.Fatalf("empty lowercase host must be dropped, got %#v", headers)
	}
	if len(headers["Host"]) != 1 || headers["Host"][0] != "cdn.example.com" {
		t.Fatalf("Host backfill = %#v, want [cdn.example.com]", headers["Host"])
	}
}

func TestNodeToMihomoProxyHTTPHeadersEmptyParityTypedVsJSON(t *testing.T) {
	// Typed map[string][]string with blank values must filter like JSON-decoded maps.
	mk := func(headers interface{}) *models.Node {
		return &models.Node{
			Tag:        "vless-http-empty",
			Type:       "vless",
			Server:     "edge.example.com",
			ServerPort: 443,
			Extra: models.JSONMap{
				"uuid": "22222222-2222-2222-2222-222222222222",
				"transport": map[string]interface{}{
					"type":    "http",
					"path":    "/",
					"headers": headers,
				},
			},
		}
	}

	typed, err := nodeToMihomoProxy(mk(map[string][]string{
		"Host":       {""},
		"User-Agent": {"", "Mozilla/5.0", ""},
		"X-Empty":    {""},
	}))
	if err != nil {
		t.Fatalf("typed: %v", err)
	}
	jsonLike, err := nodeToMihomoProxy(mk(map[string]interface{}{
		"Host":       []interface{}{""},
		"User-Agent": []interface{}{"", "Mozilla/5.0", ""},
		"X-Empty":    []interface{}{""},
	}))
	if err != nil {
		t.Fatalf("json: %v", err)
	}

	th := typed["http-opts"].(map[string]interface{})["headers"].(map[string][]string)
	jh := jsonLike["http-opts"].(map[string]interface{})["headers"].(map[string][]string)

	if _, ok := th["Host"]; ok {
		t.Fatalf("typed Host empty must be dropped, got %#v", th)
	}
	if _, ok := jh["Host"]; ok {
		t.Fatalf("json Host empty must be dropped, got %#v", jh)
	}
	if _, ok := th["X-Empty"]; ok {
		t.Fatalf("typed X-Empty must be dropped, got %#v", th)
	}
	if _, ok := jh["X-Empty"]; ok {
		t.Fatalf("json X-Empty must be dropped, got %#v", jh)
	}
	if len(th["User-Agent"]) != 1 || th["User-Agent"][0] != "Mozilla/5.0" {
		t.Fatalf("typed User-Agent = %#v", th["User-Agent"])
	}
	if len(jh["User-Agent"]) != 1 || jh["User-Agent"][0] != "Mozilla/5.0" {
		t.Fatalf("json User-Agent = %#v", jh["User-Agent"])
	}
}

func TestNodeToMihomoProxyHTTPHeadersEmptyNameRejected(t *testing.T) {
	// Empty header names must be dropped for typed and JSON-decoded maps alike.
	mk := func(headers interface{}) *models.Node {
		return &models.Node{
			Tag:        "vless-http-empty-name",
			Type:       "vless",
			Server:     "edge.example.com",
			ServerPort: 443,
			Extra: models.JSONMap{
				"uuid": "33333333-3333-3333-3333-333333333333",
				"transport": map[string]interface{}{
					"type":    "http",
					"path":    "/",
					"headers": headers,
				},
			},
		}
	}

	typed, err := nodeToMihomoProxy(mk(map[string][]string{
		"":           {"should-drop"},
		"User-Agent": {"Mozilla/5.0"},
	}))
	if err != nil {
		t.Fatalf("typed: %v", err)
	}
	jsonLike, err := nodeToMihomoProxy(mk(map[string]interface{}{
		"":           []interface{}{"should-drop"},
		"User-Agent": []interface{}{"Mozilla/5.0"},
	}))
	if err != nil {
		t.Fatalf("json: %v", err)
	}

	th := typed["http-opts"].(map[string]interface{})["headers"].(map[string][]string)
	jh := jsonLike["http-opts"].(map[string]interface{})["headers"].(map[string][]string)

	if _, ok := th[""]; ok {
		t.Fatalf("typed empty-name header must be dropped, got %#v", th)
	}
	if _, ok := jh[""]; ok {
		t.Fatalf("json empty-name header must be dropped, got %#v", jh)
	}
	if len(th["User-Agent"]) != 1 || th["User-Agent"][0] != "Mozilla/5.0" {
		t.Fatalf("typed User-Agent = %#v", th["User-Agent"])
	}
	if len(jh["User-Agent"]) != 1 || jh["User-Agent"][0] != "Mozilla/5.0" {
		t.Fatalf("json User-Agent = %#v", jh["User-Agent"])
	}
}

func TestNormalizeMihomoHTTPHeadersEmptyNameTypedAndJSON(t *testing.T) {
	// Direct unit coverage: typed map[string][]string{"": {"value"}} and same-shape JSON maps.
	typed := normalizeMihomoHTTPHeaders(map[string]interface{}{
		"headers": map[string][]string{
			"":      {"value"},
			"X-Keep": {"ok"},
		},
	})
	if _, ok := typed[""]; ok {
		t.Fatalf("typed empty key must not remain, got %#v", typed)
	}
	if len(typed["X-Keep"]) != 1 || typed["X-Keep"][0] != "ok" {
		t.Fatalf("typed keep = %#v", typed)
	}

	jsonLike := normalizeMihomoHTTPHeaders(map[string]interface{}{
		"headers": map[string]interface{}{
			"":      []interface{}{"value"},
			"X-Keep": "ok",
		},
	})
	if _, ok := jsonLike[""]; ok {
		t.Fatalf("json empty key must not remain, got %#v", jsonLike)
	}
	if len(jsonLike["X-Keep"]) != 1 || jsonLike["X-Keep"][0] != "ok" {
		t.Fatalf("json keep = %#v", jsonLike)
	}

	// map[string]string shape also drops empty names.
	strMap := normalizeMihomoHTTPHeaders(map[string]interface{}{
		"headers": map[string]string{
			"":      "value",
			"X-Keep": "ok",
		},
	})
	if _, ok := strMap[""]; ok {
		t.Fatalf("string-map empty key must not remain, got %#v", strMap)
	}
	if len(strMap["X-Keep"]) != 1 || strMap["X-Keep"][0] != "ok" {
		t.Fatalf("string-map keep = %#v", strMap)
	}
}
