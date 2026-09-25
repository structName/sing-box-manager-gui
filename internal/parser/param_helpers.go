package parser

import "net/url"

// shareLinkUTLSFingerprint returns the uTLS fingerprint from share-link query
// params. Xray-style links use fp=; some panels/Clash exporters emit fingerprint=.
// When required is true and neither is set, defaults to chrome (Reality / AnyTLS).
//
// Temporary identical copy of the helper owned by
// fix/sharelink-fingerprint-alias-for-fp / PR #135 so this branch builds
// without #135 merged. Land order: #135 then #138 (this file is #135's).
// Does not ship FirstNonEmptyParam / GetParamBoolAny (#115).
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
