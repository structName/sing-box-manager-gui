# 统一任务系统重构计划

## 目标
用 TaskManager 完全替代 SpeedTestTask，实现单一任务系统，所有定时任务统一管理。

---

## Phase 1: 后端基础设施

### 1.1 废弃旧调度器
**文件变更**:
- `internal/service/scheduler.go` → 删除（旧订阅调度器）
- `internal/speedtest/scheduler.go` → 删除（旧测速调度器）
- `internal/api/router.go` → 移除 `scheduler` 字段和相关调用

**操作**:
```go
// router.go 移除
- scheduler      *service.Scheduler
- s.scheduler.Start()
- s.scheduler.Stop()
- s.scheduler.SetUpdateCallback()
- s.scheduler.Restart()
```

### 1.2 扩展 Task 模型
**文件**: `internal/database/models/task.go`

```go
// 新增字段支持测速结果关联
type Task struct {
    // ... 现有字段

    // 测速专用字段（存储在 Result JSON 中）
    // Result.profile_id, Result.total, Result.success, Result.failed
    // Result.current_node, Result.results ([]SpeedTestResult)
}

// 任务类型常量
const (
    TaskTypeSpeedTest   = "speed_test"
    TaskTypeSubUpdate   = "sub_update"
    TaskTypeChainCheck  = "chain_check"
    TaskTypeTagRule     = "tag_rule"
    TaskTypeConfigApply = "config_apply"
)

// 触发类型
const (
    TaskTriggerManual    = "manual"
    TaskTriggerScheduled = "scheduled"
    TaskTriggerEvent     = "event"
)
```

### 1.3 TaskManager 增强
**文件**: `internal/service/task_manager.go`

```go
// 新增 SSE 广播能力
type TaskManager struct {
    // ... 现有字段
    subscribers map[string]chan *models.Task  // SSE 订阅者
    subMu       sync.RWMutex
}

// 新增方法
func (tm *TaskManager) Subscribe(clientID string) <-chan *models.Task
func (tm *TaskManager) Unsubscribe(clientID string)
func (tm *TaskManager) broadcast(task *models.Task)
```

---

## Phase 2: 测速模块迁移

### 2.1 改造 Executor
**文件**: `internal/speedtest/executor.go`

**变更**:
- 注入 `TaskManager` 依赖
- `RunWithProfile` 改为创建统一 Task
- 进度更新通过 `TaskManager.UpdateProgress`
- 结果存储到 `Task.Result` JSON

```go
type Executor struct {
    store       *database.Store
    taskManager *service.TaskManager  // 新增
    // 移除 runningTasks, cancelFuncs（由 TaskManager 管理）
}

func (e *Executor) RunWithProfile(profileID uint, nodeIDs []uint, trigger string) (*models.Task, error) {
    // 1. 创建统一 Task
    task, ctx, _ := e.taskManager.CreateTask(
        models.TaskTypeSpeedTest,
        "测速: " + profile.Name,
        trigger,
        len(nodes),
    )

    // 2. 启动任务
    e.taskManager.StartTask(task.ID)

    // 3. 异步执行，使用 ctx 支持取消
    go e.executeTask(ctx, task, profile, nodes)

    return task, nil
}
```

### 2.2 废弃 SpeedTestTask
**文件**: `internal/database/models/speedtest.go`

- 保留 `SpeedTestProfile` 和 `SpeedTestResult`
- 删除 `SpeedTestTask` 结构体
- 更新 `SpeedTestResult.TaskID` 关联到统一 Task

### 2.3 更新 API
**文件**: `internal/api/speedtest.go`

- `GetTasks` → 改为查询统一 Task 表（type=speed_test）
- `CancelTask` → 调用 `TaskManager.CancelTask`
- 移除 `speedTestScheduler` 依赖

---

## Phase 3: 定时任务统一接入

### 3.1 订阅更新
**文件**: `internal/api/router.go:initScheduleEntries`

```go
// 改造前
func() {
    s.subService.Refresh(subID)
    s.syncNodesToSQLite()
    s.autoApplyConfig()
}

// 改造后
func() {
    task, ctx, _ := s.taskManager.CreateTask(
        models.TaskTypeSubUpdate,
        "订阅更新: "+subName,
        models.TaskTriggerScheduled,
        1,
    )
    s.taskManager.StartTask(task.ID)

    if err := s.subService.Refresh(subID); err != nil {
        s.taskManager.FailTask(task.ID, err.Error())
        return
    }
    s.syncNodesToSQLite()
    s.autoApplyConfig()
    s.taskManager.CompleteTask(task.ID, "更新成功", nil)
}
```

### 3.2 链路健康检测
**文件**: `internal/api/router.go:initScheduleEntries`

```go
func() {
    chains := s.store.GetProxyChains()
    enabledChains := filterEnabled(chains)

    task, _, _ := s.taskManager.CreateTask(
        models.TaskTypeChainCheck,
        "链路健康检测",
        models.TaskTriggerScheduled,
        len(enabledChains),
    )
    s.taskManager.StartTask(task.ID)

    results := make(map[string]interface{})
    for i, chain := range enabledChains {
        s.taskManager.UpdateProgress(task.ID, i+1, chain.Name, "")
        status := s.healthCheckSvc.CheckChain(chain.ID)
        results[chain.ID] = status
    }

    s.taskManager.CompleteTask(task.ID, "检测完成", results)
}
```

### 3.3 标签规则定时
**文件**: `internal/database/models/tag.go`

```go
// TagRule 新增定时字段
type TagRule struct {
    // ... 现有字段
    ScheduleEnabled bool       `json:"schedule_enabled"`
    ScheduleCron    string     `json:"schedule_cron"`
    NextRunAt       *time.Time `json:"next_run_at,omitempty"`
}
```

**文件**: `internal/api/router.go:initScheduleEntries`

```go
// 新增标签规则定时调度
rules, _ := s.dbStore.GetTagRules()
for _, rule := range rules {
    if rule.ScheduleEnabled && rule.ScheduleCron != "" {
        ruleID := rule.ID
        ruleName := rule.Name
        s.unifiedScheduler.AddSchedule(
            service.ScheduleTypeTagRule,
            fmt.Sprintf("%d", ruleID),
            "标签规则: "+ruleName,
            rule.ScheduleCron,
            func() {
                task, _, _ := s.taskManager.CreateTask(
                    models.TaskTypeTagRule,
                    "应用规则: "+ruleName,
                    models.TaskTriggerScheduled,
                    0,
                )
                s.taskManager.StartTask(task.ID)
                result := s.tagEngine.ApplyRule(ruleID)
                s.taskManager.CompleteTask(task.ID, "应用完成", result)
            },
        )
    }
}
```

### 3.4 配置自动应用
**文件**: `internal/api/router.go`

```go
// autoApplyConfig 改造
func (s *Server) autoApplyConfig() error {
    settings := s.store.GetSettings()
    if !settings.AutoApply {
        return nil
    }

    // 创建任务记录
    task, _, _ := s.taskManager.CreateTask(
        models.TaskTypeConfigApply,
        "配置自动应用",
        models.TaskTriggerEvent,
        0,
    )
    s.taskManager.StartTask(task.ID)

    // 生成配置
    configJSON, err := s.buildConfig()
    if err != nil {
        s.taskManager.FailTask(task.ID, err.Error())
        return err
    }

    // 写入配置
    if err := s.writeConfig(configJSON); err != nil {
        s.taskManager.FailTask(task.ID, err.Error())
        return err
    }

    // 重启 sing-box
    if s.processManager.IsRunning() {
        if err := s.processManager.Restart(); err != nil {
            s.taskManager.FailTask(task.ID, err.Error())
            return err
        }
    }

    s.taskManager.CompleteTask(task.ID, "配置已应用", nil)
    return nil
}
```

---

## Phase 4: SSE 广播

### 4.1 后端 SSE 端点
**文件**: `internal/api/events.go` (新建)

```go
// EventsHandler SSE 事件处理器
type EventsHandler struct {
    taskManager *service.TaskManager
}

// Stream SSE 流
func (h *EventsHandler) Stream(c *gin.Context) {
    clientID := uuid.New().String()
    ch := h.taskManager.Subscribe(clientID)
    defer h.taskManager.Unsubscribe(clientID)

    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")

    c.Stream(func(w io.Writer) bool {
        select {
        case task := <-ch:
            data, _ := json.Marshal(task)
            c.SSEvent("task.update", string(data))
            return true
        case <-c.Request.Context().Done():
            return false
        }
    })
}
```

**路由**: `internal/api/router.go`
```go
api.GET("/events/stream", s.eventsHandler.Stream)
```

### 4.2 TaskManager 广播集成
**文件**: `internal/service/task_manager.go`

在 `UpdateProgress`, `CompleteTask`, `FailTask`, `CancelTask` 中调用 `broadcast(task)`

---

## Phase 5: 前端重构

### 5.1 目录结构
```
web/src/
├── features/
│   ├── tasks/
│   │   ├── components/
│   │   │   ├── ActiveTaskCard.tsx
│   │   │   └── TaskHistoryTable.tsx
│   │   └── hooks/
│   │       └── useTaskEvents.ts
│   └── scheduler/
│       └── components/
│           └── SchedulerList.tsx
├── pages/
│   └── Tasks.tsx
└── store/
    └── taskStore.ts
```

### 5.2 Store 改造
**文件**: `web/src/store/taskStore.ts`

```typescript
interface TaskState {
  tasks: Task[];
  activeTasks: Task[];

  upsertTask: (task: Task) => void;
  connectSSE: () => void;
  disconnectSSE: () => void;
}
```

### 5.3 SSE Hook
**文件**: `web/src/features/tasks/hooks/useTaskEvents.ts`

```typescript
export function useTaskEvents() {
  const { upsertTask } = useTaskStore();

  useEffect(() => {
    const es = new EventSource('/api/events/stream');

    es.addEventListener('task.update', (e) => {
      const task = JSON.parse(e.data);
      upsertTask(task);

      if (task.status === 'completed') {
        toast.success(`任务完成: ${task.name}`);
      } else if (task.status === 'error') {
        toast.error(`任务失败: ${task.name}`);
      }
    });

    return () => es.close();
  }, []);
}
```

---

## 实施顺序

| 阶段 | 任务 | 依赖 |
|------|------|------|
| P1.1 | 废弃旧调度器 | - |
| P1.2 | 扩展 Task 模型 | - |
| P1.3 | TaskManager 增强 | P1.2 |
| P2.1 | 改造 Executor | P1.3 |
| P2.2 | 废弃 SpeedTestTask | P2.1 |
| P2.3 | 更新测速 API | P2.2 |
| P3.1 | 订阅更新接入 | P1.3 |
| P3.2 | 链路检测接入 | P1.3 |
| P3.3 | 标签规则定时 | P1.3 |
| P3.4 | 配置自动应用 | P1.3 |
| P4.1 | SSE 端点 | P1.3 |
| P4.2 | 广播集成 | P4.1 |
| P5.1 | 前端目录重构 | - |
| P5.2 | Store 改造 | P5.1 |
| P5.3 | SSE Hook | P4.1, P5.2 |

---

## 数据迁移

### SpeedTestTask → Task 迁移
```sql
-- 迁移现有测速任务记录
INSERT INTO tasks (id, type, name, status, trigger, progress, total, result, started_at, completed_at, created_at)
SELECT
    id,
    'speed_test',
    profile_name,
    status,
    trigger_type,
    completed,
    total,
    json_object('profile_id', profile_id, 'success', success, 'failed', failed),
    started_at,
    completed_at,
    started_at
FROM speed_test_tasks;
```

---

## API 兼容性

| 原 API | 新 API | 说明 |
|--------|--------|------|
| GET /api/speedtest/tasks | GET /api/tasks?type=speed_test | 统一任务列表 |
| POST /api/speedtest/tasks/:id/cancel | POST /api/tasks/:id/cancel | 统一取消接口 |
| GET /api/speedtest/tasks/:id | GET /api/tasks/:id | 统一详情接口 |

前端需同步更新 API 调用路径。
