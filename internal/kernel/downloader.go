package kernel

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// downloadAndInstall 下载并安装内核
func (m *Manager) downloadAndInstall(version string) {
	defer func() {
		if r := recover(); r != nil {
			m.setDownloadComplete("error", fmt.Sprintf("下载过程发生错误: %v", r))
		}
	}()

	// 1. 获取 releases
	m.updateProgress("preparing", 0, "正在获取版本信息...", 0, 0)
	releases, err := m.FetchReleases()
	if err != nil {
		m.setDownloadComplete("error", fmt.Sprintf("获取版本信息失败: %v", err))
		return
	}

	// 2. 获取对应平台的资源信息
	asset, err := m.getAssetInfo(releases, version)
	if err != nil {
		m.setDownloadComplete("error", err.Error())
		return
	}

	// 3. 创建临时目录
	tmpDir, err := os.MkdirTemp("", "singbox-download")
	if err != nil {
		m.setDownloadComplete("error", fmt.Sprintf("创建临时目录失败: %v", err))
		return
	}
	defer os.RemoveAll(tmpDir)

	// 4. 下载文件
	downloadURL := m.buildDownloadURL(asset.BrowserDownloadURL)
	tmpFile := filepath.Join(tmpDir, asset.Name)

	m.updateProgress("downloading", 0, "正在下载...", 0, asset.Size)
	if err := m.downloadFile(downloadURL, tmpFile, asset.Size); err != nil {
		m.setDownloadComplete("error", fmt.Sprintf("下载失败: %v", err))
		return
	}

	// 5. 解压文件
	m.updateProgress("extracting", 80, "正在解压...", asset.Size, asset.Size)
	binaryPath, err := m.extractArchive(tmpFile, tmpDir)
	if err != nil {
		m.setDownloadComplete("error", fmt.Sprintf("解压失败: %v", err))
		return
	}

	// 6. 安装到目标路径
	m.updateProgress("installing", 90, "正在安装...", asset.Size, asset.Size)
	if err := m.installBinary(binaryPath); err != nil {
		m.setDownloadComplete("error", fmt.Sprintf("安装失败: %v", err))
		return
	}

	// 7. 完成
	m.setDownloadComplete("completed", fmt.Sprintf("sing-box %s 安装成功", version))
}

// downloadFile 下载文件
func (m *Manager) downloadFile(url, dest string, totalSize int64) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，HTTP 状态码: %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	var downloaded int64
	buffer := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			_, writeErr := out.Write(buffer[:n])
			if writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)

			// 更新进度
			progress := float64(downloaded) / float64(totalSize) * 80 // 下载阶段占 80%
			m.updateProgress("downloading", progress, fmt.Sprintf("下载中 %.1f%%", progress/0.8), downloaded, totalSize)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

// extractArchive 解压压缩包
func (m *Manager) extractArchive(archivePath, destDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		return m.extractTarGz(archivePath, destDir)
	} else if strings.HasSuffix(archivePath, ".zip") {
		return m.extractZip(archivePath, destDir)
	}
	return "", fmt.Errorf("不支持的压缩格式: %s", archivePath)
}

// extractTarGz 解压 tar.gz
func (m *Manager) extractTarGz(archivePath, destDir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	var binaryPath string
	binaryName := defaultBinaryName()

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		// 只提取 sing-box 二进制文件
		if header.Typeflag == tar.TypeReg && strings.HasSuffix(header.Name, binaryName) {
			binaryPath = filepath.Join(destDir, binaryName)
			outFile, err := os.Create(binaryPath)
			if err != nil {
				return "", err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return "", err
			}
			outFile.Close()
			break
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("未在压缩包中找到 %s", binaryName)
	}

	return binaryPath, nil
}

// extractZip 解压 zip
func (m *Manager) extractZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var binaryPath string
	binaryName := defaultBinaryName()

	for _, f := range r.File {
		if strings.HasSuffix(f.Name, binaryName) {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}

			binaryPath = filepath.Join(destDir, binaryName)
			outFile, err := os.Create(binaryPath)
			if err != nil {
				rc.Close()
				return "", err
			}

			if _, err := io.Copy(outFile, rc); err != nil {
				outFile.Close()
				rc.Close()
				return "", err
			}

			outFile.Close()
			rc.Close()
			break
		}
	}

	if binaryPath == "" {
		return "", fmt.Errorf("未在压缩包中找到 %s", binaryName)
	}

	return binaryPath, nil
}

// installBinary 安装二进制文件
func (m *Manager) installBinary(srcPath string) error {
	destPath := m.binPath

	// 确保目标目录存在
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 如果目标文件已存在，先删除
	if _, err := os.Stat(destPath); err == nil {
		if err := os.Remove(destPath); err != nil {
			return fmt.Errorf("删除旧版本失败: %w", err)
		}
	}

	// 复制文件
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dest, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dest.Close()

	if _, err := io.Copy(dest, src); err != nil {
		return err
	}

	// 设置可执行权限
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("设置权限失败: %w", err)
	}

	return nil
}
