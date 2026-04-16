package parser

import (
	"encoding/json"
	"testing"

	"github.com/xiaobei/singbox-manager/internal/builder"
	"github.com/xiaobei/singbox-manager/internal/storage"
)

// TestSSRFullPipeline_ClashYAMLParsing_to_SingboxConfig verifies the complete
// pipeline: Clash YAML parsing -> Node creation -> sing-box config generation
// for ShadowsocksR nodes, using a real subscription YAML snippet.
func TestSSRFullPipeline_ClashYAMLParsing_to_SingboxConfig(t *testing.T) {
	const clashYAML = `port: 7890
socks-port: 7891
proxies:
  - {name: 🇨🇳 台湾・01, server: 6kusjo.ps6ywnxy.com, port: 1070, type: ssr, cipher: chacha20-ietf, password: bxsnucrgk6hfish, protocol: auth_aes128_sha1, obfs: plain, protocol-param: "346320:HEQHnc", obfs-param: "11061346320.microsoft.com", tfo: false}
  - {name: 🇨🇳 台湾・02, server: 6kusjo.ps6ywnxy.com, port: 1071, type: ssr, cipher: chacha20-ietf, password: bxsnucrgk6hfish, protocol: auth_aes128_sha1, obfs: plain, protocol-param: "346320:HEQHnc", obfs-param: "11061346320.microsoft.com", tfo: false}
  - {name: 🇨🇳 台湾・03, server: 6kusjo.ps6ywnxy.com, port: 1072, type: ssr, cipher: chacha20-ietf, password: bxsnucrgk6hfish, protocol: auth_aes128_sha1, obfs: plain, protocol-param: "346320:HEQHnc", obfs-param: "11061346320.microsoft.com", tfo: false}
  - {name: 🇨🇳 台湾・04, server: 6kusjo.ps6ywnxy.com, port: 1073, type: ssr, cipher: chacha20-ietf, password: bxsnucrgk6hfish, protocol: auth_aes128_sha1, obfs: plain, protocol-param: "346320:HEQHnc", obfs-param: "11061346320.microsoft.com", tfo: false}
`

	// =========================================================================
	// Phase 1: Clash YAML Parsing
	// =========================================================================
	t.Run("Phase1_ClashYAMLParsing", func(t *testing.T) {
		nodes, err := ParseClashYAML(clashYAML)
		if err != nil {
			t.Fatalf("ParseClashYAML failed: %v", err)
		}

		// Must return exactly 4 nodes
		if len(nodes) != 4 {
			t.Fatalf("expected 4 nodes, got %d", len(nodes))
		}

		// Verify each node has correct basic fields
		expectedPorts := []int{1070, 1071, 1072, 1073}
		expectedNames := []string{
			"🇨🇳 台湾・01",
			"🇨🇳 台湾・02",
			"🇨🇳 台湾・03",
			"🇨🇳 台湾・04",
		}

		for i, node := range nodes {
			t.Run(expectedNames[i], func(t *testing.T) {
				// Type must be "shadowsocksr"
				if node.Type != "shadowsocksr" {
					t.Errorf("node[%d] type: expected 'shadowsocksr', got %q", i, node.Type)
				}

				// Tag
				if node.Tag != expectedNames[i] {
					t.Errorf("node[%d] tag: expected %q, got %q", i, expectedNames[i], node.Tag)
				}

				// Server
				if node.Server != "6kusjo.ps6ywnxy.com" {
					t.Errorf("node[%d] server: expected '6kusjo.ps6ywnxy.com', got %q", i, node.Server)
				}

				// Port
				if node.ServerPort != expectedPorts[i] {
					t.Errorf("node[%d] port: expected %d, got %d", i, expectedPorts[i], node.ServerPort)
				}

				// Extra fields validation
				extraChecks := map[string]string{
					"method":         "chacha20-ietf",
					"password":       "bxsnucrgk6hfish",
					"protocol":       "auth_aes128_sha1",
					"protocol_param": "346320:HEQHnc",
					"obfs":           "plain",
					"obfs_param":     "11061346320.microsoft.com",
				}

				for key, expected := range extraChecks {
					val, ok := node.Extra[key]
					if !ok {
						t.Errorf("node[%d] Extra missing key %q", i, key)
						continue
					}
					strVal, ok := val.(string)
					if !ok {
						t.Errorf("node[%d] Extra[%q]: expected string, got %T", i, key, val)
						continue
					}
					if strVal != expected {
						t.Errorf("node[%d] Extra[%q]: expected %q, got %q", i, key, expected, strVal)
					}
				}

				// Country detection for Taiwan
				if node.Country != "TW" && node.Country != "CN" {
					t.Logf("node[%d] country: got %q (country detection may vary based on name parsing)", i, node.Country)
				}
			})
		}
	})

	// =========================================================================
	// Phase 2: Extra field keys match sing-box ShadowsocksR option struct
	// =========================================================================
	t.Run("Phase2_ExtraFieldKeysMatchSingbox", func(t *testing.T) {
		nodes, err := ParseClashYAML(clashYAML)
		if err != nil {
			t.Fatalf("ParseClashYAML failed: %v", err)
		}

		// sing-box's ShadowsocksROutboundOptions expects these JSON fields:
		//   method, password, obfs, obfs_param, protocol, protocol_param
		// The nodeToOutbound function copies Extra fields directly into the
		// outbound map, so the key names must match exactly.
		requiredSingboxFields := []string{
			"method",
			"password",
			"protocol",
			"protocol_param",
			"obfs",
			"obfs_param",
		}

		node := nodes[0]
		for _, field := range requiredSingboxFields {
			if _, ok := node.Extra[field]; !ok {
				t.Errorf("Extra missing sing-box required field %q", field)
			}
		}

		// Ensure no unexpected field name variants that would be ignored by sing-box
		unexpectedKeys := map[string]string{
			"cipher":        "should be 'method'",
			"obfsParam":     "should be 'obfs_param' (snake_case)",
			"protocolParam": "should be 'protocol_param' (snake_case)",
			"obfs-param":    "should be 'obfs_param' (underscore, not hyphen)",
			"protocol-param": "should be 'protocol_param' (underscore, not hyphen)",
		}
		for badKey, reason := range unexpectedKeys {
			if _, ok := node.Extra[badKey]; ok {
				t.Errorf("Extra contains unexpected key %q (%s)", badKey, reason)
			}
		}
	})

	// =========================================================================
	// Phase 3: Config Builder - nodeToOutbound via Build()
	// =========================================================================
	t.Run("Phase3_ConfigBuilder", func(t *testing.T) {
		nodes, err := ParseClashYAML(clashYAML)
		if err != nil {
			t.Fatalf("ParseClashYAML failed: %v", err)
		}

		settings := storage.DefaultSettings()
		cb := builder.NewConfigBuilder(settings, nodes, nil, nil, nil)

		jsonStr, err := cb.BuildJSON()
		if err != nil {
			t.Fatalf("BuildJSON failed: %v", err)
		}

		// Parse the generated JSON
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
			t.Fatalf("failed to parse generated config JSON: %v", err)
		}

		// Find SSR outbounds in the config
		outboundsRaw, ok := config["outbounds"]
		if !ok {
			t.Fatal("config missing 'outbounds' key")
		}
		outbounds, ok := outboundsRaw.([]interface{})
		if !ok {
			t.Fatal("outbounds is not an array")
		}

		// Collect all shadowsocksr outbounds
		var ssrOutbounds []map[string]interface{}
		for _, ob := range outbounds {
			obMap, ok := ob.(map[string]interface{})
			if !ok {
				continue
			}
			if obMap["type"] == "shadowsocksr" {
				ssrOutbounds = append(ssrOutbounds, obMap)
			}
		}

		if len(ssrOutbounds) != 4 {
			t.Fatalf("expected 4 shadowsocksr outbounds, got %d", len(ssrOutbounds))
		}

		// Verify the first SSR outbound has all required sing-box fields
		ob := ssrOutbounds[0]

		checks := map[string]interface{}{
			"type":           "shadowsocksr",
			"server":         "6kusjo.ps6ywnxy.com",
			"server_port":    float64(1070), // JSON numbers decode as float64
			"method":         "chacha20-ietf",
			"password":       "bxsnucrgk6hfish",
			"protocol":       "auth_aes128_sha1",
			"protocol_param": "346320:HEQHnc",
			"obfs":           "plain",
			"obfs_param":     "11061346320.microsoft.com",
		}

		for key, expected := range checks {
			actual, ok := ob[key]
			if !ok {
				t.Errorf("outbound missing field %q", key)
				continue
			}
			if actual != expected {
				t.Errorf("outbound[%q]: expected %v (%T), got %v (%T)", key, expected, expected, actual, actual)
			}
		}

		// Verify tag is present
		if tag, ok := ob["tag"]; !ok || tag == "" {
			t.Errorf("outbound missing or empty 'tag'")
		}

		t.Logf("Generated config has %d total outbounds, %d are SSR", len(outbounds), len(ssrOutbounds))
	})

	// =========================================================================
	// Phase 4: Round-trip JSON serialization fidelity
	// =========================================================================
	t.Run("Phase4_JSONRoundTrip", func(t *testing.T) {
		nodes, err := ParseClashYAML(clashYAML)
		if err != nil {
			t.Fatalf("ParseClashYAML failed: %v", err)
		}

		settings := storage.DefaultSettings()
		cb := builder.NewConfigBuilder(settings, nodes, nil, nil, nil)

		jsonStr, err := cb.BuildJSON()
		if err != nil {
			t.Fatalf("BuildJSON failed: %v", err)
		}

		// Parse and re-marshal to verify JSON is valid and well-formed
		var config interface{}
		if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
			t.Fatalf("generated JSON is invalid: %v", err)
		}

		roundTripped, err := json.Marshal(config)
		if err != nil {
			t.Fatalf("re-marshal failed: %v", err)
		}

		if len(roundTripped) == 0 {
			t.Fatal("round-tripped JSON is empty")
		}

		t.Logf("Config JSON size: %d bytes", len(jsonStr))
	})

	// =========================================================================
	// Phase 5: ParseSubscriptionContent path (end-to-end via subscription API)
	// =========================================================================
	t.Run("Phase5_ParseSubscriptionContent", func(t *testing.T) {
		nodes, err := ParseSubscriptionContent(clashYAML)
		if err != nil {
			t.Fatalf("ParseSubscriptionContent failed: %v", err)
		}

		if len(nodes) != 4 {
			t.Fatalf("expected 4 nodes via ParseSubscriptionContent, got %d", len(nodes))
		}

		for i, node := range nodes {
			if node.Type != "shadowsocksr" {
				t.Errorf("node[%d] type: expected 'shadowsocksr', got %q", i, node.Type)
			}
		}
	})

	// =========================================================================
	// Phase 6: Verify all 4 nodes produce distinct outbound ports
	// =========================================================================
	t.Run("Phase6_DistinctPorts", func(t *testing.T) {
		nodes, err := ParseClashYAML(clashYAML)
		if err != nil {
			t.Fatalf("ParseClashYAML failed: %v", err)
		}

		settings := storage.DefaultSettings()
		cb := builder.NewConfigBuilder(settings, nodes, nil, nil, nil)

		jsonStr, err := cb.BuildJSON()
		if err != nil {
			t.Fatalf("BuildJSON failed: %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
			t.Fatalf("parse config: %v", err)
		}

		outbounds := config["outbounds"].([]interface{})
		portSet := make(map[float64]string)
		for _, ob := range outbounds {
			obMap := ob.(map[string]interface{})
			if obMap["type"] != "shadowsocksr" {
				continue
			}
			port := obMap["server_port"].(float64)
			tag := obMap["tag"].(string)
			if existing, dup := portSet[port]; dup {
				t.Errorf("duplicate port %.0f: %q and %q", port, existing, tag)
			}
			portSet[port] = tag
		}

		if len(portSet) != 4 {
			t.Errorf("expected 4 distinct ports, got %d", len(portSet))
		}
	})
}
