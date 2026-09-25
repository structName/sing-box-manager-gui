package speedtest

import (
	"reflect"
	"testing"
)

func TestTlsAlpnListTypedJSONAndTrim(t *testing.T) {
	cases := []struct {
		name string
		raw  interface{}
		want []string
	}{
		{name: "typed", raw: []string{"h2", "http/1.1"}, want: []string{"h2", "http/1.1"}},
		{name: "json interface slice", raw: []interface{}{"h2", "http/1.1"}, want: []string{"h2", "http/1.1"}},
		{name: "trim and drop blanks typed", raw: []string{" h2 ", "", "  ", "http/1.1"}, want: []string{"h2", "http/1.1"}},
		{name: "trim and drop blanks json", raw: []interface{}{" h2 ", "", "http/1.1", 1}, want: []string{"h2", "http/1.1"}},
		{name: "comma string with spaces", raw: "h2, http/1.1, ", want: []string{"h2", "http/1.1"}},
		{name: "blank string", raw: "  , ", want: nil},
		{name: "nil", raw: nil, want: nil},
		{name: "all blank typed", raw: []string{"", "  "}, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tlsAlpnList(tc.raw)
			if tc.want == nil {
				if len(got) != 0 {
					t.Fatalf("tlsAlpnList() = %#v, want empty", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("tlsAlpnList() = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestApplyMihomoTLSAlpnOmitsWhenEmpty(t *testing.T) {
	proxy := map[string]interface{}{}
	applyMihomoTLSAlpn(proxy, map[string]interface{}{"alpn": []string{"", "  "}})
	if _, ok := proxy["alpn"]; ok {
		t.Fatalf("alpn should be omitted for blank-only list, got %#v", proxy["alpn"])
	}
}

func TestApplyMihomoClientFingerprintTypedJSONTrimBlank(t *testing.T) {
	t.Run("json map", func(t *testing.T) {
		proxy := map[string]interface{}{}
		applyMihomoClientFingerprint(proxy, map[string]interface{}{
			"utls": map[string]interface{}{"fingerprint": " firefox "},
		})
		if got := proxy["client-fingerprint"]; got != "firefox" {
			t.Fatalf("client-fingerprint = %v, want firefox", got)
		}
	})
	t.Run("typed map", func(t *testing.T) {
		proxy := map[string]interface{}{}
		applyMihomoClientFingerprint(proxy, map[string]interface{}{
			"utls": map[string]string{"fingerprint": " chrome "},
		})
		if got := proxy["client-fingerprint"]; got != "chrome" {
			t.Fatalf("client-fingerprint = %v, want chrome", got)
		}
	})
	t.Run("blank omitted", func(t *testing.T) {
		proxy := map[string]interface{}{}
		applyMihomoClientFingerprint(proxy, map[string]interface{}{
			"utls": map[string]interface{}{"fingerprint": "   "},
		})
		if _, ok := proxy["client-fingerprint"]; ok {
			t.Fatalf("client-fingerprint should be omitted for blank, got %#v", proxy["client-fingerprint"])
		}
	})
	t.Run("missing utls omitted", func(t *testing.T) {
		proxy := map[string]interface{}{}
		applyMihomoClientFingerprint(proxy, map[string]interface{}{})
		if _, ok := proxy["client-fingerprint"]; ok {
			t.Fatalf("client-fingerprint should be omitted when utls missing")
		}
	})
}
