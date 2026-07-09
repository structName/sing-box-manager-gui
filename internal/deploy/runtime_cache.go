package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type RuntimeCache struct {
	dataDir string
}

type RuntimeArchive struct {
	RuntimeName      string `json:"runtime_name,omitempty"`
	Version          string `json:"version,omitempty"`
	OS               string `json:"os,omitempty"`
	Arch             string `json:"arch,omitempty"`
	Path             string `json:"path,omitempty"`
	Checksum         string `json:"checksum,omitempty"`
	ExpectedChecksum string `json:"expected_checksum,omitempty"`
	Status           string `json:"status,omitempty"`
	Downloadable     bool   `json:"downloadable"`
	DownloadURL      string `json:"download_url,omitempty"`
}

func NewRuntimeCache(dataDir string) RuntimeCache {
	return RuntimeCache{dataDir: dataDir}
}

func (c RuntimeCache) Put(runtimeName, version, osName, arch string, content []byte) (string, error) {
	if err := ValidatePinnedRuntime(runtimeName, version); err != nil {
		return "", err
	}
	if err := validateRuntimeTarget(osName, arch); err != nil {
		return "", err
	}
	dir := filepath.Join(c.dataDir, "runtime-cache", runtimeName, version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-%s.tar.gz", osName, arch))
	return path, os.WriteFile(path, content, 0644)
}

func (c RuntimeCache) Lookup(runtimeName, version, osName, arch, expectedChecksum string) (RuntimeArchive, error) {
	if err := ValidatePinnedRuntime(runtimeName, version); err != nil {
		return RuntimeArchive{}, err
	}
	if err := validateRuntimeTarget(osName, arch); err != nil {
		return RuntimeArchive{}, err
	}
	path := filepath.Join(c.dataDir, "runtime-cache", runtimeName, version, fmt.Sprintf("%s-%s.tar.gz", osName, arch))
	content, err := os.ReadFile(path)
	if err != nil {
		return RuntimeArchive{}, fmt.Errorf("runtime archive missing: %w", err)
	}
	actual := sha256Hex(content)
	if expectedChecksum != "" && actual != expectedChecksum {
		return RuntimeArchive{}, fmt.Errorf("runtime archive checksum mismatch")
	}
	return RuntimeArchive{
		RuntimeName:      runtimeName,
		Version:          version,
		OS:               osName,
		Arch:             arch,
		Path:             path,
		Checksum:         actual,
		ExpectedChecksum: expectedChecksum,
		Status:           "valid",
		Downloadable:     true,
		DownloadURL:      RuntimeDownloadURL(runtimeName, version, osName, arch),
	}, nil
}

func (c RuntimeCache) List(runtimeName, version string) ([]RuntimeArchive, error) {
	if err := ValidatePinnedRuntime(runtimeName, version); err != nil {
		return nil, err
	}
	return c.listPinnedSingBox()
}

func (c RuntimeCache) listPinnedSingBox() ([]RuntimeArchive, error) {
	targets := []struct {
		osName string
		arch   string
	}{
		{osName: "linux", arch: "amd64"},
		{osName: "linux", arch: "arm64"},
	}
	archives := make([]RuntimeArchive, 0, len(targets))
	for _, target := range targets {
		expected, _ := RuntimeChecksum("sing-box", PinnedSingBoxVersion, target.osName, target.arch)
		archive, err := c.inspectRuntimeArchive("sing-box", PinnedSingBoxVersion, target.osName, target.arch, expected)
		if err != nil {
			return nil, err
		}
		archives = append(archives, archive)
	}
	return archives, nil
}

func (c RuntimeCache) inspectRuntimeArchive(runtimeName, version, osName, arch, expectedChecksum string) (RuntimeArchive, error) {
	if err := validateRuntimeTarget(osName, arch); err != nil {
		return RuntimeArchive{}, err
	}
	path := filepath.Join(c.dataDir, "runtime-cache", runtimeName, version, fmt.Sprintf("%s-%s.tar.gz", osName, arch))
	archive := RuntimeArchive{
		RuntimeName:      runtimeName,
		Version:          version,
		OS:               osName,
		Arch:             arch,
		Path:             path,
		ExpectedChecksum: expectedChecksum,
		Status:           "missing",
		Downloadable:     true,
		DownloadURL:      RuntimeDownloadURL(runtimeName, version, osName, arch),
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return archive, nil
		}
		return RuntimeArchive{}, err
	}
	archive.Checksum = sha256Hex(content)
	if expectedChecksum != "" && archive.Checksum != expectedChecksum {
		archive.Status = "invalid"
		return archive, nil
	}
	archive.Status = "valid"
	return archive, nil
}

func RuntimeDownloadURL(runtimeName, version, osName, arch string) string {
	if ValidatePinnedRuntime(runtimeName, version) != nil {
		return ""
	}
	archiveName := fmt.Sprintf("sing-box-%s-%s-%s.tar.gz", version, osName, arch)
	return fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/%s", version, archiveName)
}

func validateRuntimeTarget(osName, arch string) error {
	if osName != "linux" {
		return fmt.Errorf("unsupported runtime os: %s", osName)
	}
	if arch != "amd64" && arch != "arm64" {
		return fmt.Errorf("unsupported runtime arch: %s", arch)
	}
	return nil
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
