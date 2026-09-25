package storage

import "testing"

func TestGenerateChainNodeCopyTagIncludesHopIndex(t *testing.T) {
	a0 := GenerateChainNodeCopyTag("chain", "A", 0)
	a2 := GenerateChainNodeCopyTag("chain", "A", 2)
	if a0 == a2 {
		t.Fatalf("repeated hop must produce distinct CopyTags: %q == %q", a0, a2)
	}
	if want := "chain-0-A"; a0 != want {
		t.Fatalf("CopyTag = %q, want %q", a0, want)
	}
	if want := "chain-2-A"; a2 != want {
		t.Fatalf("CopyTag = %q, want %q", a2, want)
	}
}

func TestDisambiguateCopyTagOnCollision(t *testing.T) {
	used := map[string]bool{"chain-0-A": true}
	got := DisambiguateCopyTag("chain-0-A", used)
	if got != "chain-0-A#2" {
		t.Fatalf("DisambiguateCopyTag = %q, want chain-0-A#2", got)
	}
	used[got] = true
	got2 := DisambiguateCopyTag("chain-0-A", used)
	if got2 != "chain-0-A#3" {
		t.Fatalf("DisambiguateCopyTag = %q, want chain-0-A#3", got2)
	}
}
