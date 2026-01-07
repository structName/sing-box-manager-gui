package models

import (
	"time"
)

// InboundPort 入站端口表
type InboundPort struct {
	ID           string `gorm:"primaryKey;type:text" json:"id"`
	Name         string `gorm:"not null;type:text" json:"name"`
	Type         string `gorm:"not null;type:text" json:"type"` // mixed/http/socks
	Listen       string `gorm:"default:'127.0.0.1';type:text" json:"listen"`
	Port         int    `gorm:"not null" json:"port"`
	Outbound     string `gorm:"type:text" json:"outbound,omitempty"` // 关联的出站 tag
	AuthUsername string `gorm:"type:text" json:"auth_username,omitempty"`
	AuthPassword string `gorm:"type:text" json:"auth_password,omitempty"`
	Enabled      bool   `gorm:"default:true" json:"enabled"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (InboundPort) TableName() string {
	return "inbound_ports"
}

// Rule 自定义规则表
type Rule struct {
	ID       string      `gorm:"primaryKey;type:text" json:"id"`
	Name     string      `gorm:"type:text" json:"name,omitempty"`
	RuleType string      `gorm:"not null;type:text" json:"rule_type"` // domain_suffix/domain_keyword/ip_cidr/geosite/geoip/port
	Values   StringSlice `gorm:"type:text" json:"values"`             // 规则值列表
	Outbound string      `gorm:"not null;type:text" json:"outbound"`
	Enabled  bool        `gorm:"default:true" json:"enabled"`
	Priority int         `gorm:"default:0" json:"priority"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Rule) TableName() string {
	return "rules"
}

// RuleGroup 预设规则组表
type RuleGroup struct {
	ID        string      `gorm:"primaryKey;type:text" json:"id"`
	Name      string      `gorm:"not null;type:text" json:"name"`
	SiteRules StringSlice `gorm:"type:text" json:"site_rules,omitempty"` // geosite 规则
	IPRules   StringSlice `gorm:"type:text" json:"ip_rules,omitempty"`   // geoip 规则
	Outbound  string      `gorm:"not null;type:text" json:"outbound"`
	Enabled   bool        `gorm:"default:true" json:"enabled"`
	Position  int         `gorm:"default:0" json:"position"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (RuleGroup) TableName() string {
	return "rule_groups"
}

// Filter 过滤器表
type Filter struct {
	ID               string      `gorm:"primaryKey;type:text" json:"id"`
	Name             string      `gorm:"not null;type:text" json:"name"`
	Include          StringSlice `gorm:"type:text" json:"include,omitempty"`
	Exclude          StringSlice `gorm:"type:text" json:"exclude,omitempty"`
	IncludeCountries StringSlice `gorm:"type:text" json:"include_countries,omitempty"`
	ExcludeCountries StringSlice `gorm:"type:text" json:"exclude_countries,omitempty"`
	Mode             string      `gorm:"type:text" json:"mode,omitempty"`         // urltest / select
	Subscriptions    StringSlice `gorm:"type:text" json:"subscriptions,omitempty"` // 适用的订阅ID
	AllNodes         bool        `gorm:"default:false" json:"all_nodes"`
	Enabled          bool        `gorm:"default:true" json:"enabled"`

	// URLTest 配置
	URLTestURL       string `gorm:"type:text" json:"urltest_url,omitempty"`
	URLTestInterval  string `gorm:"type:text" json:"urltest_interval,omitempty"`
	URLTestTolerance int    `gorm:"default:0" json:"urltest_tolerance,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (Filter) TableName() string {
	return "filters"
}

// HostEntry DNS hosts 映射表
type HostEntry struct {
	ID      string      `gorm:"primaryKey;type:text" json:"id"`
	Domain  string      `gorm:"not null;type:text" json:"domain"`
	IPs     StringSlice `gorm:"type:text" json:"ips"`
	Enabled bool        `gorm:"default:true" json:"enabled"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (HostEntry) TableName() string {
	return "host_entries"
}
