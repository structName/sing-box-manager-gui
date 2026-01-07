# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

sing-box-manager (sbm) 是一个 sing-box 代理软件的 Web 管理面板。采用前后端分离架构，最终构建为单一二进制文件（前端嵌入）。

## 常用命令

### 构建
```bash
# 构建当前平台
./build.sh current

# 仅构建前端
./build.sh frontend

# 构建所有平台 (Linux/macOS x amd64/arm64)
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
- **后端**: Go + Gin + gopsutil
- **前端**: React 19 + TypeScript + NextUI + Tailwind CSS + Zustand
- **构建**: 前端通过 `web/embed.go` 嵌入到 Go 二进制

### 目录结构
```
cmd/sbm/           # 主入口
internal/
├── api/           # HTTP API 路由和处理器 (Gin)
├── builder/       # sing-box 配置生成器
├── daemon/        # 进程管理 (process/launchd/systemd)
├── kernel/        # sing-box 内核下载和管理
├── logger/        # 日志系统
├── parser/        # 代理协议解析器 (SS/VMess/VLESS/Trojan/Hysteria2/TUIC)
├── service/       # 业务服务 (订阅/调度/健康检查/链路同步)
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
3. `internal/storage/json_store.go` 持久化到 `~/.singbox-manager/data.json`
4. `internal/builder/singbox.go` 生成 sing-box 配置
5. `internal/daemon/process.go` 管理 sing-box 进程生命周期

### API 路由结构
所有 API 在 `/api/` 前缀下，主要分组：
- `/api/subscriptions` - 订阅管理
- `/api/nodes` - 节点管理
- `/api/rules`, `/api/rule-groups` - 规则配置
- `/api/settings` - 系统设置
- `/api/service` - sing-box 进程控制
- `/api/monitor` - 系统监控和日志
- `/api/kernel` - 内核版本管理

### Swagger 文档
- 默认关闭：启动时加 `-swagger` 才会挂载 `/swagger` 与 `/swagger/openapi.json`
- 生成 JSON 文件：使用 `-swagger-out /path/to/openapi.json`（可在不启用 UI 时导出）
- OpenAPI JSON 来自运行时 gin 路由（响应/模型为占位）
- UI 使用 `swagger-ui-dist` CDN 资源，离线环境需自行替换为本地资源

### 代理协议解析
`internal/parser/` 包含各协议解析器，统一接口：
- `parser.Parse(link string) (*Node, error)` - 解析单个节点链接
- 支持 Clash YAML 批量解析 (`clash_yaml.go`)
