package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xiaobei/singbox-manager/internal/service"
)

// SchedulerHandler 调度器 API 处理器
type SchedulerHandler struct {
	scheduler *service.UnifiedScheduler
}

// NewSchedulerHandler 创建调度器处理器
func NewSchedulerHandler(scheduler *service.UnifiedScheduler) *SchedulerHandler {
	return &SchedulerHandler{scheduler: scheduler}
}

// GetStatus 获取调度器状态
func (h *SchedulerHandler) GetStatus(c *gin.Context) {
	status := h.scheduler.GetStatus()
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// GetEntries 获取所有调度条目
func (h *SchedulerHandler) GetEntries(c *gin.Context) {
	scheduleType := c.Query("type")

	var entries []*service.ScheduleEntry
	if scheduleType != "" {
		entries = h.scheduler.GetEntriesByType(service.ScheduleType(scheduleType))
	} else {
		entries = h.scheduler.GetEntries()
	}

	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// EnableEntry 启用调度条目
func (h *SchedulerHandler) EnableEntry(c *gin.Context) {
	key := c.Param("key")

	if err := h.scheduler.EnableSchedule(key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "调度已启用"})
}

// DisableEntry 禁用调度条目
func (h *SchedulerHandler) DisableEntry(c *gin.Context) {
	key := c.Param("key")

	if err := h.scheduler.DisableSchedule(key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "调度已禁用"})
}

// TriggerEntry 立即触发调度
func (h *SchedulerHandler) TriggerEntry(c *gin.Context) {
	key := c.Param("key")

	if err := h.scheduler.TriggerNow(key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "调度已触发"})
}

// PauseScheduler 暂停调度器
func (h *SchedulerHandler) PauseScheduler(c *gin.Context) {
	h.scheduler.Stop()
	c.JSON(http.StatusOK, gin.H{"message": "调度器已暂停"})
}

// ResumeScheduler 恢复调度器
func (h *SchedulerHandler) ResumeScheduler(c *gin.Context) {
	if err := h.scheduler.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "调度器已恢复"})
}
