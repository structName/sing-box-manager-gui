package models

import (
	"time"
)

// 任务类型常量
const (
	TaskTypeSpeedTest   = "speed_test"   // 节点测速
	TaskTypeSubUpdate   = "sub_update"   // 订阅更新
	TaskTypeTagRule     = "tag_rule"     // 标签规则
	TaskTypeChainCheck  = "chain_check"  // 链路检测
	TaskTypeConfigApply = "config_apply" // 配置应用
)

// 任务状态常量
const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusCancelled = "cancelled"
	TaskStatusError     = "error"
)

// 触发方式常量
const (
	TaskTriggerManual    = "manual"
	TaskTriggerScheduled = "scheduled"
	TaskTriggerEvent     = "event"
)

// Task 任务模型
type Task struct {
	ID      string `gorm:"primaryKey;type:text" json:"id"`
	Type    string `gorm:"index;type:text" json:"type"`
	Name    string `gorm:"type:text" json:"name"`
	Status  string `gorm:"index;type:text;default:'pending'" json:"status"`
	Trigger string `gorm:"type:text" json:"trigger"`

	Progress    int     `gorm:"default:0" json:"progress"`
	Total       int     `gorm:"default:0" json:"total"`
	CurrentItem string  `gorm:"type:text" json:"current_item,omitempty"`
	Message     string  `gorm:"type:text" json:"message,omitempty"`
	Result      JSONMap `gorm:"type:text" json:"result,omitempty"`

	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (Task) TableName() string {
	return "tasks"
}
