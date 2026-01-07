package database

import (
	"fmt"
	"strconv"

	"github.com/xiaobei/singbox-manager/internal/database/models"
	"gorm.io/gorm"
)

// Store 数据库存储接口
type Store struct {
	db *gorm.DB
}

// NewStore 创建存储实例
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// ==================== 订阅操作 ====================

// GetSubscriptions 获取所有订阅
func (s *Store) GetSubscriptions() ([]models.Subscription, error) {
	var subs []models.Subscription
	err := s.db.Order("created_at DESC").Find(&subs).Error
	return subs, err
}

// GetSubscription 获取单个订阅
func (s *Store) GetSubscription(id string) (*models.Subscription, error) {
	var sub models.Subscription
	err := s.db.First(&sub, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// GetSubscriptionWithNodes 获取订阅及其节点
func (s *Store) GetSubscriptionWithNodes(id string) (*models.Subscription, error) {
	var sub models.Subscription
	err := s.db.Preload("Nodes").First(&sub, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// CreateSubscription 创建订阅
func (s *Store) CreateSubscription(sub *models.Subscription) error {
	return s.db.Create(sub).Error
}

// UpdateSubscription 更新订阅
func (s *Store) UpdateSubscription(sub *models.Subscription) error {
	return s.db.Save(sub).Error
}

// DeleteSubscription 删除订阅（同时删除关联节点）
func (s *Store) DeleteSubscription(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 删除关联节点
		if err := tx.Where("source = ?", id).Delete(&models.Node{}).Error; err != nil {
			return err
		}
		// 删除订阅
		return tx.Delete(&models.Subscription{}, "id = ?", id).Error
	})
}

// ==================== 节点操作 ====================

// GetNodes 获取所有节点
func (s *Store) GetNodes() ([]models.Node, error) {
	var nodes []models.Node
	err := s.db.Order("created_at DESC").Find(&nodes).Error
	return nodes, err
}

// GetEnabledNodes 获取所有启用的节点
func (s *Store) GetEnabledNodes() ([]models.Node, error) {
	var nodes []models.Node
	err := s.db.Where("enabled = ?", true).Order("created_at DESC").Find(&nodes).Error
	return nodes, err
}

// GetNodesBySource 按来源获取节点
func (s *Store) GetNodesBySource(source string) ([]models.Node, error) {
	var nodes []models.Node
	err := s.db.Where("source = ?", source).Find(&nodes).Error
	return nodes, err
}

// GetNodesByCountry 按国家获取节点
func (s *Store) GetNodesByCountry(country string) ([]models.Node, error) {
	var nodes []models.Node
	err := s.db.Where("country = ? AND enabled = ?", country, true).Find(&nodes).Error
	return nodes, err
}

// GetNode 获取单个节点
func (s *Store) GetNode(id uint) (*models.Node, error) {
	var node models.Node
	err := s.db.First(&node, id).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// GetNodeByTag 按 tag 获取节点
func (s *Store) GetNodeByTag(tag string) (*models.Node, error) {
	var node models.Node
	err := s.db.First(&node, "tag = ?", tag).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// CreateNode 创建节点
func (s *Store) CreateNode(node *models.Node) error {
	return s.db.Create(node).Error
}

// UpdateNode 更新节点
func (s *Store) UpdateNode(node *models.Node) error {
	return s.db.Save(node).Error
}

// DeleteNode 删除节点
func (s *Store) DeleteNode(id uint) error {
	return s.db.Delete(&models.Node{}, id).Error
}

// BatchCreateNodes 批量创建节点
func (s *Store) BatchCreateNodes(nodes []models.Node) error {
	if len(nodes) == 0 {
		return nil
	}
	return s.db.CreateInBatches(nodes, 100).Error
}

// DeleteNodesBySource 删除指定来源的所有节点
func (s *Store) DeleteNodesBySource(source string) error {
	return s.db.Where("source = ?", source).Delete(&models.Node{}).Error
}

// GetCountryStats 获取国家统计
func (s *Store) GetCountryStats() ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	err := s.db.Model(&models.Node{}).
		Select("country, country_emoji, COUNT(*) as count").
		Where("enabled = ? AND country != ''", true).
		Group("country").
		Order("count DESC").
		Find(&results).Error
	return results, err
}

// ==================== 代理链路操作 ====================

// GetProxyChains 获取所有链路
func (s *Store) GetProxyChains() ([]models.ProxyChain, error) {
	var chains []models.ProxyChain
	err := s.db.Preload("ChainNodes", func(db *gorm.DB) *gorm.DB {
		return db.Order("position ASC")
	}).Preload("ChainNodes.Node").Find(&chains).Error
	return chains, err
}

// GetProxyChain 获取单个链路
func (s *Store) GetProxyChain(id string) (*models.ProxyChain, error) {
	var chain models.ProxyChain
	err := s.db.Preload("ChainNodes", func(db *gorm.DB) *gorm.DB {
		return db.Order("position ASC")
	}).Preload("ChainNodes.Node").First(&chain, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &chain, nil
}

// CreateProxyChain 创建链路
func (s *Store) CreateProxyChain(chain *models.ProxyChain) error {
	return s.db.Create(chain).Error
}

// UpdateProxyChain 更新链路
func (s *Store) UpdateProxyChain(chain *models.ProxyChain) error {
	return s.db.Save(chain).Error
}

// DeleteProxyChain 删除链路
func (s *Store) DeleteProxyChain(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 删除链路节点关联
		if err := tx.Where("chain_id = ?", id).Delete(&models.ProxyChainNode{}).Error; err != nil {
			return err
		}
		// 删除链路
		return tx.Delete(&models.ProxyChain{}, "id = ?", id).Error
	})
}

// SetChainNodes 设置链路节点（替换所有）
func (s *Store) SetChainNodes(chainID string, nodeIDs []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 删除旧的关联
		if err := tx.Where("chain_id = ?", chainID).Delete(&models.ProxyChainNode{}).Error; err != nil {
			return err
		}
		// 创建新的关联
		for i, nodeID := range nodeIDs {
			chainNode := &models.ProxyChainNode{
				ChainID:  chainID,
				NodeID:   nodeID,
				Position: i,
			}
			if err := tx.Create(chainNode).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ==================== 设置操作 ====================

// GetSetting 获取设置值
func (s *Store) GetSetting(key string) (string, error) {
	var setting models.Setting
	err := s.db.First(&setting, "key = ?", key).Error
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

// GetSettingInt 获取整数设置
func (s *Store) GetSettingInt(key string, defaultVal int) int {
	val, err := s.GetSetting(key)
	if err != nil {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

// GetSettingBool 获取布尔设置
func (s *Store) GetSettingBool(key string, defaultVal bool) bool {
	val, err := s.GetSetting(key)
	if err != nil {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}

// SetSetting 设置值
func (s *Store) SetSetting(key, value string) error {
	return s.db.Save(&models.Setting{Key: key, Value: value}).Error
}

// GetAllSettings 获取所有设置
func (s *Store) GetAllSettings() (map[string]string, error) {
	var settings []models.Setting
	if err := s.db.Find(&settings).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

// ==================== 规则操作 ====================

// GetRules 获取所有规则
func (s *Store) GetRules() ([]models.Rule, error) {
	var rules []models.Rule
	err := s.db.Order("priority ASC").Find(&rules).Error
	return rules, err
}

// GetRule 获取单个规则
func (s *Store) GetRule(id string) (*models.Rule, error) {
	var rule models.Rule
	err := s.db.First(&rule, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// CreateRule 创建规则
func (s *Store) CreateRule(rule *models.Rule) error {
	return s.db.Create(rule).Error
}

// UpdateRule 更新规则
func (s *Store) UpdateRule(rule *models.Rule) error {
	return s.db.Save(rule).Error
}

// DeleteRule 删除规则
func (s *Store) DeleteRule(id string) error {
	return s.db.Delete(&models.Rule{}, "id = ?", id).Error
}

// ==================== 规则组操作 ====================

// GetRuleGroups 获取所有规则组
func (s *Store) GetRuleGroups() ([]models.RuleGroup, error) {
	var groups []models.RuleGroup
	err := s.db.Order("position ASC").Find(&groups).Error
	return groups, err
}

// GetRuleGroup 获取单个规则组
func (s *Store) GetRuleGroup(id string) (*models.RuleGroup, error) {
	var group models.RuleGroup
	err := s.db.First(&group, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

// UpdateRuleGroup 更新规则组
func (s *Store) UpdateRuleGroup(group *models.RuleGroup) error {
	return s.db.Save(group).Error
}

// ==================== 过滤器操作 ====================

// GetFilters 获取所有过滤器
func (s *Store) GetFilters() ([]models.Filter, error) {
	var filters []models.Filter
	err := s.db.Find(&filters).Error
	return filters, err
}

// GetFilter 获取单个过滤器
func (s *Store) GetFilter(id string) (*models.Filter, error) {
	var filter models.Filter
	err := s.db.First(&filter, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &filter, nil
}

// CreateFilter 创建过滤器
func (s *Store) CreateFilter(filter *models.Filter) error {
	return s.db.Create(filter).Error
}

// UpdateFilter 更新过滤器
func (s *Store) UpdateFilter(filter *models.Filter) error {
	return s.db.Save(filter).Error
}

// DeleteFilter 删除过滤器
func (s *Store) DeleteFilter(id string) error {
	return s.db.Delete(&models.Filter{}, "id = ?", id).Error
}

// ==================== 入站端口操作 ====================

// GetInboundPorts 获取所有入站端口
func (s *Store) GetInboundPorts() ([]models.InboundPort, error) {
	var ports []models.InboundPort
	err := s.db.Find(&ports).Error
	return ports, err
}

// GetInboundPort 获取单个入站端口
func (s *Store) GetInboundPort(id string) (*models.InboundPort, error) {
	var port models.InboundPort
	err := s.db.First(&port, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &port, nil
}

// CreateInboundPort 创建入站端口
func (s *Store) CreateInboundPort(port *models.InboundPort) error {
	return s.db.Create(port).Error
}

// UpdateInboundPort 更新入站端口
func (s *Store) UpdateInboundPort(port *models.InboundPort) error {
	return s.db.Save(port).Error
}

// DeleteInboundPort 删除入站端口
func (s *Store) DeleteInboundPort(id string) error {
	return s.db.Delete(&models.InboundPort{}, "id = ?", id).Error
}

// ==================== Hosts 操作 ====================

// GetHostEntries 获取所有 hosts
func (s *Store) GetHostEntries() ([]models.HostEntry, error) {
	var hosts []models.HostEntry
	err := s.db.Find(&hosts).Error
	return hosts, err
}

// CreateHostEntry 创建 host
func (s *Store) CreateHostEntry(host *models.HostEntry) error {
	return s.db.Create(host).Error
}

// UpdateHostEntry 更新 host
func (s *Store) UpdateHostEntry(host *models.HostEntry) error {
	return s.db.Save(host).Error
}

// DeleteHostEntry 删除 host
func (s *Store) DeleteHostEntry(id string) error {
	return s.db.Delete(&models.HostEntry{}, "id = ?", id).Error
}

// ==================== Profile 操作 ====================

// GetProfiles 获取所有 Profile
func (s *Store) GetProfiles() ([]models.Profile, error) {
	var profiles []models.Profile
	err := s.db.Order("created_at DESC").Find(&profiles).Error
	return profiles, err
}

// GetProfile 获取单个 Profile
func (s *Store) GetProfile(id string) (*models.Profile, error) {
	var profile models.Profile
	err := s.db.First(&profile, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetActiveProfile 获取激活的 Profile
func (s *Store) GetActiveProfile() (*models.Profile, error) {
	var profile models.Profile
	err := s.db.First(&profile, "is_active = ?", true).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// CreateProfile 创建 Profile
func (s *Store) CreateProfile(profile *models.Profile) error {
	return s.db.Create(profile).Error
}

// UpdateProfile 更新 Profile
func (s *Store) UpdateProfile(profile *models.Profile) error {
	return s.db.Save(profile).Error
}

// DeleteProfile 删除 Profile
func (s *Store) DeleteProfile(id string) error {
	return s.db.Delete(&models.Profile{}, "id = ?", id).Error
}

// SetActiveProfile 设置激活的 Profile
func (s *Store) SetActiveProfile(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 取消所有激活状态
		if err := tx.Model(&models.Profile{}).Where("is_active = ?", true).Update("is_active", false).Error; err != nil {
			return err
		}
		// 设置新的激活状态
		return tx.Model(&models.Profile{}).Where("id = ?", id).Update("is_active", true).Error
	})
}

// ==================== 数据库实用方法 ====================

// Transaction 事务
func (s *Store) Transaction(fn func(tx *gorm.DB) error) error {
	return s.db.Transaction(fn)
}

// DB 获取原始 DB 实例
func (s *Store) DB() *gorm.DB {
	return s.db
}

// ExportToJSON 导出所有数据为 JSON（用于备份）
func (s *Store) ExportToJSON() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// 导出订阅
	subs, err := s.GetSubscriptions()
	if err != nil {
		return nil, fmt.Errorf("导出订阅失败: %w", err)
	}
	result["subscriptions"] = subs

	// 导出节点
	nodes, err := s.GetNodes()
	if err != nil {
		return nil, fmt.Errorf("导出节点失败: %w", err)
	}
	result["nodes"] = nodes

	// 导出链路
	chains, err := s.GetProxyChains()
	if err != nil {
		return nil, fmt.Errorf("导出链路失败: %w", err)
	}
	result["proxy_chains"] = chains

	// 导出规则
	rules, err := s.GetRules()
	if err != nil {
		return nil, fmt.Errorf("导出规则失败: %w", err)
	}
	result["rules"] = rules

	// 导出规则组
	groups, err := s.GetRuleGroups()
	if err != nil {
		return nil, fmt.Errorf("导出规则组失败: %w", err)
	}
	result["rule_groups"] = groups

	// 导出过滤器
	filters, err := s.GetFilters()
	if err != nil {
		return nil, fmt.Errorf("导出过滤器失败: %w", err)
	}
	result["filters"] = filters

	// 导出入站端口
	ports, err := s.GetInboundPorts()
	if err != nil {
		return nil, fmt.Errorf("导出入站端口失败: %w", err)
	}
	result["inbound_ports"] = ports

	// 导出设置
	settings, err := s.GetAllSettings()
	if err != nil {
		return nil, fmt.Errorf("导出设置失败: %w", err)
	}
	result["settings"] = settings

	return result, nil
}

// ==================== 测速策略操作 ====================

// GetSpeedTestProfiles 获取所有测速策略
func (s *Store) GetSpeedTestProfiles() ([]models.SpeedTestProfile, error) {
	var profiles []models.SpeedTestProfile
	err := s.db.Order("created_at DESC").Find(&profiles).Error
	return profiles, err
}

// GetSpeedTestProfile 获取单个测速策略
func (s *Store) GetSpeedTestProfile(id uint) (*models.SpeedTestProfile, error) {
	var profile models.SpeedTestProfile
	err := s.db.First(&profile, id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetDefaultSpeedTestProfile 获取默认测速策略
func (s *Store) GetDefaultSpeedTestProfile() (*models.SpeedTestProfile, error) {
	var profile models.SpeedTestProfile
	err := s.db.First(&profile, "is_default = ?", true).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// CreateSpeedTestProfile 创建测速策略
func (s *Store) CreateSpeedTestProfile(profile *models.SpeedTestProfile) error {
	return s.db.Create(profile).Error
}

// UpdateSpeedTestProfile 更新测速策略
func (s *Store) UpdateSpeedTestProfile(profile *models.SpeedTestProfile) error {
	return s.db.Save(profile).Error
}

// DeleteSpeedTestProfile 删除测速策略
func (s *Store) DeleteSpeedTestProfile(id uint) error {
	return s.db.Delete(&models.SpeedTestProfile{}, id).Error
}

// ==================== 测速任务操作 ====================

// GetSpeedTestTasks 获取所有测速任务
func (s *Store) GetSpeedTestTasks(limit int) ([]models.SpeedTestTask, error) {
	var tasks []models.SpeedTestTask
	query := s.db.Preload("Profile").Order("started_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&tasks).Error
	return tasks, err
}

// GetSpeedTestTask 获取单个测速任务
func (s *Store) GetSpeedTestTask(id string) (*models.SpeedTestTask, error) {
	var task models.SpeedTestTask
	err := s.db.Preload("Profile").First(&task, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// CreateSpeedTestTask 创建测速任务
func (s *Store) CreateSpeedTestTask(task *models.SpeedTestTask) error {
	return s.db.Create(task).Error
}

// UpdateSpeedTestTask 更新测速任务
func (s *Store) UpdateSpeedTestTask(task *models.SpeedTestTask) error {
	return s.db.Save(task).Error
}

// ==================== 测速历史操作 ====================

// GetSpeedTestHistory 获取测速历史
func (s *Store) GetSpeedTestHistory(nodeID uint, limit int) ([]models.SpeedTestHistory, error) {
	var history []models.SpeedTestHistory
	query := s.db.Where("node_id = ?", nodeID).Order("tested_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&history).Error
	return history, err
}

// GetSpeedTestHistoryByTask 按任务获取测速历史
func (s *Store) GetSpeedTestHistoryByTask(taskID string) ([]models.SpeedTestHistory, error) {
	var history []models.SpeedTestHistory
	err := s.db.Preload("Node").Where("task_id = ?", taskID).Find(&history).Error
	return history, err
}

// CreateSpeedTestHistory 创建测速历史
func (s *Store) CreateSpeedTestHistory(history *models.SpeedTestHistory) error {
	return s.db.Create(history).Error
}

// BatchCreateSpeedTestHistory 批量创建测速历史
func (s *Store) BatchCreateSpeedTestHistory(histories []models.SpeedTestHistory) error {
	if len(histories) == 0 {
		return nil
	}
	return s.db.CreateInBatches(histories, 100).Error
}

// CleanOldSpeedTestHistory 清理旧的测速历史 (保留最近 N 天)
func (s *Store) CleanOldSpeedTestHistory(days int) error {
	return s.db.Where("tested_at < datetime('now', ?)", fmt.Sprintf("-%d days", days)).
		Delete(&models.SpeedTestHistory{}).Error
}

// ==================== 标签操作 ====================

// GetTags 获取所有标签
func (s *Store) GetTags() ([]models.Tag, error) {
	var tags []models.Tag
	err := s.db.Order("created_at DESC").Find(&tags).Error
	return tags, err
}

// GetTag 获取单个标签
func (s *Store) GetTag(id uint) (*models.Tag, error) {
	var tag models.Tag
	err := s.db.First(&tag, id).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// GetTagByName 按名称获取标签
func (s *Store) GetTagByName(name string) (*models.Tag, error) {
	var tag models.Tag
	err := s.db.First(&tag, "name = ?", name).Error
	if err != nil {
		return nil, err
	}
	return &tag, nil
}

// CreateTag 创建标签
func (s *Store) CreateTag(tag *models.Tag) error {
	return s.db.Create(tag).Error
}

// UpdateTag 更新标签
func (s *Store) UpdateTag(tag *models.Tag) error {
	return s.db.Save(tag).Error
}

// DeleteTag 删除标签（同时删除关联的规则和节点标签）
func (s *Store) DeleteTag(id uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 删除关联的标签规则
		if err := tx.Where("tag_id = ?", id).Delete(&models.TagRule{}).Error; err != nil {
			return err
		}
		// 删除节点标签关联
		if err := tx.Exec("DELETE FROM node_tags WHERE tag_id = ?", id).Error; err != nil {
			return err
		}
		// 删除标签
		return tx.Delete(&models.Tag{}, id).Error
	})
}

// GetTagGroups 获取所有标签组名称
func (s *Store) GetTagGroups() ([]string, error) {
	var groups []string
	err := s.db.Model(&models.Tag{}).
		Where("tag_group != '' AND tag_group IS NOT NULL").
		Distinct("tag_group").
		Pluck("tag_group", &groups).Error
	return groups, err
}

// GetTagsByGroup 获取指定组的所有标签
func (s *Store) GetTagsByGroup(groupName string) ([]models.Tag, error) {
	var tags []models.Tag
	err := s.db.Where("tag_group = ?", groupName).Find(&tags).Error
	return tags, err
}

// ==================== 标签规则操作 ====================

// GetTagRules 获取所有标签规则
func (s *Store) GetTagRules() ([]models.TagRule, error) {
	var rules []models.TagRule
	err := s.db.Preload("Tag").Order("created_at DESC").Find(&rules).Error
	return rules, err
}

// GetTagRule 获取单个标签规则
func (s *Store) GetTagRule(id uint) (*models.TagRule, error) {
	var rule models.TagRule
	err := s.db.Preload("Tag").First(&rule, id).Error
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

// GetTagRulesByTag 按标签获取规则
func (s *Store) GetTagRulesByTag(tagID uint) ([]models.TagRule, error) {
	var rules []models.TagRule
	err := s.db.Where("tag_id = ?", tagID).Find(&rules).Error
	return rules, err
}

// GetEnabledTagRulesByTrigger 按触发类型获取启用的规则
func (s *Store) GetEnabledTagRulesByTrigger(triggerType string) ([]models.TagRule, error) {
	var rules []models.TagRule
	err := s.db.Preload("Tag").
		Where("enabled = ? AND trigger_type = ?", true, triggerType).
		Find(&rules).Error
	return rules, err
}

// CreateTagRule 创建标签规则
func (s *Store) CreateTagRule(rule *models.TagRule) error {
	return s.db.Create(rule).Error
}

// UpdateTagRule 更新标签规则
func (s *Store) UpdateTagRule(rule *models.TagRule) error {
	return s.db.Save(rule).Error
}

// DeleteTagRule 删除标签规则
func (s *Store) DeleteTagRule(id uint) error {
	return s.db.Delete(&models.TagRule{}, id).Error
}

// ==================== 节点标签操作 ====================

// GetNodeTags 获取节点的所有标签
func (s *Store) GetNodeTags(nodeID uint) ([]models.Tag, error) {
	var node models.Node
	err := s.db.Preload("Tags").First(&node, nodeID).Error
	if err != nil {
		return nil, err
	}
	return node.Tags, nil
}

// AddNodeTag 为节点添加标签
func (s *Store) AddNodeTag(nodeID uint, tagID uint) error {
	return s.db.Exec("INSERT OR IGNORE INTO node_tags (node_id, tag_id) VALUES (?, ?)", nodeID, tagID).Error
}

// RemoveNodeTag 移除节点标签
func (s *Store) RemoveNodeTag(nodeID uint, tagID uint) error {
	return s.db.Exec("DELETE FROM node_tags WHERE node_id = ? AND tag_id = ?", nodeID, tagID).Error
}

// SetNodeTags 设置节点的标签（替换所有）
func (s *Store) SetNodeTags(nodeID uint, tagIDs []uint) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 删除所有现有标签
		if err := tx.Exec("DELETE FROM node_tags WHERE node_id = ?", nodeID).Error; err != nil {
			return err
		}
		// 添加新标签
		for _, tagID := range tagIDs {
			if err := tx.Exec("INSERT INTO node_tags (node_id, tag_id) VALUES (?, ?)", nodeID, tagID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetNodesByTag 获取具有指定标签的所有节点
func (s *Store) GetNodesByTag(tagID uint) ([]models.Node, error) {
	var nodes []models.Node
	err := s.db.Joins("JOIN node_tags ON node_tags.node_id = nodes.id").
		Where("node_tags.tag_id = ?", tagID).
		Find(&nodes).Error
	return nodes, err
}

// ClearTagFromAllNodes 从所有节点移除指定标签
func (s *Store) ClearTagFromAllNodes(tagID uint) error {
	return s.db.Exec("DELETE FROM node_tags WHERE tag_id = ?", tagID).Error
}

// BatchAddNodeTags 批量为节点添加标签
func (s *Store) BatchAddNodeTags(nodeIDs []uint, tagID uint) error {
	if len(nodeIDs) == 0 {
		return nil
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, nodeID := range nodeIDs {
			if err := tx.Exec("INSERT OR IGNORE INTO node_tags (node_id, tag_id) VALUES (?, ?)", nodeID, tagID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// BatchRemoveNodeTags 批量移除节点标签
func (s *Store) BatchRemoveNodeTags(nodeIDs []uint, tagID uint) error {
	if len(nodeIDs) == 0 {
		return nil
	}
	return s.db.Exec("DELETE FROM node_tags WHERE node_id IN ? AND tag_id = ?", nodeIDs, tagID).Error
}
