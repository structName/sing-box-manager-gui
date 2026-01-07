package models

import (
	"time"
)

// SpeedTestProfile 测速策略表
type SpeedTestProfile struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string `gorm:"not null;type:text" json:"name"`
	Enabled   bool   `gorm:"default:true" json:"enabled"`
	IsDefault bool   `gorm:"default:false" json:"is_default"`

	// 定时设置
	AutoTest         bool   `gorm:"default:false" json:"auto_test"`
	ScheduleType     string `gorm:"type:text" json:"schedule_type,omitempty"`
	ScheduleInterval int    `gorm:"default:0" json:"schedule_interval,omitempty"`
	ScheduleCron     string `gorm:"type:text" json:"schedule_cron,omitempty"`

	// 测速模式
	Mode string `gorm:"default:'delay';type:text" json:"mode"`

	// URL 配置
	LatencyURL string `gorm:"default:'https://cp.cloudflare.com/generate_204';type:text" json:"latency_url"`
	SpeedURL   string `gorm:"default:'https://speed.cloudflare.com/__down?bytes=5000000';type:text" json:"speed_url"`

	// 并发与超时
	Timeout            int `gorm:"default:7" json:"timeout"`
	LatencyConcurrency int `gorm:"default:50" json:"latency_concurrency"`
	SpeedConcurrency   int `gorm:"default:5" json:"speed_concurrency"`

	// 高级选项
	IncludeHandshake bool   `gorm:"default:false" json:"include_handshake"`
	DetectCountry    bool   `gorm:"default:false" json:"detect_country"`
	LandingIPURL     string `gorm:"default:'https://api.ipify.org';type:text" json:"landing_ip_url"`

	// 速度记录模式
	SpeedRecordMode    string `gorm:"default:'average';type:text" json:"speed_record_mode"`
	PeakSampleInterval int    `gorm:"default:100" json:"peak_sample_interval"`

	// 节点筛选 (JSON)
	SourceFilter  StringSlice `gorm:"type:text" json:"source_filter,omitempty"`
	CountryFilter StringSlice `gorm:"type:text" json:"country_filter,omitempty"`
	TagFilter     StringSlice `gorm:"type:text" json:"tag_filter,omitempty"`

	// 执行记录
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SpeedTestProfile) TableName() string {
	return "speed_test_profiles"
}

// SpeedTestTask 测速任务表
type SpeedTestTask struct {
	ID          string `gorm:"primaryKey;type:text" json:"id"`
	ProfileID   *uint  `json:"profile_id,omitempty"`
	ProfileName string `gorm:"type:text" json:"profile_name,omitempty"`
	Status      string `gorm:"default:'pending';type:text;index" json:"status"`
	TriggerType string `gorm:"type:text" json:"trigger_type,omitempty"`

	Total       int    `gorm:"default:0" json:"total"`
	Completed   int    `gorm:"default:0" json:"completed"`
	Success     int    `gorm:"default:0" json:"success"`
	Failed      int    `gorm:"default:0" json:"failed"`
	CurrentNode string `gorm:"type:text" json:"current_node,omitempty"`
	Error       string `gorm:"type:text" json:"error,omitempty"`

	StartedAt  *time.Time `gorm:"index" json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	Profile *SpeedTestProfile `gorm:"foreignKey:ProfileID" json:"profile,omitempty"`
}

func (SpeedTestTask) TableName() string {
	return "speed_test_tasks"
}

// SpeedTestHistory 测速历史表
type SpeedTestHistory struct {
	ID        uint    `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID    string  `gorm:"type:text" json:"task_id,omitempty"`
	NodeID    uint    `gorm:"not null;index" json:"node_id"`
	Delay     int     `json:"delay,omitempty"`
	Speed     float64 `json:"speed,omitempty"`
	Status    string  `gorm:"type:text" json:"status,omitempty"`
	LandingIP string  `gorm:"type:text" json:"landing_ip,omitempty"`

	TestedAt time.Time `gorm:"autoCreateTime;index" json:"tested_at"`

	Node *Node `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

func (SpeedTestHistory) TableName() string {
	return "speed_test_history"
}
