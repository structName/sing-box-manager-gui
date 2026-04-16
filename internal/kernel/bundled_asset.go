package kernel

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const bundledVersion = "ve7cfc42-ssr"

type bundledAsset struct {
	archiveName string
	data        []byte
}

func DefaultBinPath(dataDir string) string {
	return filepath.Join(dataDir, "bin", defaultBinaryName())
}

func BundledVersion() string {
	if !HasBundledAsset() {
		return ""
	}
	return bundledVersion
}

func HasBundledAsset() bool {
	return bundledAssetForCurrentPlatform() != nil
}

func EnsureBundledInstalled(dataDir string) (bool, error) {
	asset := bundledAssetForCurrentPlatform()
	if asset == nil {
		return false, nil
	}

	binPath := DefaultBinPath(dataDir)
	_, err := os.Stat(binPath)
	if err == nil {
		return false, nil
	}
	if !os.IsNotExist(err) {
		return false, fmt.Errorf("检查 sing-box 内核失败: %w", err)
	}

	manager := &Manager{
		dataDir: dataDir,
		binPath: binPath,
	}
	if err := manager.installBundledAsset(asset); err != nil {
		return false, err
	}

	return true, nil
}

func defaultBinaryName() string {
	if runtime.GOOS == "windows" {
		return "sing-box.exe"
	}
	return "sing-box"
}

func bundledAssetForCurrentPlatform() *bundledAsset {
	if bundledArchiveName == "" || len(bundledArchiveData) == 0 {
		return nil
	}
	return &bundledAsset{
		archiveName: bundledArchiveName,
		data:        bundledArchiveData,
	}
}

func (m *Manager) installBundledAsset(asset *bundledAsset) error {
	tmpDir, err := os.MkdirTemp("", "singbox-bundled")
	if err != nil {
		return fmt.Errorf("创建内置内核临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.archiveName)
	if err := os.WriteFile(archivePath, asset.data, 0644); err != nil {
		return fmt.Errorf("写入内置内核归档失败: %w", err)
	}

	binaryPath, err := m.extractArchive(archivePath, tmpDir)
	if err != nil {
		return fmt.Errorf("解压内置内核失败: %w", err)
	}

	if err := m.installBinary(binaryPath); err != nil {
		return fmt.Errorf("安装内置内核失败: %w", err)
	}

	return nil
}
