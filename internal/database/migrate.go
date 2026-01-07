package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/xiaobei/singbox-manager/internal/database/models"
	"github.com/xiaobei/singbox-manager/internal/logger"
	"gorm.io/gorm"
)

// LegacyAppData 旧版 data.json 结构
type LegacyAppData struct {
	Subscriptions []LegacySubscription `json:"subscriptions"`
	ManualNodes   []LegacyManualNode   `json:"manual_nodes"`
	Filters       []LegacyFilter       `json:"filters"`
	Rules         []LegacyRule         `json:"rules"`
	RuleGroups    []LegacyRuleGroup    `json:"rule_groups"`
	Settings      *LegacySettings      `json:"settings"`
	Profiles      []LegacyProfile      `json:"profiles,omitempty"`
	ActiveProfile string               `json:"active_profile,omitempty"`
	InboundPorts  []LegacyInboundPort  `json:"inbound_ports,omitempty"`
	ProxyChains   []LegacyProxyChain   `json:"proxy_chains,omitempty"`
}

type LegacySubscription struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	URL       string          `json:"url"`
	NodeCount int             `json:"node_count"`
	UpdatedAt string          `json:"updated_at"`
	ExpireAt  *string         `json:"expire_at,omitempty"`
	Traffic   *LegacyTraffic  `json:"traffic,omitempty"`
	Nodes     []LegacyNode    `json:"nodes"`
	Enabled   bool            `json:"enabled"`
}

type LegacyTraffic struct {
	Total     int64 `json:"total"`
	Used      int64 `json:"used"`
	Remaining int64 `json:"remaining"`
}

type LegacyNode struct {
	Tag          string                 `json:"tag"`
	Type         string                 `json:"type"`
	Server       string                 `json:"server"`
	ServerPort   int                    `json:"server_port"`
	Extra        map[string]interface{} `json:"extra,omitempty"`
	Country      string                 `json:"country,omitempty"`
	CountryEmoji string                 `json:"country_emoji,omitempty"`
	Source       string                 `json:"source,omitempty"`
	SourceName   string                 `json:"source_name,omitempty"`
}

type LegacyManualNode struct {
	ID      string     `json:"id"`
	Node    LegacyNode `json:"node"`
	Enabled bool       `json:"enabled"`
}

type LegacyFilter struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Include          []string            `json:"include"`
	Exclude          []string            `json:"exclude"`
	IncludeCountries []string            `json:"include_countries"`
	ExcludeCountries []string            `json:"exclude_countries"`
	Mode             string              `json:"mode"`
	URLTestConfig    *LegacyURLTestConfig `json:"urltest_config,omitempty"`
	Subscriptions    []string            `json:"subscriptions"`
	AllNodes         bool                `json:"all_nodes"`
	Enabled          bool                `json:"enabled"`
}

type LegacyURLTestConfig struct {
	URL       string `json:"url"`
	Interval  string `json:"interval"`
	Tolerance int    `json:"tolerance"`
}

type LegacyRule struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	RuleType string   `json:"rule_type"`
	Values   []string `json:"values"`
	Outbound string   `json:"outbound"`
	Enabled  bool     `json:"enabled"`
	Priority int      `json:"priority"`
}

type LegacyRuleGroup struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	SiteRules []string `json:"site_rules"`
	IPRules   []string `json:"ip_rules"`
	Outbound  string   `json:"outbound"`
	Enabled   bool     `json:"enabled"`
}

type LegacySettings struct {
	SingBoxPath          string                    `json:"singbox_path"`
	ConfigPath           string                    `json:"config_path"`
	MixedPort            int                       `json:"mixed_port"`
	TunEnabled           bool                      `json:"tun_enabled"`
	ProxyDNS             string                    `json:"proxy_dns"`
	DirectDNS            string                    `json:"direct_dns"`
	Hosts                []LegacyHostEntry         `json:"hosts,omitempty"`
	FakeIPEnabled        bool                      `json:"fakeip_enabled,omitempty"`
	WebPort              int                       `json:"web_port"`
	ClashAPIPort         int                       `json:"clash_api_port"`
	ClashUIPath          string                    `json:"clash_ui_path"`
	FinalOutbound        string                    `json:"final_outbound"`
	RuleSetBaseURL       string                    `json:"ruleset_base_url"`
	AutoApply            bool                      `json:"auto_apply"`
	SubscriptionInterval int                       `json:"subscription_interval"`
	GithubProxy          string                    `json:"github_proxy"`
	ChainHealthConfig    *LegacyChainHealthConfig  `json:"chain_health_config,omitempty"`
}

type LegacyHostEntry struct {
	ID      string   `json:"id"`
	Domain  string   `json:"domain"`
	IPs     []string `json:"ips"`
	Enabled bool     `json:"enabled"`
}

type LegacyChainHealthConfig struct {
	Enabled      bool   `json:"enabled"`
	Interval     int    `json:"interval"`
	Timeout      int    `json:"timeout"`
	URL          string `json:"url"`
	AlertEnabled bool   `json:"alert_enabled"`
	AutoSwitch   bool   `json:"auto_switch"`
}

type LegacyProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

type LegacyInboundPort struct {
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Type     string              `json:"type"`
	Listen   string              `json:"listen"`
	Port     int                 `json:"port"`
	Auth     *LegacyInboundAuth  `json:"auth,omitempty"`
	Outbound string              `json:"outbound"`
	Enabled  bool                `json:"enabled"`
}

type LegacyInboundAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LegacyProxyChain struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	Nodes        []string                `json:"nodes"`
	ChainNodes   []LegacyChainNode       `json:"chain_nodes,omitempty"`
	Enabled      bool                    `json:"enabled"`
	HealthConfig *LegacyChainHealthConfig `json:"health_config,omitempty"`
}

type LegacyChainNode struct {
	OriginalTag string `json:"original_tag"`
	CopyTag     string `json:"copy_tag"`
	Source      string `json:"source"`
}

// MigrateFromJSON 从 data.json 迁移数据到 SQLite
func MigrateFromJSON(dataDir string, db *gorm.DB) error {
	jsonPath := filepath.Join(dataDir, "data.json")

	// 检查文件是否存在
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		logger.Info("未找到 data.json，跳过迁移")
		return nil
	}

	// 检查是否已迁移
	var nodeCount int64
	db.Model(&models.Node{}).Count(&nodeCount)
	if nodeCount > 0 {
		logger.Info("数据库已有数据，跳过迁移")
		return nil
	}

	logger.Info("开始从 data.json 迁移数据...")

	// 读取旧数据
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("读取 data.json 失败: %w", err)
	}

	var appData LegacyAppData
	if err := json.Unmarshal(data, &appData); err != nil {
		return fmt.Errorf("解析 data.json 失败: %w", err)
	}

	// 开始事务
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. 迁移订阅
		nodeTagMap := make(map[string]uint) // tag -> node ID
		for _, sub := range appData.Subscriptions {
			dbSub := &models.Subscription{
				ID:      sub.ID,
				Name:    sub.Name,
				URL:     sub.URL,
				Enabled: sub.Enabled,
			}
			if sub.Traffic != nil {
				dbSub.TrafficTotal = sub.Traffic.Total
				dbSub.TrafficUsed = sub.Traffic.Used
				dbSub.TrafficRemaining = sub.Traffic.Remaining
			}
			if err := tx.Create(dbSub).Error; err != nil {
				return fmt.Errorf("迁移订阅 %s 失败: %w", sub.Name, err)
			}

			// 迁移订阅中的节点
			for _, node := range sub.Nodes {
				dbNode := &models.Node{
					Tag:          node.Tag,
					Type:         node.Type,
					Server:       node.Server,
					ServerPort:   node.ServerPort,
					Source:       sub.ID,
					SourceName:   sub.Name,
					Country:      node.Country,
					CountryEmoji: node.CountryEmoji,
					Extra:        models.JSONMap(node.Extra),
					Link:         rebuildNodeLink(node),
					Enabled:      true,
					DelayStatus:  "untested",
					SpeedStatus:  "untested",
				}
				if err := tx.Create(dbNode).Error; err != nil {
					return fmt.Errorf("迁移节点 %s 失败: %w", node.Tag, err)
				}
				nodeTagMap[node.Tag] = dbNode.ID
			}
		}

		// 2. 迁移手动节点
		for _, mn := range appData.ManualNodes {
			dbNode := &models.Node{
				Tag:          mn.Node.Tag,
				Type:         mn.Node.Type,
				Server:       mn.Node.Server,
				ServerPort:   mn.Node.ServerPort,
				Source:       "manual",
				SourceName:   "手动添加",
				Country:      mn.Node.Country,
				CountryEmoji: mn.Node.CountryEmoji,
				Extra:        models.JSONMap(mn.Node.Extra),
				Link:         rebuildNodeLink(mn.Node),
				Enabled:      mn.Enabled,
				DelayStatus:  "untested",
				SpeedStatus:  "untested",
			}
			if err := tx.Create(dbNode).Error; err != nil {
				return fmt.Errorf("迁移手动节点 %s 失败: %w", mn.Node.Tag, err)
			}
			nodeTagMap[mn.Node.Tag] = dbNode.ID
		}

		// 3. 迁移链路
		for _, chain := range appData.ProxyChains {
			dbChain := &models.ProxyChain{
				ID:          chain.ID,
				Name:        chain.Name,
				Description: chain.Description,
				Enabled:     chain.Enabled,
			}
			if chain.HealthConfig != nil {
				dbChain.HealthEnabled = chain.HealthConfig.Enabled
				dbChain.HealthInterval = chain.HealthConfig.Interval
				dbChain.HealthTimeout = chain.HealthConfig.Timeout
				dbChain.HealthURL = chain.HealthConfig.URL
			}
			if err := tx.Create(dbChain).Error; err != nil {
				return fmt.Errorf("迁移链路 %s 失败: %w", chain.Name, err)
			}

			// 迁移链路节点
			for i, nodeTag := range chain.Nodes {
				if nodeID, ok := nodeTagMap[nodeTag]; ok {
					chainNode := &models.ProxyChainNode{
						ChainID:  chain.ID,
						NodeID:   nodeID,
						Position: i,
					}
					if err := tx.Create(chainNode).Error; err != nil {
						return fmt.Errorf("迁移链路节点失败: %w", err)
					}
				}
			}
		}

		// 4. 迁移过滤器
		for _, filter := range appData.Filters {
			dbFilter := &models.Filter{
				ID:               filter.ID,
				Name:             filter.Name,
				Include:          filter.Include,
				Exclude:          filter.Exclude,
				IncludeCountries: filter.IncludeCountries,
				ExcludeCountries: filter.ExcludeCountries,
				Mode:             filter.Mode,
				Subscriptions:    filter.Subscriptions,
				AllNodes:         filter.AllNodes,
				Enabled:          filter.Enabled,
			}
			if filter.URLTestConfig != nil {
				dbFilter.URLTestURL = filter.URLTestConfig.URL
				dbFilter.URLTestInterval = filter.URLTestConfig.Interval
				dbFilter.URLTestTolerance = filter.URLTestConfig.Tolerance
			}
			if err := tx.Create(dbFilter).Error; err != nil {
				return fmt.Errorf("迁移过滤器 %s 失败: %w", filter.Name, err)
			}
		}

		// 5. 迁移自定义规则
		for _, rule := range appData.Rules {
			dbRule := &models.Rule{
				ID:       rule.ID,
				Name:     rule.Name,
				RuleType: rule.RuleType,
				Values:   rule.Values,
				Outbound: rule.Outbound,
				Enabled:  rule.Enabled,
				Priority: rule.Priority,
			}
			if err := tx.Create(dbRule).Error; err != nil {
				return fmt.Errorf("迁移规则 %s 失败: %w", rule.Name, err)
			}
		}

		// 6. 迁移规则组（更新而非创建，因为默认数据已存在）
		for _, rg := range appData.RuleGroups {
			if err := tx.Model(&models.RuleGroup{}).Where("id = ?", rg.ID).Updates(map[string]interface{}{
				"name":       rg.Name,
				"site_rules": models.StringSlice(rg.SiteRules),
				"ip_rules":   models.StringSlice(rg.IPRules),
				"outbound":   rg.Outbound,
				"enabled":    rg.Enabled,
			}).Error; err != nil {
				logger.Warn("更新规则组 %s 失败: %v", rg.ID, err)
			}
		}

		// 7. 迁移入站端口
		for _, port := range appData.InboundPorts {
			dbPort := &models.InboundPort{
				ID:       port.ID,
				Name:     port.Name,
				Type:     port.Type,
				Listen:   port.Listen,
				Port:     port.Port,
				Outbound: port.Outbound,
				Enabled:  port.Enabled,
			}
			if port.Auth != nil {
				dbPort.AuthUsername = port.Auth.Username
				dbPort.AuthPassword = port.Auth.Password
			}
			if err := tx.Create(dbPort).Error; err != nil {
				return fmt.Errorf("迁移入站端口 %s 失败: %w", port.Name, err)
			}
		}

		// 8. 迁移 hosts
		if appData.Settings != nil {
			for _, host := range appData.Settings.Hosts {
				dbHost := &models.HostEntry{
					ID:      host.ID,
					Domain:  host.Domain,
					IPs:     host.IPs,
					Enabled: host.Enabled,
				}
				if err := tx.Create(dbHost).Error; err != nil {
					return fmt.Errorf("迁移 hosts %s 失败: %w", host.Domain, err)
				}
			}
		}

		// 9. 迁移设置
		if appData.Settings != nil {
			settingsMap := map[string]string{
				"singbox_path":          appData.Settings.SingBoxPath,
				"config_path":           appData.Settings.ConfigPath,
				"mixed_port":            strconv.Itoa(appData.Settings.MixedPort),
				"tun_enabled":           strconv.FormatBool(appData.Settings.TunEnabled),
				"proxy_dns":             appData.Settings.ProxyDNS,
				"direct_dns":            appData.Settings.DirectDNS,
				"fakeip_enabled":        strconv.FormatBool(appData.Settings.FakeIPEnabled),
				"web_port":              strconv.Itoa(appData.Settings.WebPort),
				"clash_api_port":        strconv.Itoa(appData.Settings.ClashAPIPort),
				"clash_ui_path":         appData.Settings.ClashUIPath,
				"final_outbound":        appData.Settings.FinalOutbound,
				"ruleset_base_url":      appData.Settings.RuleSetBaseURL,
				"auto_apply":            strconv.FormatBool(appData.Settings.AutoApply),
				"subscription_interval": strconv.Itoa(appData.Settings.SubscriptionInterval),
				"github_proxy":          appData.Settings.GithubProxy,
			}
			if appData.Settings.ChainHealthConfig != nil {
				settingsMap["chain_health_enabled"] = strconv.FormatBool(appData.Settings.ChainHealthConfig.Enabled)
				settingsMap["chain_health_interval"] = strconv.Itoa(appData.Settings.ChainHealthConfig.Interval)
				settingsMap["chain_health_timeout"] = strconv.Itoa(appData.Settings.ChainHealthConfig.Timeout)
				settingsMap["chain_health_url"] = appData.Settings.ChainHealthConfig.URL
			}
			for key, value := range settingsMap {
				if value != "" {
					if err := tx.Model(&models.Setting{}).Where("key = ?", key).Update("value", value).Error; err != nil {
						logger.Warn("更新设置 %s 失败: %v", key, err)
					}
				}
			}
		}

		// 10. 迁移 Profile
		for _, profile := range appData.Profiles {
			dbProfile := &models.Profile{
				ID:          profile.ID,
				Name:        profile.Name,
				Description: profile.Description,
				IsActive:    profile.IsActive,
			}
			if err := tx.Create(dbProfile).Error; err != nil {
				return fmt.Errorf("迁移 Profile %s 失败: %w", profile.Name, err)
			}
		}

		logger.Info("数据迁移完成")

		// 备份旧文件
		backupPath := jsonPath + ".migrated"
		if err := os.Rename(jsonPath, backupPath); err != nil {
			logger.Warn("备份 data.json 失败: %v", err)
		} else {
			logger.Info("旧数据文件已备份至: %s", backupPath)
		}

		return nil
	})
}

// rebuildNodeLink 从节点信息重建链接（简化版本）
// TODO: 完善各协议的链接重建
func rebuildNodeLink(node LegacyNode) string {
	// 这里可以根据 node.Extra 重建完整的代理链接
	// 暂时返回空，后续测速功能实现时再完善
	return ""
}
