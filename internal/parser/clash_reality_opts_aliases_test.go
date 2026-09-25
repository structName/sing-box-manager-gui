package parser

import "testing"

func TestClashRealityOptsKeyAliases(t *testing.T) {
	cases := []struct {
		name string
		opts string
		pbk  string
		sid  string
	}{
		{
			name: "canonical kebab-case",
			opts: `
      public-key: KebabPublicKeyValue0123456789ab
      short-id: "ef01"
`,
			pbk: "KebabPublicKeyValue0123456789ab",
			sid: "ef01",
		},
		{
			name: "camelCase publicKey/shortId",
			opts: `
      publicKey: CamelCasePublicKeyValue012345
      shortId: abcd1234
`,
			pbk: "CamelCasePublicKeyValue012345",
			sid: "abcd1234",
		},
		{
			name: "snake_case public_key/short_id",
			opts: `
      public_key: SnakePublicKeyValue0123456789abc
      short_id: "9900"
`,
			pbk: "SnakePublicKeyValue0123456789abc",
			sid: "9900",
		},
		{
			name: "pbk/sid short aliases",
			opts: `
      pbk: PbkAliasPublicKeyValue012345678
      sid: aa01
`,
			pbk: "PbkAliasPublicKeyValue012345678",
			sid: "aa01",
		},
		{
			name: "prefer kebab when mixed with camel",
			opts: `
      public-key: PreferKebabPublicKeyValue01234
      publicKey: IgnoredCamelPublicKeyValue012
      short-id: "1111"
      shortId: "2222"
`,
			pbk: "PreferKebabPublicKeyValue01234",
			sid: "1111",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			yaml := "proxies:\n  - name: " + tc.name + "\n    type: vless\n    server: 1.2.3.4\n    port: 443\n    uuid: 11111111-1111-1111-1111-111111111111\n    tls: true\n    servername: www.example.com\n    reality-opts:\n" + tc.opts
			nodes, err := ParseClashYAML(yaml)
			if err != nil {
				t.Fatalf("ParseClashYAML: %v", err)
			}
			if len(nodes) != 1 {
				t.Fatalf("nodes = %d, want 1", len(nodes))
			}
			tls, _ := nodes[0].Extra["tls"].(map[string]interface{})
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

func TestClashTrojanRealityOptsCamelCase(t *testing.T) {
	yaml := `
proxies:
  - name: trojan-reality
    type: trojan
    server: 5.6.7.8
    port: 443
    password: secret
    sni: www.example.com
    reality-opts:
      publicKey: TrojanCamelPublicKeyValue0123
      shortId: bb02
`
	nodes, err := ParseClashYAML(yaml)
	if err != nil {
		t.Fatalf("ParseClashYAML: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(nodes))
	}
	tls, _ := nodes[0].Extra["tls"].(map[string]interface{})
	reality, _ := tls["reality"].(map[string]interface{})
	if reality == nil {
		t.Fatal("reality missing")
	}
	if got, _ := reality["public_key"].(string); got != "TrojanCamelPublicKeyValue0123" {
		t.Fatalf("public_key = %q", got)
	}
	if got, _ := reality["short_id"].(string); got != "bb02" {
		t.Fatalf("short_id = %q", got)
	}
}
