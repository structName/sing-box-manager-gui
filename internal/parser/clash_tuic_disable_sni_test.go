package parser

import "testing"

func TestClashYAMLMapsTUICDisableSNI(t *testing.T) {
	content := `
proxies:
  - name: tuic-disable-sni
    type: tuic
    server: t.example.com
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    password: secret
    sni: t.example.com
    disable-sni: true
    skip-cert-verify: true
    congestion-controller: bbr
`
	nodes, err := ParseClashYAML(content)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	tls, ok := nodes[0].Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing: %#v", nodes[0].Extra)
	}
	if enabled, _ := tls["enabled"].(bool); !enabled {
		t.Fatalf("tls.enabled = %v, want true", tls["enabled"])
	}
	if got, _ := tls["disable_sni"].(bool); !got {
		t.Fatalf("tls.disable_sni = %v, want true (Clash Meta disable-sni)", tls["disable_sni"])
	}
	// server_name still recorded for cert verification; disable_sni controls ClientHello
	if got, _ := tls["server_name"].(string); got != "t.example.com" {
		t.Fatalf("tls.server_name = %q, want t.example.com", got)
	}
}

func TestClashYAMLDisableSNIFalseOmitsField(t *testing.T) {
	content := `
proxies:
  - name: tuic-sni-on
    type: tuic
    server: t.example.com
    port: 443
    uuid: 11111111-1111-4111-8111-111111111111
    password: secret
    sni: t.example.com
    disable-sni: false
`
	nodes, err := ParseClashYAML(content)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("got %d nodes, want 1", len(nodes))
	}
	tls, ok := nodes[0].Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing: %#v", nodes[0].Extra)
	}
	if _, present := tls["disable_sni"]; present {
		t.Fatalf("disable_sni should be omitted when false, got %#v", tls["disable_sni"])
	}
}

func TestTuicURLDisableSNIStillWorks(t *testing.T) {
	// Regression: share-link path already mapped disable-sni; keep it green.
	node, err := ParseURL("tuic://11111111-1111-4111-8111-111111111111:secret@t.example.com:443?sni=t.example.com&disable-sni=1#url-nosni")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, ok := node.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tls missing: %#v", node.Extra)
	}
	if got, _ := tls["disable_sni"].(bool); !got {
		t.Fatalf("url tls.disable_sni = %v, want true", tls["disable_sni"])
	}
}
