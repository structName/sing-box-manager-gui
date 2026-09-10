package speedtest

// applySocksToMihomo maps sing-box SOCKS node extras onto a mihomo proxy map.
func applySocksToMihomo(proxy map[string]interface{}, extra map[string]interface{}) {
	version := "5"
	if v, ok := extra["version"].(string); ok && v != "" {
		version = v
	}
	switch version {
	case "4", "4a":
		proxy["type"] = "socks4"
	default:
		proxy["type"] = "socks5"
	}
	if username, ok := extra["username"].(string); ok && username != "" {
		proxy["username"] = username
	}
	if password, ok := extra["password"].(string); ok && password != "" {
		proxy["password"] = password
	}
	if uot, ok := extra["udp_over_tcp"].(map[string]interface{}); ok {
		if enabled, ok := uot["enabled"].(bool); ok && enabled {
			proxy["udp-over-tcp"] = true
		}
	} else if enabled, ok := extra["udp_over_tcp"].(bool); ok && enabled {
		proxy["udp-over-tcp"] = true
	}
}
