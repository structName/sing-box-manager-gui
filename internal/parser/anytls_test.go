package parser

import "testing"

func TestAnyTLSParserParsesDurationsForSingBox(t *testing.T) {
	node, err := ParseURL("anytls://secret@example.com:443?sni=example.com&idle-check-interval=30&idle-timeout=45&min-idle-session=5#any")
	if err != nil {
		t.Fatalf("ParseURL returned error: %v", err)
	}

	if node.Type != "anytls" {
		t.Fatalf("type = %q, want anytls", node.Type)
	}
	if got := node.Extra["idle_session_check_interval"]; got != "30s" {
		t.Fatalf("idle_session_check_interval = %v, want 30s", got)
	}
	if got := node.Extra["idle_session_timeout"]; got != "45s" {
		t.Fatalf("idle_session_timeout = %v, want 45s", got)
	}
	if got := node.Extra["min_idle_session"]; got != 5 {
		t.Fatalf("min_idle_session = %v, want 5", got)
	}
}

// Clash Meta / sublinkPro emit idle-session-* query keys (not the short
// idle-check-* aliases). Those must map onto sing-box AnyTLS durations.
func TestAnyTLSParserAcceptsClashMetaIdleSessionParams(t *testing.T) {
	raw := "anytls://secret@edge.example.com:443" +
		"?sni=cdn.example.com" +
		"&idle-session-check-interval=30" +
		"&idle-session-timeout=60" +
		"&min-idle-session=3#meta"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if got := node.Extra["idle_session_check_interval"]; got != "30s" {
		t.Fatalf("idle_session_check_interval = %#v, want 30s", got)
	}
	if got := node.Extra["idle_session_timeout"]; got != "60s" {
		t.Fatalf("idle_session_timeout = %#v, want 60s", got)
	}
	if got := node.Extra["min_idle_session"]; got != 3 {
		t.Fatalf("min_idle_session = %#v, want 3", got)
	}
}

func TestAnyTLSParserPrefersClashMetaIdleSessionNames(t *testing.T) {
	// When both naming styles appear, prefer Clash Meta / sing-box-aligned names.
	raw := "anytls://secret@edge.example.com:443" +
		"?idle-session-check-interval=30&idle-check-interval=99" +
		"&idle-session-timeout=60&idle-timeout=99#both"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if got := node.Extra["idle_session_check_interval"]; got != "30s" {
		t.Fatalf("idle_session_check_interval = %#v, want 30s (Clash Meta name wins)", got)
	}
	if got := node.Extra["idle_session_timeout"]; got != "60s" {
		t.Fatalf("idle_session_timeout = %#v, want 60s (Clash Meta name wins)", got)
	}
}
