package parser

import "net/url"

// FirstNonEmptyParam returns the first non-empty query value among keys.
// Canonical share-link alias helper (owned by fix/sharelink-param-aliases / PR #115).
func FirstNonEmptyParam(params url.Values, keys ...string) string {
	for _, key := range keys {
		if v := params.Get(key); v != "" {
			return v
		}
	}
	return ""
}

// GetParamBoolAny returns true if any of the keys is a truthy bool param.
// Canonical share-link alias helper (owned by fix/sharelink-param-aliases / PR #115).
func GetParamBoolAny(params url.Values, keys ...string) bool {
	for _, key := range keys {
		if getParamBool(params, key) {
			return true
		}
	}
	return false
}
