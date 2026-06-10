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
