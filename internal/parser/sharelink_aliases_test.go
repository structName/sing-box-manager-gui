package parser

import (
	"testing"
)

func TestTrojanGRPCServiceNameSnakeCase(t *testing.T) {
	node, err := ParseURL("trojan://pwd@t.example.com:443?type=grpc&service_name=GunService&sni=t.example.com#grpc-snake")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	if transport["type"] != "grpc" {
		t.Fatalf("transport type = %v", transport["type"])
	}
	if transport["service_name"] != "GunService" {
		t.Fatalf("service_name = %v, want GunService (snake_case alias)", transport["service_name"])
	}
}

func TestVLESSGRPCServiceNameSnakeCase(t *testing.T) {
	node, err := ParseURL("vless://00000000-0000-0000-0000-000000000001@v.example.com:443?type=grpc&service_name=GunService&security=tls&sni=v.example.com#vgrpc")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	if transport["service_name"] != "GunService" {
		t.Fatalf("service_name = %v, want GunService", transport["service_name"])
	}
}

func TestTrojanPeerAsSNIAndAllowInsecure(t *testing.T) {
	node, err := ParseURL("trojan://pwd@t.example.com:443?peer=cdn.example.com&allow_insecure=1#peer")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "cdn.example.com" {
		t.Fatalf("server_name = %v, want peer value cdn.example.com", tls["server_name"])
	}
	if tls["insecure"] != true {
		t.Fatalf("insecure = %v, want true from allow_insecure", tls["insecure"])
	}
}

func TestTrojanWSPeerAsHost(t *testing.T) {
	node, err := ParseURL("trojan://pwd@t.example.com:443?type=ws&path=/ws&peer=cdn.example.com#ws-peer")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	headers, _ := transport["headers"].(map[string]string)
	if headers["Host"] != "cdn.example.com" {
		t.Fatalf("WS Host = %v, want peer cdn.example.com", headers["Host"])
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "cdn.example.com" {
		t.Fatalf("server_name = %v, want peer fallback", tls["server_name"])
	}
}

func TestTrojanServiceNamePrefersCamelCase(t *testing.T) {
	node, err := ParseURL("trojan://pwd@t.example.com:443?type=grpc&serviceName=CamelSvc&service_name=SnakeSvc&sni=t.example.com#pref")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	transport, _ := node.Extra["transport"].(map[string]interface{})
	if transport["service_name"] != "CamelSvc" {
		t.Fatalf("prefer serviceName, got %v", transport["service_name"])
	}
}

func TestVLESSPeerAndAllowInsecure(t *testing.T) {
	node, err := ParseURL("vless://00000000-0000-0000-0000-000000000001@v.example.com:443?security=tls&peer=cdn.example.com&allow_insecure=true#vp")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	if tls["server_name"] != "cdn.example.com" {
		t.Fatalf("server_name = %v", tls["server_name"])
	}
	if tls["insecure"] != true {
		t.Fatalf("insecure = %v", tls["insecure"])
	}
}
