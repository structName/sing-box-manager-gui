# 任务管理系统重构计划

## 1. 现状分析

### 1.1 现有分散的调度器
1. **service/scheduler.go**: 简单的订阅定时刷新（固定间隔 ticker）
2. **speedtest/scheduler.go**: 测速策略定时调度（使用 robfig/cron）
3. **service/health_check.go**: 链路健康检测（固定间隔 ticker）
4. **service/tag_engine.go**: 标签规则引擎（无调度，被动触发）

### 1.2 问题
- 调度器分散，无法统一管理
- 任务没有统一模型，无法追踪进度和历史
- 缺少定时标签任务
- 配置变更后无法智能触发关联任务

## 2. 参考 sublinkPro 设计

### 2.1 核心设计
- **Task 模型**: 统一任务表（ID、类型、状态、进度、结果）
- **TaskManager**: 任务生命周期管理、进度广播、取消能力
- **Scheduler**: 基于 cron 的统一调度器
- **SSE 集成**: 实时推送任务进度

### 2.2 任务类型
```go
const (
    TaskTypeSpeedTest   = "speed_test"      // 节点测速
    TaskTypeSubUpdate   = "sub_update"      // 订阅更新
    TaskTypeTagRule     = "tag_rule"        // 标签规则
    TaskTypeChainCheck  = "chain_check"     // 链路检测
    TaskTypeConfigApply = "config_apply"    // 配置应用
)
```

## 3. 重构方案

### 3.1 新增模块

#### 3.1.1 Task 模型 (`internal/database/models/task.go`)
```go
type Task struct {
    ID          string     `gorm:"primaryKey" json:"id"`
    Type        string     `gorm:"index" json:"type"`
    Name        string     `json:"name"`
    Status      string     `gorm:"index" json:"status"`       // pending/running/completed/cancelled/error
    Trigger     string     `json:"trigger"`                    // manual/scheduled/event

    Progress    int        `json:"progress"`
    Total       int        `json:"total"`
    CurrentItem string     `json:"current_item"`
    Message     string     `json:"message"`
    Result      string     `gorm:"type:text" json:"result"`    // JSON

    StartedAt   *time.Time `json:"started_at"`
    CompletedAt *time.Time `json:"completed_at"`
    CreatedAt   time.Time  `json:"created_at"`
}
```

#### 3.1.2 TaskManager (`internal/service/task_manager.go`)
```go
type TaskManager struct {
    store        *database.Store
    runningTasks map[string]*RunningTask
    mu           sync.RWMutex
}

// 核心方法
func (tm *TaskManager) CreateTask(taskType, name, trigger string, total int) (*Task, context.Context, error)
func (tm *TaskManager) UpdateProgress(taskID string, progress int, currentItem string, result interface{}) error
func (tm *TaskManager) CompleteTask(taskID string, message string, result interface{}) error
func (tm *TaskManager) FailTask(taskID string, errMsg string) error
func (tm *TaskManager) CancelTask(taskID string) error
```

#### 3.1.3 统一调度器 (`internal/service/unified_scheduler.go`)
```go
type UnifiedScheduler struct {
    cron         *cron.Cron
    store        *database.Store
    taskManager  *TaskManager
    entryMap     map[string]cron.EntryID  // scheduleKey -> entryID

    // 回调
    onSpeedTest       func(profileID uint, trigger string) error
    onSubscriptionRefresh func(subID string) error
    onChainCheck      func(chainID string) error
    onTagApply        func(triggerType string, nodeIDs []uint) error
    onConfigApply     func() error
}
```

### 3.2 调度任务类型

#### 3.2.1 定时测速 (SpeedTest)
- 来源: SpeedTestProfile.AutoTest + ScheduleType/ScheduleInterval/ScheduleCron
- 粒度: 每个策略独立调度
- 测试内容: 延迟/下载速度

#### 3.2.2 定时订阅 (SubUpdate)
- 来源: Subscription.AutoUpdate + UpdateInterval
- 粒度: 每个订阅独立调度
- 后置动作: 触发标签规则

#### 3.2.3 定时链路检测 (ChainCheck)
- 来源: Settings.ChainHealthConfig.Enabled + Interval
- 粒度: 全局统一调度，检测所有启用的链路
- 检测内容: TCP 连通性、HTTP 延迟、速度

#### 3.2.4 定时打标签 (TagRule)
- **新增**: TagRule 表增加定时触发能力
- 来源: TagRule.ScheduleEnabled + ScheduleCron
- 粒度: 每条规则独立调度，或按触发类型批量

### 3.3 配置变更自动应用

#### 3.3.1 配置文件监控器 (`internal/service/config_watcher.go`)
```go
type ConfigWatcher struct {
    configPath     string
    lastModTime    time.Time
    autoRestart    bool          // 是否自动重启 sing-box（可配置开关）
    debounceDelay  time.Duration // 防抖延迟（避免频繁重启）

    onConfigChange func()        // 配置变更回调
    restartFunc    func() error  // sing-box 重启函数

    stopCh         chan struct{}
    mu             sync.Mutex
}

// 核心方法
func (w *ConfigWatcher) Start()                    // 启动监控
func (w *ConfigWatcher) Stop()                     // 停止监控
func (w *ConfigWatcher) SetAutoRestart(enabled bool) // 设置是否自动重启
func (w *ConfigWatcher) TriggerRestart()           // 手动触发重启
func (w *ConfigWatcher) NotifyConfigChanged()      // 通知配置已变更（由事件触发器调用）
```

#### 3.3.2 事件触发器 (`internal/service/event_trigger.go`)
```go
type EventTrigger struct {
    taskManager    *TaskManager
    configBuilder  func() error      // 配置生成器
    configWatcher  *ConfigWatcher    // 配置监控器
    tagEngine      *TagEngine        // 标签引擎
}

// 事件处理（只负责生成配置，不负责重启）
func (e *EventTrigger) OnSubscriptionUpdate(subID string)   // 订阅刷新后 -> 应用标签 -> 生成配置
func (e *EventTrigger) OnNodeChange(nodeID uint)            // 节点变更后 -> 生成配置
func (e *EventTrigger) OnRuleChange(ruleID string)          // 规则变更后 -> 生成配置
func (e *EventTrigger) OnChainChange(chainID string)        // 链路变更后 -> 生成配置
func (e *EventTrigger) OnSpeedTestComplete(nodeIDs []uint)  // 测速完成后 -> 应用标签
```

#### 3.3.3 变更触发场景（解耦设计）
| 触发源 | 动作 | 配置监控器 |
|--------|------|-----------|
| 订阅刷新完成 | 应用标签规则 → 重新生成配置 | 检测到配置变更 → 根据开关决定是否重启 |
| 节点信息变更 | 重新生成配置 | 同上 |
| 规则变更 | 重新生成配置 | 同上 |
| 链路变更 | 重新生成配置 | 同上 |
| 测速完成 | 应用标签规则 | 无配置变更 |

#### 3.3.4 Settings 新增字段
```go
type Settings struct {
    // ... 现有字段

    // 配置变更自动应用
    AutoApplyConfig    bool `json:"auto_apply_config"`     // 配置变更后自动重启 sing-box
    ApplyDebounceDelay int  `json:"apply_debounce_delay"`  // 防抖延迟（秒），默认 3 秒
}
```

### 3.4 API 设计

#### 3.4.1 任务管理 API
```
GET  /api/tasks                    # 获取任务列表（支持分页、过滤）
GET  /api/tasks/:id                # 获取任务详情
POST /api/tasks/:id/cancel         # 取消任务
GET  /api/tasks/running            # 获取运行中任务
GET  /api/tasks/stats              # 获取任务统计
DELETE /api/tasks/history          # 清理历史任务
```

#### 3.4.2 调度管理 API
```
GET  /api/scheduler/status         # 获取调度器状态
GET  /api/scheduler/entries        # 获取所有调度条目
POST /api/scheduler/pause          # 暂停调度器
POST /api/scheduler/resume         # 恢复调度器
```

## 4. 实施步骤

### Phase 1: 基础设施
1. 创建 Task 模型和数据库迁移
2. 实现 TaskManager 核心逻辑
3. 集成 SSE 广播

### Phase 2: 统一调度器
1. 创建 UnifiedScheduler
2. 迁移测速调度逻辑
3. 迁移订阅调度逻辑
4. 添加链路检测定时调度
5. 添加标签规则定时调度

### Phase 3: 事件触发
1. 创建 EventTrigger
2. 在订阅刷新后触发配置更新
3. 在节点变更后触发配置更新
4. 在规则/链路变更后触发配置更新

### Phase 4: API 和前端
1. 添加任务管理 API
2. 添加调度管理 API
3. 前端任务列表页面

## 5. 数据模型变更

### 5.1 新增表
- `tasks`: 任务记录表

### 5.2 修改表
- `tag_rules`: 添加定时调度字段
  ```go
  ScheduleEnabled bool   `json:"schedule_enabled"`
  ScheduleCron    string `json:"schedule_cron"`
  NextRunAt       *time.Time
  ```

- `subscriptions` (storage.Subscription): 添加独立调度字段
  ```go
  AutoUpdate     bool  `json:"auto_update"`
  UpdateInterval int   `json:"update_interval"`  // 分钟
  NextUpdateAt   *time.Time
  ```

## 6. 兼容性考虑

- 保留现有 SpeedTestProfile 的调度字段
- 旧版 Settings.SubscriptionInterval 迁移到各订阅独立配置
- 渐进式迁移，不影响现有功能
