package rulesets

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

const (
	ruleTypeGeosite     = "geosite"
	ruleTypeGeoIP       = "geoip"
	generatedRuleSetDir = "generated/rulesets"
	dirPermission       = 0o755
	filePermission      = 0o644
)

var safeRuleSetNamePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

//go:embed assets/geosite/*.srs assets/geoip/*.srs
var bundledFS embed.FS

func Has(ruleType, name string) bool {
	path, ok := assetPath(ruleType, name)
	if !ok {
		return false
	}

	_, err := fs.Stat(bundledFS, path)
	return err == nil
}

func Read(ruleType, name string) ([]byte, error) {
	path, ok := assetPath(ruleType, name)
	if !ok {
		return nil, fmt.Errorf("未找到内置规则集: %s/%s", ruleType, name)
	}

	data, err := bundledFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取内置规则集失败: %w", err)
	}

	return data, nil
}

func Materialize(dataDir, ruleType, name string) (string, error) {
	if dataDir == "" {
		return "", fmt.Errorf("数据目录为空，无法写入内置规则集")
	}

	data, err := Read(ruleType, name)
	if err != nil {
		return "", err
	}

	targetPath := filepath.Join(dataDir, generatedRuleSetDir, ruleType, fileName(ruleType, name))
	if err := os.MkdirAll(filepath.Dir(targetPath), dirPermission); err != nil {
		return "", fmt.Errorf("创建规则集目录失败: %w", err)
	}

	if existingData, err := os.ReadFile(targetPath); err == nil && bytes.Equal(existingData, data) {
		return targetPath, nil
	}

	if err := os.WriteFile(targetPath, data, filePermission); err != nil {
		return "", fmt.Errorf("写入内置规则集失败: %w", err)
	}

	return targetPath, nil
}

func RemoteURL(baseURL, ruleType, name string) string {
	switch ruleType {
	case ruleTypeGeosite:
		return fmt.Sprintf("%s/geosite-%s.srs", baseURL, name)
	case ruleTypeGeoIP:
		return fmt.Sprintf("%s/../rule-set-geoip/geoip-%s.srs", baseURL, name)
	default:
		return ""
	}
}

func assetPath(ruleType, name string) (string, bool) {
	if !isSupportedRuleType(ruleType) || !safeRuleSetNamePattern.MatchString(name) {
		return "", false
	}

	return filepath.ToSlash(filepath.Join("assets", ruleType, fileName(ruleType, name))), true
}

func fileName(ruleType, name string) string {
	return fmt.Sprintf("%s-%s.srs", ruleType, name)
}

func isSupportedRuleType(ruleType string) bool {
	return ruleType == ruleTypeGeosite || ruleType == ruleTypeGeoIP
}
