package speedtest

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

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

// TaskManagerInterface 任务管理器接口（避免循环依赖）
type TaskManagerInterface interface {
	CreateTask(taskType, name, trigger string, total int) (*models.Task, context.Context, error)
	StartTask(taskID string) error
	UpdateProgress(taskID string, progress int, currentItem string, message string) error
	CompleteTask(taskID string, message string, result map[string]any) error
	FailTask(taskID string, errMsg string) error
	CancelTask(taskID string) error
	GetTaskContext(taskID string) (context.Context, bool)
}

// Executor 测速任务执行器
type Executor struct {
	store       *database.Store
	taskManager TaskManagerInterface
}

// NewExecutor 创建执行器
func NewExecutor(store *database.Store) *Executor {
	return &Executor{
		store: store,
	}
}

// Rebind 更新内部 store 引用（用于 Profile 切换）
func (e *Executor) Rebind(store *database.Store) {
	e.store = store
}

// SetTaskManager 设置任务管理器
func (e *Executor) SetTaskManager(tm TaskManagerInterface) {
	e.taskManager = tm
}

// RunWithProfileConfig 使用指定策略配置执行测速
func (e *Executor) RunWithProfileConfig(profile *models.SpeedTestProfile, nodeIDs []uint, triggerType string) (*models.Task, error) {
	if profile == nil {
		return nil, fmt.Errorf("策略不能为空")
	}
	return e.runWithProfile(profile, nodeIDs, triggerType)
}

// RunWithProfile 使用策略执行测速
func (e *Executor) RunWithProfile(profileID uint, nodeIDs []uint, triggerType string) (*models.Task, error) {
	// 获取策略
	profile, err := e.store.GetSpeedTestProfile(profileID)
	if err != nil {
		return nil, fmt.Errorf("获取策略失败: %w", err)
	}

	return e.runWithProfile(profile, nodeIDs, triggerType)
}

func (e *Executor) runWithProfile(profile *models.SpeedTestProfile, nodeIDs []uint, triggerType string) (*models.Task, error) {
	if e.taskManager == nil {
		return nil, fmt.Errorf("任务管理器未初始化")
	}

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

	// 使用 TaskManager 创建统一任务
	task, ctx, err := e.taskManager.CreateTask(
		models.TaskTypeSpeedTest,
		"测速: "+profile.Name,
		triggerType,
		len(nodes),
	)
	if err != nil {
		return nil, fmt.Errorf("创建任务失败: %w", err)
	}

	// 启动任务
	e.taskManager.StartTask(task.ID)

	// 异步执行测速
	go e.executeTask(ctx, task, profile, nodes)

	return task, nil
}

// getNodesByProfile 根据策略筛选条件获取节点
func (e *Executor) getNodesByProfile(profile *models.SpeedTestProfile) ([]models.Node, error) {
	nodes, err := e.store.GetEnabledNodes()
	if err != nil {
		return nil, err
	}

	// 无筛选条件时直接返回
	if len(profile.SourceFilter) == 0 && len(profile.CountryFilter) == 0 {
		return nodes, nil
	}

	// 应用筛选条件
	var filtered []models.Node
	for _, node := range nodes {
		if len(profile.SourceFilter) > 0 && !slices.Contains(profile.SourceFilter, node.Source) {
			continue
		}
		if len(profile.CountryFilter) > 0 && !slices.Contains(profile.CountryFilter, node.Country) {
			continue
		}
		// TODO: 标签筛选
		filtered = append(filtered, node)
	}

	return filtered, nil
}

// clamp 将值限制在指定范围内
func clamp(val, min, max int) int {
	if val <= 0 || val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

// executeTask 执行测速任务
func (e *Executor) executeTask(ctx context.Context, task *models.Task, profile *models.SpeedTestProfile, nodes []models.Node) {
	logger.Info("开始执行测速任务 [%s], 节点数: %d, 策略: %s, 模式: %s",
		task.ID, len(nodes), profile.Name, profile.Mode)

	// 创建测速器
	tester := NewTester(profile)

	// 并发控制
	latencyConcurrency := clamp(profile.LatencyConcurrency, 50, 200)
	speedConcurrency := clamp(profile.SpeedConcurrency, 5, 32)

	// 统计
	var successCount, failCount, completedCount int
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
			progress := completedCount
			currentNode := n.Tag
			mu.Unlock()

			// 通过 TaskManager 更新进度
			e.taskManager.UpdateProgress(task.ID, progress, currentNode, "")
		}(i, node)
	}
	wg.Wait()

	logger.Info("阶段一完成: 延迟测试结束")

	// 检查是否被取消
	if cancelled || ctx.Err() != nil {
		e.taskManager.FailTask(task.ID, "任务已取消")
		logger.Info("任务已取消")
		goto SaveResults
	}

	// ========== 阶段二: 速度测试 (如果模式为 speed) ==========
	if profile.Mode == TestModeSpeed {
		logger.Info("阶段二: 开始速度测试, 并发数: %d", speedConcurrency)

		speedCompletedCount := 0
		speedSem := make(chan struct{}, speedConcurrency)
		var speedWg sync.WaitGroup

		for i := range results {
			select {
			case <-ctx.Done():
				mu.Lock()
				cancelled = true
				mu.Unlock()
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
				progress := len(nodes) + speedCompletedCount
				currentNode := node.Tag
				mu.Unlock()

				e.taskManager.UpdateProgress(task.ID, progress, currentNode, "")
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

	// 更新策略的上次执行时间
	now := time.Now()
	profile.LastRunAt = &now
	e.store.UpdateSpeedTestProfile(profile)

	// 完成任务
	if !cancelled && ctx.Err() == nil {
		e.taskManager.CompleteTask(task.ID, "测速完成", map[string]any{
			"profile_id": profile.ID,
			"total":      len(nodes),
			"success":    successCount,
			"failed":     failCount,
		})
	}

	logger.Info("测速任务完成 - 总计: %d, 成功: %d, 失败: %d", len(nodes), successCount, failCount)
}

// CancelTask 取消任务（委托给 TaskManager）
func (e *Executor) CancelTask(taskID string) error {
	if e.taskManager == nil {
		return fmt.Errorf("任务管理器未初始化")
	}
	return e.taskManager.CancelTask(taskID)
}

// GetRunningTasks 获取运行中的任务（返回空，由 TaskManager 管理）
func (e *Executor) GetRunningTasks() []*models.Task {
	return nil
}

// IsTaskRunning 检查任务是否在运行（委托给 TaskManager）
func (e *Executor) IsTaskRunning(taskID string) bool {
	if e.taskManager == nil {
		return false
	}
	_, ok := e.taskManager.GetTaskContext(taskID)
	return ok
}
