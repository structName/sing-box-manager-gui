package parser

import "testing"

// Clash Meta and many converters emit hysteria2 proxies with only
// obfs-password (salamander implied). Requiring both obfs + obfs-password
// silently dropped the whole obfs block, so imported nodes dialed without
// salamander and failed against servers that require it.
func TestParseClashYAMLHysteria2ObfsPasswordDefaultsType(t *testing.T) {
	yaml := `
proxies:
  - name: hy2-obfs-pw-only
    type: hysteria2
    server: hy2.example.com
    port: 443
    password: secret
    obfs-password: only-pw
  - name: hy2-obfs-both
    type: hysteria2
    server: hy2.example.com
    port: 443
    password: secret
    obfs: salamander
    obfs-password: both-pw
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("len(nodes) = %d, want 2", len(nodes))
	}

	obfs, ok := nodes[0].Extra["obfs"].(map[string]interface{})
	if !ok {
		t.Fatalf("hy2-obfs-pw-only missing obfs: %#v", nodes[0].Extra)
	}
	if obfs["type"] != "salamander" {
		t.Fatalf("obfs.type = %v, want salamander default", obfs["type"])
	}
	if obfs["password"] != "only-pw" {
		t.Fatalf("obfs.password = %v, want only-pw", obfs["password"])
	}

	obfs2, ok := nodes[1].Extra["obfs"].(map[string]interface{})
	if !ok {
		t.Fatalf("hy2-obfs-both missing obfs: %#v", nodes[1].Extra)
	}
	if obfs2["type"] != "salamander" {
		t.Fatalf("obfs.type = %v, want salamander", obfs2["type"])
	}
	if obfs2["password"] != "both-pw" {
		t.Fatalf("obfs.password = %v, want both-pw", obfs2["password"])
	}
}

func TestParseClashYAMLHysteria2ObfsTypeWithoutPasswordSkipped(t *testing.T) {
	yaml := `
proxies:
  - name: hy2-obfs-type-only
    type: hysteria2
    server: hy2.example.com
    port: 443
    password: secret
    obfs: salamander
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	if _, has := nodes[0].Extra["obfs"]; has {
		t.Fatalf("obfs type without password should not create obfs block, got %#v", nodes[0].Extra["obfs"])
	}
}
