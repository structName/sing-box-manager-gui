package models

import (
	"time"
)

// Setting 全局设置表
type Setting struct {
	Key       string    `gorm:"primaryKey;type:text" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Setting) TableName() string {
	return "settings"
}

// Profile 配置方案表
type Profile struct {
	ID          string `gorm:"primaryKey;type:text" json:"id"`
	Name        string `gorm:"not null;type:text" json:"name"`
	Description string `gorm:"type:text" json:"description,omitempty"`
	IsActive    bool   `gorm:"default:false" json:"is_active"`
	Snapshot    string `gorm:"type:text" json:"snapshot,omitempty"` // JSON: 配置快照

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Profile) TableName() string {
	return "profiles"
}

// DefaultSettings 默认设置值
var DefaultSettings = map[string]string{
	"singbox_path":          "bin/sing-box",
	"config_path":           "generated/config.json",
	"mixed_port":            "2080",
	"tun_enabled":           "false",
	"lan_proxy_enabled":     "false",
	"lan_listen_ip":         "0.0.0.0",
	"proxy_dns":             "https://1.1.1.1/dns-query",
	"direct_dns":            "https://dns.alidns.com/dns-query",
	"fakeip_enabled":        "false",
	"web_port":              "9090",
	"clash_api_port":        "9091",
	"clash_ui_enabled":      "true",
	"clash_ui_path":         "zashboard",
	"clash_api_secret":      "",
	"final_outbound":        "Proxy",
	"ruleset_base_url":      "https://github.com/lyc8503/sing-box-rules/raw/rule-set-geosite",
	"auto_apply":            "true",
	"subscription_interval": "60",
	"github_proxy":          "",
	"chain_health_enabled":  "false",
	"chain_health_interval": "300",
	"chain_health_timeout":  "10",
	"chain_health_url":      "https://www.gstatic.com/generate_204",
}

// DefaultRuleGroups 默认规则组
var DefaultRuleGroups = []RuleGroup{
	{ID: "ad-block", Name: "广告拦截", SiteRules: []string{"category-ads-all"}, Outbound: "REJECT", Enabled: true, Position: 1},
	{ID: "ai-services", Name: "AI 服务", SiteRules: []string{"openai", "anthropic", "jetbrains-ai"}, Outbound: "Proxy", Enabled: true, Position: 2},
	{ID: "google", Name: "Google", SiteRules: []string{"google"}, IPRules: []string{"google"}, Outbound: "Proxy", Enabled: true, Position: 3},
	{ID: "youtube", Name: "YouTube", SiteRules: []string{"youtube"}, Outbound: "Proxy", Enabled: true, Position: 4},
	{ID: "github", Name: "GitHub", SiteRules: []string{"github"}, Outbound: "Proxy", Enabled: true, Position: 5},
	{ID: "telegram", Name: "Telegram", SiteRules: []string{"telegram"}, IPRules: []string{"telegram"}, Outbound: "Proxy", Enabled: true, Position: 6},
	{ID: "twitter", Name: "Twitter/X", SiteRules: []string{"twitter"}, Outbound: "Proxy", Enabled: true, Position: 7},
	{ID: "netflix", Name: "Netflix", SiteRules: []string{"netflix"}, Outbound: "Proxy", Enabled: false, Position: 8},
	{ID: "spotify", Name: "Spotify", SiteRules: []string{"spotify"}, Outbound: "Proxy", Enabled: false, Position: 9},
	{ID: "apple", Name: "Apple", SiteRules: []string{"apple"}, Outbound: "DIRECT", Enabled: true, Position: 10},
	{ID: "microsoft", Name: "Microsoft", SiteRules: []string{"microsoft"}, Outbound: "DIRECT", Enabled: true, Position: 11},
	{ID: "cn", Name: "中国地区", SiteRules: []string{"geolocation-cn"}, IPRules: []string{"cn"}, Outbound: "DIRECT", Enabled: true, Position: 12},
	{ID: "private", Name: "私有网络", SiteRules: []string{"private"}, IPRules: []string{"private"}, Outbound: "DIRECT", Enabled: true, Position: 13},
}
