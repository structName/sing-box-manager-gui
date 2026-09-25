package speedtest

import "strings"

// Owned by PR #109 (fix/vmess-vless-speedtest-tls-alpn-fp). Call sites only — do not fork.

// applyMihomoTLSAlpn maps Extra.tls.alpn onto the mihomo proxy "alpn" field
// used by delay/speed checks. Accepts typed []string, JSON []interface{}, or
// a single comma-separated string; trims and drops blanks.
func applyMihomoTLSAlpn(proxy map[string]interface{}, tls map[string]interface{}) {
	if alpn := tlsAlpnList(tls["alpn"]); len(alpn) > 0 {
		proxy["alpn"] = alpn
	}
}

// applyMihomoClientFingerprint maps Extra.tls.utls.fingerprint onto the mihomo
// proxy "client-fingerprint" field (overrides Reality's chrome default when set).
// Accepts JSON map[string]interface{} or typed map[string]string utls; trims blanks.
func applyMihomoClientFingerprint(proxy map[string]interface{}, tls map[string]interface{}) {
	fp := tlsClientFingerprint(tls["utls"])
	if fp == "" {
		return
	}
	proxy["client-fingerprint"] = fp
}

// tlsClientFingerprint extracts a trimmed non-empty fingerprint from utls.
func tlsClientFingerprint(raw interface{}) string {
	switch utls := raw.(type) {
	case map[string]interface{}:
		fp, _ := utls["fingerprint"].(string)
		return strings.TrimSpace(fp)
	case map[string]string:
		return strings.TrimSpace(utls["fingerprint"])
	default:
		return ""
	}
}

// tlsAlpnList normalizes Extra.tls.alpn from typed ([]string), JSON
// ([]interface{}), or a comma-separated string into a non-empty []string.
func tlsAlpnList(raw interface{}) []string {
	switch value := raw.(type) {
	case []string:
		return trimNonEmptyStrings(value)
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
	case string:
		parts := strings.Split(value, ",")
		return trimNonEmptyStrings(parts)
	default:
		return nil
	}
}

func trimNonEmptyStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
