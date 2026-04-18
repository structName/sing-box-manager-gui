package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/service"
)

// TaskHandler 任务 API 处理器
type TaskHandler struct {
	store       *database.Store
	taskManager *service.TaskManager
}

// NewTaskHandler 创建任务处理器
func NewTaskHandler(store *database.Store, taskManager *service.TaskManager) *TaskHandler {
	return &TaskHandler{
		store:       store,
		taskManager: taskManager,
	}
}

// Rebind 更新内部引用（用于 Profile 切换）
func (h *TaskHandler) Rebind(store *database.Store) {
	h.store = store
}

// GetTasks 获取任务列表
func (h *TaskHandler) GetTasks(c *gin.Context) {
	limit := 50
	offset := 0
	taskType := c.Query("type")
	status := c.Query("status")

	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o := c.Query("offset"); o != "" {
		if n, err := strconv.Atoi(o); err == nil && n >= 0 {
			offset = n
		}
	}

	tasks, err := h.store.GetTasks(limit, offset, taskType, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tasks})
}

// GetTask 获取单个任务
func (h *TaskHandler) GetTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.store.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": task})
}

// CancelTask 取消任务
func (h *TaskHandler) CancelTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.taskManager.CancelTask(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已取消"})
}

// GetRunningTasks 获取运行中的任务
func (h *TaskHandler) GetRunningTasks(c *gin.Context) {
	tasks := h.taskManager.GetRunningTasks()
	c.JSON(http.StatusOK, gin.H{"data": tasks})
}

// GetTaskStats 获取任务统计
func (h *TaskHandler) GetTaskStats(c *gin.Context) {
	stats := h.taskManager.GetStats()
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// CleanupTasks 清理历史任务
func (h *TaskHandler) CleanupTasks(c *gin.Context) {
	days := 7
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}

	before := time.Now().AddDate(0, 0, -days)
	if err := h.taskManager.CleanupOldTasks(before); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "历史任务已清理"})
}
