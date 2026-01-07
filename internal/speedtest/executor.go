package speedtest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/database/models"
	"github.com/xiaobei/singbox-manager/internal/logger"
)

// TaskStatus 任务状态常量
const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusCancelled = "cancelled"
	TaskStatusFailed    = "failed"
)

// TriggerType 触发类型
const (
	TriggerTypeManual    = "manual"
	TriggerTypeScheduled = "scheduled"
)

// TestMode 测试模式
const (
	TestModeDelay = "delay" // 仅测延迟
	TestModeSpeed = "speed" // 延迟 + 速度
)

// Executor 测速任务执行器
type Executor struct {
	store *database.Store
	mu    sync.RWMutex
	// 运行中的任务
	runningTasks map[string]*RunningTask
	// 任务取消函数
	cancelFuncs map[string]context.CancelFunc
}

// RunningTask 运行中的任务
type RunningTask struct {
	Task    *models.SpeedTestTask
	Started time.Time
}

// TaskProgress 任务进度
type TaskProgress struct {
	TaskID      string `json:"task_id"`
	Status      string `json:"status"`
	Total       int    `json:"total"`
	Completed   int    `json:"completed"`
	Success     int    `json:"success"`
	Failed      int    `json:"failed"`
	CurrentNode string `json:"current_node,omitempty"`
	Error       string `json:"error,omitempty"`
}

// NewExecutor 创建执行器
func NewExecutor(store *database.Store) *Executor {
	return &Executor{
		store:        store,
		runningTasks: make(map[string]*RunningTask),
		cancelFuncs:  make(map[string]context.CancelFunc),
	}
}

// RunWithProfileConfig 使用指定策略配置执行测速
func (e *Executor) RunWithProfileConfig(profile *models.SpeedTestProfile, nodeIDs []uint, triggerType string) (*models.SpeedTestTask, error) {
	if profile == nil {
		return nil, fmt.Errorf("策略不能为空")
	}
	return e.runWithProfile(profile, nodeIDs, triggerType)
}

// RunWithProfile 使用策略执行测速
func (e *Executor) RunWithProfile(profileID uint, nodeIDs []uint, triggerType string) (*models.SpeedTestTask, error) {
	// 获取策略
	profile, err := e.store.GetSpeedTestProfile(profileID)
	if err != nil {
		return nil, fmt.Errorf("获取策略失败: %w", err)
	}

	return e.runWithProfile(profile, nodeIDs, triggerType)
}

func (e *Executor) runWithProfile(profile *models.SpeedTestProfile, nodeIDs []uint, triggerType string) (*models.SpeedTestTask, error) {
	// 获取节点列表
	var nodes []models.Node
	var err error
	if len(nodeIDs) > 0 {
		// 使用指定的节点
		for _, id := range nodeIDs {
			node, err := e.store.GetNode(id)
			if err == nil && node != nil {
				nodes = append(nodes, *node)
			}
		}
	} else {
		// 根据策略筛选条件获取节点
		nodes, err = e.getNodesByProfile(profile)
		if err != nil {
			return nil, fmt.Errorf("获取节点失败: %w", err)
		}
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("没有符合条件的节点")
	}

	// 创建任务
	task := &models.SpeedTestTask{
		ID:          uuid.New().String(),
		ProfileID:   &profile.ID,
		ProfileName: profile.Name,
		Status:      TaskStatusPending,
		TriggerType: triggerType,
		Total:       len(nodes),
	}

	now := time.Now()
	task.StartedAt = &now

	if err := e.store.CreateSpeedTestTask(task); err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	// 异步执行测速
	go e.executeTask(task, profile, nodes)

	return task, nil
}

// getNodesByProfile 根据策略筛选条件获取节点
func (e *Executor) getNodesByProfile(profile *models.SpeedTestProfile) ([]models.Node, error) {
	nodes, err := e.store.GetEnabledNodes()
	if err != nil {
		return nil, err
	}

	// 应用筛选条件
	var filtered []models.Node
	for _, node := range nodes {
		// 来源筛选
		if len(profile.SourceFilter) > 0 {
			matched := false
			for _, src := range profile.SourceFilter {
				if node.Source == src {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 国家筛选
		if len(profile.CountryFilter) > 0 {
			matched := false
			for _, country := range profile.CountryFilter {
				if node.Country == country {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// TODO: 标签筛选

		filtered = append(filtered, node)
	}

	return filtered, nil
}

// executeTask 执行测速任务
func (e *Executor) executeTask(task *models.SpeedTestTask, profile *models.SpeedTestProfile, nodes []models.Node) {
	logger.Info("开始执行测速任务 [%s], 节点数: %d, 策略: %s, 模式: %s",
		task.ID, len(nodes), profile.Name, profile.Mode)

	// 创建取消上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 注册任务
	e.mu.Lock()
	e.runningTasks[task.ID] = &RunningTask{
		Task:    task,
		Started: time.Now(),
	}
	e.cancelFuncs[task.ID] = cancel
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.runningTasks, task.ID)
		delete(e.cancelFuncs, task.ID)
		e.mu.Unlock()
	}()

	// 更新任务状态为运行中
	task.Status = TaskStatusRunning
	e.store.UpdateSpeedTestTask(task)

	// 创建测速器
	tester := NewTester(profile)

	// 并发控制
	var latencyConcurrency, speedConcurrency int

	if profile.LatencyConcurrency > 0 {
		latencyConcurrency = profile.LatencyConcurrency
	} else {
		latencyConcurrency = 50 // 默认延迟并发
	}
	if latencyConcurrency < 1 {
		latencyConcurrency = 1
	}
	if latencyConcurrency > 200 {
		latencyConcurrency = 200
	}

	if profile.SpeedConcurrency > 0 {
		speedConcurrency = profile.SpeedConcurrency
	} else {
		speedConcurrency = 5 // 默认速度并发
	}
	if speedConcurrency < 1 {
		speedConcurrency = 1
	}
	if speedConcurrency > 32 {
		speedConcurrency = 32
	}

	// 统计 (使用 mutex 保护，不需要 atomic)
	var successCount, failCount int
	var completedCount int
	var cancelled bool
	var mu sync.Mutex

	// 保存结果
	results := make([]TestResult, len(nodes))

	// ========== 阶段一: 延迟测试 ==========
	logger.Info("阶段一: 开始延迟测试, 并发数: %d", latencyConcurrency)

	sem := make(chan struct{}, latencyConcurrency)
	var wg sync.WaitGroup

	for i, node := range nodes {
		select {
		case <-ctx.Done():
			mu.Lock()
			cancelled = true
			mu.Unlock()
			break
		default:
		}

		if cancelled {
			break
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, n models.Node) {
			defer wg.Done()
			defer func() { <-sem }()

			select {
			case <-ctx.Done():
				return
			default:
			}

			// 执行延迟测试
			result := tester.TestDelay(&n)
			results[idx] = result

			mu.Lock()
			completedCount++

			if result.Status == "success" {
				successCount++
			} else {
				failCount++
			}

			// 更新任务进度
			task.Completed = completedCount
			task.Success = successCount
			task.Failed = failCount
			task.CurrentNode = n.Tag
			mu.Unlock()

			e.store.UpdateSpeedTestTask(task)
		}(i, node)
	}
	wg.Wait()

	logger.Info("阶段一完成: 延迟测试结束")

	// 检查是否被取消
	if cancelled || ctx.Err() != nil {
		task.Status = TaskStatusCancelled
		now := time.Now()
		task.FinishedAt = &now
		e.store.UpdateSpeedTestTask(task)
		logger.Info("任务已取消")
		goto SaveResults
	}

	// ========== 阶段二: 速度测试 (如果模式为 speed) ==========
	if profile.Mode == TestModeSpeed {
		logger.Info("阶段二: 开始速度测试, 并发数: %d", speedConcurrency)

		// 重置阶段二计数（用于进度展示，保留阶段一统计）
		speedCompletedCount := 0
		speedSem := make(chan struct{}, speedConcurrency)
		var speedWg sync.WaitGroup

		for i := range results {
			select {
			case <-ctx.Done():
				mu.Lock()
				cancelled = true
				mu.Unlock()
				break
			default:
			}

			if cancelled {
				break
			}

			// 跳过延迟测试失败的节点
			if results[i].Status != "success" {
				mu.Lock()
				speedCompletedCount++
				mu.Unlock()
				continue
			}

			speedWg.Add(1)
			speedSem <- struct{}{}

			go func(idx int) {
				defer speedWg.Done()
				defer func() { <-speedSem }()

				select {
				case <-ctx.Done():
					return
				default:
				}

				node := nodes[idx]
				result := tester.TestSpeed(&node)

				mu.Lock()
				results[idx] = result
				speedCompletedCount++

				// 阶段二进度：显示为 len(nodes) + speedCompletedCount，但不超过 2*len(nodes)
				task.Completed = len(nodes) + speedCompletedCount
				task.CurrentNode = node.Tag
				mu.Unlock()

				e.store.UpdateSpeedTestTask(task)
			}(i)
		}
		speedWg.Wait()

		logger.Info("阶段二完成: 速度测试结束")
	}

SaveResults:
	// 保存测速结果到节点表
	for i, result := range results {
		node := &nodes[i]
		now := time.Now()

		if result.Status == "success" {
			node.Delay = result.Delay
			node.DelayStatus = "success"
			if profile.Mode == TestModeSpeed && result.Speed > 0 {
				node.Speed = result.Speed
				node.SpeedStatus = "success"
			}
		} else if result.Status == "timeout" {
			node.Delay = -1
			node.DelayStatus = "timeout"
			node.Speed = 0
			node.SpeedStatus = "untested"
		} else {
			node.Delay = -1
			node.DelayStatus = "error"
			node.Speed = 0
			node.SpeedStatus = "untested"
		}

		if result.LandingIP != "" {
			node.LandingIP = result.LandingIP
		}

		node.TestedAt = &now
		e.store.UpdateNode(node)

		// 保存历史记录
		history := &models.SpeedTestHistory{
			TaskID:    task.ID,
			NodeID:    node.ID,
			Delay:     result.Delay,
			Speed:     result.Speed,
			Status:    result.Status,
			LandingIP: result.LandingIP,
		}
		e.store.CreateSpeedTestHistory(history)
	}

	// 更新任务完成状态
	if !cancelled && ctx.Err() == nil {
		task.Status = TaskStatusCompleted
	}
	now := time.Now()
	task.FinishedAt = &now
	e.store.UpdateSpeedTestTask(task)

	// 更新策略的上次执行时间
	profile.LastRunAt = &now
	e.store.UpdateSpeedTestProfile(profile)

	logger.Info("测速任务完成 - 总计: %d, 成功: %d, 失败: %d", len(nodes), successCount, failCount)
}

// CancelTask 取消任务
func (e *Executor) CancelTask(taskID string) error {
	e.mu.RLock()
	cancel, ok := e.cancelFuncs[taskID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("任务不存在或已完成")
	}

	cancel()
	return nil
}

// GetRunningTasks 获取运行中的任务
func (e *Executor) GetRunningTasks() []*models.SpeedTestTask {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tasks := make([]*models.SpeedTestTask, 0, len(e.runningTasks))
	for _, rt := range e.runningTasks {
		tasks = append(tasks, rt.Task)
	}
	return tasks
}

// IsTaskRunning 检查任务是否在运行
func (e *Executor) IsTaskRunning(taskID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, ok := e.runningTasks[taskID]
	return ok
}
