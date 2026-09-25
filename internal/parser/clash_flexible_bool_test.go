package parser

import (
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestParseClashYAMLQuotedBoolFields(t *testing.T) {
	content := `
proxies:
  - name: vmess-quoted-tls
    type: vmess
    server: v.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    tls: "true"
    skip-cert-verify: "true"
    network: ws
    ws-opts:
      path: /ws
  - name: ss-quoted-udp
    type: ss
    server: ss.example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
    udp: "true"
  - name: tuic-quoted-reduce-rtt
    type: tuic
    server: t.example.com
    port: 443
    uuid: 22222222-2222-2222-2222-222222222222
    password: secret
    congestion-controller: bbr
    reduce-rtt: "true"
    skip-cert-verify: "false"
`

	nodes, err := ParseClashYAML(content)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("got %d nodes, want 3 (quoted bools must not drop proxies)", len(nodes))
	}

	vmess := findClashNode(t, nodes, "vmess-quoted-tls")
	tls, ok := vmess.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("vmess tls missing: %#v", vmess.Extra)
	}
	if enabled, _ := tls["enabled"].(bool); !enabled {
		t.Fatalf("vmess tls.enabled = %v, want true", tls["enabled"])
	}
	if insecure, _ := tls["insecure"].(bool); !insecure {
		t.Fatalf("vmess tls.insecure = %v, want true from skip-cert-verify: \"true\"", tls["insecure"])
	}

	ss := findClashNode(t, nodes, "ss-quoted-udp")
	if ss.Type != "shadowsocks" {
		t.Fatalf("ss type = %q, want shadowsocks", ss.Type)
	}

	tuic := findClashNode(t, nodes, "tuic-quoted-reduce-rtt")
	if got, _ := tuic.Extra["zero_rtt_handshake"].(bool); !got {
		t.Fatalf("tuic zero_rtt_handshake = %v, want true from reduce-rtt: \"true\"", tuic.Extra["zero_rtt_handshake"])
	}
	tuicTLS, ok := tuic.Extra["tls"].(map[string]interface{})
	if !ok {
		t.Fatalf("tuic tls missing: %#v", tuic.Extra)
	}
	if insecure, _ := tuicTLS["insecure"].(bool); insecure {
		t.Fatalf("tuic tls.insecure = true, want false from skip-cert-verify: \"false\"")
	}
}

func TestParseClashYAMLNativeBoolsStillWork(t *testing.T) {
	content := `
proxies:
  - name: vmess-native-tls
    type: vmess
    server: v.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    alterId: 0
    cipher: auto
    tls: true
    skip-cert-verify: false
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
		t.Fatalf("tls missing")
	}
	if enabled, _ := tls["enabled"].(bool); !enabled {
		t.Fatalf("tls.enabled = %v, want true", tls["enabled"])
	}
	if insecure, _ := tls["insecure"].(bool); insecure {
		t.Fatalf("tls.insecure = true, want false")
	}
}

func TestParseClashYAMLQuotedUDPDoesNotDropProxy(t *testing.T) {
	// Regression: udp is unused after decode, but quoted udp previously
	// failed Decode and silently skipped the entire proxy.
	cases := []string{`"true"`, `"false"`, `1`, `0`, `yes`, `off`}
	for _, udp := range cases {
		content := `
proxies:
  - name: ss-udp
    type: ss
    server: ss.example.com
    port: 8388
    cipher: aes-128-gcm
    password: secret
    udp: ` + udp + `
`
		nodes, err := ParseClashYAML(content)
		if err != nil {
			t.Fatalf("udp %s: %v", udp, err)
		}
		if len(nodes) != 1 {
			t.Fatalf("udp %s: got %d nodes, want 1", udp, len(nodes))
		}
	}
}

func findClashNode(t *testing.T, nodes []storage.Node, tag string) storage.Node {
	t.Helper()
	for _, n := range nodes {
		if n.Tag == tag {
			return n
		}
	}
	t.Fatalf("node %q not found in %#v", tag, nodeTags(nodes))
	return storage.Node{}
}

func nodeTags(nodes []storage.Node) []string {
	tags := make([]string, len(nodes))
	for i, n := range nodes {
		tags[i] = n.Tag
	}
	return tags
}
