package profile

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/logger"
	"gorm.io/gorm"
)

// Info Profile 元信息
type Info struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Manager Profile 管理器
type Manager struct {
	baseDir       string // ~/.singbox-manager
	activeProfile string
	mu            sync.RWMutex
	db            *gorm.DB
	dbStore       *database.Store
}

// NewManager 创建 Profile 管理器
func NewManager(baseDir string) (*Manager, error) {
	m := &Manager{baseDir: baseDir}

	// 确保 profiles 目录存在
	if err := os.MkdirAll(m.profilesDir(), 0755); err != nil {
		return nil, fmt.Errorf("创建 profiles 目录失败: %w", err)
	}

	// 读取当前激活的 Profile
	active, err := m.readActiveProfile()
	if err != nil || active == "" {
		active = "default"
	}
	m.activeProfile = active

	// 确保 default profile 存在
	if err := m.ensureProfile("default"); err != nil {
		return nil, err
	}

	// 初始化数据库连接
	if err := m.initDB(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) profilesDir() string {
	return filepath.Join(m.baseDir, "profiles")
}

func (m *Manager) profileDir(name string) string {
	return filepath.Join(m.profilesDir(), name)
}

func (m *Manager) activeProfileFile() string {
	return filepath.Join(m.baseDir, "active_profile")
}

func (m *Manager) readActiveProfile() (string, error) {
	data, err := os.ReadFile(m.activeProfileFile())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (m *Manager) writeActiveProfile(name string) error {
	return os.WriteFile(m.activeProfileFile(), []byte(name), 0644)
}

// ensureProfile 确保 Profile 目录和元信息存在
func (m *Manager) ensureProfile(name string) error {
	dir := m.profileDir(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 检查 info.json 是否存在
	infoFile := filepath.Join(dir, "info.json")
	if _, err := os.Stat(infoFile); os.IsNotExist(err) {
		info := Info{
			Name:      name,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		data, _ := json.MarshalIndent(info, "", "  ")
		return os.WriteFile(infoFile, data, 0644)
	}
	return nil
}

// initDB 初始化当前 Profile 的数据库
func (m *Manager) initDB() error {
	profileDir := m.profileDir(m.activeProfile)
	db, err := database.InitDBWithPath(profileDir)
	if err != nil {
		return err
	}
	m.db = db
	m.dbStore = database.NewStore(db)
	return nil
}

// GetDB 获取当前数据库实例
func (m *Manager) GetDB() *gorm.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.db
}

// GetStore 获取当前数据库 Store
func (m *Manager) GetStore() *database.Store {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.dbStore
}

// GetActiveProfile 获取当前激活的 Profile 名称
func (m *Manager) GetActiveProfile() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeProfile
}

// GetProfileDir 获取当前 Profile 的目录
func (m *Manager) GetProfileDir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.profileDir(m.activeProfile)
}

// List 列出所有 Profile
func (m *Manager) List() ([]Info, error) {
	entries, err := os.ReadDir(m.profilesDir())
	if err != nil {
		return nil, err
	}

	var profiles []Info
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		infoFile := filepath.Join(m.profileDir(entry.Name()), "info.json")
		data, err := os.ReadFile(infoFile)
		if err != nil {
			// 如果没有 info.json，创建一个默认的
			info := Info{Name: entry.Name(), CreatedAt: time.Now(), UpdatedAt: time.Now()}
			profiles = append(profiles, info)
			continue
		}
		var info Info
		if err := json.Unmarshal(data, &info); err != nil {
			info = Info{Name: entry.Name()}
		}
		profiles = append(profiles, info)
	}
	return profiles, nil
}

// Create 创建新 Profile
func (m *Manager) Create(name, description string) error {
	if name == "" {
		return fmt.Errorf("Profile 名称不能为空")
	}

	// 检查是否已存在
	dir := m.profileDir(name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("Profile '%s' 已存在", name)
	}

	// 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入元信息
	info := Info{
		Name:        name,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "info.json"), data, 0644); err != nil {
		return err
	}

	// 初始化该 Profile 的数据库（创建空数据库，不影响全局活跃连接）
	newDB, err := database.OpenDB(dir)
	if err != nil {
		return fmt.Errorf("初始化 Profile 数据库失败: %w", err)
	}
	// 关闭连接（仅用于建表，不需要保持）
	sqlDB, _ := newDB.DB()
	sqlDB.Close()

	logger.Info("创建 Profile: %s", name)
	return nil
}

// Switch 切换到指定 Profile
func (m *Manager) Switch(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == m.activeProfile {
		return nil // 已经是当前 Profile
	}

	// 检查目标 Profile 是否存在
	dir := m.profileDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("Profile '%s' 不存在", name)
	}

	// 关闭当前数据库连接
	if m.db != nil {
		sqlDB, err := m.db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}

	// 重置数据库单例
	database.ResetDB()

	// 切换到新 Profile
	m.activeProfile = name
	if err := m.writeActiveProfile(name); err != nil {
		return err
	}

	// 初始化新数据库
	db, err := database.InitDBWithPath(dir)
	if err != nil {
		return err
	}
	m.db = db
	m.dbStore = database.NewStore(db)

	logger.Info("切换到 Profile: %s", name)
	return nil
}

// Delete 删除 Profile
func (m *Manager) Delete(name string) error {
	if name == "default" {
		return fmt.Errorf("不能删除默认 Profile")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if name == m.activeProfile {
		return fmt.Errorf("不能删除当前激活的 Profile")
	}

	dir := m.profileDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("Profile '%s' 不存在", name)
	}

	if err := os.RemoveAll(dir); err != nil {
		return err
	}

	logger.Info("删除 Profile: %s", name)
	return nil
}

// Export 导出 Profile 为 zip 文件
func (m *Manager) Export(name string, writer io.Writer) error {
	dir := m.profileDir(name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("Profile '%s' 不存在", name)
	}

	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)
		w, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(w, f)
		return err
	})
}

// Import 从 zip 文件导入 Profile
func (m *Manager) Import(name string, reader io.ReaderAt, size int64) error {
	if name == "" {
		return fmt.Errorf("Profile 名称不能为空")
	}

	dir := m.profileDir(name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("Profile '%s' 已存在", name)
	}

	// 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 解压
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		os.RemoveAll(dir)
		return err
	}

	for _, f := range zipReader.File {
		destPath := filepath.Join(dir, f.Name)

		// 安全检查：防止 zip slip 攻击
		if !strings.HasPrefix(destPath, dir) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, 0755)
			continue
		}

		// 确保父目录存在
		os.MkdirAll(filepath.Dir(destPath), 0755)

		destFile, err := os.Create(destPath)
		if err != nil {
			return err
		}

		srcFile, err := f.Open()
		if err != nil {
			destFile.Close()
			return err
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		destFile.Close()
		if err != nil {
			return err
		}
	}

	// 更新 info.json 中的名称
	infoFile := filepath.Join(dir, "info.json")
	if data, err := os.ReadFile(infoFile); err == nil {
		var info Info
		if json.Unmarshal(data, &info) == nil {
			info.Name = name
			info.UpdatedAt = time.Now()
			newData, _ := json.MarshalIndent(info, "", "  ")
			os.WriteFile(infoFile, newData, 0644)
		}
	}

	logger.Info("导入 Profile: %s", name)
	return nil
}

// UpdateInfo 更新 Profile 元信息
func (m *Manager) UpdateInfo(name, description string) error {
	dir := m.profileDir(name)
	infoFile := filepath.Join(dir, "info.json")

	var info Info
	if data, err := os.ReadFile(infoFile); err == nil {
		json.Unmarshal(data, &info)
	}

	info.Name = name
	info.Description = description
	info.UpdatedAt = time.Now()

	data, _ := json.MarshalIndent(info, "", "  ")
	return os.WriteFile(infoFile, data, 0644)
}

// Clone 克隆 Profile
func (m *Manager) Clone(srcName, destName string) error {
	if destName == "" {
		return fmt.Errorf("目标 Profile 名称不能为空")
	}

	srcDir := m.profileDir(srcName)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("源 Profile '%s' 不存在", srcName)
	}

	destDir := m.profileDir(destName)
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("目标 Profile '%s' 已存在", destName)
	}

	// 复制目录
	if err := copyDir(srcDir, destDir); err != nil {
		os.RemoveAll(destDir)
		return err
	}

	// 更新 info.json
	infoFile := filepath.Join(destDir, "info.json")
	info := Info{
		Name:        destName,
		Description: fmt.Sprintf("克隆自 %s", srcName),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	os.WriteFile(infoFile, data, 0644)

	logger.Info("克隆 Profile: %s -> %s", srcName, destName)
	return nil
}

// copyDir 递归复制目录
func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(src, path)
		destPath := filepath.Join(dest, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return err
	})
}
