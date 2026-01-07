package models

import (
	"time"
)

// Subscription 订阅表
type Subscription struct {
	ID      string `gorm:"primaryKey;type:text" json:"id"`
	Name    string `gorm:"not null;type:text" json:"name"`
	URL     string `gorm:"not null;type:text" json:"url"`
	Enabled bool   `gorm:"default:true" json:"enabled"`

	// 流量信息
	TrafficTotal     int64 `gorm:"default:0" json:"traffic_total"`     // 总流量 (bytes)
	TrafficUsed      int64 `gorm:"default:0" json:"traffic_used"`      // 已用流量
	TrafficRemaining int64 `gorm:"default:0" json:"traffic_remaining"` // 剩余流量

	// 到期时间
	ExpireAt *time.Time `json:"expire_at,omitempty"`

	// 更新设置
	AutoUpdate     bool `gorm:"default:true" json:"auto_update"`    // 自动更新
	UpdateInterval int  `gorm:"default:60" json:"update_interval"`  // 更新间隔（分钟）

	LastUpdateAt *time.Time `json:"last_update_at,omitempty"`
	NextUpdateAt *time.Time `json:"next_update_at,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// 关联
	Nodes []Node `gorm:"foreignKey:Source;references:ID" json:"nodes,omitempty"`
}

func (Subscription) TableName() string {
	return "subscriptions"
}
