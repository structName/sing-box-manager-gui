package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xiaobei/singbox-manager/internal/api"
	"github.com/xiaobei/singbox-manager/internal/daemon"
	"github.com/xiaobei/singbox-manager/internal/kernel"
	"github.com/xiaobei/singbox-manager/internal/logger"
	"github.com/xiaobei/singbox-manager/internal/profile"
)

var (
	version        = "0.2.9"
	dataDir        string
	port           int
	swaggerEnabled bool
	swaggerOut     string
)

func init() {
	// 获取默认数据目录
	homeDir, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(homeDir, ".singbox-manager")

	flag.StringVar(&dataDir, "data", defaultDataDir, "数据目录")
	flag.IntVar(&port, "port", 9090, "Web 服务端口")
	flag.BoolVar(&swaggerEnabled, "swagger", false, "启用 Swagger UI")
	flag.StringVar(&swaggerOut, "swagger-out", "", "导出 OpenAPI JSON 文件")
}

func main() {
	flag.Parse()

	// 将 dataDir 转换为绝对路径，避免相对路径在子进程中出错
	var err error
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取绝对路径失败: %v\n", err)
		os.Exit(1)
	}

	// 获取当前可执行文件的绝对路径（用于 launchd 安装）
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取可执行文件路径失败: %v\n", err)
		os.Exit(1)
	}
	execPath, _ = filepath.EvalSymlinks(execPath)

	// 初始化日志系统
	if err := logger.InitLogManager(dataDir); err != nil {
		fmt.Fprintf(os.Stderr, "初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	// 打印启动信息
	logger.Printf("singbox-manager v%s", version)
	logger.Printf("数据目录: %s", dataDir)
	logger.Printf("Web 端口: %d", port)

	installed, err := kernel.EnsureBundledInstalled(dataDir)
	if err != nil {
		logger.Printf("安装内置 sing-box 失败: %v", err)
		os.Exit(1)
	}
	if installed {
		logger.Printf("已安装内置 sing-box %s", kernel.BundledVersion())
	}

	// 初始化 Profile 管理器
	profileMgr, err := profile.NewManager(dataDir)
	if err != nil {
		logger.Printf("初始化 Profile 管理器失败: %v", err)
		os.Exit(1)
	}
	logger.Printf("当前 Profile: %s", profileMgr.GetActiveProfile())

	// 初始化进程管理器
	// sing-box 二进制文件路径固定在 dataDir/bin 下
	singboxPath := kernel.DefaultBinPath(dataDir)
	// 配置文件放在当前 Profile 目录下
	profileDir := profileMgr.GetProfileDir()
	configPath := filepath.Join(profileDir, "generated", "config.json")
	// 确保 generated 目录存在
	os.MkdirAll(filepath.Join(profileDir, "generated"), 0755)
	processManager := daemon.NewProcessManager(singboxPath, configPath, dataDir)

	// 初始化 launchd 管理器
	launchdManager, err := daemon.NewLaunchdManager()
	if err != nil {
		logger.Printf("初始化 launchd 管理器失败: %v", err)
	}

	// 初始化 systemd 管理器
	systemdManager, err := daemon.NewSystemdManager()
	if err != nil {
		logger.Printf("初始化 systemd 管理器失败: %v", err)
	}

	// 创建 API 服务器
	server, err := api.NewServer(profileMgr, processManager, launchdManager, systemdManager, execPath, port, version, swaggerEnabled)
	if err != nil {
		logger.Printf("初始化 API 服务器失败: %v", err)
		os.Exit(1)
	}

	if swaggerOut != "" {
		if err := server.WriteOpenAPISpec(swaggerOut); err != nil {
			logger.Printf("生成 Swagger OpenAPI 失败: %v", err)
			os.Exit(1)
		}
		logger.Printf("Swagger OpenAPI 已生成: %s", swaggerOut)
	}

	// 启动统一调度器（包含订阅更新、测速、链路检测等）
	server.StartUnifiedScheduler()

	// 启动服务
	addr := fmt.Sprintf(":%d", port)
	logger.Printf("启动 Web 服务: http://0.0.0.0%s", addr)

	if err := server.Run(addr); err != nil {
		logger.Printf("启动服务失败: %v", err)
		os.Exit(1)
	}
}
