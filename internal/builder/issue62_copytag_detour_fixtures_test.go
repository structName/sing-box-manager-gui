package builder

import (
	"testing"

	"github.com/structName/sing-box-manager-gui/internal/storage"
)

// Issue #62 / #48: builder must emit distinct per-hop CopyTags and working detours
// for repeated hops (e.g. [A,B,A]). Aligns with #64 TestBuildOutboundsRepeatedHop*.

func TestIssue62_RepeatedHopCopyTagAndDetourFixtures(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		nodes []string
	}{
		{"aba", []string{"A", "B", "A"}},
		{"aaa", []string{"A", "A", "A"}},
		{"abba", []string{"A", "B", "B", "A"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			nodes := make([]storage.Node, 0, 2)
			seen := map[string]bool{}
			for _, tag := range tc.nodes {
				if seen[tag] {
					continue
				}
				seen[tag] = true
				port := 1080
				if tag == "B" {
					port = 1081
				}
				nodes = append(nodes, storage.Node{Tag: tag, Type: "socks", Server: "127.0.0.1", ServerPort: port})
			}
			chain := storage.ProxyChain{
				ID: "id-" + tc.name, Name: tc.name + "-chain", Enabled: true, Nodes: tc.nodes,
			}
			builder := &ConfigBuilder{
				settings:    &storage.Settings{FinalOutbound: "Proxy"},
				nodes:       nodes,
				proxyChains: []storage.ProxyChain{chain},
			}
			outbounds, err := builder.buildOutbounds()
			if err != nil {
				t.Fatalf("buildOutbounds: %v", err)
			}
			byTag := map[string]Outbound{}
			for _, ob := range outbounds {
				if tag, _ := ob["tag"].(string); tag != "" {
					if _, exists := byTag[tag]; exists {
						t.Fatalf("#48: duplicate outbound tag %q", tag)
					}
					byTag[tag] = ob
				}
			}

			var prev string
			copyTags := make([]string, len(tc.nodes))
			for i, nodeTag := range tc.nodes {
				want := storage.GenerateChainNodeCopyTag(chain.Name, nodeTag, i)
				ob, ok := byTag[want]
				if !ok {
					t.Fatalf("missing hop %d copy %q", i, want)
				}
				copyTags[i] = want
				if i == 0 {
					if _, has := ob["detour"]; has {
						t.Fatalf("entry hop should have no detour, got %v", ob["detour"])
					}
				} else if got := ob["detour"]; got != prev {
					t.Fatalf("hop %d detour = %v, want %q", i, got, prev)
				}
				prev = want
			}
			uniq := map[string]bool{}
			for _, tag := range copyTags {
				if uniq[tag] {
					t.Fatalf("#48: CopyTag collision in %v", copyTags)
				}
				uniq[tag] = true
			}
		})
	}
}
