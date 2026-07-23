package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckConfigContentUsesTemporaryCandidate(t *testing.T) {
	dataDir := t.TempDir()
	binPath := filepath.Join(dataDir, "sing-box")
	configPath := filepath.Join(dataDir, "generated", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"current":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	script := `#!/bin/sh
if [ "$1" != "check" ] || [ "$2" != "-c" ]; then
  exit 2
fi
if grep -q 'future-protocol' "$3"; then
  printf 'FATAL initialize outbound[2]: unsupported outbound type\n' >&2
  exit 1
fi
exit 0
`
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	pm := &ProcessManager{
		singboxPath: binPath,
		configPath:  configPath,
		dataDir:     dataDir,
	}
	err := pm.CheckConfigContent(`{"outbounds":[{"type":"future-protocol"}]}`)
	if err == nil || !strings.Contains(err.Error(), "outbound[2]") {
		t.Fatalf("CheckConfigContent() error = %v, want outbound index", err)
	}
	var indexed interface {
		OutboundIndex() (int, bool)
	}
	if !errors.As(err, &indexed) {
		t.Fatalf("CheckConfigContent() error type = %T, want indexed config error", err)
	}
	if index, ok := indexed.OutboundIndex(); !ok || index != 2 {
		t.Fatalf("OutboundIndex() = %d, %v, want 2, true", index, ok)
	}

	current, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != `{"current":true}` {
		t.Fatalf("live config was replaced: %s", current)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(configPath), ".sbm-check-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary check configs not removed: %v", matches)
	}

	if err := pm.CheckConfigContent(`{"outbounds":[{"type":"direct"}]}`); err != nil {
		t.Fatalf("valid candidate rejected: %v", err)
	}
}
