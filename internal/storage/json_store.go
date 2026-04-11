package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// JSONStore JSON 文件存储实现
type JSONStore struct {
	dataDir string
	mu      sync.RWMutex
	data    *AppData
}

// NewJSONStore 创建新的 JSON 存储
func NewJSONStore(dataDir string) (*JSONStore, error) {
	store := &JSONStore{
		dataDir: dataDir,
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 确保 generated 子目录存在
	generatedDir := filepath.Join(dataDir, "generated")
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		return nil, fmt.Errorf("创建 generated 目录失败: %w", err)
	}

	// 加载数据
	if err := store.load(); err != nil {
		return nil, err
	}

	return store, nil
}

// load 加载数据
func (s *JSONStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dataFile := filepath.Join(s.dataDir, "data.json")

	// 如果文件不存在，初始化默认数据
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		s.data = &AppData{
			Subscriptions: []Subscription{},
			ManualNodes:   []ManualNode{},
			Filters:       []Filter{},
			Rules:         []Rule{},
			RuleGroups:    DefaultRuleGroups(),
			Settings:      DefaultSettings(),
		}
		return s.saveInternal()
	}

	// 读取文件
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return fmt.Errorf("读取数据文件失败: %w", err)
	}

	s.data = &AppData{}
	if err := json.Unmarshal(data, s.data); err != nil {
		return fmt.Errorf("解析数据文件失败: %w", err)
	}

	needSave := false

	// 确保 Settings 不为空
	if s.data.Settings == nil {
		s.data.Settings = DefaultSettings()
	} else {
		migrateLegacyZashboardSettings(data, s.data.Settings, &needSave)
	}

	// 确保 RuleGroups 不为空
	if len(s.data.RuleGroups) == 0 {
		s.data.RuleGroups = DefaultRuleGroups()
	}

	// 迁移旧的路径格式（移除多余的 data/ 前缀）
	if s.data.Settings.SingBoxPath == "data/bin/sing-box" {
		s.data.Settings.SingBoxPath = "bin/sing-box"
		needSave = true
	}
	if s.data.Settings.ConfigPath == "data/generated/config.json" {
		s.data.Settings.ConfigPath = "generated/config.json"
		needSave = true
	}

	// 迁移订阅字段：为旧数据补齐 auto_update 和 update_interval 默认值
	for i := range s.data.Subscriptions {
		if s.data.Subscriptions[i].AutoUpdate == nil {
			defaultAutoUpdate := false // 默认不开启自动更新，避免意外行为
			s.data.Subscriptions[i].AutoUpdate = &defaultAutoUpdate
			needSave = true
		}
		if s.data.Subscriptions[i].UpdateInterval <= 0 {
			s.data.Subscriptions[i].UpdateInterval = 60 // 默认 60 分钟
			needSave = true
		}
	}

	if needSave {
		return s.saveInternal()
	}

	return nil
}

// saveInternal 内部保存方法（不加锁）
func (s *JSONStore) saveInternal() error {
	dataFile := filepath.Join(s.dataDir, "data.json")

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化数据失败: %w", err)
	}

	if err := os.WriteFile(dataFile, data, 0644); err != nil {
		return fmt.Errorf("写入数据文件失败: %w", err)
	}

	return nil
}

// Save 保存数据
func (s *JSONStore) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveInternal()
}

// ==================== 订阅操作 ====================

// GetSubscriptions 获取所有订阅
func (s *JSONStore) GetSubscriptions() []Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Subscriptions
}

// GetSubscription 获取单个订阅
func (s *JSONStore) GetSubscription(id string) *Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.data.Subscriptions {
		if s.data.Subscriptions[i].ID == id {
			return &s.data.Subscriptions[i]
		}
	}
	return nil
}

// AddSubscription 添加订阅
func (s *JSONStore) AddSubscription(sub Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Subscriptions = append(s.data.Subscriptions, sub)
	return s.saveInternal()
}

// UpdateSubscription 更新订阅
func (s *JSONStore) UpdateSubscription(sub Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Subscriptions {
		if s.data.Subscriptions[i].ID == sub.ID {
			s.data.Subscriptions[i] = sub
			return s.saveInternal()
		}
	}
	return fmt.Errorf("订阅不存在: %s", sub.ID)
}

// DeleteSubscription 删除订阅
func (s *JSONStore) DeleteSubscription(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Subscriptions {
		if s.data.Subscriptions[i].ID == id {
			s.data.Subscriptions = append(s.data.Subscriptions[:i], s.data.Subscriptions[i+1:]...)
			return s.saveInternal()
		}
	}
	return fmt.Errorf("订阅不存在: %s", id)
}

// ==================== 过滤器操作 ====================

// GetFilters 获取所有过滤器
func (s *JSONStore) GetFilters() []Filter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Filters
}

// GetFilter 获取单个过滤器
func (s *JSONStore) GetFilter(id string) *Filter {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.data.Filters {
		if s.data.Filters[i].ID == id {
			return &s.data.Filters[i]
		}
	}
	return nil
}

// AddFilter 添加过滤器
func (s *JSONStore) AddFilter(filter Filter) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Filters = append(s.data.Filters, filter)
	return s.saveInternal()
}

// UpdateFilter 更新过滤器
func (s *JSONStore) UpdateFilter(filter Filter) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Filters {
		if s.data.Filters[i].ID == filter.ID {
			s.data.Filters[i] = filter
			return s.saveInternal()
		}
	}
	return fmt.Errorf("过滤器不存在: %s", filter.ID)
}

// DeleteFilter 删除过滤器
func (s *JSONStore) DeleteFilter(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Filters {
		if s.data.Filters[i].ID == id {
			s.data.Filters = append(s.data.Filters[:i], s.data.Filters[i+1:]...)
			return s.saveInternal()
		}
	}
	return fmt.Errorf("过滤器不存在: %s", id)
}

// ==================== 规则操作 ====================

// GetRules 获取所有自定义规则
func (s *JSONStore) GetRules() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Rules
}

// AddRule 添加规则
func (s *JSONStore) AddRule(rule Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Rules = append(s.data.Rules, rule)
	return s.saveInternal()
}

// UpdateRule 更新规则
func (s *JSONStore) UpdateRule(rule Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Rules {
		if s.data.Rules[i].ID == rule.ID {
			s.data.Rules[i] = rule
			return s.saveInternal()
		}
	}
	return fmt.Errorf("规则不存在: %s", rule.ID)
}

// DeleteRule 删除规则
func (s *JSONStore) DeleteRule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Rules {
		if s.data.Rules[i].ID == id {
			s.data.Rules = append(s.data.Rules[:i], s.data.Rules[i+1:]...)
			return s.saveInternal()
		}
	}
	return fmt.Errorf("规则不存在: %s", id)
}

// ==================== 规则组操作 ====================

// GetRuleGroups 获取所有预设规则组
func (s *JSONStore) GetRuleGroups() []RuleGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.RuleGroups
}

// UpdateRuleGroup 更新规则组
func (s *JSONStore) UpdateRuleGroup(ruleGroup RuleGroup) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.RuleGroups {
		if s.data.RuleGroups[i].ID == ruleGroup.ID {
			s.data.RuleGroups[i] = ruleGroup
			return s.saveInternal()
		}
	}
	return fmt.Errorf("规则组不存在: %s", ruleGroup.ID)
}

// ==================== 设置操作 ====================

// GetSettings 获取设置
func (s *JSONStore) GetSettings() *Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Settings
}

// UpdateSettings 更新设置
func (s *JSONStore) UpdateSettings(settings *Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	merged := *settings
	if s.data.Settings != nil {
		preserveAuthenticationSettings(s.data.Settings, &merged)
	}
	if merged.SessionTTLMinutes <= 0 {
		merged.SessionTTLMinutes = DefaultSettings().SessionTTLMinutes
	}

	s.data.Settings = &merged
	return s.saveInternal()
}

func preserveAuthenticationSettings(current *Settings, next *Settings) {
	if next.ClashAPISecret == "" {
		next.ClashAPISecret = current.ClashAPISecret
	}
	if next.AdminPasswordHash == "" {
		next.AdminPasswordHash = current.AdminPasswordHash
	}
	if next.SessionTTLMinutes <= 0 {
		next.SessionTTLMinutes = current.SessionTTLMinutes
	}
	if next.AuthBootstrappedAt == "" {
		next.AuthBootstrappedAt = current.AuthBootstrappedAt
	}
}

func migrateLegacyZashboardSettings(rawData []byte, settings *Settings, needSave *bool) {
	if !bytes.Contains(rawData, []byte(`"clash_api_lan_enabled"`)) {
		settings.ClashAPILanEnabled = DefaultSettings().ClashAPILanEnabled
		*needSave = true
	}

	if settings.ClashUIPath == "" {
		settings.ClashUIPath = DefaultSettings().ClashUIPath
		*needSave = true
	}

	if settings.ClashAPISecret == "" {
		settings.ClashAPISecret = mustNewZashboardSecret()
		*needSave = true
	}

	if !bytes.Contains(rawData, []byte(`"clash_ui_enabled"`)) {
		settings.ClashUIEnabled = true
		*needSave = true
	}
}

// ==================== 手动节点操作 ====================

// GetManualNodes 获取所有手动节点
func (s *JSONStore) GetManualNodes() []ManualNode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.ManualNodes
}

// AddManualNode 添加手动节点
func (s *JSONStore) AddManualNode(node ManualNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.ManualNodes = append(s.data.ManualNodes, node)
	return s.saveInternal()
}

// UpdateManualNode 更新手动节点
func (s *JSONStore) UpdateManualNode(node ManualNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.ManualNodes {
		if s.data.ManualNodes[i].ID == node.ID {
			s.data.ManualNodes[i] = node
			return s.saveInternal()
		}
	}
	return fmt.Errorf("手动节点不存在: %s", node.ID)
}

// DeleteManualNode 删除手动节点
func (s *JSONStore) DeleteManualNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.ManualNodes {
		if s.data.ManualNodes[i].ID == id {
			s.data.ManualNodes = append(s.data.ManualNodes[:i], s.data.ManualNodes[i+1:]...)
			return s.saveInternal()
		}
	}
	return fmt.Errorf("手动节点不存在: %s", id)
}

// ==================== 辅助方法 ====================

// GetAllNodes 获取所有启用的节点（订阅节点 + 手动节点），填充来源信息
func (s *JSONStore) GetAllNodes() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var nodes []Node
	// 添加订阅节点
	for _, sub := range s.data.Subscriptions {
		if sub.Enabled {
			for _, node := range sub.Nodes {
				// 填充来源信息
				node.Source = sub.ID
				node.SourceName = sub.Name
				nodes = append(nodes, node)
			}
		}
	}
	// 添加手动节点
	for _, mn := range s.data.ManualNodes {
		if mn.Enabled {
			// 填充来源信息
			mn.Node.Source = "manual"
			mn.Node.SourceName = "手动添加"
			nodes = append(nodes, mn.Node)
		}
	}
	return nodes
}

// GetNodesGrouped 获取按来源分组的节点
func (s *JSONStore) GetNodesGrouped() []NodeGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var groups []NodeGroup

	// 手动节点分组
	var manualNodes []Node
	for _, mn := range s.data.ManualNodes {
		if mn.Enabled {
			node := mn.Node
			node.Source = "manual"
			node.SourceName = "手动添加"
			manualNodes = append(manualNodes, node)
		}
	}
	if len(manualNodes) > 0 {
		groups = append(groups, NodeGroup{
			Source:     "manual",
			SourceName: "手动添加",
			Nodes:      manualNodes,
		})
	}

	// 订阅分组
	for _, sub := range s.data.Subscriptions {
		if !sub.Enabled || len(sub.Nodes) == 0 {
			continue
		}
		var subNodes []Node
		for _, node := range sub.Nodes {
			node.Source = sub.ID
			node.SourceName = sub.Name
			subNodes = append(subNodes, node)
		}
		groups = append(groups, NodeGroup{
			Source:     sub.ID,
			SourceName: sub.Name,
			Nodes:      subNodes,
		})
	}

	return groups
}

// GetNodesByCountry 按国家获取节点，支持分页（limit<=0 表示不分页）
func (s *JSONStore) GetNodesByCountry(countryCode string, limit, offset int) ([]Node, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []Node
	// 订阅节点
	for _, sub := range s.data.Subscriptions {
		if sub.Enabled {
			for _, node := range sub.Nodes {
				if node.Country == countryCode {
					all = append(all, node)
				}
			}
		}
	}
	// 手动节点
	for _, mn := range s.data.ManualNodes {
		if mn.Enabled && mn.Node.Country == countryCode {
			all = append(all, mn.Node)
		}
	}

	total := len(all)
	if limit <= 0 {
		return all, total
	}

	if offset >= total {
		return nil, total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return all[offset:end], total
}

// GetCountryGroups 获取所有国家节点分组
func (s *JSONStore) GetCountryGroups() []CountryGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()

	countryCount := make(map[string]int)

	// 统计订阅节点
	for _, sub := range s.data.Subscriptions {
		if sub.Enabled {
			for _, node := range sub.Nodes {
				if node.Country != "" {
					countryCount[node.Country]++
				}
			}
		}
	}
	// 统计手动节点
	for _, mn := range s.data.ManualNodes {
		if mn.Enabled && mn.Node.Country != "" {
			countryCount[mn.Node.Country]++
		}
	}

	var groups []CountryGroup
	for code, count := range countryCount {
		groups = append(groups, CountryGroup{
			Code:      code,
			Name:      GetCountryName(code),
			Emoji:     GetCountryEmoji(code),
			NodeCount: count,
		})
	}

	return groups
}

// GetDataDir 获取数据目录
func (s *JSONStore) GetDataDir() string {
	return s.dataDir
}

// Reload 重新加载数据
func (s *JSONStore) Reload() error {
	return s.load()
}

// ==================== Profile 操作 ====================

// GetProfiles 获取所有 Profile 元数据
func (s *JSONStore) GetProfiles() []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Profiles
}

// GetProfile 获取单个 Profile
func (s *JSONStore) GetProfile(id string) *Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.data.Profiles {
		if s.data.Profiles[i].ID == id {
			return &s.data.Profiles[i]
		}
	}
	return nil
}

// AddProfile 添加 Profile
func (s *JSONStore) AddProfile(profile Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.Profiles = append(s.data.Profiles, profile)
	return s.saveInternal()
}

// UpdateProfile 更新 Profile 元数据
func (s *JSONStore) UpdateProfile(profile Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Profiles {
		if s.data.Profiles[i].ID == profile.ID {
			s.data.Profiles[i] = profile
			return s.saveInternal()
		}
	}
	return fmt.Errorf("Profile 不存在: %s", profile.ID)
}

// DeleteProfile 删除 Profile
func (s *JSONStore) DeleteProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Profiles {
		if s.data.Profiles[i].ID == id {
			s.data.Profiles = append(s.data.Profiles[:i], s.data.Profiles[i+1:]...)
			return s.saveInternal()
		}
	}
	return fmt.Errorf("Profile 不存在: %s", id)
}

// GetActiveProfile 获取当前激活的 Profile ID
func (s *JSONStore) GetActiveProfile() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.ActiveProfile
}

// SetActiveProfile 设置当前激活的 Profile ID
func (s *JSONStore) SetActiveProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新所有 Profile 的 IsActive 状态
	for i := range s.data.Profiles {
		s.data.Profiles[i].IsActive = (s.data.Profiles[i].ID == id)
	}

	s.data.ActiveProfile = id
	return s.saveInternal()
}

// GetProfilesDir 获取 profiles 目录
func (s *JSONStore) GetProfilesDir() string {
	return filepath.Join(s.dataDir, "profiles")
}

// EnsureProfilesDir 确保 profiles 目录存在
func (s *JSONStore) EnsureProfilesDir() error {
	return os.MkdirAll(s.GetProfilesDir(), 0755)
}

// SaveProfileData 保存 Profile 完整数据到文件
func (s *JSONStore) SaveProfileData(id string, data *AppData) error {
	if err := s.EnsureProfilesDir(); err != nil {
		return err
	}

	profileFile := filepath.Join(s.GetProfilesDir(), id+".json")
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 Profile 数据失败: %w", err)
	}

	return os.WriteFile(profileFile, content, 0644)
}

// LoadProfileData 从文件加载 Profile 完整数据
func (s *JSONStore) LoadProfileData(id string) (*AppData, error) {
	profileFile := filepath.Join(s.GetProfilesDir(), id+".json")
	content, err := os.ReadFile(profileFile)
	if err != nil {
		return nil, fmt.Errorf("读取 Profile 数据失败: %w", err)
	}

	var data AppData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, fmt.Errorf("解析 Profile 数据失败: %w", err)
	}

	return &data, nil
}

// DeleteProfileData 删除 Profile 数据文件
func (s *JSONStore) DeleteProfileData(id string) error {
	profileFile := filepath.Join(s.GetProfilesDir(), id+".json")
	if _, err := os.Stat(profileFile); os.IsNotExist(err) {
		return nil // 文件不存在，无需删除
	}
	return os.Remove(profileFile)
}

// CreateSnapshotData 创建当前配置的快照数据
func (s *JSONStore) CreateSnapshotData() *AppData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 深拷贝当前数据（不包含 Profiles 和 ActiveProfile）
	snapshot := &AppData{
		Subscriptions: make([]Subscription, len(s.data.Subscriptions)),
		ManualNodes:   make([]ManualNode, len(s.data.ManualNodes)),
		Filters:       make([]Filter, len(s.data.Filters)),
		Rules:         make([]Rule, len(s.data.Rules)),
		RuleGroups:    make([]RuleGroup, len(s.data.RuleGroups)),
		InboundPorts:  make([]InboundPort, len(s.data.InboundPorts)),
		ProxyChains:   make([]ProxyChain, len(s.data.ProxyChains)),
		Settings:      s.data.Settings,
	}

	copy(snapshot.Subscriptions, s.data.Subscriptions)
	copy(snapshot.ManualNodes, s.data.ManualNodes)
	copy(snapshot.Filters, s.data.Filters)
	copy(snapshot.Rules, s.data.Rules)
	copy(snapshot.RuleGroups, s.data.RuleGroups)
	copy(snapshot.InboundPorts, s.data.InboundPorts)
	copy(snapshot.ProxyChains, s.data.ProxyChains)

	return snapshot
}

// RestoreFromProfileData 从 Profile 数据恢复配置
func (s *JSONStore) RestoreFromProfileData(data *AppData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 保留 Profiles 和 ActiveProfile
	profiles := s.data.Profiles
	activeProfile := s.data.ActiveProfile

	// 恢复数据
	s.data.Subscriptions = data.Subscriptions
	s.data.ManualNodes = data.ManualNodes
	s.data.Filters = data.Filters
	s.data.Rules = data.Rules
	s.data.RuleGroups = data.RuleGroups
	s.data.Settings = data.Settings
	s.data.InboundPorts = data.InboundPorts
	s.data.ProxyChains = data.ProxyChains

	// 恢复 Profiles 相关
	s.data.Profiles = profiles
	s.data.ActiveProfile = activeProfile

	return s.saveInternal()
}

// ==================== InboundPort 操作 ====================

// GetInboundPorts 获取所有入站端口
func (s *JSONStore) GetInboundPorts() []InboundPort {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.InboundPorts
}

// GetInboundPort 获取单个入站端口
func (s *JSONStore) GetInboundPort(id string) *InboundPort {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.data.InboundPorts {
		if s.data.InboundPorts[i].ID == id {
			return &s.data.InboundPorts[i]
		}
	}
	return nil
}

// AddInboundPort 添加入站端口
func (s *JSONStore) AddInboundPort(port InboundPort) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.InboundPorts = append(s.data.InboundPorts, port)
	return s.saveInternal()
}

// UpdateInboundPort 更新入站端口
func (s *JSONStore) UpdateInboundPort(port InboundPort) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.InboundPorts {
		if s.data.InboundPorts[i].ID == port.ID {
			s.data.InboundPorts[i] = port
			return s.saveInternal()
		}
	}
	return fmt.Errorf("入站端口不存在: %s", port.ID)
}

// DeleteInboundPort 删除入站端口
func (s *JSONStore) DeleteInboundPort(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.InboundPorts {
		if s.data.InboundPorts[i].ID == id {
			s.data.InboundPorts = append(s.data.InboundPorts[:i], s.data.InboundPorts[i+1:]...)
			return s.saveInternal()
		}
	}
	return fmt.Errorf("入站端口不存在: %s", id)
}

// ==================== ProxyChain 操作 ====================

// GetProxyChains 获取所有代理链路
func (s *JSONStore) GetProxyChains() []ProxyChain {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.ProxyChains
}

// GetProxyChain 获取单个代理链路
func (s *JSONStore) GetProxyChain(id string) *ProxyChain {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.data.ProxyChains {
		if s.data.ProxyChains[i].ID == id {
			return &s.data.ProxyChains[i]
		}
	}
	return nil
}

// AddProxyChain 添加代理链路
func (s *JSONStore) AddProxyChain(chain ProxyChain) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data.ProxyChains = append(s.data.ProxyChains, chain)
	return s.saveInternal()
}

// UpdateProxyChain 更新代理链路
func (s *JSONStore) UpdateProxyChain(chain ProxyChain) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.ProxyChains {
		if s.data.ProxyChains[i].ID == chain.ID {
			s.data.ProxyChains[i] = chain
			return s.saveInternal()
		}
	}
	return fmt.Errorf("代理链路不存在: %s", chain.ID)
}

// DeleteProxyChain 删除代理链路
func (s *JSONStore) DeleteProxyChain(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.ProxyChains {
		if s.data.ProxyChains[i].ID == id {
			s.data.ProxyChains = append(s.data.ProxyChains[:i], s.data.ProxyChains[i+1:]...)
			return s.saveInternal()
		}
	}
	return fmt.Errorf("代理链路不存在: %s", id)
}
