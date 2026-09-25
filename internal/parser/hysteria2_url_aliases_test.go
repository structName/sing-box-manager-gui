package parser

import "testing"

func TestHysteria2URLPortsAlias(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@hy2.example.com:443?sni=hy2.example.com&ports=20000-50000&hop-interval=30#ports-alias")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if got, _ := node.Extra["ports"].(string); got != "20000-50000" {
		t.Fatalf("ports = %v, want 20000-50000 from ports= alias", node.Extra["ports"])
	}
	if got, _ := node.Extra["hop_interval"].(string); got != "30" {
		t.Fatalf("hop_interval = %v, want 30", node.Extra["hop_interval"])
	}
}

func TestHysteria2URLMportPrefersOverPorts(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@hy2.example.com:443?mport=10000-20000&ports=20000-50000#pref")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if got, _ := node.Extra["ports"].(string); got != "10000-20000" {
		t.Fatalf("ports = %v, want mport value 10000-20000", node.Extra["ports"])
	}
}

func TestHysteria2URLObfsPasswordCamelCase(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@hy2.example.com:443?sni=hy2.example.com&obfs=salamander&obfsPassword=obfs-secret#obfs-camel")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	obfs, ok := node.Extra["obfs"].(map[string]interface{})
	if !ok {
		t.Fatalf("obfs missing: %#v", node.Extra)
	}
	if obfs["type"] != "salamander" {
		t.Fatalf("obfs.type = %v, want salamander", obfs["type"])
	}
	if obfs["password"] != "obfs-secret" {
		t.Fatalf("obfs.password = %v, want obfs-secret from obfsPassword=", obfs["password"])
	}
}

func TestHysteria2URLObfsPasswordHyphenPrefers(t *testing.T) {
	node, err := ParseURL("hysteria2://secret@hy2.example.com:443?obfs=salamander&obfs-password=hyphen-secret&obfsPassword=camel-secret#pref")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	obfs, _ := node.Extra["obfs"].(map[string]interface{})
	if obfs["password"] != "hyphen-secret" {
		t.Fatalf("obfs.password = %v, want hyphen-secret", obfs["password"])
	}
}
