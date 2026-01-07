package models

import (
	"time"
)

// ProxyChain 代理链路表
type ProxyChain struct {
	ID          string `gorm:"primaryKey;type:text" json:"id"`
	Name        string `gorm:"not null;type:text" json:"name"`
	Description string `gorm:"type:text" json:"description,omitempty"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`

	// 健康检测配置
	HealthEnabled  bool   `gorm:"default:false" json:"health_enabled"`
	HealthInterval int    `gorm:"default:300" json:"health_interval"` // 检测间隔（秒）
	HealthTimeout  int    `gorm:"default:10" json:"health_timeout"`
	HealthURL      string `gorm:"type:text" json:"health_url,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// 关联
	ChainNodes []ProxyChainNode `gorm:"foreignKey:ChainID" json:"chain_nodes,omitempty"`
}

func (ProxyChain) TableName() string {
	return "proxy_chains"
}

// ProxyChainNode 链路节点关联表（有序）
type ProxyChainNode struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	ChainID  string `gorm:"not null;type:text;index" json:"chain_id"`
	NodeID   uint   `gorm:"not null" json:"node_id"`
	Position int    `gorm:"not null" json:"position"` // 顺序位置

	// 关联
	Chain *ProxyChain `gorm:"foreignKey:ChainID" json:"chain,omitempty"`
	Node  *Node       `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

func (ProxyChainNode) TableName() string {
	return "proxy_chain_nodes"
}
