package speedtest

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/database/models"
	"github.com/xiaobei/singbox-manager/internal/logger"
)

// Scheduler 测速定时调度器
type Scheduler struct {
	cron     *cron.Cron
	store    *database.Store
	executor *Executor
	mu       sync.RWMutex
	// 策略ID -> cron entry ID 映射
	entryIDs map[uint]cron.EntryID
	// 是否已启动
	started bool
}

// NewScheduler 创建调度器
func NewScheduler(store *database.Store, executor *Executor) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithSeconds()),
		store:    store,
		executor: executor,
		entryIDs: make(map[uint]cron.EntryID),
	}
}

// Start 启动调度器
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	// 加载所有启用自动测速的策略
	profiles, err := s.store.GetSpeedTestProfiles()
	if err != nil {
		return fmt.Errorf("加载测速策略失败: %w", err)
	}

	for _, profile := range profiles {
		if profile.AutoTest && profile.Enabled {
			if err := s.addSchedule(&profile); err != nil {
				logger.Warn("添加策略 [%s] 调度失败: %v", profile.Name, err)
			}
		}
	}

	s.cron.Start()
	s.started = true
	logger.Info("测速调度器已启动")
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.started = false
	logger.Info("测速调度器已停止")
}

// addSchedule 添加策略调度
func (s *Scheduler) addSchedule(profile *models.SpeedTestProfile) error {
	var cronExpr string

	switch profile.ScheduleType {
	case "interval":
		// 间隔时间 (分钟)
		interval := profile.ScheduleInterval
		if interval <= 0 {
			interval = 60
		}
		// 转换为 cron 表达式
		if interval < 60 {
			// 每 N 分钟
			cronExpr = fmt.Sprintf("0 */%d * * * *", interval)
		} else {
			// 每 N 小时
			hours := interval / 60
			cronExpr = fmt.Sprintf("0 0 */%d * * *", hours)
		}

	case "cron":
		// 自定义 cron 表达式
		cronExpr = profile.ScheduleCron
		if cronExpr == "" {
			return fmt.Errorf("cron 表达式为空")
		}

	default:
		// 默认每小时
		cronExpr = "0 0 * * * *"
	}

	// 添加任务
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.runScheduledTask(profile.ID)
	})
	if err != nil {
		return fmt.Errorf("添加 cron 任务失败: %w", err)
	}

	s.entryIDs[profile.ID] = entryID

	// 计算下次执行时间
	entry := s.cron.Entry(entryID)
	if !entry.Next.IsZero() {
		profile.NextRunAt = &entry.Next
		s.store.UpdateSpeedTestProfile(profile)
	}

	logger.Info("已添加策略 [%s] 调度, cron: %s, 下次执行: %v", profile.Name, cronExpr, entry.Next)
	return nil
}

// runScheduledTask 执行定时任务
func (s *Scheduler) runScheduledTask(profileID uint) {
	logger.Info("定时执行测速策略 ID: %d", profileID)

	_, err := s.executor.RunWithProfile(profileID, nil, TriggerTypeScheduled)
	if err != nil {
		logger.Error("执行定时测速失败: %v", err)
		return
	}

	// 更新下次执行时间
	s.mu.RLock()
	entryID, ok := s.entryIDs[profileID]
	s.mu.RUnlock()

	if ok {
		entry := s.cron.Entry(entryID)
		if !entry.Next.IsZero() {
			profile, err := s.store.GetSpeedTestProfile(profileID)
			if err == nil {
				profile.NextRunAt = &entry.Next
				s.store.UpdateSpeedTestProfile(profile)
			}
		}
	}
}

// UpdateSchedule 更新策略调度
func (s *Scheduler) UpdateSchedule(profile *models.SpeedTestProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 先移除旧的调度
	if entryID, ok := s.entryIDs[profile.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryIDs, profile.ID)
	}

	// 如果启用自动测速，添加新调度
	if profile.AutoTest && profile.Enabled {
		return s.addSchedule(profile)
	}

	return nil
}

// RemoveSchedule 移除策略调度
func (s *Scheduler) RemoveSchedule(profileID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entryIDs[profileID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryIDs, profileID)
		logger.Info("已移除策略 ID: %d 的调度", profileID)
	}
}

// GetNextRunTime 获取策略下次执行时间
func (s *Scheduler) GetNextRunTime(profileID uint) *time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entryID, ok := s.entryIDs[profileID]
	if !ok {
		return nil
	}

	entry := s.cron.Entry(entryID)
	if entry.Next.IsZero() {
		return nil
	}

	return &entry.Next
}

// GetScheduledProfiles 获取所有已调度的策略ID
func (s *Scheduler) GetScheduledProfiles() []uint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]uint, 0, len(s.entryIDs))
	for id := range s.entryIDs {
		ids = append(ids, id)
	}
	return ids
}
