package parser

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestVLESSURLPacketEncodingXUDP(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?type=tcp&security=reality&pbk=abc&sid=01&fp=chrome&packetEncoding=xudp#pe"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if got, _ := node.Extra["packet_encoding"].(string); got != "xudp" {
		t.Fatalf("packet_encoding = %#v, want xudp", node.Extra["packet_encoding"])
	}
}

func TestVLESSURLPacketEncodingNoneDisables(t *testing.T) {
	// Generators emit packetEncoding=none to mean "disable". Omitted would
	// default to xudp in sing-box — so we must store explicit "".
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?security=tls&sni=cdn.example.com&packetEncoding=none#pe-none"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	got, ok := node.Extra["packet_encoding"].(string)
	if !ok {
		t.Fatalf("packet_encoding missing or wrong type: %#v", node.Extra["packet_encoding"])
	}
	if got != "" {
		t.Fatalf("packet_encoding = %#v, want empty string (disabled)", got)
	}
}

func TestVLESSURLPacketEncodingPacketaddrAlias(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?packet-encoding=packetaddr#pe-pa"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if got, _ := node.Extra["packet_encoding"].(string); got != "packetaddr" {
		t.Fatalf("packet_encoding = %#v, want packetaddr", node.Extra["packet_encoding"])
	}
}

func TestVLESSURLOmitsPacketEncodingWhenAbsent(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?security=tls&sni=cdn.example.com#plain"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if _, ok := node.Extra["packet_encoding"]; ok {
		t.Fatalf("packet_encoding should be omitted when absent, got %#v", node.Extra["packet_encoding"])
	}
}

func TestVLESSURLDropsUnknownPacketEncoding(t *testing.T) {
	raw := "vless://11111111-1111-1111-1111-111111111111@edge.example.com:443" +
		"?packetEncoding=bogus#bad"
	node, err := ParseURL(raw)
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if _, ok := node.Extra["packet_encoding"]; ok {
		t.Fatalf("unknown packetEncoding must be dropped, got %#v", node.Extra["packet_encoding"])
	}
}

func TestVMessURLPacketEncoding(t *testing.T) {
	cfg := map[string]interface{}{
		"v":              "2",
		"ps":             "vmess-pe",
		"add":            "edge.example.com",
		"port":           443,
		"id":             "11111111-1111-1111-1111-111111111111",
		"aid":            0,
		"scy":            "auto",
		"net":            "tcp",
		"tls":            "tls",
		"sni":            "cdn.example.com",
		"packetEncoding": "packetaddr",
	}
	rawJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	node, err := ParseURL("vmess://" + base64.StdEncoding.EncodeToString(rawJSON))
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	if got, _ := node.Extra["packet_encoding"].(string); got != "packetaddr" {
		t.Fatalf("packet_encoding = %#v, want packetaddr", node.Extra["packet_encoding"])
	}
}

func TestClashYAMLPacketEncoding(t *testing.T) {
	yaml := `
proxies:
  - name: clash-vless-pe
    type: vless
    server: edge.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    tls: true
    servername: cdn.example.com
    packet-encoding: xudp
  - name: clash-vmess-none
    type: vmess
    server: edge.example.com
    port: 443
    uuid: 22222222-2222-2222-2222-222222222222
    alterId: 0
    cipher: auto
    packet-encoding: none
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("len=%d, want 2", len(nodes))
	}
	if got, _ := nodes[0].Extra["packet_encoding"].(string); got != "xudp" {
		t.Fatalf("vless packet_encoding = %#v, want xudp", nodes[0].Extra["packet_encoding"])
	}
	got, ok := nodes[1].Extra["packet_encoding"].(string)
	if !ok {
		t.Fatalf("vmess packet_encoding missing: %#v", nodes[1].Extra["packet_encoding"])
	}
	if got != "" {
		t.Fatalf("vmess none → %#v, want empty string", got)
	}
}
