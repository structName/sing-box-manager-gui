package models

import (
	"time"
)

// Node 节点表
type Node struct {
	ID         uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Tag        string `gorm:"uniqueIndex;not null;type:text" json:"tag"` // 节点标识（唯一）
	Type       string `gorm:"not null;type:text" json:"type"`            // 协议: ss/vmess/vless/trojan/hysteria2/tuic
	Server     string `gorm:"not null;type:text" json:"server"`
	ServerPort int    `gorm:"not null" json:"server_port"`

	// 来源
	Source     string `gorm:"not null;type:text;index" json:"source"`      // 'manual' 或 subscription_id
	SourceName string `gorm:"type:text" json:"source_name"`                // 来源名称

	// 地理信息
	Country      string `gorm:"type:text;index" json:"country,omitempty"`       // 国家代码
	CountryEmoji string `gorm:"type:text" json:"country_emoji,omitempty"`       // 国旗 emoji
	LandingIP    string `gorm:"type:text" json:"landing_ip,omitempty"`          // 落地 IP

	// 原始数据
	Link  string  `gorm:"type:text" json:"link,omitempty"`  // 原始链接（用于测速/导出）
	Extra JSONMap `gorm:"type:text" json:"extra,omitempty"` // JSON: 协议特定参数

	// 测速结果
	Delay       int       `gorm:"default:0" json:"delay"`                        // 延迟 (ms), -1 表示超时
	Speed       float64   `gorm:"default:0" json:"speed"`                        // 速度 (MB/s)
	DelayStatus string    `gorm:"default:'untested';type:text" json:"delay_status"` // untested/success/timeout/error
	SpeedStatus string    `gorm:"default:'untested';type:text" json:"speed_status"`
	TestedAt    *time.Time `json:"tested_at,omitempty"`

	Enabled   bool      `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// 标签关联
	Tags []Tag `gorm:"many2many:node_tags;" json:"tags,omitempty"`
}

func (Node) TableName() string {
	return "nodes"
}
