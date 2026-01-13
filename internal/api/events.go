package api

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/xiaobei/singbox-manager/internal/service"
)

// EventsHandler SSE 事件处理器
type EventsHandler struct {
	taskManager *service.TaskManager
}

// NewEventsHandler 创建事件处理器
func NewEventsHandler(taskManager *service.TaskManager) *EventsHandler {
	return &EventsHandler{taskManager: taskManager}
}

// StreamTasks SSE 任务更新流
func (h *EventsHandler) StreamTasks(c *gin.Context) {
	clientID := uuid.New().String()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	taskCh := h.taskManager.Subscribe(clientID)
	defer h.taskManager.Unsubscribe(clientID)

	c.Stream(func(w io.Writer) bool {
		select {
		case task, ok := <-taskCh:
			if !ok {
				return false
			}
			data, _ := json.Marshal(task)
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(interface{ Flush() }); ok {
				f.Flush()
			}
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}
