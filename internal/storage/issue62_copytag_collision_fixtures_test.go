package storage

import "testing"

// Issue #62 regression fixtures for #48 (CopyTag collision).
// Contracts match #64 (fix/chain-copytag-and-autoapply-races):
// hop-indexed GenerateChainNodeCopyTag + DisambiguateCopyTag residual uniqueness.
// (#49 / health false-positive portion of #62 is covered by #66.)

func TestIssue62_CopyTagCollisionFixtures(t *testing.T) {
	t.Parallel()

	t.Run("repeated_hop_distinct_tags", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			chain string
			tag   string
			hop   int
			want  string
		}{
			{"aba", "A", 0, "aba-0-A"},
			{"aba", "B", 1, "aba-1-B"},
			{"aba", "A", 2, "aba-2-A"},
			{"shared", "relay", 0, "shared-0-relay"},
			{"shared", "relay", 3, "shared-3-relay"},
		}
		for _, tc := range cases {
			got := GenerateChainNodeCopyTag(tc.chain, tc.tag, tc.hop)
			if got != tc.want {
				t.Fatalf("GenerateChainNodeCopyTag(%q,%q,%d)=%q want %q", tc.chain, tc.tag, tc.hop, got, tc.want)
			}
		}
		if GenerateChainNodeCopyTag("aba", "A", 0) == GenerateChainNodeCopyTag("aba", "A", 2) {
			t.Fatal("#48: repeated hop A must not share CopyTag")
		}
	})

	t.Run("disambiguate_residual_collisions", func(t *testing.T) {
		t.Parallel()
		used := map[string]bool{"chain-0-A": true, "chain-0-A#2": true}
		got := DisambiguateCopyTag("chain-0-A", used)
		if got != "chain-0-A#3" {
			t.Fatalf("DisambiguateCopyTag = %q, want chain-0-A#3", got)
		}
		if DisambiguateCopyTag("fresh", used) != "fresh" {
			t.Fatalf("unused base should pass through")
		}
		if DisambiguateCopyTag("", used) != "" {
			t.Fatalf("empty base should pass through")
		}
	})

	t.Run("country_and_auto_candidate_tags_include_hop", func(t *testing.T) {
		t.Parallel()
		country := GenerateChainCountryCandidateCopyTag("c", "country:JP", "n1", 1)
		if want := "c-1-country:JP-n1"; country != want {
			t.Fatalf("country candidate = %q, want %q", country, want)
		}
		auto := GenerateChainAutoCandidateCopyTag("c", "n1", 2)
		if want := "c-2-" + ChainAutoNodeTag + "-n1"; auto != want {
			t.Fatalf("auto candidate = %q, want %q", auto, want)
		}
	})
}
