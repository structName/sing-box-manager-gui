package daemon

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"
)

// systemdTemplate 系统级服务模板
const systemdTemplate = `[Unit]
Description=SingBox Manager
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=10

[Service]
Type=simple
ExecStart={{.SbmPath}} -data {{.DataDir}} -port {{.Port}}
WorkingDirectory={{.WorkingDir}}
Restart={{if .KeepAlive}}always{{else}}no{{end}}
RestartSec=5
{{if .UseJournal}}StandardOutput=journal
StandardError=journal{{else}}StandardOutput=append:{{.LogPath}}/sbm.log
StandardError=append:{{.LogPath}}/sbm.error.log{{end}}
Environment="HOME={{.HomeDir}}"
{{if .User}}User={{.User}}
{{end}}{{if .Group}}Group={{.Group}}
{{end}}
[Install]
WantedBy=multi-user.target
`

// systemdUserTemplate 用户级服务模板
const systemdUserTemplate = `[Unit]
Description=SingBox Manager
After=network.target
StartLimitIntervalSec=60
StartLimitBurst=10

[Service]
Type=simple
ExecStart={{.SbmPath}} -data {{.DataDir}} -port {{.Port}}
WorkingDirectory={{.WorkingDir}}
Restart={{if .KeepAlive}}always{{else}}no{{end}}
RestartSec=5
StandardOutput=append:{{.LogPath}}/sbm.log
StandardError=append:{{.LogPath}}/sbm.error.log
Environment="HOME={{.HomeDir}}"

[Install]
WantedBy=default.target
`

// SystemdConfig systemd 配置
type SystemdConfig struct {
	SbmPath    string
	DataDir    string
	Port       string
	LogPath    string
	WorkingDir string
	HomeDir    string
	RunAtLoad  bool
	KeepAlive  bool
	User       string // 系统级模式下的运行用户
	Group      string // 系统级模式下的运行组
	UseJournal bool   // 是否使用 journald 输出（旧版 systemd 兼容）
}

// SystemdManager systemd 管理器
type SystemdManager struct {
	serviceName string
	servicePath string
	userMode    bool // true=用户级, false=系统级
}

// isSystemdAvailable 检测 systemd 是否可用
func isSystemdAvailable() bool {
	cmd := exec.Command("systemctl", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	// 检查输出是否包含 systemd 版本信息
	return len(output) > 0 && strings.Contains(string(output), "systemd")
}

// isUserDBusAvailable 检测用户级 D-Bus 是否可用
func isUserDBusAvailable() bool {
	cmd := exec.Command("systemctl", "--user", "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		// 检查是否是 D-Bus 相关错误
		if strings.Contains(outputStr, "D-Bus") ||
			strings.Contains(outputStr, "No such file or directory") ||
			strings.Contains(outputStr, "Failed to get D-Bus connection") {
			return false
		}
		// 其他错误可能意味着服务不存在，但 D-Bus 可用
		// 例如 "Loaded: not-found" 表示 D-Bus 正常但服务未安装
	}
	// 如果没有 D-Bus 错误，则认为 D-Bus 可用
	return true
}

// NewSystemdManager 创建 systemd 管理器
func NewSystemdManager() (*SystemdManager, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("systemd 仅在 Linux 上支持")
	}

	// 检测 systemd 是否可用
	if !isSystemdAvailable() {
		return nil, fmt.Errorf("systemd 不可用")
	}

	serviceName := "singbox-manager.service"
	userMode := false
	var servicePath string

	if os.Getuid() == 0 {
		// root 用户，使用系统级
		servicePath = filepath.Join("/etc/systemd/system", serviceName)
		userMode = false
	} else {
		// 普通用户，检测用户级 D-Bus 是否可用
		if isUserDBusAvailable() {
			homeDir, err := getUserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("获取用户目录失败: %w", err)
			}
			servicePath = filepath.Join(homeDir, ".config", "systemd", "user", serviceName)
			userMode = true
		} else {
			// D-Bus 不可用，回退系统级
			servicePath = filepath.Join("/etc/systemd/system", serviceName)
			userMode = false
		}
	}

	return &SystemdManager{
		serviceName: serviceName,
		servicePath: servicePath,
		userMode:    userMode,
	}, nil
}

// GetMode 获取当前运行模式
func (sm *SystemdManager) GetMode() string {
	if sm.userMode {
		return "user"
	}
	return "system"
}

// Install 安装 systemd 服务
func (sm *SystemdManager) Install(config SystemdConfig) error {
	if err := os.MkdirAll(config.LogPath, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 创建服务目录
	if err := os.MkdirAll(filepath.Dir(sm.servicePath), 0755); err != nil {
		return fmt.Errorf("创建 systemd 目录失败: %w", err)
	}

	// 选择模板
	var tmplStr string
	if sm.userMode {
		tmplStr = systemdUserTemplate
	} else {
		tmplStr = systemdTemplate
	}

	tmpl, err := template.New("systemd").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("解析模板失败: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return fmt.Errorf("生成 service 文件失败: %w", err)
	}

	if err := os.WriteFile(sm.servicePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("写入 service 文件失败: %w", err)
	}

	// 重新加载 systemd 配置
	if err := sm.runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("重新加载配置失败: %w", err)
	}

	// 启用服务（开机自启）
	if config.RunAtLoad {
		if err := sm.runSystemctl("enable", sm.serviceName); err != nil {
			return fmt.Errorf("启用服务失败: %w", err)
		}
		// 用户级服务需要 linger 才能在系统启动时（无用户登录）运行
		if sm.userMode {
			if err := enableLinger(); err != nil {
				return fmt.Errorf("启用用户 linger 失败（开机自启需要 root 权限）: %w\n请使用 sudo 重新运行，或以 root 身份安装系统级服务", err)
			}
		}
	}

	return nil
}

// Uninstall 卸载 systemd 服务
func (sm *SystemdManager) Uninstall() error {
	sm.Stop()
	sm.runSystemctl("disable", sm.serviceName)

	if err := os.Remove(sm.servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 service 文件失败: %w", err)
	}

	sm.runSystemctl("daemon-reload")

	// 用户级模式：尽力取消 linger（失败不阻塞卸载）
	if sm.userMode {
		_ = disableLinger()
	}
	return nil
}

// Start 启动服务
func (sm *SystemdManager) Start() error {
	return sm.runSystemctl("start", sm.serviceName)
}

// Stop 停止服务
func (sm *SystemdManager) Stop() error {
	return sm.runSystemctl("stop", sm.serviceName)
}

// Restart 重启服务
func (sm *SystemdManager) Restart() error {
	sm.Stop()
	time.Sleep(500 * time.Millisecond)
	sm.runSystemctl("start", sm.serviceName)

	maxRetries := 20
	for i := 0; i < maxRetries; i++ {
		time.Sleep(500 * time.Millisecond)
		if sm.IsRunning() {
			return nil
		}
	}
	return fmt.Errorf("服务重启失败：服务在 %v 内未能启动", time.Duration(maxRetries)*500*time.Millisecond)
}

// IsInstalled 检查是否已安装
func (sm *SystemdManager) IsInstalled() bool {
	_, err := os.Stat(sm.servicePath)
	return err == nil
}

// IsRunning 检查是否运行中
func (sm *SystemdManager) IsRunning() bool {
	err := sm.runSystemctl("is-active", "--quiet", sm.serviceName)
	return err == nil
}

// GetServicePath 获取 service 文件路径
func (sm *SystemdManager) GetServicePath() string {
	return sm.servicePath
}

// runSystemctl 执行 systemctl 命令
func (sm *SystemdManager) runSystemctl(args ...string) error {
	if sm.userMode {
		args = append([]string{"--user"}, args...)
	}
	cmd := exec.Command("systemctl", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(output))
	}
	return nil
}

// currentUsername 获取当前进程的用户名（用于 loginctl linger）
func currentUsername() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u
	}
	return ""
}

// enableLinger 为当前用户启用 linger，使得用户级 systemd 服务在系统启动时运行
// （无需用户登录）。该操作需要 root 权限或 polkit 授权，否则失败。
func enableLinger() error {
	user := currentUsername()
	if user == "" {
		return fmt.Errorf("无法获取当前用户名（USER/LOGNAME 未设置）")
	}
	cmd := exec.Command("loginctl", "enable-linger", user)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// disableLinger 取消当前用户的 linger（卸载时调用，失败不影响主流程）
func disableLinger() error {
	user := currentUsername()
	if user == "" {
		return nil
	}
	cmd := exec.Command("loginctl", "disable-linger", user)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
