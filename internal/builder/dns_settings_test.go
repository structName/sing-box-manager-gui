package builder

import (
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

func TestParseSettingsDNSServerSchemes(t *testing.T) {
	cases := []struct {
		raw        string
		wantType   string
		wantServer string
		wantPath   string
		wantPort   int
	}{
		{"https://1.1.1.1/dns-query", "https", "1.1.1.1", "", 0},
		{"https://dns.alidns.com/dns-query", "https", "dns.alidns.com", "", 0},
		{"https://dns.example/custom", "https", "dns.example", "/custom", 0},
		{"h3://1.1.1.1/dns-query", "h3", "1.1.1.1", "", 0},
		{"tls://1.1.1.1", "tls", "1.1.1.1", "", 0},
		{"quic://1.1.1.1:853", "quic", "1.1.1.1", "", 853},
		{"tcp://8.8.8.8", "tcp", "8.8.8.8", "", 0},
		{"udp://223.5.5.5", "udp", "223.5.5.5", "", 0},
		{"8.8.8.8", "udp", "8.8.8.8", "", 0},
		{"local", "local", "", "", 0},
		{"https://[2606:4700:4700::1111]/dns-query", "https", "2606:4700:4700::1111", "", 0},
	}
	for _, tc := range cases {
		got, err := parseSettingsDNSServer("t", tc.raw, "")
		if err != nil {
			t.Fatalf("parseSettingsDNSServer(%q) error = %v", tc.raw, err)
		}
		if got.Type != tc.wantType || got.Server != tc.wantServer || got.Path != tc.wantPath || got.ServerPort != tc.wantPort {
			t.Fatalf("parseSettingsDNSServer(%q) = type=%q server=%q path=%q port=%d, want type=%q server=%q path=%q port=%d",
				tc.raw, got.Type, got.Server, got.Path, got.ServerPort, tc.wantType, tc.wantServer, tc.wantPath, tc.wantPort)
		}
	}
}

func TestBuildDNSHonorsProxyAndDirectSettings(t *testing.T) {
	settings := storage.DefaultSettings()
	settings.ProxyDNS = "https://1.0.0.1/dns-query"
	settings.DirectDNS = "udp://223.5.5.5"
	settings.FakeIPEnabled = false
	settings.Hosts = nil

	b := NewConfigBuilder(settings, nil, nil, nil, nil)
	dns := b.buildDNS()
	if dns == nil {
		t.Fatal("buildDNS() returned nil")
	}

	byTag := map[string]DNSServer{}
	for _, srv := range dns.Servers {
		byTag[srv.Tag] = srv
	}
	proxy, ok := byTag["dns_proxy"]
	if !ok {
		t.Fatalf("dns_proxy missing, servers=%#v", dns.Servers)
	}
	if proxy.Type != "https" || proxy.Server != "1.0.0.1" || proxy.Detour != "Proxy" {
		t.Fatalf("dns_proxy = %#v, want https/1.0.0.1 detour Proxy", proxy)
	}
	direct, ok := byTag["dns_direct"]
	if !ok {
		t.Fatalf("dns_direct missing, servers=%#v", dns.Servers)
	}
	if direct.Type != "udp" || direct.Server != "223.5.5.5" {
		t.Fatalf("dns_direct = %#v, want udp/223.5.5.5", direct)
	}
	if _, ok := byTag["dns_local"]; ok {
		t.Fatalf("dns_local should not be present when direct DNS is an IP, got %#v", dns.Servers)
	}
}

func TestBuildDNSAddsLocalResolverForHostnameDirectDNS(t *testing.T) {
	settings := storage.DefaultSettings()
	// Defaults already use hostname DirectDNS; keep explicit for clarity.
	settings.ProxyDNS = "https://1.1.1.1/dns-query"
	settings.DirectDNS = "https://dns.alidns.com/dns-query"
	settings.FakeIPEnabled = false
	settings.Hosts = nil

	b := NewConfigBuilder(settings, nil, nil, nil, nil)
	dns := b.buildDNS()
	byTag := map[string]DNSServer{}
	for _, srv := range dns.Servers {
		byTag[srv.Tag] = srv
	}
	if _, ok := byTag["dns_local"]; !ok {
		t.Fatalf("expected dns_local bootstrap for hostname DirectDNS, servers=%#v", dns.Servers)
	}
	direct := byTag["dns_direct"]
	if direct.Type != "https" || direct.Server != "dns.alidns.com" || direct.DomainResolver != "dns_local" {
		t.Fatalf("dns_direct = %#v, want https/dns.alidns.com domain_resolver=dns_local", direct)
	}
	proxy := byTag["dns_proxy"]
	if proxy.Server != "1.1.1.1" || proxy.DomainResolver != "" {
		t.Fatalf("dns_proxy = %#v, want IP without domain_resolver", proxy)
	}
}

func TestBuildDNSProxyHostnameUsesDirectResolver(t *testing.T) {
	settings := storage.DefaultSettings()
	settings.ProxyDNS = "https://dns.google/dns-query"
	settings.DirectDNS = "8.8.8.8"
	settings.FakeIPEnabled = false
	settings.Hosts = nil

	b := NewConfigBuilder(settings, nil, nil, nil, nil)
	dns := b.buildDNS()
	byTag := map[string]DNSServer{}
	for _, srv := range dns.Servers {
		byTag[srv.Tag] = srv
	}
	proxy := byTag["dns_proxy"]
	if proxy.Server != "dns.google" || proxy.DomainResolver != "dns_direct" {
		t.Fatalf("dns_proxy = %#v, want dns.google with domain_resolver=dns_direct", proxy)
	}
	if _, ok := byTag["dns_local"]; ok {
		t.Fatalf("dns_local not needed when DirectDNS is IP, got %#v", dns.Servers)
	}
}

func TestDnsServerNeedsDomainResolver(t *testing.T) {
	if dnsServerNeedsDomainResolver(DNSServer{Type: "https", Server: "1.1.1.1"}) {
		t.Fatal("IP should not need domain_resolver")
	}
	if !dnsServerNeedsDomainResolver(DNSServer{Type: "https", Server: "dns.google"}) {
		t.Fatal("hostname should need domain_resolver")
	}
	if dnsServerNeedsDomainResolver(DNSServer{Type: "local"}) {
		t.Fatal("local type should not need domain_resolver")
	}
}
