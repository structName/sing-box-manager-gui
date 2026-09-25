package parser

import "net/url"

// shareLinkUTLSFingerprint returns the uTLS fingerprint from share-link query
// params. Xray-style links use fp=; some panels/Clash exporters emit fingerprint=.
// When required is true and neither is set, defaults to chrome (Reality / AnyTLS).
// Canonical helper (owned by fix/sharelink-fingerprint-alias-for-fp / PR #135).
//
// Does not export FirstNonEmptyParam / GetParamBoolAny — those stay on PR #115.
// Merge order: #115 → #135 → #138.
func shareLinkUTLSFingerprint(params url.Values, required bool) string {
	fp := params.Get("fp")
	if fp == "" {
		fp = params.Get("fingerprint")
	}
	if fp == "" && required {
		return "chrome"
	}
	return fp
}
