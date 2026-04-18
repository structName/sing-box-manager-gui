# sing-box-manager

[English](#english) | [中文](#中文)

---

<a name="english"></a>

## English

A modern web-based management panel for [sing-box](https://github.com/SagerNet/sing-box), providing an intuitive interface to manage subscriptions, rules, filters, and more.

### Features

- **Subscription Management**
  - Support multiple protocols: SS, SSR, VMess, VLESS, Trojan, Hysteria2, TUIC, SOCKS
  - Clash YAML and Base64 encoded subscriptions
  - Traffic statistics (used/remaining/total)
  - Expiration date tracking
  - Auto-refresh with configurable intervals

- **Node Management**
  - Auto-parse nodes from subscriptions
  - Manual node addition with protocol-specific form fields
  - Country grouping with emoji flags
  - Node filtering by keywords, countries, and protocols
  - Tag-based node classification with auto-tagging rules

- **Speed Testing**
  - Latency testing (delay mode)
  - Download speed testing (speed mode)
  - Configurable test profiles with scheduling (cron)
  - Concurrent testing with customizable concurrency
  - Node speed history tracking
  - Landing IP detection

- **Rule Configuration**
  - Custom rules (domain, IP, port, geosite, geoip)
  - 13 preset rule groups (Ads, AI services, streaming, etc.)
  - Rule priority management
  - Rule set validation tool

- **Proxy Chains**
  - Multi-hop proxy chain configuration
  - Country-based auto-selection
  - Custom outbound routing per inbound port

- **DNS Management**
  - Multiple DNS protocols (UDP, DoT, DoH)
  - Custom hosts mapping
  - DNS routing rules
  - FakeIP support

- **Service Control**
  - Start/Stop/Restart sing-box
  - Configuration hot-reload
  - Auto-apply on config changes
  - Process recovery on startup
  - systemd service integration (Linux)
  - launchd service integration (macOS)

- **System Monitoring**
  - Real-time CPU and memory usage
  - Application and sing-box logs via SSE
  - Service status dashboard

- **Multi-Profile Support**
  - Multiple configuration profiles
  - Profile import/export (zip)
  - Profile switching

- **Kernel Management**
  - Built-in SSR-enabled sing-box binary
  - Version checking and online updates
  - Multi-platform support (Linux, macOS, Windows)

### Supported Protocols

| Protocol | Subscription | Speed Test | URL Parse | Clash YAML |
|----------|:-----------:|:----------:|:---------:|:----------:|
| Shadowsocks (SS) | ✅ | ✅ | ✅ | ✅ |
| ShadowsocksR (SSR) | ✅ | ✅ | ✅ | ✅ |
| VMess | ✅ | ✅ | ✅ | ✅ |
| VLESS | ✅ | ✅ | ✅ | ✅ |
| Trojan | ✅ | ✅ | ✅ | ✅ |
| Hysteria2 | ✅ | ✅ | ✅ | ✅ |
| TUIC | ✅ | ✅ | ✅ | ✅ |
| SOCKS | ✅ | - | ✅ | ✅ |

### Screenshots

![Dashboard](docs/screenshots/dashbord.png)
![Subscriptions](docs/screenshots/subscriptions.png)
![Rules](docs/screenshots/rules.png)
![Settings](docs/screenshots/settings.png)
![Logs](docs/screenshots/log.png)

### Installation

#### Pre-built Binaries

Download from [Releases](https://github.com/structName/sing-box-manager-gui/releases) page.

#### Build from Source

```bash
# Clone the repository
git clone https://github.com/structName/sing-box-manager-gui.git
cd sing-box-manager-gui

# Build for all platforms
./build.sh all

# Or build for current platform only
./build.sh current

# Output binaries are in ./dist/
```

**Build Options:**
```bash
./build.sh all       # Build all platforms (Linux/macOS/Windows x amd64/arm64)
./build.sh linux     # Build for Linux only
./build.sh darwin    # Build for macOS only
./build.sh current   # Build for current platform
./build.sh frontend  # Build frontend only
./build.sh clean     # Clean build directory

# Environment variables
SKIP_FRONTEND=1 ./build.sh current   # Skip frontend build
VERSION=1.0.0 ./build.sh all         # Custom version
```

### Usage

```bash
# Basic usage (default port: 19090)
./sbm

# Custom data directory and port
./sbm -data /opt/singbox-manager -port 8080

# Enable Swagger API docs
./sbm -swagger

# Export OpenAPI spec
./sbm -swagger-out ./openapi.json
```

**Command Line Options:**
| Option | Default | Description |
|--------|---------|-------------|
| `-data` | `~/.singbox-manager` | Data directory path |
| `-port` | `19090` | Web server port |
| `-swagger` | `false` | Enable Swagger UI at `/swagger` |
| `-swagger-out` | - | Export OpenAPI JSON spec to file |

After starting, open `http://localhost:19090` in your browser.

### Configuration

**Data Directory Structure:**
```
~/.singbox-manager/
├── bin/
│   └── sing-box              # Bundled sing-box binary (SSR-enabled)
├── profiles/
│   └── default/
│       ├── sbm.db            # SQLite database
│       ├── generated/
│       │   └── config.json   # Generated sing-box config
│       └── zashboard/        # Clash dashboard UI
├── logs/
│   ├── sbm.log               # Application logs
│   └── singbox.log           # sing-box logs
├── singbox.pid                # Process PID file
├── active_profile             # Current active profile
└── cache.db                   # Cache database
```

### API Endpoints

All APIs are under the `/api/` prefix. Key endpoint groups:

| Group | Path | Description |
|-------|------|-------------|
| Auth | `/api/auth/*` | Authentication (bootstrap, login, logout) |
| Subscriptions | `/api/subscriptions/*` | Subscription CRUD and refresh |
| Nodes | `/api/nodes/*` | Node listing and management |
| Rules | `/api/rules/*`, `/api/rule-groups/*` | Rule configuration |
| Tags | `/api/tags/*` | Tag-based node classification |
| Settings | `/api/settings` | System settings |
| Service | `/api/service/*` | sing-box process control |
| Speed Test | `/api/speedtest/*` | Speed test profiles, execution, results |
| Inbound Ports | `/api/inbound-ports/*` | Multi-port inbound management |
| Proxy Chains | `/api/proxy-chains/*` | Proxy chain configuration |
| Kernel | `/api/kernel/*` | Kernel version management |
| Config | `/api/config/*` | Config preview, export, apply |
| Profiles | `/api/profiles/*` | Profile management |

### SSR Support

This project bundles a custom sing-box build with ShadowsocksR support. The SSR-enabled kernel is built from [structName/sing-box](https://github.com/structName/sing-box) (`feat/ssr-support` branch).

**Supported SSR configurations:**

| Component | Supported Values |
|-----------|-----------------|
| Cipher | chacha20-ietf, aes-128/192/256-cfb, aes-128/192/256-ctr, rc4-md5, chacha20 |
| Protocol | origin, auth_aes128_sha1, auth_aes128_md5, auth_chain_a, auth_chain_b |
| Obfs | plain, http_simple, http_post, tls1.2_ticket_auth, tls1.2_ticket_fastauth |

### Tech Stack

- **Backend:** Go 1.24+, Gin, gopsutil, GORM (SQLite)
- **Frontend:** React 19, TypeScript, NextUI v2, Tailwind CSS, Zustand, Recharts
- **Speed Test:** mihomo (Clash Meta) library for proxy adapter
- **Build:** Single binary with embedded frontend via `go:embed`

### Requirements

- Go 1.24+ (for building)
- Node.js 18+ / pnpm (for building frontend)
- sing-box (bundled with SSR support)

### License

MIT License

---

<a name="中文"></a>

## 中文

一个现代化的 [sing-box](https://github.com/SagerNet/sing-box) Web 管理面板，提供直观的界面来管理订阅、规则、过滤器等。

### 功能特性

- **订阅管理**
  - 支持多种协议：SS、SSR、VMess、VLESS、Trojan、Hysteria2、TUIC、SOCKS
  - 兼容 Clash YAML 和 Base64 编码订阅
  - 流量统计（已用/剩余/总量）
  - 过期时间追踪
  - 可配置间隔的自动刷新

- **节点管理**
  - 自动从订阅解析节点
  - 手动添加节点（协议专属表单字段）
  - 按国家分组（带 emoji 国旗）
  - 按关键字、国家、协议过滤节点
  - 基于标签的节点分类与自动打标规则

- **测速功能**
  - 延迟测试（delay 模式）
  - 下载速度测试（speed 模式）
  - 可配置测速策略与定时调度（cron）
  - 并发测试，可自定义并发数
  - 节点测速历史记录
  - 落地 IP 检测

- **规则配置**
  - 自定义规则（域名、IP、端口、geosite、geoip）
  - 13 个预设规则组（广告、AI 服务、流媒体等）
  - 规则优先级管理
  - 规则集验证工具

- **代理链**
  - 多跳代理链配置
  - 按国家自动选择
  - 每个入站端口可配置独立出站

- **DNS 管理**
  - 多种 DNS 协议（UDP、DoT、DoH）
  - 自定义 hosts 映射
  - DNS 路由规则
  - FakeIP 支持

- **服务控制**
  - 启动/停止/重启 sing-box
  - 配置热重载
  - 配置变更后自动应用
  - 启动时自动恢复进程
  - systemd 服务集成（Linux）
  - launchd 服务集成（macOS）

- **系统监控**
  - 实时 CPU 和内存使用率
  - 应用和 sing-box 日志（SSE 实时推送）
  - 服务状态仪表盘

- **多 Profile 支持**
  - 多配置文件管理
  - Profile 导入/导出（zip）
  - 一键切换 Profile

- **内核管理**
  - 内置支持 SSR 的 sing-box 二进制文件
  - 版本检查与在线更新
  - 多平台支持（Linux、macOS、Windows）

### 支持的协议

| 协议 | 订阅导入 | 测速 | URL 解析 | Clash YAML |
|------|:-------:|:----:|:--------:|:----------:|
| Shadowsocks (SS) | ✅ | ✅ | ✅ | ✅ |
| ShadowsocksR (SSR) | ✅ | ✅ | ✅ | ✅ |
| VMess | ✅ | ✅ | ✅ | ✅ |
| VLESS | ✅ | ✅ | ✅ | ✅ |
| Trojan | ✅ | ✅ | ✅ | ✅ |
| Hysteria2 | ✅ | ✅ | ✅ | ✅ |
| TUIC | ✅ | ✅ | ✅ | ✅ |
| SOCKS | ✅ | - | ✅ | ✅ |

### 截图

![仪表盘](docs/screenshots/dashbord.png)
![订阅管理](docs/screenshots/subscriptions.png)
![规则配置](docs/screenshots/rules.png)
![设置](docs/screenshots/settings.png)
![日志](docs/screenshots/log.png)

### 安装

#### 预编译二进制文件

从 [Releases](https://github.com/structName/sing-box-manager-gui/releases) 页面下载。

#### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/structName/sing-box-manager-gui.git
cd sing-box-manager-gui

# 构建所有平台
./build.sh all

# 或只构建当前平台
./build.sh current

# 输出文件在 ./dist/ 目录
```

**构建选项：**
```bash
./build.sh all       # 构建所有平台（Linux/macOS/Windows x amd64/arm64）
./build.sh linux     # 仅构建 Linux
./build.sh darwin    # 仅构建 macOS
./build.sh current   # 仅构建当前平台
./build.sh frontend  # 仅构建前端
./build.sh clean     # 清理构建目录

# 环境变量
SKIP_FRONTEND=1 ./build.sh current   # 跳过前端构建
VERSION=1.0.0 ./build.sh all         # 自定义版本号
```

### 使用方法

```bash
# 基本用法（默认端口 19090）
./sbm

# 自定义数据目录和端口
./sbm -data /opt/singbox-manager -port 8080

# 启用 Swagger API 文档
./sbm -swagger

# 导出 OpenAPI 规范
./sbm -swagger-out ./openapi.json
```

**命令行参数：**
| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-data` | `~/.singbox-manager` | 数据目录路径 |
| `-port` | `19090` | Web 服务端口 |
| `-swagger` | `false` | 启用 Swagger UI（`/swagger`） |
| `-swagger-out` | - | 导出 OpenAPI JSON 规范到文件 |

启动后，在浏览器中打开 `http://localhost:19090`。

### 配置

**数据目录结构：**
```
~/.singbox-manager/
├── bin/
│   └── sing-box              # 内置 sing-box 二进制（支持 SSR）
├── profiles/
│   └── default/
│       ├── sbm.db            # SQLite 数据库
│       ├── generated/
│       │   └── config.json   # 生成的 sing-box 配置
│       └── zashboard/        # Clash 面板 UI
├── logs/
│   ├── sbm.log               # 应用日志
│   └── singbox.log           # sing-box 日志
├── singbox.pid                # 进程 PID 文件
├── active_profile             # 当前活动 Profile
└── cache.db                   # 缓存数据库
```

### API 端点

所有 API 在 `/api/` 前缀下。主要端点分组：

| 分组 | 路径 | 说明 |
|------|------|------|
| 认证 | `/api/auth/*` | 认证（初始化、登录、登出） |
| 订阅 | `/api/subscriptions/*` | 订阅增删改查与刷新 |
| 节点 | `/api/nodes/*` | 节点列表与管理 |
| 规则 | `/api/rules/*`, `/api/rule-groups/*` | 规则配置 |
| 标签 | `/api/tags/*` | 基于标签的节点分类 |
| 设置 | `/api/settings` | 系统设置 |
| 服务 | `/api/service/*` | sing-box 进程控制 |
| 测速 | `/api/speedtest/*` | 测速策略、执行、结果 |
| 入站端口 | `/api/inbound-ports/*` | 多端口入站管理 |
| 代理链 | `/api/proxy-chains/*` | 代理链配置 |
| 内核 | `/api/kernel/*` | 内核版本管理 |
| 配置 | `/api/config/*` | 配置预览、导出、应用 |
| Profile | `/api/profiles/*` | Profile 管理 |

### SSR 支持

本项目内置了支持 ShadowsocksR 协议的 sing-box 定制版。SSR 内核基于 [structName/sing-box](https://github.com/structName/sing-box)（`feat/ssr-support` 分支）构建。

**支持的 SSR 配置：**

| 组件 | 支持的值 |
|------|---------|
| 加密方式 | chacha20-ietf, aes-128/192/256-cfb, aes-128/192/256-ctr, rc4-md5, chacha20 |
| 协议 | origin, auth_aes128_sha1, auth_aes128_md5, auth_chain_a, auth_chain_b |
| 混淆 | plain, http_simple, http_post, tls1.2_ticket_auth, tls1.2_ticket_fastauth |

### 技术栈

- **后端：** Go 1.24+、Gin、gopsutil、GORM（SQLite）
- **前端：** React 19、TypeScript、NextUI v2、Tailwind CSS、Zustand、Recharts
- **测速：** mihomo（Clash Meta）库作为代理适配器
- **构建：** 通过 `go:embed` 将前端嵌入单一二进制文件

### 环境要求

- Go 1.24+（用于构建）
- Node.js 18+ / pnpm（用于构建前端）
- sing-box（已内置，支持 SSR）

### 许可证

MIT License
