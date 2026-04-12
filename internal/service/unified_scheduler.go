package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/logger"
)

// ScheduleType 调度类型
type ScheduleType string

const (
	ScheduleTypeSpeedTest   ScheduleType = "speed_test"
	ScheduleTypeSubUpdate   ScheduleType = "sub_update"
	ScheduleTypeChainCheck  ScheduleType = "chain_check"
	ScheduleTypeTagRule     ScheduleType = "tag_rule"
	ScheduleTypeConfigWatch ScheduleType = "config_watch"
)

// ScheduleEntry 调度条目
type ScheduleEntry struct {
	Key      string       `json:"key"`
	Type     ScheduleType `json:"type"`
	Name     string       `json:"name"`
	CronExpr string       `json:"cron_expr"`
	Enabled  bool         `json:"enabled"`
	NextRun  *time.Time   `json:"next_run"`
	LastRun  *time.Time   `json:"last_run"`
	EntryID  cron.EntryID `json:"-"`
	handler  func()
}

// UnifiedScheduler 统一调度器
type UnifiedScheduler struct {
	cron        *cron.Cron
	store       *database.Store
	taskManager *TaskManager
	mu          sync.RWMutex

	entries map[string]*ScheduleEntry // key -> entry
	started bool

	// 回调函数
	onSpeedTest   func(profileID uint, trigger string) error
	onSubRefresh  func(subID string) error
	onChainCheck  func() error
	onTagApply    func(triggerType string) error
}

// NewUnifiedScheduler 创建统一调度器
func NewUnifiedScheduler(store *database.Store, taskManager *TaskManager) *UnifiedScheduler {
	return &UnifiedScheduler{
		cron:        cron.New(cron.WithSeconds()),
		store:       store,
		taskManager: taskManager,
		entries:     make(map[string]*ScheduleEntry),
	}
}

// SetCallbacks 设置回调函数
func (s *UnifiedScheduler) SetCallbacks(
	onSpeedTest func(profileID uint, trigger string) error,
	onSubRefresh func(subID string) error,
	onChainCheck func() error,
	onTagApply func(triggerType string) error,
) {
	s.onSpeedTest = onSpeedTest
	s.onSubRefresh = onSubRefresh
	s.onChainCheck = onChainCheck
	s.onTagApply = onTagApply
}

// Start 启动调度器
func (s *UnifiedScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	s.cron.Start()
	s.started = true
	logger.Info("统一调度器已启动")
	return nil
}

// Stop 停止调度器
func (s *UnifiedScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()
	s.started = false
	logger.Info("统一调度器已停止")
}

// AddSchedule 添加调度
func (s *UnifiedScheduler) AddSchedule(scheduleType ScheduleType, id string, name string, cronExpr string, handler func()) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", scheduleType, id)

	// 移除已存在的调度
	if entry, ok := s.entries[key]; ok {
		s.cron.Remove(entry.EntryID)
		delete(s.entries, key)
	}

	// 包装 handler 以记录执行时间
	wrappedHandler := func() {
		now := time.Now()
		if entry, ok := s.entries[key]; ok {
			s.mu.Lock()
			entry.LastRun = &now
			s.mu.Unlock()
		}
		handler()
		s.updateNextRun(key)
	}

	entryID, err := s.cron.AddFunc(cronExpr, wrappedHandler)
	if err != nil {
		return fmt.Errorf("添加 cron 任务失败: %w", err)
	}

	// 计算下次执行时间
	cronEntry := s.cron.Entry(entryID)
	var nextRun *time.Time
	if !cronEntry.Next.IsZero() {
		nextRun = &cronEntry.Next
	}

	s.entries[key] = &ScheduleEntry{
		Key:      key,
		Type:     scheduleType,
		Name:     name,
		CronExpr: cronExpr,
		Enabled:  true,
		NextRun:  nextRun,
		EntryID:  entryID,
		handler:  handler,
	}

	logger.Info("已添加调度 [%s] %s, cron: %s", scheduleType, name, cronExpr)
	return nil
}

// updateNextRun 更新下次执行时间
func (s *UnifiedScheduler) updateNextRun(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok {
		return
	}

	cronEntry := s.cron.Entry(entry.EntryID)
	if !cronEntry.Next.IsZero() {
		entry.NextRun = &cronEntry.Next
	}
}

// EnableSchedule 启用调度
func (s *UnifiedScheduler) EnableSchedule(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok {
		return fmt.Errorf("调度不存在: %s", key)
	}

	if entry.Enabled {
		return nil
	}

	// 重新添加到 cron
	wrappedHandler := func() {
		now := time.Now()
		s.mu.Lock()
		entry.LastRun = &now
		s.mu.Unlock()
		entry.handler()
		s.updateNextRun(key)
	}

	entryID, err := s.cron.AddFunc(entry.CronExpr, wrappedHandler)
	if err != nil {
		return fmt.Errorf("启用调度失败: %w", err)
	}

	entry.EntryID = entryID
	entry.Enabled = true

	cronEntry := s.cron.Entry(entryID)
	if !cronEntry.Next.IsZero() {
		entry.NextRun = &cronEntry.Next
	}

	logger.Info("已启用调度: %s", key)
	return nil
}

// DisableSchedule 禁用调度
func (s *UnifiedScheduler) DisableSchedule(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok {
		return fmt.Errorf("调度不存在: %s", key)
	}

	if !entry.Enabled {
		return nil
	}

	s.cron.Remove(entry.EntryID)
	entry.Enabled = false
	entry.NextRun = nil

	logger.Info("已禁用调度: %s", key)
	return nil
}

// TriggerNow 立即触发调度
func (s *UnifiedScheduler) TriggerNow(key string) error {
	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("调度不存在: %s", key)
	}

	go func() {
		now := time.Now()
		s.mu.Lock()
		entry.LastRun = &now
		s.mu.Unlock()
		entry.handler()
	}()

	logger.Info("已手动触发调度: %s", key)
	return nil
}

// RemoveSchedule 移除调度
func (s *UnifiedScheduler) RemoveSchedule(scheduleType ScheduleType, id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := fmt.Sprintf("%s:%s", scheduleType, id)
	if entry, ok := s.entries[key]; ok {
		s.cron.Remove(entry.EntryID)
		delete(s.entries, key)
		logger.Info("已移除调度 [%s] %s", scheduleType, id)
	}
}

// GetEntries 获取所有调度条目
func (s *UnifiedScheduler) GetEntries() []*ScheduleEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := make([]*ScheduleEntry, 0, len(s.entries))
	for _, e := range s.entries {
		// 更新下次执行时间
		if e.Enabled {
			cronEntry := s.cron.Entry(e.EntryID)
			if !cronEntry.Next.IsZero() {
				e.NextRun = &cronEntry.Next
			}
		}
		entries = append(entries, e)
	}
	return entries
}

// GetEntry 获取单个调度条目
func (s *UnifiedScheduler) GetEntry(key string) *ScheduleEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.entries[key]
}

// SchedulerStatus 调度器状态
type SchedulerStatus struct {
	Running    bool `json:"running"`
	EntryCount int  `json:"entry_count"`
	Enabled    int  `json:"enabled"`
	Disabled   int  `json:"disabled"`
}

// GetStatus 获取调度器状态
func (s *UnifiedScheduler) GetStatus() *SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &SchedulerStatus{
		Running:    s.started,
		EntryCount: len(s.entries),
	}

	for _, e := range s.entries {
		if e.Enabled {
			status.Enabled++
		} else {
			status.Disabled++
		}
	}

	return status
}

// GetEntriesByType 按类型获取调度条目
func (s *UnifiedScheduler) GetEntriesByType(scheduleType ScheduleType) []*ScheduleEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []*ScheduleEntry
	for _, e := range s.entries {
		if e.Type == scheduleType {
			entries = append(entries, e)
		}
	}
	return entries
}

// IsRunning 检查是否运行中
func (s *UnifiedScheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// IntervalToCron 将间隔分钟数转换为调度表达式
// 使用 @every 语法实现真正的固定间隔调度，避免 */N 对齐到固定时钟点的问题
func IntervalToCron(minutes int) string {
	if minutes <= 0 {
		minutes = 60
	}
	hours := minutes / 60
	remainingMinutes := minutes % 60
	if hours > 0 && remainingMinutes > 0 {
		return fmt.Sprintf("@every %dh%dm", hours, remainingMinutes)
	}
	if hours > 0 {
		return fmt.Sprintf("@every %dh", hours)
	}
	return fmt.Sprintf("@every %dm", minutes)
}
