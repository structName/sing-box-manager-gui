package builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/structName/sing-box-manager-gui/internal/storage"
	"github.com/structName/sing-box-manager-gui/internal/zashboard"
)

// SingBoxConfig sing-box 配置结构
type SingBoxConfig struct {
	Log          *LogConfig          `json:"log,omitempty"`
	DNS          *DNSConfig          `json:"dns,omitempty"`
	NTP          *NTPConfig          `json:"ntp,omitempty"`
	Inbounds     []Inbound           `json:"inbounds,omitempty"`
	Outbounds    []Outbound          `json:"outbounds"`
	Route        *RouteConfig        `json:"route,omitempty"`
	Experimental *ExperimentalConfig `json:"experimental,omitempty"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level     string `json:"level,omitempty"`
	Timestamp bool   `json:"timestamp,omitempty"`
	Output    string `json:"output,omitempty"`
}

// DNSConfig DNS 配置
type DNSConfig struct {
	Strategy string      `json:"strategy,omitempty"`
	Servers  []DNSServer `json:"servers,omitempty"`
	Rules    []DNSRule   `json:"rules,omitempty"`
	Final    string      `json:"final,omitempty"`
}

// DNSServer DNS 服务器 (新格式，支持 FakeIP 和 hosts)
type DNSServer struct {
	Tag        string         `json:"tag"`
	Type       string         `json:"type"`                  // udp, tcp, https, tls, quic, h3, fakeip, rcode, hosts
	Server     string         `json:"server,omitempty"`      // 服务器地址
	Detour     string         `json:"detour,omitempty"`      // 出站代理
	Inet4Range string         `json:"inet4_range,omitempty"` // FakeIP IPv4 地址池
	Inet6Range string         `json:"inet6_range,omitempty"` // FakeIP IPv6 地址池
	Predefined map[string]any `json:"predefined,omitempty"`  // hosts 类型专用：预定义域名映射
}

// DNSRule DNS 规则
type DNSRule struct {
	Outbound  string   `json:"outbound,omitempty"` // 匹配出站的 DNS 查询，如 "any" 表示代理服务器地址解析
	RuleSet   []string `json:"rule_set,omitempty"`
	QueryType []string `json:"query_type,omitempty"`
	Domain    []string `json:"domain,omitempty"` // 完整域名匹配
	Server    string   `json:"server,omitempty"`
	Action    string   `json:"action,omitempty"` // route, reject 等
}

// NTPConfig NTP 配置
type NTPConfig struct {
	Enabled bool   `json:"enabled"`
	Server  string `json:"server,omitempty"`
}

// Inbound 入站配置
// 注意: sniff/sniff_override_destination 已在 sing-box 1.11.0 中从 inbound 移除，
// 改为通过 route rule_actions 配置（见 buildRoute 中的 sniff action）
type Inbound struct {
	Type        string        `json:"type"`
	Tag         string        `json:"tag"`
	Listen      string        `json:"listen,omitempty"`
	ListenPort  int           `json:"listen_port,omitempty"`
	Address     []string      `json:"address,omitempty"`
	AutoRoute   bool          `json:"auto_route,omitempty"`
	StrictRoute bool          `json:"strict_route,omitempty"`
	Stack       string        `json:"stack,omitempty"`
	Users       []InboundUser `json:"users,omitempty"`
}

// InboundUser 入站用户认证
type InboundUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Outbound 出站配置
type Outbound map[string]interface{}

// DomainResolver 域名解析器配置
type DomainResolver struct {
	Server     string `json:"server"`
	RewriteTTL int    `json:"rewrite_ttl,omitempty"`
}

// RouteConfig 路由配置
type RouteConfig struct {
	Rules                 []RouteRule     `json:"rules,omitempty"`
	Final                 string          `json:"final,omitempty"`
	AutoDetectInterface   bool            `json:"auto_detect_interface,omitempty"`
	DefaultDomainResolver *DomainResolver `json:"default_domain_resolver,omitempty"`
}

// RouteRule 路由规则
type RouteRule map[string]interface{}

// ExperimentalConfig 实验性配置
type ExperimentalConfig struct {
	ClashAPI  *ClashAPIConfig  `json:"clash_api,omitempty"`
	CacheFile *CacheFileConfig `json:"cache_file,omitempty"`
}

// ClashAPIConfig Clash API 配置
type ClashAPIConfig struct {
	ExternalController    string `json:"external_controller,omitempty"`
	ExternalUI            string `json:"external_ui,omitempty"`
	ExternalUIDownloadURL string `json:"external_ui_download_url,omitempty"`
	Secret                string `json:"secret,omitempty"`
	DefaultMode           string `json:"default_mode,omitempty"`
}

const defaultZashboardExternalUIDownloadURL = "https://github.com/Zephyruso/zashboard/releases/latest/download/dist.zip"

// CacheFileConfig 缓存文件配置
type CacheFileConfig struct {
	Enabled     bool   `json:"enabled"`
	Path        string `json:"path,omitempty"`
	StoreFakeIP bool   `json:"store_fakeip,omitempty"` // 持久化 FakeIP 映射
}

// ConfigBuilder 配置生成器
type ConfigBuilder struct {
	settings     *storage.Settings
	nodes        []storage.Node
	filters      []storage.Filter
	inboundPorts []storage.InboundPort
	proxyChains  []storage.ProxyChain
	dataDir      string // 数据目录路径
}

// SkippedNode 被排除在运行配置之外的无效代理节点。
type SkippedNode struct {
	Tag    string
	Type   string
	Reason string
}

// ValidatedConfig 已通过内核校验的 sing-box 配置。
type ValidatedConfig struct {
	JSON         string
	SkippedNodes []SkippedNode
}

// NewConfigBuilder 创建配置生成器
func NewConfigBuilder(settings *storage.Settings, nodes []storage.Node, filters []storage.Filter, inboundPorts []storage.InboundPort, proxyChains []storage.ProxyChain) *ConfigBuilder {
	return &ConfigBuilder{
		settings:     settings,
		nodes:        nodes,
		filters:      filters,
		inboundPorts: inboundPorts,
		proxyChains:  proxyChains,
	}
}

// SetDataDir 设置数据目录
func (b *ConfigBuilder) SetDataDir(dataDir string) {
	b.dataDir = dataDir
}

// Build 构建 sing-box 配置
func (b *ConfigBuilder) Build() (*SingBoxConfig, error) {
	outbounds, err := b.buildOutbounds()
	if err != nil {
		return nil, err
	}
	route := b.buildRoute()

	config := &SingBoxConfig{
		Log:          b.buildLog(),
		DNS:          b.buildDNS(),
		NTP:          b.buildNTP(),
		Inbounds:     b.buildInbounds(),
		Outbounds:    outbounds,
		Route:        route,
		Experimental: b.buildExperimental(), // 始终启用，FakeIP 需要 cache_file
	}

	return config, nil
}

func fallbackMissingRouteOutbounds(route *RouteConfig, outbounds []Outbound, excluded map[string]struct{}) {
	if route == nil {
		return
	}
	available := make(map[string]struct{}, len(outbounds))
	for _, outbound := range outbounds {
		if tag, _ := outbound["tag"].(string); tag != "" {
			available[tag] = struct{}{}
		}
	}
	fallback := "DIRECT"
	if _, ok := available["Proxy"]; ok {
		fallback = "Proxy"
	}
	for _, rule := range route.Rules {
		target, _ := rule["outbound"].(string)
		if target == "" {
			continue
		}
		_, wasExcluded := excluded[target]
		if _, ok := available[target]; !ok && wasExcluded {
			rule["outbound"] = fallback
		}
	}
}

// BuildJSON 构建 JSON 字符串
func (b *ConfigBuilder) BuildJSON() (string, error) {
	config, err := b.Build()
	if err != nil {
		return "", err
	}
	return marshalConfigJSON(config)
}

func marshalConfigJSON(config *SingBoxConfig) (string, error) {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %w", err)
	}

	return string(data), nil
}

// BuildValidatedJSON 排除被 sing-box 拒绝的代理节点，重建分组和链路后再次校验。
func (b *ConfigBuilder) BuildValidatedJSON(validate func(string) error) (*ValidatedConfig, error) {
	remaining := append([]storage.Node(nil), b.nodes...)
	skipped := make([]SkippedNode, 0)
	placeholderNodes := append([]storage.Node(nil), b.nodes...)
	hasPlaceholders := false

	buildable := make([]storage.Node, 0, len(remaining))
	for index, node := range remaining {
		if _, err := b.nodeToOutbound(node); err != nil {
			skipped = append(skipped, skippedNode(node, err))
			placeholderNodes[index] = validationPlaceholderNode(node)
			hasPlaceholders = true
			continue
		}
		buildable = append(buildable, node)
	}
	remaining = buildable

	var baselineTags map[string]struct{}
	if hasPlaceholders {
		outbounds, err := b.withNodes(placeholderNodes).buildOutbounds()
		if err != nil {
			return nil, err
		}
		baselineTags = outboundTagSet(outbounds)
	}

	for {
		candidate := b.withNodes(remaining)
		config, err := candidate.Build()
		if err != nil {
			return nil, err
		}
		currentTags := outboundTagSet(config.Outbounds)
		if baselineTags == nil {
			baselineTags = currentTags
		}
		fallbackMissingRouteOutbounds(config.Route, config.Outbounds, missingOutboundTags(baselineTags, currentTags))
		configJSON, err := marshalConfigJSON(config)
		if err != nil {
			return nil, err
		}
		if validate == nil {
			return &ValidatedConfig{JSON: configJSON, SkippedNodes: skipped}, nil
		}

		validationErr := validate(configJSON)
		if validationErr == nil {
			return &ValidatedConfig{JSON: configJSON, SkippedNodes: skipped}, nil
		}

		outboundIndex, ok := outboundIndexFromError(validationErr)
		if !ok {
			return nil, fmt.Errorf("sing-box 配置校验失败且无法定位到代理节点: %w", validationErr)
		}

		if outboundIndex < 0 || outboundIndex >= len(config.Outbounds) {
			return nil, fmt.Errorf("sing-box 配置校验返回无效出站索引 %d: %w", outboundIndex, validationErr)
		}

		nodeIndex := candidate.nodeIndexForOutbound(config.Outbounds[outboundIndex])
		if nodeIndex < 0 {
			return nil, fmt.Errorf("sing-box 配置校验失败，出站 %d 不是可排除的代理节点: %w", outboundIndex, validationErr)
		}

		skipped = append(skipped, skippedNode(remaining[nodeIndex], validationErr))
		remaining = append(remaining[:nodeIndex], remaining[nodeIndex+1:]...)
	}
}

func (b *ConfigBuilder) withNodes(nodes []storage.Node) *ConfigBuilder {
	return &ConfigBuilder{
		settings:     b.settings,
		nodes:        nodes,
		filters:      b.filters,
		inboundPorts: b.inboundPorts,
		proxyChains:  b.proxyChains,
		dataDir:      b.dataDir,
	}
}

func validationPlaceholderNode(node storage.Node) storage.Node {
	node.Type = "socks"
	node.Server = "127.0.0.1"
	node.ServerPort = 1
	node.Extra = map[string]interface{}{"version": "5"}
	return node
}

func outboundTagSet(outbounds []Outbound) map[string]struct{} {
	tags := make(map[string]struct{}, len(outbounds))
	for _, outbound := range outbounds {
		if tag, _ := outbound["tag"].(string); tag != "" {
			tags[tag] = struct{}{}
		}
	}
	return tags
}

func missingOutboundTags(baseline, current map[string]struct{}) map[string]struct{} {
	missing := make(map[string]struct{})
	for tag := range baseline {
		if _, ok := current[tag]; !ok {
			missing[tag] = struct{}{}
		}
	}
	return missing
}

func (b *ConfigBuilder) nodeIndexForOutbound(outbound Outbound) int {
	if tag, _ := outbound["tag"].(string); tag != "" {
		for index, node := range b.nodes {
			if node.Tag == tag {
				return index
			}
		}
	}

	want := cloneOutbound(outbound)
	delete(want, "tag")
	delete(want, "detour")
	for index, node := range b.nodes {
		candidate, err := b.nodeToOutbound(node)
		if err != nil {
			continue
		}
		delete(candidate, "tag")
		delete(candidate, "detour")
		if outboundJSONEqual(candidate, want) {
			return index
		}
	}
	return -1
}

func outboundJSONEqual(left, right Outbound) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func outboundIndexFromError(err error) (int, bool) {
	var indexed interface {
		OutboundIndex() (int, bool)
	}
	if !errors.As(err, &indexed) {
		return 0, false
	}
	return indexed.OutboundIndex()
}

func skippedNode(node storage.Node, err error) SkippedNode {
	return SkippedNode{
		Tag:    node.Tag,
		Type:   node.Type,
		Reason: strings.TrimSpace(err.Error()),
	}
}

// buildLog 构建日志配置
func (b *ConfigBuilder) buildLog() *LogConfig {
	return &LogConfig{
		Level:     "info",
		Timestamp: true,
	}
}

// ParseSystemHosts 解析系统 /etc/hosts 文件
func ParseSystemHosts() map[string][]string {
	hosts := make(map[string][]string)

	data, err := os.ReadFile("/etc/hosts")
	if err != nil {
		return hosts
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 去除行内注释
		if idx := strings.Index(line, "#"); idx != -1 {
			line = line[:idx]
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		ip := fields[0]
		// 跳过 localhost 相关条目
		for _, domain := range fields[1:] {
			if domain == "localhost" || strings.HasSuffix(domain, ".localhost") {
				continue
			}
			hosts[domain] = append(hosts[domain], ip)
		}
	}

	return hosts
}

// buildDNS 构建 DNS 配置
func (b *ConfigBuilder) buildDNS() *DNSConfig {
	// 基础 DNS 服务器
	servers := []DNSServer{
		{
			Tag:    "dns_proxy",
			Type:   "https",
			Server: "8.8.8.8",
			Detour: "Proxy",
		},
		{
			Tag:    "dns_direct",
			Type:   "udp",
			Server: "223.5.5.5",
		},
	}

	// 基础 DNS 规则
	var rules []DNSRule

	// 如果启用 FakeIP
	if b.settings.FakeIPEnabled {
		servers = append(servers, DNSServer{
			Tag:        "dns_fakeip",
			Type:       "fakeip",
			Inet4Range: "198.18.0.0/15",
			Inet6Range: "fc00::/18",
		})

		rules = append(rules, DNSRule{
			QueryType: []string{"A", "AAAA"},
			Server:    "dns_fakeip",
			Action:    "route",
		})
	}

	// 1. 读取系统 hosts
	systemHosts := ParseSystemHosts()

	// 2. 收集用户自定义 hosts（用户优先，会覆盖系统 hosts）
	predefined := make(map[string]any)
	var domains []string

	// 先添加系统 hosts
	for domain, ips := range systemHosts {
		if len(ips) == 1 {
			predefined[domain] = ips[0]
		} else {
			predefined[domain] = ips
		}
		domains = append(domains, domain)
	}

	// 再添加用户 hosts（覆盖同名系统 hosts）
	for _, host := range b.settings.Hosts {
		if host.Enabled && host.Domain != "" && len(host.IPs) > 0 {
			if len(host.IPs) == 1 {
				predefined[host.Domain] = host.IPs[0]
			} else {
				predefined[host.Domain] = host.IPs
			}
			// 如果是新域名，加入列表
			if _, exists := systemHosts[host.Domain]; !exists {
				domains = append(domains, host.Domain)
			}
		}
	}

	// 3. 如果有映射，添加 hosts 服务器和规则
	if len(predefined) > 0 {
		// 在服务器列表开头插入 hosts 服务器
		hostsServer := DNSServer{
			Tag:        "dns_hosts",
			Type:       "hosts",
			Predefined: predefined,
		}
		servers = append([]DNSServer{hostsServer}, servers...)

		// 在规则列表开头插入 hosts 规则（优先匹配）
		hostsRule := DNSRule{
			Domain: domains,
			Server: "dns_hosts",
			Action: "route",
		}
		rules = append([]DNSRule{hostsRule}, rules...)
	}

	return &DNSConfig{
		Strategy: "prefer_ipv4",
		Servers:  servers,
		Rules:    rules,
		Final:    "dns_proxy",
	}
}

// buildNTP 默认不生成 NTP 配置，避免额外的外部时间同步请求
func (b *ConfigBuilder) buildNTP() *NTPConfig {
	return nil
}

// buildInbounds 构建入站配置
func (b *ConfigBuilder) buildInbounds() []Inbound {
	var inbounds []Inbound

	if b.settings.TunEnabled {
		inbounds = append(inbounds, Inbound{
			Type:        "tun",
			Tag:         "tun-in",
			Address:     []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"},
			AutoRoute:   true,
			StrictRoute: true,
			Stack:       "system",
		})
	}

	// 所有入站端口统一由多端口管理
	for _, port := range b.inboundPorts {
		if !port.Enabled {
			continue
		}

		inbound := Inbound{
			Type:       port.Type,
			Tag:        fmt.Sprintf("custom-%s", port.ID),
			Listen:     port.Listen,
			ListenPort: port.Port,
		}

		if port.Auth != nil && port.Auth.Username != "" {
			inbound.Users = []InboundUser{
				{
					Username: port.Auth.Username,
					Password: port.Auth.Password,
				},
			}
		}

		inbounds = append(inbounds, inbound)
	}

	return inbounds
}

// buildOutbounds 构建出站配置
func (b *ConfigBuilder) buildOutbounds() ([]Outbound, error) {
	outbounds := []Outbound{
		{"type": "direct", "tag": "DIRECT"},
		{"type": "block", "tag": "REJECT"},
		// 移除 dns-out，改用路由 action: hijack-dns
	}

	// 收集所有节点标签和按国家分组
	var allNodeTags []string
	countryNodes := make(map[string][]string) // 国家代码 -> 节点标签列表

	// 构建节点 Tag 到节点的映射，并预先整理国家分组
	nodeMap := make(map[string]storage.Node)
	for _, node := range b.nodes {
		nodeMap[node.Tag] = node
		allNodeTags = append(allNodeTags, node.Tag)

		countryCode := node.Country
		if countryCode == "" {
			countryCode = "OTHER"
		}
		countryNodes[countryCode] = append(countryNodes[countryCode], node.Tag)
	}

	chainNodeExists := func(nodeTag string) bool {
		if storage.IsChainCountryNodeTag(nodeTag) {
			countryCode := storage.ParseChainCountryNodeCode(nodeTag)
			return len(countryNodes[countryCode]) > 0
		}
		_, exists := nodeMap[nodeTag]
		return exists
	}

	// 生成链路节点副本（独立的副本，不影响原始节点）
	chainCopyTags := make(map[string]bool) // 已创建的副本 Tag
	activeTorChainIDs := b.activeTorChainIDs()
	for _, chain := range b.proxyChains {
		if !chain.Enabled || len(chain.Nodes) < 2 {
			continue
		}
		if storage.ChainContainsTor(chain.Nodes) {
			if activeTorChainIDs[chain.ID] {
				generated, err := b.appendTorChainOutbounds(outbounds, chainCopyTags, chain, nodeMap, countryNodes)
				if err != nil {
					return nil, err
				}
				outbounds = generated
			}
			continue
		}

		// 验证链路中的所有节点是否存在
		allNodesExist := true
		for _, nodeTag := range chain.Nodes {
			if !chainNodeExists(nodeTag) {
				allNodesExist = false
				break
			}
		}
		if !allNodesExist {
			continue
		}

		// 为链路中的每个节点创建副本
		// 链路顺序: [入口, 中间..., 出口]
		// detour 方向: 出口节点的 detour 指向前一个节点
		// 流量路径: 客户端 → 入口 → 中间... → 出口 → 目标
		var prevCopyTag string
		for _, nodeTag := range chain.Nodes {
			if storage.IsChainCountryNodeTag(nodeTag) {
				countryCode := storage.ParseChainCountryNodeCode(nodeTag)
				candidateTags := countryNodes[countryCode]
				if len(candidateTags) == 0 {
					continue
				}

				groupCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, nodeTag)
				virtualOutbounds := make([]string, 0, len(candidateTags))
				for _, candidateTag := range candidateTags {
					candidateCopyTag := storage.GenerateChainCountryCandidateCopyTag(chain.Name, nodeTag, candidateTag)
					virtualOutbounds = append(virtualOutbounds, candidateCopyTag)
					if chainCopyTags[candidateCopyTag] {
						continue
					}

					copyOutbound, err := b.nodeToOutbound(nodeMap[candidateTag])
					if err != nil {
						return nil, err
					}
					copyOutbound["tag"] = candidateCopyTag
					if prevCopyTag != "" {
						copyOutbound["detour"] = prevCopyTag
					}

					outbounds = append(outbounds, copyOutbound)
					chainCopyTags[candidateCopyTag] = true
				}

				if !chainCopyTags[groupCopyTag] {
					outbounds = append(outbounds, Outbound{
						"tag":       groupCopyTag,
						"type":      "urltest",
						"outbounds": virtualOutbounds,
						"url":       "https://www.gstatic.com/generate_204",
						"interval":  "30m",
						"tolerance": 50,
					})
					chainCopyTags[groupCopyTag] = true
				}
				prevCopyTag = groupCopyTag
				continue
			}

			copyTag := storage.GenerateChainNodeCopyTag(chain.Name, nodeTag)
			if !chainCopyTags[copyTag] {
				copyOutbound, err := b.nodeToOutbound(nodeMap[nodeTag])
				if err != nil {
					return nil, err
				}
				copyOutbound["tag"] = copyTag
				if prevCopyTag != "" {
					copyOutbound["detour"] = prevCopyTag
				}

				outbounds = append(outbounds, copyOutbound)
				chainCopyTags[copyTag] = true
			}

			prevCopyTag = copyTag
		}
	}

	// 添加所有原始节点（不设置 detour，保持独立）
	for _, node := range b.nodes {
		outbound, err := b.nodeToOutbound(node)
		if err != nil {
			return nil, err
		}

		outbounds = append(outbounds, outbound)
	}

	// 收集过滤器分组
	var filterGroupTags []string

	for _, filter := range b.filters {
		if !filter.Enabled {
			continue
		}

		// 根据过滤器筛选节点
		var filteredTags []string
		for _, node := range b.nodes {
			if b.matchFilter(node, filter) {
				filteredTags = append(filteredTags, node.Tag)
			}
		}

		if len(filteredTags) == 0 {
			continue
		}

		groupTag := filter.Name
		filterGroupTags = append(filterGroupTags, groupTag)

		// 创建分组
		group := Outbound{
			"tag":       groupTag,
			"type":      filter.Mode,
			"outbounds": filteredTags,
		}

		if filter.Mode == "urltest" {
			if filter.URLTestConfig != nil {
				group["url"] = filter.URLTestConfig.URL
				group["interval"] = filter.URLTestConfig.Interval
				group["tolerance"] = filter.URLTestConfig.Tolerance
			} else {
				group["url"] = "https://www.gstatic.com/generate_204"
				group["interval"] = "1h"
				group["tolerance"] = 50
			}
		}

		outbounds = append(outbounds, group)
	}

	// 创建按国家分组的出站选择器
	var countryGroupTags []string
	// 按国家代码排序，确保顺序一致
	var countryCodes []string
	for code := range countryNodes {
		countryCodes = append(countryCodes, code)
	}
	sort.Strings(countryCodes)

	for _, code := range countryCodes {
		nodes := countryNodes[code]
		if len(nodes) == 0 {
			continue
		}

		// 创建国家分组标签，格式: "🇭🇰 香港" 或 "HK"
		emoji := storage.GetCountryEmoji(code)
		name := storage.GetCountryName(code)
		groupTag := fmt.Sprintf("%s %s", emoji, name)
		countryGroupTags = append(countryGroupTags, groupTag)

		// 创建自动选择分组
		outbounds = append(outbounds, Outbound{
			"tag":       groupTag,
			"type":      "urltest",
			"outbounds": nodes,
			"url":       "https://www.gstatic.com/generate_204",
			"interval":  "30m",
			"tolerance": 50,
		})
	}

	// 创建自动选择组（所有节点）
	if len(allNodeTags) > 0 {
		outbounds = append(outbounds, Outbound{
			"tag":       "Auto",
			"type":      "urltest",
			"outbounds": allNodeTags,
			"url":       "https://www.gstatic.com/generate_204",
			"interval":  "30m",
			"tolerance": 50,
		})
	}

	// 为代理链路创建选择器（指向副本出口节点）
	var chainGroupTags []string
	for _, chain := range b.proxyChains {
		if !chain.Enabled || len(chain.Nodes) == 0 {
			continue
		}
		if storage.ChainContainsTor(chain.Nodes) {
			continue
		}

		// 验证链路中的所有节点是否存在
		allNodesExist := true
		for _, nodeTag := range chain.Nodes {
			if !chainNodeExists(nodeTag) {
				allNodesExist = false
				break
			}
		}
		if !allNodesExist {
			continue
		}

		chainGroupTags = append(chainGroupTags, chain.Name)

		// 创建链路选择器，指向链路的副本出口节点（最后一个）
		// 流量路径: 选择器 → 出口节点 → (detour) 中间节点... → 入口节点 → 目标
		exitCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, chain.Nodes[len(chain.Nodes)-1])
		outbounds = append(outbounds, Outbound{
			"tag":       chain.Name,
			"type":      "selector",
			"outbounds": []string{exitCopyTag},
			"default":   exitCopyTag,
		})
	}

	// 创建主选择器（精简版：只包含分组，不包含单节点）
	var proxyOutbounds []string
	proxyDefault := "DIRECT"

	// 只有在有节点时才添加 Auto
	if len(allNodeTags) > 0 {
		proxyOutbounds = append(proxyOutbounds, "Auto")
		proxyDefault = "Auto"
	}
	proxyOutbounds = append(proxyOutbounds, countryGroupTags...) // 添加国家分组
	proxyOutbounds = append(proxyOutbounds, filterGroupTags...)
	proxyOutbounds = append(proxyOutbounds, chainGroupTags...) // 添加链路分组
	proxyOutbounds = append(proxyOutbounds, "DIRECT")          // 始终添加 DIRECT 作为备选

	outbounds = append(outbounds, Outbound{
		"tag":       "Proxy",
		"type":      "selector",
		"outbounds": proxyOutbounds,
		"default":   proxyDefault,
	})

	// 创建漏网规则选择器
	fallbackOutbounds := []string{"Proxy", "DIRECT"}
	fallbackOutbounds = append(fallbackOutbounds, countryGroupTags...) // 添加国家分组
	fallbackOutbounds = append(fallbackOutbounds, filterGroupTags...)
	fallbackOutbounds = append(fallbackOutbounds, chainGroupTags...) // 添加链路分组
	outbounds = append(outbounds, Outbound{
		"tag":       "Final",
		"type":      "selector",
		"outbounds": fallbackOutbounds,
		"default":   b.settings.FinalOutbound,
	})

	return outbounds, nil
}

func (b *ConfigBuilder) activeTorChainIDs() map[string]bool {
	active := make(map[string]bool)
	for _, port := range b.inboundPorts {
		if !port.Enabled || !port.UseTorExit || strings.TrimSpace(port.TorChainID) == "" {
			continue
		}
		active[port.TorChainID] = true
	}
	return active
}

func (b *ConfigBuilder) appendTorChainOutbounds(
	outbounds []Outbound,
	chainCopyTags map[string]bool,
	chain storage.ProxyChain,
	nodeMap map[string]storage.Node,
	countryNodes map[string][]string,
) ([]Outbound, error) {
	torIndex := -1
	for index, nodeTag := range chain.Nodes {
		if storage.IsChainTorNodeTag(nodeTag) {
			torIndex = index
			break
		}
	}
	if torIndex <= 0 {
		return outbounds, nil
	}

	var prevCopyTag string
	for _, nodeTag := range chain.Nodes[:torIndex] {
		generated, copyTag, ok, err := b.appendTorChainHopOutbounds(outbounds, chainCopyTags, chain, nodeTag, prevCopyTag, nodeMap, countryNodes)
		if err != nil {
			return nil, err
		}
		if !ok {
			return generated, nil
		}
		outbounds = generated
		prevCopyTag = copyTag
	}
	if prevCopyTag == "" {
		return outbounds, nil
	}

	torTag := storage.GenerateChainTorOutboundTag(chain.ID)
	if !chainCopyTags[torTag] {
		outbounds = append(outbounds, b.buildTorOutbound(torTag, chain.ID, prevCopyTag))
		chainCopyTags[torTag] = true
	}
	exitTag := torTag
	for _, nodeTag := range chain.Nodes[torIndex+1:] {
		generated, copyTag, ok, err := b.appendTorChainHopOutbounds(outbounds, chainCopyTags, chain, nodeTag, exitTag, nodeMap, countryNodes)
		if err != nil {
			return nil, err
		}
		if !ok {
			return generated, nil
		}
		outbounds = generated
		exitTag = copyTag
	}
	if !chainCopyTags[chain.Name] {
		outbounds = append(outbounds, Outbound{
			"tag":       chain.Name,
			"type":      "selector",
			"outbounds": []string{exitTag},
			"default":   exitTag,
		})
		chainCopyTags[chain.Name] = true
	}

	return outbounds, nil
}

func (b *ConfigBuilder) appendTorChainHopOutbounds(
	outbounds []Outbound,
	chainCopyTags map[string]bool,
	chain storage.ProxyChain,
	nodeTag string,
	prevCopyTag string,
	nodeMap map[string]storage.Node,
	countryNodes map[string][]string,
) ([]Outbound, string, bool, error) {
	if storage.IsChainAutoNodeTag(nodeTag) {
		candidateTags := make([]string, 0, len(nodeMap))
		for candidateTag := range nodeMap {
			candidateTags = append(candidateTags, candidateTag)
		}
		sort.Strings(candidateTags)
		if len(candidateTags) == 0 {
			return outbounds, "", false, nil
		}

		groupCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, nodeTag)
		virtualOutbounds := make([]string, 0, len(candidateTags))
		for _, candidateTag := range candidateTags {
			candidateCopyTag := storage.GenerateChainAutoCandidateCopyTag(chain.Name, candidateTag)
			virtualOutbounds = append(virtualOutbounds, candidateCopyTag)
			if chainCopyTags[candidateCopyTag] {
				continue
			}

			copyOutbound, err := b.nodeToOutbound(nodeMap[candidateTag])
			if err != nil {
				return nil, "", false, err
			}
			copyOutbound["tag"] = candidateCopyTag
			if prevCopyTag != "" {
				copyOutbound["detour"] = prevCopyTag
			}
			outbounds = append(outbounds, copyOutbound)
			chainCopyTags[candidateCopyTag] = true
		}

		if !chainCopyTags[groupCopyTag] {
			outbounds = append(outbounds, Outbound{
				"tag":       groupCopyTag,
				"type":      "urltest",
				"outbounds": virtualOutbounds,
				"url":       "https://www.gstatic.com/generate_204",
				"interval":  "30m",
				"tolerance": 50,
			})
			chainCopyTags[groupCopyTag] = true
		}
		return outbounds, groupCopyTag, true, nil
	}

	if storage.IsChainCountryNodeTag(nodeTag) {
		countryCode := storage.ParseChainCountryNodeCode(nodeTag)
		candidateTags := countryNodes[countryCode]
		if len(candidateTags) == 0 {
			return outbounds, "", false, nil
		}

		groupCopyTag := storage.GenerateChainNodeCopyTag(chain.Name, nodeTag)
		virtualOutbounds := make([]string, 0, len(candidateTags))
		for _, candidateTag := range candidateTags {
			candidateCopyTag := storage.GenerateChainCountryCandidateCopyTag(chain.Name, nodeTag, candidateTag)
			virtualOutbounds = append(virtualOutbounds, candidateCopyTag)
			if chainCopyTags[candidateCopyTag] {
				continue
			}

			copyOutbound, err := b.nodeToOutbound(nodeMap[candidateTag])
			if err != nil {
				return nil, "", false, err
			}
			copyOutbound["tag"] = candidateCopyTag
			if prevCopyTag != "" {
				copyOutbound["detour"] = prevCopyTag
			}
			outbounds = append(outbounds, copyOutbound)
			chainCopyTags[candidateCopyTag] = true
		}

		if !chainCopyTags[groupCopyTag] {
			outbounds = append(outbounds, Outbound{
				"tag":       groupCopyTag,
				"type":      "urltest",
				"outbounds": virtualOutbounds,
				"url":       "https://www.gstatic.com/generate_204",
				"interval":  "30m",
				"tolerance": 50,
			})
			chainCopyTags[groupCopyTag] = true
		}
		return outbounds, groupCopyTag, true, nil
	}

	node, exists := nodeMap[nodeTag]
	if !exists {
		return outbounds, "", false, nil
	}
	copyTag := storage.GenerateChainNodeCopyTag(chain.Name, nodeTag)
	if !chainCopyTags[copyTag] {
		copyOutbound, err := b.nodeToOutbound(node)
		if err != nil {
			return nil, "", false, err
		}
		copyOutbound["tag"] = copyTag
		if prevCopyTag != "" {
			copyOutbound["detour"] = prevCopyTag
		}

		outbounds = append(outbounds, copyOutbound)
		chainCopyTags[copyTag] = true
	}
	return outbounds, copyTag, true, nil
}

func (b *ConfigBuilder) buildTorOutbound(tag, chainID, detour string) Outbound {
	torrc := make(map[string]interface{})
	if b.settings != nil {
		for key, value := range b.settings.TorrcValues {
			torrc[key] = value
		}
	}
	torrc["ClientOnly"] = 1

	dataDirectory := filepath.Join("tor", chainID)
	if b.dataDir != "" {
		dataDirectory = filepath.Join(b.dataDir, "tor", chainID)
	}

	outbound := Outbound{
		"tag":            tag,
		"type":           "tor",
		"data_directory": dataDirectory,
		"torrc":          torrc,
		"detour":         detour,
	}
	if b.settings != nil {
		outbound["executable_path"] = b.settings.TorExecutablePath
		outbound["extra_args"] = b.settings.TorExtraArgs
	}
	return outbound
}

// nodeToOutbound 将节点转换为出站配置
func (b *ConfigBuilder) nodeToOutbound(node storage.Node) (Outbound, error) {
	outbound := Outbound{
		"tag":         node.Tag,
		"type":        node.Type,
		"server":      node.Server,
		"server_port": node.ServerPort,
	}

	// 复制 Extra 字段，避免多轮配置校验修改存储中的节点数据。
	for k, v := range node.Extra {
		if isNodeMetadataField(k) {
			continue
		}
		outbound[k] = cloneOutboundValue(v)
	}

	if err := normalizeOutbound(outbound); err != nil {
		return nil, fmt.Errorf("节点 %q 配置无效: %w", node.Tag, err)
	}

	return outbound, nil
}

func cloneOutbound(outbound Outbound) Outbound {
	clone := make(Outbound, len(outbound))
	for key, value := range outbound {
		clone[key] = cloneOutboundValue(value)
	}
	return clone
}

func cloneOutboundValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		clone := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			clone[key] = cloneOutboundValue(nested)
		}
		return clone
	case map[string]string:
		clone := make(map[string]string, len(typed))
		for key, nested := range typed {
			clone[key] = nested
		}
		return clone
	case []interface{}:
		clone := make([]interface{}, len(typed))
		for index, nested := range typed {
			clone[index] = cloneOutboundValue(nested)
		}
		return clone
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

func isNodeMetadataField(key string) bool {
	switch key {
	case "node_origin", "entry_method", "deployment_run_id":
		return true
	default:
		return false
	}
}

func normalizeOutbound(outbound Outbound) error {
	outboundType, _ := outbound["type"].(string)
	switch outboundType {
	case "shadowsocks":
		return normalizeShadowsocksOutbound(outbound)
	case "vless":
		normalizeVLESSOutbound(outbound)
		return nil
	case "anytls":
		normalizeAnyTLSOutbound(outbound)
		return nil
	default:
		return nil
	}
}

func normalizeShadowsocksOutbound(outbound Outbound) error {
	plugin, _ := outbound["plugin"].(string)
	if plugin == "" {
		return nil
	}

	normalizedPlugin, normalizedOpts, err := normalizeShadowsocksPlugin(plugin, outbound["plugin_opts"])
	if err != nil {
		return err
	}
	outbound["plugin"] = normalizedPlugin
	if normalizedOpts == "" {
		delete(outbound, "plugin_opts")
	} else {
		outbound["plugin_opts"] = normalizedOpts
	}

	return nil
}

func normalizeVLESSOutbound(outbound Outbound) {
	tlsEnabled := false
	switch value := outbound["tls"].(type) {
	case bool:
		tlsEnabled = value
	case map[string]interface{}:
		if enabled, ok := value["enabled"].(bool); ok {
			tlsEnabled = enabled
		}
	case nil:
	default:
		return
	}

	reality, hasReality := mapValue(outbound["reality"])
	if !hasReality {
		if tlsMap, ok := outbound["tls"].(map[string]interface{}); ok {
			reality, hasReality = mapValue(tlsMap["reality"])
		}
	}
	if security, _ := outbound["security"].(string); strings.EqualFold(strings.TrimSpace(security), "reality") {
		hasReality = true
	}
	if !tlsEnabled && !hasReality {
		return
	}

	tls, ok := outbound["tls"].(map[string]interface{})
	if !ok {
		tls = map[string]interface{}{}
		outbound["tls"] = tls
	}
	tls["enabled"] = true

	if serverName, ok := outbound["server_name"].(string); ok && strings.TrimSpace(serverName) != "" {
		tls["server_name"] = strings.TrimSpace(serverName)
		delete(outbound, "server_name")
	}
	if hasReality {
		if reality == nil {
			reality = map[string]interface{}{}
		}
		reality["enabled"] = true
		tls["reality"] = reality
		utls, ok := tls["utls"].(map[string]interface{})
		if !ok {
			utls = map[string]interface{}{}
			tls["utls"] = utls
		}
		utls["enabled"] = true
		if _, ok := utls["fingerprint"].(string); !ok {
			utls["fingerprint"] = "chrome"
		}
		delete(outbound, "reality")
	}
	delete(outbound, "security")
}

func normalizeAnyTLSOutbound(outbound Outbound) {
	tls, ok := outbound["tls"].(map[string]interface{})
	if !ok {
		tls = map[string]interface{}{}
		outbound["tls"] = tls
	}
	tls["enabled"] = true

	for _, field := range []string{"idle_session_check_interval", "idle_session_timeout"} {
		if value, ok := anyTLSDurationValue(outbound[field]); ok {
			outbound[field] = value
		} else {
			delete(outbound, field)
		}
	}
}

func mapValue(raw interface{}) (map[string]interface{}, bool) {
	switch value := raw.(type) {
	case map[string]interface{}:
		return value, true
	case map[string]string:
		result := make(map[string]interface{}, len(value))
		for key, item := range value {
			result[key] = item
		}
		return result, true
	default:
		return nil, false
	}
}

func anyTLSDurationValue(raw interface{}) (string, bool) {
	switch value := raw.(type) {
	case nil:
		return "", false
	case string:
		value = strings.TrimSpace(value)
		if value == "" {
			return "", false
		}
		if _, err := strconv.ParseFloat(value, 64); err == nil {
			return value + "s", true
		}
		return value, true
	case int:
		return fmt.Sprintf("%ds", value), true
	case int8:
		return fmt.Sprintf("%ds", value), true
	case int16:
		return fmt.Sprintf("%ds", value), true
	case int32:
		return fmt.Sprintf("%ds", value), true
	case int64:
		return fmt.Sprintf("%ds", value), true
	case uint:
		return fmt.Sprintf("%ds", value), true
	case uint8:
		return fmt.Sprintf("%ds", value), true
	case uint16:
		return fmt.Sprintf("%ds", value), true
	case uint32:
		return fmt.Sprintf("%ds", value), true
	case uint64:
		return fmt.Sprintf("%ds", value), true
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32) + "s", true
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64) + "s", true
	case json.Number:
		return value.String() + "s", true
	default:
		return "", false
	}
}

func normalizeShadowsocksPlugin(plugin string, rawOpts interface{}) (string, string, error) {
	switch strings.ToLower(plugin) {
	case "obfs", "obfs-local":
		opts, err := serializeSimpleObfsPluginOpts(rawOpts)
		if err != nil {
			return "", "", err
		}
		return "obfs-local", opts, nil
	case "v2ray-plugin":
		opts, err := serializeSIP003PluginOpts(rawOpts)
		if err != nil {
			return "", "", err
		}
		return "v2ray-plugin", opts, nil
	default:
		return "", "", fmt.Errorf("shadowsocks plugin %q 不受 sing-box 支持", plugin)
	}
}

func serializeSimpleObfsPluginOpts(rawOpts interface{}) (string, error) {
	if text, ok := rawOpts.(string); ok {
		return text, nil
	}

	opts, err := mapPluginOpts(rawOpts)
	if err != nil || opts == nil {
		return "", err
	}

	var parts []string
	if mode := stringifyPluginOpt(opts["mode"]); mode != "" {
		parts = append(parts, "obfs="+mode)
	}
	if host := stringifyPluginOpt(opts["host"]); host != "" {
		parts = append(parts, "obfs-host="+host)
	}
	uri := stringifyPluginOpt(opts["uri"])
	if uri == "" {
		uri = stringifyPluginOpt(opts["path"])
	}
	if uri != "" {
		parts = append(parts, "obfs-uri="+uri)
	}

	return strings.Join(parts, ";"), nil
}

func serializeSIP003PluginOpts(rawOpts interface{}) (string, error) {
	opts, err := mapPluginOpts(rawOpts)
	if err != nil || opts == nil {
		return "", err
	}

	keys := make([]string, 0, len(opts))
	for key := range opts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		part, ok, err := serializeSIP003PluginOpt(key, opts[key])
		if err != nil {
			return "", err
		}
		if ok {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, ";"), nil
}

func mapPluginOpts(rawOpts interface{}) (map[string]interface{}, error) {
	switch opts := rawOpts.(type) {
	case nil:
		return nil, nil
	case string:
		return map[string]interface{}{"": opts}, nil
	case map[string]interface{}:
		return opts, nil
	default:
		return nil, fmt.Errorf("plugin_opts 类型无效: %T", rawOpts)
	}
}

func serializeSIP003PluginOpt(key string, value interface{}) (string, bool, error) {
	if key == "" {
		text, ok := value.(string)
		if !ok {
			return "", false, fmt.Errorf("plugin_opts 字符串值类型无效: %T", value)
		}
		return text, true, nil
	}

	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "", false, nil
		}
		return key + "=" + typed, true, nil
	case bool:
		if typed {
			return key, true, nil
		}
		return "", false, nil
	case int, int8, int16, int32, int64, float32, float64:
		return fmt.Sprintf("%s=%v", key, typed), true, nil
	default:
		return "", false, fmt.Errorf("plugin_opts.%s 类型无效: %T", key, value)
	}
}

func stringifyPluginOpt(value interface{}) string {
	text, _ := value.(string)
	return text
}

// matchFilter 检查节点是否匹配过滤器
func (b *ConfigBuilder) matchFilter(node storage.Node, filter storage.Filter) bool {
	name := strings.ToLower(node.Tag)

	// 1. 检查国家包含条件
	if len(filter.IncludeCountries) > 0 {
		matched := false
		for _, country := range filter.IncludeCountries {
			if strings.EqualFold(node.Country, country) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 2. 检查国家排除条件
	for _, country := range filter.ExcludeCountries {
		if strings.EqualFold(node.Country, country) {
			return false
		}
	}

	// 3. 检查关键字包含条件
	if len(filter.Include) > 0 {
		matched := false
		for _, keyword := range filter.Include {
			if strings.Contains(name, strings.ToLower(keyword)) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// 4. 检查关键字排除条件
	for _, keyword := range filter.Exclude {
		if strings.Contains(name, strings.ToLower(keyword)) {
			return false
		}
	}

	return true
}

// buildRoute 构建路由配置
func (b *ConfigBuilder) buildRoute() *RouteConfig {
	route := &RouteConfig{
		AutoDetectInterface: true,
		Final:               "Final",
		// 默认域名解析器：用于解析所有 outbound 的服务器地址，避免 DNS 循环
		DefaultDomainResolver: &DomainResolver{
			Server:     "dns_direct",
			RewriteTTL: 60,
		},
	}

	// 构建路由规则
	var rules []RouteRule

	// 1. 添加 sniff action（嗅探流量类型，配合 FakeIP 使用）
	rules = append(rules, RouteRule{
		"action":  "sniff",
		"sniffer": []string{"dns", "http", "tls", "quic"},
		"timeout": "500ms",
	})

	// 2. DNS 劫持使用 action（替代已弃用的 dns-out）
	rules = append(rules, RouteRule{
		"protocol": "dns",
		"action":   "hijack-dns",
	})

	// 3. 添加 hosts 域名的路由规则（优先级高，在其他规则之前）
	// 使用 override_address 直接指定目标 IP，避免 DIRECT outbound 重新 DNS 解析
	// 这解决了 sniff_override_destination 导致的 NXDOMAIN 问题
	systemHosts := ParseSystemHosts()
	for domain, ips := range systemHosts {
		if len(ips) > 0 {
			rules = append(rules, RouteRule{
				"domain":           []string{domain},
				"outbound":         "DIRECT",
				"override_address": ips[0],
			})
		}
	}
	for _, host := range b.settings.Hosts {
		if host.Enabled && host.Domain != "" && len(host.IPs) > 0 {
			rules = append(rules, RouteRule{
				"domain":           []string{host.Domain},
				"outbound":         "DIRECT",
				"override_address": host.IPs[0],
			})
		}
	}

	// 自定义入站端口绑定的出站应优先于普通分流规则，
	// 否则会被域名/IP 规则提前命中，导致指定链路或节点失效。
	for _, port := range b.inboundPorts {
		if !port.Enabled {
			continue
		}

		outbound := ""
		if port.UseTorExit {
			outbound = b.torChainRouteOutbound(port.TorChainID)
			if outbound == "" {
				outbound = "REJECT"
			}
		} else {
			outbound = port.Outbound
			// 国家代码（如 "JP"）需要映射为 outbound tag（如 "🇯🇵 日本"）
			if _, isCountry := storage.CountryEmojis[outbound]; isCountry {
				outbound = fmt.Sprintf("%s %s", storage.GetCountryEmoji(outbound), storage.GetCountryName(outbound))
			}
		}
		if outbound == "" {
			continue
		}

		rules = append(rules, RouteRule{
			"inbound":  []string{fmt.Sprintf("custom-%s", port.ID)},
			"outbound": outbound,
		})
	}

	route.Rules = rules

	return route
}

func (b *ConfigBuilder) torChainRouteOutbound(chainID string) string {
	for _, chain := range b.proxyChains {
		if chain.ID == chainID && chain.Enabled && storage.ChainContainsTor(chain.Nodes) {
			return chain.Name
		}
	}
	return ""
}

// buildExperimental 构建实验性配置
func (b *ConfigBuilder) buildExperimental() *ExperimentalConfig {
	// 计算 cache.db 的路径
	cachePath := "cache.db"
	if b.dataDir != "" {
		cachePath = filepath.Join(b.dataDir, "cache.db")
	}

	exp := &ExperimentalConfig{
		// CacheFile 用于存储缓存数据
		CacheFile: &CacheFileConfig{
			Enabled:     true,
			Path:        cachePath,
			StoreFakeIP: b.settings.FakeIPEnabled, // 根据设置存储 FakeIP 映射
		},
	}

	// 如果启用了 Clash API，添加配置
	if b.settings.ClashAPIPort > 0 {
		externalControllerHost := "127.0.0.1"
		if b.settings.ClashAPILanEnabled {
			externalControllerHost = "0.0.0.0"
		}

		exp.ClashAPI = &ClashAPIConfig{
			ExternalController: fmt.Sprintf("%s:%d", externalControllerHost, b.settings.ClashAPIPort),
			DefaultMode:        "rule",
		}
		if b.settings.ClashUIEnabled {
			externalUIPath := strings.TrimSpace(b.settings.ClashUIPath)
			if externalUIPath == "" {
				externalUIPath = zashboard.DefaultUIPath
			}

			// 转换为绝对路径，避免 sing-box 找不到文件
			if !filepath.IsAbs(externalUIPath) {
				externalUIPath = filepath.Join(b.dataDir, externalUIPath)
			}

			exp.ClashAPI.ExternalUI = externalUIPath
			if !zashboard.UsesEmbeddedPath(b.settings.ClashUIPath) {
				exp.ClashAPI.ExternalUIDownloadURL = b.buildGitHubDownloadURL(defaultZashboardExternalUIDownloadURL)
			}
		}
		if b.settings.ClashAPISecret != "" {
			exp.ClashAPI.Secret = b.settings.ClashAPISecret
		}
	}

	return exp
}

func (b *ConfigBuilder) buildGitHubDownloadURL(originalURL string) string {
	if b.settings == nil {
		return originalURL
	}

	githubProxy := strings.TrimSpace(b.settings.GithubProxy)
	if githubProxy == "" {
		return originalURL
	}

	if !strings.HasSuffix(githubProxy, "/") {
		githubProxy += "/"
	}

	return githubProxy + originalURL
}
