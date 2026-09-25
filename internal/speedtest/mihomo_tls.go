package speedtest

import "strings"

// applyMihomoTLSAlpn maps Extra.tls.alpn ([]string or []interface{}) onto the
// mihomo proxy "alpn" field used by delay/speed checks.
// Canonical helper (owned by fix/vmess-vless-speedtest-tls-alpn-fp / PR #109).
func applyMihomoTLSAlpn(proxy map[string]interface{}, tls map[string]interface{}) {
	if alpn := tlsAlpnList(tls["alpn"]); len(alpn) > 0 {
		proxy["alpn"] = alpn
	}
}

// applyMihomoClientFingerprint maps Extra.tls.utls.fingerprint onto the mihomo
// proxy "client-fingerprint" field (overrides Reality's chrome default when set).
// Canonical helper (owned by fix/vmess-vless-speedtest-tls-alpn-fp / PR #109).
func applyMihomoClientFingerprint(proxy map[string]interface{}, tls map[string]interface{}) {
	utls, ok := tls["utls"].(map[string]interface{})
	if !ok {
		return
	}
	fp, ok := utls["fingerprint"].(string)
	if !ok {
		return
	}
	fp = strings.TrimSpace(fp)
	if fp == "" {
		return
	}
	proxy["client-fingerprint"] = fp
}

// tlsAlpnList normalizes Extra.tls.alpn from JSON ([]interface{}) or typed
// ([]string) into a non-empty []string for mihomo.
// Canonical helper (owned by fix/vmess-vless-speedtest-tls-alpn-fp / PR #109).
func tlsAlpnList(raw interface{}) []string {
	switch value := raw.(type) {
	case []string:
		out := make([]string, 0, len(value))
		for _, item := range value {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	default:
		return nil
	}
}
