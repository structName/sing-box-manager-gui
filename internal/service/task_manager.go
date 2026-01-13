package service

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

// RunningTask 运行中的任务
type RunningTask struct {
	Task   *models.Task
	Cancel context.CancelFunc
	Ctx    context.Context
}

// TaskManager 任务管理器
type TaskManager struct {
	store        *database.Store
	runningTasks map[string]*RunningTask
	mu           sync.RWMutex

	// SSE 订阅者
	subscribers map[string]chan *models.Task
	subMu       sync.RWMutex
}

// NewTaskManager 创建任务管理器
func NewTaskManager(store *database.Store) *TaskManager {
	return &TaskManager{
		store:        store,
		runningTasks: make(map[string]*RunningTask),
		subscribers:  make(map[string]chan *models.Task),
	}
}

// Subscribe 订阅任务更新
func (tm *TaskManager) Subscribe(clientID string) <-chan *models.Task {
	tm.subMu.Lock()
	defer tm.subMu.Unlock()

	ch := make(chan *models.Task, 10)
	tm.subscribers[clientID] = ch
	logger.Info("SSE 客户端已订阅: %s", clientID)
	return ch
}

// Unsubscribe 取消订阅
func (tm *TaskManager) Unsubscribe(clientID string) {
	tm.subMu.Lock()
	defer tm.subMu.Unlock()

	if ch, ok := tm.subscribers[clientID]; ok {
		close(ch)
		delete(tm.subscribers, clientID)
		logger.Info("SSE 客户端已取消订阅: %s", clientID)
	}
}

// broadcast 广播任务更新
func (tm *TaskManager) broadcast(task *models.Task) {
	tm.subMu.RLock()
	defer tm.subMu.RUnlock()

	for _, ch := range tm.subscribers {
		select {
		case ch <- task:
		default:
			// 通道满了，跳过
		}
	}
}

// CreateTask 创建任务
func (tm *TaskManager) CreateTask(taskType, name, trigger string, total int) (*models.Task, context.Context, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	task := &models.Task{
		ID:        uuid.New().String(),
		Type:      taskType,
		Name:      name,
		Status:    models.TaskStatusPending,
		Trigger:   trigger,
		Progress:  0,
		Total:     total,
		CreatedAt: time.Now(),
	}

	if err := tm.store.CreateTask(task); err != nil {
		return nil, nil, fmt.Errorf("创建任务失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	tm.runningTasks[task.ID] = &RunningTask{
		Task:   task,
		Cancel: cancel,
		Ctx:    ctx,
	}

	logger.Info("任务已创建: [%s] %s (%s)", task.Type, task.Name, task.ID)
	tm.broadcast(task)
	return task, ctx, nil
}

// StartTask 开始任务
func (tm *TaskManager) StartTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	running, ok := tm.runningTasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	now := time.Now()
	running.Task.Status = models.TaskStatusRunning
	running.Task.StartedAt = &now

	if err := tm.store.UpdateTask(running.Task); err != nil {
		return err
	}
	tm.broadcast(running.Task)
	return nil
}

// UpdateProgress 更新任务进度
func (tm *TaskManager) UpdateProgress(taskID string, progress int, currentItem string, message string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	running, ok := tm.runningTasks[taskID]
	if !ok {
		// 任务可能已完成，从数据库获取
		task, err := tm.store.GetTask(taskID)
		if err != nil {
			return fmt.Errorf("任务不存在: %s", taskID)
		}
		task.Progress = progress
		task.CurrentItem = currentItem
		task.Message = message
		if err := tm.store.UpdateTask(task); err != nil {
			return err
		}
		tm.broadcast(task)
		return nil
	}

	running.Task.Progress = progress
	running.Task.CurrentItem = currentItem
	running.Task.Message = message

	if err := tm.store.UpdateTask(running.Task); err != nil {
		return err
	}
	tm.broadcast(running.Task)
	return nil
}

// CompleteTask 完成任务
func (tm *TaskManager) CompleteTask(taskID string, message string, result map[string]interface{}) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	running, ok := tm.runningTasks[taskID]
	if !ok {
		task, err := tm.store.GetTask(taskID)
		if err != nil {
			return fmt.Errorf("任务不存在: %s", taskID)
		}
		now := time.Now()
		task.Status = models.TaskStatusCompleted
		task.CompletedAt = &now
		task.Message = message
		task.Result = models.JSONMap(result)
		task.Progress = task.Total
		if err := tm.store.UpdateTask(task); err != nil {
			return err
		}
		tm.broadcast(task)
		return nil
	}

	now := time.Now()
	running.Task.Status = models.TaskStatusCompleted
	running.Task.CompletedAt = &now
	running.Task.Message = message
	running.Task.Result = models.JSONMap(result)
	running.Task.Progress = running.Task.Total

	if err := tm.store.UpdateTask(running.Task); err != nil {
		return err
	}

	tm.broadcast(running.Task)
	delete(tm.runningTasks, taskID)
	logger.Info("任务已完成: [%s] %s", running.Task.Type, running.Task.Name)
	return nil
}

// FailTask 任务失败
func (tm *TaskManager) FailTask(taskID string, errMsg string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	running, ok := tm.runningTasks[taskID]
	if !ok {
		task, err := tm.store.GetTask(taskID)
		if err != nil {
			return fmt.Errorf("任务不存在: %s", taskID)
		}
		now := time.Now()
		task.Status = models.TaskStatusError
		task.CompletedAt = &now
		task.Message = errMsg
		if err := tm.store.UpdateTask(task); err != nil {
			return err
		}
		tm.broadcast(task)
		return nil
	}

	now := time.Now()
	running.Task.Status = models.TaskStatusError
	running.Task.CompletedAt = &now
	running.Task.Message = errMsg

	if err := tm.store.UpdateTask(running.Task); err != nil {
		return err
	}

	delete(tm.runningTasks, taskID)
	logger.Error("任务失败: [%s] %s - %s", running.Task.Type, running.Task.Name, errMsg)
	return nil
}

// CancelTask 取消任务
func (tm *TaskManager) CancelTask(taskID string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	running, ok := tm.runningTasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在或已完成: %s", taskID)
	}

	// 取消 context
	running.Cancel()

	now := time.Now()
	running.Task.Status = models.TaskStatusCancelled
	running.Task.CompletedAt = &now
	running.Task.Message = "用户取消"

	if err := tm.store.UpdateTask(running.Task); err != nil {
		return err
	}

	tm.broadcast(running.Task)
	delete(tm.runningTasks, taskID)
	logger.Info("任务已取消: [%s] %s", running.Task.Type, running.Task.Name)
	return nil
}

// GetRunningTasks 获取运行中的任务
func (tm *TaskManager) GetRunningTasks() []*models.Task {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*models.Task, 0, len(tm.runningTasks))
	for _, rt := range tm.runningTasks {
		tasks = append(tasks, rt.Task)
	}
	return tasks
}

// GetTaskContext 获取任务的 context
func (tm *TaskManager) GetTaskContext(taskID string) (context.Context, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	running, ok := tm.runningTasks[taskID]
	if !ok {
		return nil, false
	}
	return running.Ctx, true
}

// GetStats 获取任务统计
func (tm *TaskManager) GetStats() map[string]int {
	tasks, _ := tm.store.GetTasks(100, 0, "", "")
	stats := map[string]int{
		"total":     len(tasks),
		"running":   0,
		"pending":   0,
		"completed": 0,
		"failed":    0,
	}

	for _, task := range tasks {
		switch task.Status {
		case models.TaskStatusRunning:
			stats["running"]++
		case models.TaskStatusPending:
			stats["pending"]++
		case models.TaskStatusCompleted:
			stats["completed"]++
		case models.TaskStatusError, models.TaskStatusCancelled:
			stats["failed"]++
		}
	}

	return stats
}

// CleanupOldTasks 清理旧任务
func (tm *TaskManager) CleanupOldTasks(before time.Time) error {
	return tm.store.DeleteTasksBefore(before)
}
