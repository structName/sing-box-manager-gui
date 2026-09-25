package parser

import "testing"

func TestVLESSRealityPublicKeyShortIdAliases(t *testing.T) {
	cases := []struct {
		name string
		url  string
		pbk  string
		sid  string
	}{
		{
			name: "canonical pbk/sid",
			url:  "vless://00000000-0000-0000-0000-000000000001@v.example.com:443?security=reality&pbk=AbCdEfGh&sid=abcd&fp=chrome&sni=www.example.com#canon",
			pbk:  "AbCdEfGh",
			sid:  "abcd",
		},
		{
			name: "camelCase publicKey/shortId",
			url:  "vless://00000000-0000-0000-0000-000000000001@v.example.com:443?security=reality&publicKey=CamelPublic&shortId=01ab&fp=chrome&sni=www.example.com#camel",
			pbk:  "CamelPublic",
			sid:  "01ab",
		},
		{
			name: "kebab-case public-key/short-id",
			url:  "vless://00000000-0000-0000-0000-000000000001@v.example.com:443?security=reality&public-key=KebabPublic&short-id=ef01&fp=chrome&sni=www.example.com#kebab",
			pbk:  "KebabPublic",
			sid:  "ef01",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := ParseURL(tc.url)
			if err != nil {
				t.Fatalf("ParseURL: %v", err)
			}
			tls, _ := node.Extra["tls"].(map[string]interface{})
			if tls == nil {
				t.Fatal("tls missing")
			}
			reality, _ := tls["reality"].(map[string]interface{})
			if reality == nil {
				t.Fatal("reality missing")
			}
			if got, _ := reality["public_key"].(string); got != tc.pbk {
				t.Fatalf("public_key = %q, want %q", got, tc.pbk)
			}
			if got, _ := reality["short_id"].(string); got != tc.sid {
				t.Fatalf("short_id = %q, want %q", got, tc.sid)
			}
		})
	}
}

func TestTrojanRealityPublicKeyShortIdAliases(t *testing.T) {
	cases := []struct {
		name string
		url  string
		pbk  string
		sid  string
	}{
		{
			name: "camelCase publicKey/shortId",
			url:  "trojan://pwd@t.example.com:443?security=reality&publicKey=TrojanCamel&shortId=aa01&sni=www.example.com#t-camel",
			pbk:  "TrojanCamel",
			sid:  "aa01",
		},
		{
			name: "kebab-case public-key/short-id",
			url:  "trojan://pwd@t.example.com:443?security=reality&public-key=TrojanKebab&short-id=bb02&sni=www.example.com#t-kebab",
			pbk:  "TrojanKebab",
			sid:  "bb02",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			node, err := ParseURL(tc.url)
			if err != nil {
				t.Fatalf("ParseURL: %v", err)
			}
			tls, _ := node.Extra["tls"].(map[string]interface{})
			reality, _ := tls["reality"].(map[string]interface{})
			if reality == nil {
				t.Fatal("reality missing")
			}
			if got, _ := reality["public_key"].(string); got != tc.pbk {
				t.Fatalf("public_key = %q, want %q", got, tc.pbk)
			}
			if got, _ := reality["short_id"].(string); got != tc.sid {
				t.Fatalf("short_id = %q, want %q", got, tc.sid)
			}
		})
	}
}

func TestVLESSRealityPrefersPbkOverPublicKey(t *testing.T) {
	node, err := ParseURL("vless://00000000-0000-0000-0000-000000000001@v.example.com:443?security=reality&pbk=PreferMe&publicKey=IgnoreMe&sid=ab&shortId=cd&fp=chrome&sni=www.example.com#pref")
	if err != nil {
		t.Fatalf("ParseURL: %v", err)
	}
	tls, _ := node.Extra["tls"].(map[string]interface{})
	reality, _ := tls["reality"].(map[string]interface{})
	if got, _ := reality["public_key"].(string); got != "PreferMe" {
		t.Fatalf("public_key = %q, want PreferMe (pbk first)", got)
	}
	if got, _ := reality["short_id"].(string); got != "ab" {
		t.Fatalf("short_id = %q, want ab (sid first)", got)
	}
}
