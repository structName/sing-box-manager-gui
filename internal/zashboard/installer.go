package zashboard

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// DefaultUIPath 与默认设置中的 clash_ui_path 保持一致。
	DefaultUIPath = "zashboard"
	zipRootPrefix = "dist/"
	markerFile    = ".sbm-embedded-ui.sha256"
)

// UsesEmbeddedPath 判断当前面板路径是否使用内置 zashboard 资源。
func UsesEmbeddedPath(uiPath string) bool {
	normalized := strings.TrimSpace(uiPath)
	return normalized == "" || normalized == DefaultUIPath
}

// EnsureEmbeddedUI 在默认面板路径下准备内置 zashboard 资源。
func EnsureEmbeddedUI(dataDir, uiPath string) error {
	if !UsesEmbeddedPath(uiPath) {
		return nil
	}

	targetDir := resolveTargetDir(dataDir, uiPath)
	distHash := embeddedDistHash()
	if isEmbeddedUICurrent(targetDir, distHash) {
		return nil
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("cleanup stale zashboard dir: %w", err)
	}

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("create zashboard dir: %w", err)
	}

	if err := extractEmbeddedDist(targetDir); err != nil {
		return fmt.Errorf("extract embedded zashboard dist: %w", err)
	}

	if err := os.WriteFile(filepath.Join(targetDir, markerFile), []byte(distHash), 0o644); err != nil {
		return fmt.Errorf("write zashboard marker: %w", err)
	}

	return nil
}

func embeddedDistHash() string {
	sum := sha256.Sum256(embeddedDistZip)
	return hex.EncodeToString(sum[:])
}

func isEmbeddedUICurrent(targetDir, distHash string) bool {
	indexPath := filepath.Join(targetDir, "index.html")
	if stat, err := os.Stat(indexPath); err != nil || stat.IsDir() {
		return false
	}

	markerPath := filepath.Join(targetDir, markerFile)
	markerData, err := os.ReadFile(markerPath)
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(markerData)) == distHash
}

func resolveTargetDir(dataDir, uiPath string) string {
	normalized := strings.TrimSpace(uiPath)
	if normalized == "" {
		normalized = DefaultUIPath
	}
	if filepath.IsAbs(normalized) {
		return normalized
	}
	return filepath.Join(dataDir, normalized)
}

func extractEmbeddedDist(targetDir string) error {
	reader, err := zip.NewReader(bytes.NewReader(embeddedDistZip), int64(len(embeddedDistZip)))
	if err != nil {
		return fmt.Errorf("open embedded zip: %w", err)
	}

	for _, file := range reader.File {
		if err := writeZipEntry(file, targetDir); err != nil {
			return err
		}
	}
	return nil
}

func writeZipEntry(file *zip.File, targetDir string) error {
	if !strings.HasPrefix(file.Name, zipRootPrefix) {
		return nil
	}

	relativePath := strings.TrimPrefix(file.Name, zipRootPrefix)
	if relativePath == "" {
		return nil
	}

	cleanRelativePath := filepath.Clean(filepath.FromSlash(relativePath))
	targetPath := filepath.Join(targetDir, cleanRelativePath)
	if !isPathWithinRoot(targetDir, targetPath) {
		return fmt.Errorf("invalid embedded path: %s", file.Name)
	}

	if file.FileInfo().IsDir() {
		return os.MkdirAll(targetPath, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", targetPath, err)
	}

	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("open zip entry %s: %w", file.Name, err)
	}
	defer reader.Close()

	mode := file.Mode()
	if mode == 0 {
		mode = 0o644
	}

	output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create extracted file %s: %w", targetPath, err)
	}
	defer output.Close()

	if _, err := io.Copy(output, reader); err != nil {
		return fmt.Errorf("write extracted file %s: %w", targetPath, err)
	}

	return nil
}

func isPathWithinRoot(root, path string) bool {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	if absolutePath == absoluteRoot {
		return true
	}

	rootPrefix := absoluteRoot + string(os.PathSeparator)
	return strings.HasPrefix(absolutePath, rootPrefix)
}
