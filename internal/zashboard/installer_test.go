package zashboard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUsesEmbeddedPath(t *testing.T) {
	tests := []struct {
		name   string
		uiPath string
		want   bool
	}{
		{name: "empty", uiPath: "", want: true},
		{name: "default", uiPath: "zashboard", want: true},
		{name: "default with spaces", uiPath: " zashboard ", want: true},
		{name: "custom", uiPath: "custom-ui", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := UsesEmbeddedPath(test.uiPath); got != test.want {
				t.Fatalf("UsesEmbeddedPath(%q) = %v, want %v", test.uiPath, got, test.want)
			}
		})
	}
}

func TestEnsureEmbeddedUIExtractsDefaultAssets(t *testing.T) {
	dataDir := t.TempDir()

	if err := EnsureEmbeddedUI(dataDir, DefaultUIPath); err != nil {
		t.Fatalf("EnsureEmbeddedUI returned error: %v", err)
	}

	indexPath := filepath.Join(dataDir, DefaultUIPath, "index.html")
	if stat, err := os.Stat(indexPath); err != nil || stat.IsDir() {
		t.Fatalf("index.html not found after extraction: %v", err)
	}

	if err := EnsureEmbeddedUI(dataDir, DefaultUIPath); err != nil {
		t.Fatalf("EnsureEmbeddedUI second run returned error: %v", err)
	}
}

func TestEnsureEmbeddedUIReplacesLegacyFiles(t *testing.T) {
	dataDir := t.TempDir()
	targetDir := filepath.Join(dataDir, DefaultUIPath)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	legacyContent := []byte("legacy-ui")
	if err := os.WriteFile(filepath.Join(targetDir, "index.html"), legacyContent, 0o644); err != nil {
		t.Fatalf("WriteFile(index.html) error = %v", err)
	}

	if err := EnsureEmbeddedUI(dataDir, DefaultUIPath); err != nil {
		t.Fatalf("EnsureEmbeddedUI() error = %v", err)
	}

	indexPath := filepath.Join(targetDir, "index.html")
	currentContent, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("ReadFile(index.html) error = %v", err)
	}
	if string(currentContent) == string(legacyContent) {
		t.Fatalf("index.html remains legacy content")
	}

	markerPath := filepath.Join(targetDir, markerFile)
	if stat, err := os.Stat(markerPath); err != nil || stat.IsDir() {
		t.Fatalf("marker file not found: %v", err)
	}
}
