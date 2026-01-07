package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/database/models"
	"github.com/xiaobei/singbox-manager/internal/service"
	"github.com/xiaobei/singbox-manager/internal/speedtest"
)

// SpeedTestHandler 测速 API 处理器
type SpeedTestHandler struct {
	store            *database.Store
	executor         *speedtest.Executor
	scheduler        *speedtest.Scheduler
	unifiedScheduler *service.UnifiedScheduler
}

// NewSpeedTestHandler 创建测速处理器
func NewSpeedTestHandler(store *database.Store, executor *speedtest.Executor, scheduler *speedtest.Scheduler) *SpeedTestHandler {
	return &SpeedTestHandler{
		store:     store,
		executor:  executor,
		scheduler: scheduler,
	}
}

// SetUnifiedScheduler 设置统一调度器
func (h *SpeedTestHandler) SetUnifiedScheduler(us *service.UnifiedScheduler) {
	h.unifiedScheduler = us
}

// updateUnifiedSchedule 同步更新统一调度器
func (h *SpeedTestHandler) updateUnifiedSchedule(profile *models.SpeedTestProfile) {
	if h.unifiedScheduler == nil {
		return
	}

	profileID := profile.ID
	key := fmt.Sprintf("%d", profileID)

	// 先移除旧的调度
	h.unifiedScheduler.RemoveSchedule(service.ScheduleTypeSpeedTest, key)

	// 如果启用自动测速，添加新调度
	if profile.AutoTest && profile.Enabled {
		var cronExpr string
		if profile.ScheduleType == "cron" && profile.ScheduleCron != "" {
			cronExpr = profile.ScheduleCron
		} else {
			cronExpr = service.IntervalToCron(profile.ScheduleInterval)
		}
		h.unifiedScheduler.AddSchedule(
			service.ScheduleTypeSpeedTest,
			key,
			"定时测速: "+profile.Name,
			cronExpr,
			func() {
				if h.executor != nil {
					h.executor.RunWithProfile(profileID, nil, speedtest.TriggerTypeScheduled)
				}
			},
		)
	}
}

// ==================== 测速策略 API ====================

// GetProfiles 获取所有测速策略
func (h *SpeedTestHandler) GetProfiles(c *gin.Context) {
	profiles, err := h.store.GetSpeedTestProfiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profiles)
}

// GetProfile 获取单个测速策略
func (h *SpeedTestHandler) GetProfile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	profile, err := h.store.GetSpeedTestProfile(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// CreateProfileRequest 创建策略请求
type CreateProfileRequest struct {
	Name               string   `json:"name" binding:"required"`
	Enabled            bool     `json:"enabled"`
	AutoTest           bool     `json:"auto_test"`
	ScheduleType       string   `json:"schedule_type"`
	ScheduleInterval   int      `json:"schedule_interval"`
	ScheduleCron       string   `json:"schedule_cron"`
	Mode               string   `json:"mode"`
	LatencyURL         string   `json:"latency_url"`
	SpeedURL           string   `json:"speed_url"`
	Timeout            int      `json:"timeout"`
	LatencyConcurrency int      `json:"latency_concurrency"`
	SpeedConcurrency   int      `json:"speed_concurrency"`
	IncludeHandshake   bool     `json:"include_handshake"`
	DetectCountry      bool     `json:"detect_country"`
	LandingIPURL       string   `json:"landing_ip_url"`
	SpeedRecordMode    string   `json:"speed_record_mode"`
	PeakSampleInterval int      `json:"peak_sample_interval"`
	SourceFilter       []string `json:"source_filter"`
	CountryFilter      []string `json:"country_filter"`
	TagFilter          []string `json:"tag_filter"`
}

// CreateProfile 创建测速策略
func (h *SpeedTestHandler) CreateProfile(c *gin.Context) {
	var req CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile := &models.SpeedTestProfile{
		Name:               req.Name,
		Enabled:            req.Enabled,
		AutoTest:           req.AutoTest,
		ScheduleType:       req.ScheduleType,
		ScheduleInterval:   req.ScheduleInterval,
		ScheduleCron:       req.ScheduleCron,
		Mode:               req.Mode,
		LatencyURL:         req.LatencyURL,
		SpeedURL:           req.SpeedURL,
		Timeout:            req.Timeout,
		LatencyConcurrency: req.LatencyConcurrency,
		SpeedConcurrency:   req.SpeedConcurrency,
		IncludeHandshake:   req.IncludeHandshake,
		DetectCountry:      req.DetectCountry,
		LandingIPURL:       req.LandingIPURL,
		SpeedRecordMode:    req.SpeedRecordMode,
		PeakSampleInterval: req.PeakSampleInterval,
		SourceFilter:       req.SourceFilter,
		CountryFilter:      req.CountryFilter,
		TagFilter:          req.TagFilter,
	}

	// 设置默认值
	if profile.Mode == "" {
		profile.Mode = "speed" // 默认延迟+速度
	}
	if profile.LatencyURL == "" {
		profile.LatencyURL = "https://cp.cloudflare.com/generate_204"
	}
	if profile.SpeedURL == "" {
		profile.SpeedURL = "https://speed.cloudflare.com/__down?bytes=5000000"
	}
	if profile.Timeout == 0 {
		profile.Timeout = 7
	}
	if profile.LatencyConcurrency == 0 {
		profile.LatencyConcurrency = 50
	}
	if profile.SpeedConcurrency == 0 {
		profile.SpeedConcurrency = 5
	}
	if profile.SpeedRecordMode == "" {
		profile.SpeedRecordMode = "average"
	}
	if profile.PeakSampleInterval == 0 {
		profile.PeakSampleInterval = 100
	}
	if profile.LandingIPURL == "" {
		profile.LandingIPURL = "https://api.ipify.org"
	}

	if err := h.store.CreateSpeedTestProfile(profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 如果启用自动测速，添加调度
	if profile.AutoTest && profile.Enabled {
		h.scheduler.UpdateSchedule(profile)
	}
	// 同步更新统一调度器
	h.updateUnifiedSchedule(profile)

	c.JSON(http.StatusCreated, profile)
}

// UpdateProfile 更新测速策略
func (h *SpeedTestHandler) UpdateProfile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	profile, err := h.store.GetSpeedTestProfile(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在"})
		return
	}

	var req CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新字段
	profile.Name = req.Name
	profile.Enabled = req.Enabled
	profile.AutoTest = req.AutoTest
	profile.ScheduleType = req.ScheduleType
	profile.ScheduleInterval = req.ScheduleInterval
	profile.ScheduleCron = req.ScheduleCron
	profile.Mode = req.Mode
	profile.LatencyURL = req.LatencyURL
	profile.SpeedURL = req.SpeedURL
	profile.Timeout = req.Timeout
	profile.LatencyConcurrency = req.LatencyConcurrency
	profile.SpeedConcurrency = req.SpeedConcurrency
	profile.IncludeHandshake = req.IncludeHandshake
	profile.DetectCountry = req.DetectCountry
	profile.LandingIPURL = req.LandingIPURL
	profile.SpeedRecordMode = req.SpeedRecordMode
	profile.PeakSampleInterval = req.PeakSampleInterval
	profile.SourceFilter = req.SourceFilter
	profile.CountryFilter = req.CountryFilter
	profile.TagFilter = req.TagFilter

	if err := h.store.UpdateSpeedTestProfile(profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 更新调度
	h.scheduler.UpdateSchedule(profile)
	// 同步更新统一调度器
	h.updateUnifiedSchedule(profile)

	c.JSON(http.StatusOK, profile)
}

// DeleteProfile 删除测速策略
func (h *SpeedTestHandler) DeleteProfile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	profile, err := h.store.GetSpeedTestProfile(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "策略不存在"})
		return
	}

	// 不允许删除默认策略
	if profile.IsDefault {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不能删除默认策略"})
		return
	}

	// 移除调度
	h.scheduler.RemoveSchedule(uint(id))
	// 同步移除统一调度器中的条目
	if h.unifiedScheduler != nil {
		h.unifiedScheduler.RemoveSchedule(service.ScheduleTypeSpeedTest, fmt.Sprintf("%d", id))
	}

	if err := h.store.DeleteSpeedTestProfile(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ==================== 测速执行 API ====================

// RunTestRequest 执行测速请求
type RunTestRequest struct {
	ProfileID uint   `json:"profile_id"`
	NodeIDs   []uint `json:"node_ids,omitempty"`
}

// RunTest 执行测速
func (h *SpeedTestHandler) RunTest(c *gin.Context) {
	var req RunTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果没有指定策略，使用默认策略
	profileID := req.ProfileID
	if profileID == 0 {
		defaultProfile, err := h.store.GetDefaultSpeedTestProfile()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "未找到默认策略"})
			return
		}
		profileID = defaultProfile.ID
	}

	task, err := h.executor.RunWithProfile(profileID, req.NodeIDs, speedtest.TriggerTypeManual)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// GetTasks 获取测速任务列表
func (h *SpeedTestHandler) GetTasks(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	tasks, err := h.store.GetSpeedTestTasks(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 添加运行状态
	runningTasks := h.executor.GetRunningTasks()
	runningIDs := make(map[string]bool)
	for _, t := range runningTasks {
		runningIDs[t.ID] = true
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":   tasks,
		"running": runningIDs,
	})
}

// GetTask 获取单个测速任务
func (h *SpeedTestHandler) GetTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.store.GetSpeedTestTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	// 获取任务历史
	history, _ := h.store.GetSpeedTestHistoryByTask(taskID)

	c.JSON(http.StatusOK, gin.H{
		"task":    task,
		"history": history,
		"running": h.executor.IsTaskRunning(taskID),
	})
}

// CancelTask 取消测速任务
func (h *SpeedTestHandler) CancelTask(c *gin.Context) {
	taskID := c.Param("id")

	if err := h.executor.CancelTask(taskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
}

// ==================== 测速历史 API ====================

// GetNodeHistory 获取节点测速历史
func (h *SpeedTestHandler) GetNodeHistory(c *gin.Context) {
	nodeIDStr := c.Param("nodeId")
	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	history, err := h.store.GetSpeedTestHistory(uint(nodeID), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}
