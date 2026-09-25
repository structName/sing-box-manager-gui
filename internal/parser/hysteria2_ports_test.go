package parser

import "testing"

func TestHysteria2URLParsesPortHoppingAndBandwidth(t *testing.T) {
	p := &Hysteria2Parser{}
	node, err := p.Parse("hysteria2://secret@hy2.example.com:443?mport=20000-50000&hop-interval=30&upmbps=100&downmbps=200&alpn=h3#hop")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if got, _ := node.Extra["ports"].(string); got != "20000-50000" {
		t.Fatalf("ports = %v, want 20000-50000", node.Extra["ports"])
	}
	if got, _ := node.Extra["hop_interval"].(string); got != "30" {
		t.Fatalf("hop_interval = %v, want 30", node.Extra["hop_interval"])
	}
	if got := node.Extra["up_mbps"]; got != 100 {
		t.Fatalf("up_mbps = %v (%T), want 100", got, got)
	}
	if got := node.Extra["down_mbps"]; got != 200 {
		t.Fatalf("down_mbps = %v (%T), want 200", got, got)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	alpn, _ := tls["alpn"].([]string)
	if len(alpn) != 1 || alpn[0] != "h3" {
		t.Fatalf("alpn = %#v, want [h3]", tls["alpn"])
	}
}
