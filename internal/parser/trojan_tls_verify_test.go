package parser

import "testing"

func TestClashTrojanRequiresTLSAndMapsSNI(t *testing.T) {
	nodes, err := ParseClashYAML(`
proxies:
  - name: with-sni
    type: trojan
    server: trojan.example.com
    port: 443
    password: secret
    sni: edge.example.com
    skip-cert-verify: true
    client-fingerprint: chrome
  - name: default-sni
    type: trojan
    server: trojan.example.com
    port: 443
    password: secret
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("len=%d", len(nodes))
	}
	tls0 := nodes[0].Extra["tls"].(map[string]interface{})
	if tls0["server_name"] != "edge.example.com" || tls0["insecure"] != true {
		t.Fatalf("with-sni tls=%#v", tls0)
	}
	utls := tls0["utls"].(map[string]interface{})
	if utls["fingerprint"] != "chrome" {
		t.Fatalf("utls=%#v", utls)
	}
	tls1 := nodes[1].Extra["tls"].(map[string]interface{})
	if tls1["server_name"] != "trojan.example.com" {
		t.Fatalf("default-sni tls=%#v", tls1)
	}
}
