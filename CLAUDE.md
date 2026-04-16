# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

sing-box-manager (sbm) 是一个 sing-box 代理软件的 Web 管理面板。采用前后端分离架构，最终构建为单一二进制文件（前端嵌入）。

内置支持 SSR 的定制版 sing-box 内核（基于 github.com/structName/sing-box feat/ssr-support 分支）。

## 常用命令

### 构建
```bash
# 构建当前平台
./build.sh current

# 仅构建前端
./build.sh frontend

# 构建所有平台 (Linux/macOS/Windows x amd64/arm64)
./build.sh all

# 跳过前端构建（前端已构建时）
SKIP_FRONTEND=1 ./build.sh current
```

### 开发
```bash
# 前端开发（热重载）
cd web && pnpm dev

# 后端开发（手动重启）
go run ./cmd/sbm/ -port 9090

# 运行测试
go test ./...

# 运行 SSR 相关测试
go test ./internal/parser/ -v -run SSR
```

### 前端
```bash
cd web
pnpm install    # 安装依赖
pnpm dev        # 开发服务器
pnpm build      # 构建生产版本
pnpm lint       # ESLint 检查
```

## 架构

### 技术栈
- **后端**: Go + Gin + gopsutil + GORM (SQLite)
- **前端**: React 19 + TypeScript + NextUI + Tailwind CSS + Zustand
- **测速**: mihomo (Clash Meta) 库作为代理适配器
- **构建**: 前端通过 `web/embed.go` 嵌入到 Go 二进制

### 目录结构
```
cmd/sbm/           # 主入口
internal/
├── api/           # HTTP API 路由和处理器 (Gin)
├── builder/       # sing-box 配置生成器
├── daemon/        # 进程管理 (process/launchd/systemd)
├── database/      # 数据库模型和迁移
├── kernel/        # sing-box 内核管理和内嵌二进制
│   └── assets/    # 各平台 sing-box 二进制归档 (go:embed)
├── logger/        # 日志系统
├── parser/        # 代理协议解析器 (SS/SSR/VMess/VLESS/Trojan/Hysteria2/TUIC/SOCKS)
├── profile/       # 多 Profile 管理
├── service/       # 业务服务 (订阅/调度/健康检查/链路同步)
├── speedtest/     # 测速模块 (基于 mihomo 代理适配器)
└── storage/       # JSON 持久化存储
web/
├── src/
│   ├── api/       # Axios API 客户端
│   ├── components/# React 组件
│   ├── pages/     # 页面组件
│   └── store/     # Zustand 状态管理
└── embed.go       # 前端静态资源嵌入
```

### 核心数据流
1. 用户通过前端 UI 操作 → Zustand store → API 调用
2. API 路由 (`internal/api/router.go`) → 业务服务处理
3. `internal/database/` SQLite 持久化（profile 隔离）
4. `internal/builder/singbox.go` 生成 sing-box 配置
5. `internal/daemon/process.go` 管理 sing-box 进程生命周期

### API 路由结构
所有 API 在 `/api/` 前缀下，主要分组：
- `/api/auth` - 认证（bootstrap/login/logout）
- `/api/subscriptions` - 订阅管理
- `/api/nodes` - 节点管理
- `/api/rules`, `/api/rule-groups` - 规则配置
- `/api/tags` - 标签管理
- `/api/settings` - 系统设置
- `/api/service` - sing-box 进程控制
- `/api/speedtest` - 测速（策略/任务/历史）
- `/api/inbound-ports` - 多端口入站管理
- `/api/proxy-chains` - 代理链配置
- `/api/config` - 配置预览/导出/应用
- `/api/kernel` - 内核版本管理
- `/api/profiles` - Profile 管理
- `/api/monitor` - 系统监控和日志

### Swagger 文档
- 默认关闭：启动时加 `-swagger` 才会挂载 `/swagger` 与 `/swagger/openapi.json`
- 生成 JSON 文件：使用 `-swagger-out /path/to/openapi.json`（可在不启用 UI 时导出）
- OpenAPI JSON 来自运行时 gin 路由（响应/模型为占位）
- UI 使用 `swagger-ui-dist` CDN 资源，离线环境需自行替换为本地资源

### 代理协议解析
`internal/parser/` 包含各协议解析器，统一接口：
- `parser.ParseURL(link string) (*Node, error)` - 解析单个节点链接
- `parser.ParseSubscriptionContent(content string) ([]Node, error)` - 解析订阅内容
- 支持 Clash YAML 批量解析 (`clash_yaml.go`)
- 支持协议：SS (`ss://`)、SSR (`ssr://`)、VMess (`vmess://`)、VLESS (`vless://`)、Trojan (`trojan://`)、Hysteria2 (`hy2://`)、TUIC (`tuic://`)、SOCKS (`socks://`)

### 测速模块
`internal/speedtest/` 使用 mihomo (Clash Meta) 库直接建立代理连接（不经过 sing-box），需要在 `nodeToMihomoProxy()` 中维护各协议到 mihomo 格式的转换。

### 内嵌内核
- 各平台二进制归档在 `internal/kernel/assets/` 目录
- 通过 `//go:embed` 嵌入，各平台有独立的 `bundled_asset_<os>_<arch>.go`
- 版本常量在 `internal/kernel/bundled_asset.go` 的 `bundledVersion`
- 当前内嵌的是支持 SSR 的定制版 sing-box（来自 structName/sing-box fork）

### sing-box 配置生成
- `internal/builder/singbox.go` 中的 `ConfigBuilder` 负责生成配置
- `nodeToOutbound()` 通过复制 `Node.Extra` map 生成出站配置，SSR 字段名需与 sing-box `option.ShadowsocksROutboundOptions` 的 JSON tag 完全一致
- `buildInbounds()` 不应包含 `sniff`/`sniff_override_destination` 等 legacy 字段（sing-box 1.11+ 已移除），这些功能通过 `buildRoute()` 中的 route rule_actions 实现
