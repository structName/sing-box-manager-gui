package builder

import (
	"reflect"
	"testing"
)

func TestParseHostsFileContentSkipsLoopbackMulticastAndLocalhost(t *testing.T) {
	content := `
# comment
127.0.0.1 localhost
127.0.0.1 my-hostname
127.0.1.1 ubuntu-box
::1 localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
10.0.0.5 intranet.example
192.168.1.1 router.lan # home gateway
2001:db8::1 ipv6.example
0.0.0.0 ignored-unspecified
fe80::1 link-local.example
8.8.8.8 dns.google broadcasthost
`
	got := parseHostsFileContent(content)
	want := map[string][]string{
		"intranet.example": {"10.0.0.5"},
		"router.lan":       {"192.168.1.1"},
		"ipv6.example":     {"2001:db8::1"},
		"dns.google":       {"8.8.8.8"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseHostsFileContent() = %#v, want %#v", got, want)
	}
}

func TestParseHostsFileContentKeepsLANMappings(t *testing.T) {
	content := "192.168.50.10 nas.local nas\nfd12::10 ula.local\n"
	got := parseHostsFileContent(content)
	want := map[string][]string{
		"nas.local": {"192.168.50.10"},
		"nas":       {"192.168.50.10"},
		"ula.local": {"fd12::10"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseHostsFileContent() = %#v, want %#v", got, want)
	}
}

func TestShouldSkipSystemHostIP(t *testing.T) {
	cases := []struct {
		ip   string
		skip bool
	}{
		{"127.0.0.1", true},
		{"127.0.1.1", true},
		{"::1", true},
		{"0.0.0.0", true},
		{"::", true},
		{"ff02::1", true},
		{"fe80::1", true},
		{"10.0.0.1", false},
		{"192.168.0.1", false},
		{"8.8.8.8", false},
		{"2001:db8::1", false},
		{"not-an-ip", true},
	}
	for _, tc := range cases {
		if got := shouldSkipSystemHostIP(tc.ip); got != tc.skip {
			t.Fatalf("shouldSkipSystemHostIP(%q)=%v, want %v", tc.ip, got, tc.skip)
		}
	}
}

func TestParseSystemHostsSkipsMachineHostnameLoopback(t *testing.T) {
	hosts := ParseSystemHosts()
	for domain, ips := range hosts {
		for _, ip := range ips {
			if shouldSkipSystemHostIP(ip) {
				t.Fatalf("ParseSystemHosts() retained skippable IP %s for domain %s", ip, domain)
			}
			if shouldSkipSystemHostDomain(domain) {
				t.Fatalf("ParseSystemHosts() retained skippable domain %s -> %s", domain, ip)
			}
		}
	}
}
