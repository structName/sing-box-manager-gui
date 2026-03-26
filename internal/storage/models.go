package storage

import "time"

// Profile 配置方案
type Profile struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	IsActive    bool      `json:"is_active"`
}

// InboundPort 入站端口配置
type InboundPort struct {
	ID       string       `json:"id"`
	Name     string       `json:"name"`   // 端口名称，如 "家人专用"
	Type     string       `json:"type"`   // mixed/http/socks
	Listen   string       `json:"listen"` // 监听地址，默认 127.0.0.1
	Port     int          `json:"port"`   // 端口号
	Auth     *InboundAuth `json:"auth,omitempty"`
	Outbound string       `json:"outbound"` // 关联的出站 tag
	Enabled  bool         `json:"enabled"`
}

// InboundAuth 入站认证
type InboundAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ProxyChain 代理链路配置
type ProxyChain struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`                  // 链路名称，如 "香港中转链"
	Description  string             `json:"description"`           // 描述
	Nodes        []string           `json:"nodes"`                 // 有序的节点 Tag 列表（兼容旧版）
	ChainNodes   []ChainNode        `json:"chain_nodes,omitempty"` // 节点副本列表
	Enabled      bool               `json:"enabled"`
	HealthConfig *ChainHealthConfig `json:"health_config,omitempty"` // 健康检测配置
}

// ChainNode 链路节点副本引用
type ChainNode struct {
	OriginalTag string `json:"original_tag"` // 原节点 Tag
	CopyTag     string `json:"copy_tag"`     // 副本 Tag: "{链路名}-{原Tag}"
	Source      string `json:"source"`       // 来源订阅 ID 或 "manual"
}

// ChainHealthConfig 链路健康检测配置
type ChainHealthConfig struct {
	Enabled      bool   `json:"enabled"`
	Interval     int    `json:"interval"`      // 检测间隔（秒），默认 300
	Timeout      int    `json:"timeout"`       // 超时（秒），默认 10
	URL          string `json:"url"`           // 测试 URL
	AlertEnabled bool   `json:"alert_enabled"` // 启用告警
	AutoSwitch   bool   `json:"auto_switch"`   // 自动切换
}

// ChainHealthStatus 链路健康状态
type ChainHealthStatus struct {
	ChainID      string             `json:"chain_id"`
	LastCheck    time.Time          `json:"last_check"`
	Status       string             `json:"status"`  // "healthy" | "degraded" | "unhealthy"
	Latency      int                `json:"latency"` // 总延迟 (ms)
	NodeStatuses []NodeHealthStatus `json:"node_statuses"`
}

// NodeHealthStatus 节点健康状态
type NodeHealthStatus struct {
	Tag     string `json:"tag"`
	Status  string `json:"status"`  // "healthy" | "unhealthy" | "timeout"
	Latency int    `json:"latency"` // ms
	Error   string `json:"error,omitempty"`
}

// ChainSpeedResult 链路速度测试结果
type ChainSpeedResult struct {
	ChainID    string    `json:"chain_id"`
	TestTime   time.Time `json:"test_time"`
	SpeedMbps  float64   `json:"speed_mbps"`  // 下载速度 Mbps
	BytesTotal int64     `json:"bytes_total"` // 下载字节数
	Duration   int64     `json:"duration"`    // 耗时 ms
}

// GenerateChainNodeCopyTag 生成链路节点副本 Tag
func GenerateChainNodeCopyTag(chainName, originalTag string) string {
	return chainName + "-" + originalTag
}

// Subscription 订阅
type Subscription struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	URL            string     `json:"url"`
	NodeCount      int        `json:"node_count"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ExpireAt       *time.Time `json:"expire_at,omitempty"`
	Traffic        *Traffic   `json:"traffic,omitempty"`
	Nodes          []Node     `json:"nodes"`
	Enabled        bool       `json:"enabled"`
	AutoUpdate     *bool      `json:"auto_update,omitempty"`     // 是否自动更新
	UpdateInterval int        `json:"update_interval,omitempty"` // 更新间隔（分钟）
}

// Traffic 流量信息
type Traffic struct {
	Total     int64 `json:"total"`     // 总流量 (bytes)
	Used      int64 `json:"used"`      // 已用流量
	Remaining int64 `json:"remaining"` // 剩余流量
}

// Node 节点
type Node struct {
	Tag          string                 `json:"tag"`
	Type         string                 `json:"type"` // shadowsocks/vmess/vless/trojan/hysteria2/tuic
	Server       string                 `json:"server"`
	ServerPort   int                    `json:"server_port"`
	Extra        map[string]interface{} `json:"extra,omitempty"`         // 协议特定字段
	Country      string                 `json:"country,omitempty"`       // 国家代码
	CountryEmoji string                 `json:"country_emoji,omitempty"` // 国家 emoji
	Source       string                 `json:"source,omitempty"`        // 来源: "manual" 或订阅 ID
	SourceName   string                 `json:"source_name,omitempty"`   // 来源名称: "手动添加" 或机场名称
}

// NodeGroup 节点分组（按来源）
type NodeGroup struct {
	Source     string `json:"source"`
	SourceName string `json:"source_name"`
	Nodes      []Node `json:"nodes"`
}

// ManualNode 手动添加的节点
type ManualNode struct {
	ID      string `json:"id"`
	Node    Node   `json:"node"`
	Enabled bool   `json:"enabled"`
}

// CountryGroup 国家节点分组
type CountryGroup struct {
	Code      string `json:"code"`       // 国家代码 (如 HK, US, JP)
	Name      string `json:"name"`       // 国家名称 (如 香港, 美国, 日本)
	Emoji     string `json:"emoji"`      // 国旗 emoji
	NodeCount int    `json:"node_count"` // 节点数量
}

// Filter 过滤器
type Filter struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Include          []string       `json:"include"`           // 包含关键字
	Exclude          []string       `json:"exclude"`           // 排除关键字
	IncludeCountries []string       `json:"include_countries"` // 包含的国家代码
	ExcludeCountries []string       `json:"exclude_countries"` // 排除的国家代码
	Mode             string         `json:"mode"`              // urltest / select
	URLTestConfig    *URLTestConfig `json:"urltest_config,omitempty"`
	Subscriptions    []string       `json:"subscriptions"` // 适用的订阅ID，空表示全部
	AllNodes         bool           `json:"all_nodes"`     // 是否应用于所有节点
	Enabled          bool           `json:"enabled"`
}

// URLTestConfig urltest 模式配置
type URLTestConfig struct {
	URL       string `json:"url"`
	Interval  string `json:"interval"`
	Tolerance int    `json:"tolerance"`
}

// Rule 自定义规则
type Rule struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	RuleType string   `json:"rule_type"` // domain_suffix/domain_keyword/ip_cidr/geosite/geoip/port
	Values   []string `json:"values"`    // 规则值列表
	Outbound string   `json:"outbound"`  // 目标出站
	Enabled  bool     `json:"enabled"`
	Priority int      `json:"priority"` // 优先级 (越小越优先)
}

// RuleGroup 预设规则组
type RuleGroup struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	SiteRules []string `json:"site_rules"` // geosite 规则
	IPRules   []string `json:"ip_rules"`   // geoip 规则
	Outbound  string   `json:"outbound"`
	Enabled   bool     `json:"enabled"`
}

// HostEntry DNS hosts 映射条目
type HostEntry struct {
	ID      string   `json:"id"`
	Domain  string   `json:"domain"` // 域名
	IPs     []string `json:"ips"`    // IP 地址列表
	Enabled bool     `json:"enabled"`
}

// Settings 全局设置
type Settings struct {
	// sing-box 路径
	SingBoxPath string `json:"singbox_path"`
	ConfigPath  string `json:"config_path"`

	// 入站配置
	MixedPort       int    `json:"mixed_port"`        // HTTP/SOCKS5 混合端口
	TunEnabled      bool   `json:"tun_enabled"`       // TUN 模式
	LanProxyEnabled bool   `json:"lan_proxy_enabled"` // 是否开启局域网代理（Mixed 端口）
	LanListenIP     string `json:"lan_listen_ip"`     // 局域网监听地址（默认 0.0.0.0）

	// DNS 配置
	ProxyDNS      string      `json:"proxy_dns"`                // 代理 DNS
	DirectDNS     string      `json:"direct_dns"`               // 直连 DNS
	Hosts         []HostEntry `json:"hosts,omitempty"`          // DNS hosts 映射
	FakeIPEnabled bool        `json:"fakeip_enabled,omitempty"` // 启用 FakeIP 模式

	// 控制面板
	WebPort            int    `json:"web_port"`                       // 管理界面端口
	ClashAPIPort       int    `json:"clash_api_port"`                 // Clash API 端口
	ClashUIEnabled     bool   `json:"clash_ui_enabled"`               // 是否启用 zashboard
	ClashUIPath        string `json:"clash_ui_path"`                  // zashboard 路径
	ClashAPISecret     string `json:"clash_api_secret"`               // zashboard 鉴权密码
	SessionTTLMinutes  int    `json:"session_ttl_minutes,omitempty"`  // Web 会话有效期（分钟）
	AdminPasswordHash  string `json:"admin_password_hash,omitempty"`  // 管理面板密码哈希
	AuthBootstrappedAt string `json:"auth_bootstrapped_at,omitempty"` // 管理面板密码初始化时间

	// 漏网规则
	FinalOutbound string `json:"final_outbound"` // 默认出站

	// 规则集源
	RuleSetBaseURL string `json:"ruleset_base_url"` // 规则集下载地址

	// 自动化设置
	AutoApply            bool `json:"auto_apply"`            // 配置变更后自动应用
	SubscriptionInterval int  `json:"subscription_interval"` // 订阅自动更新间隔 (分钟)，0 表示禁用

	// GitHub 代理设置
	GithubProxy string `json:"github_proxy"` // GitHub 代理地址，如 https://ghproxy.com/

	// 链路健康检测配置
	ChainHealthConfig *ChainHealthConfig `json:"chain_health_config,omitempty"`
}

// DefaultSettings 默认设置
func DefaultSettings() *Settings {
	return &Settings{
		SingBoxPath:          "bin/sing-box",
		ConfigPath:           "generated/config.json",
		MixedPort:            2080,
		TunEnabled:           false,
		LanProxyEnabled:      false,
		LanListenIP:          "0.0.0.0",
		ProxyDNS:             "https://1.1.1.1/dns-query",
		DirectDNS:            "https://dns.alidns.com/dns-query",
		WebPort:              9090,
		ClashAPIPort:         9091,
		ClashUIEnabled:       true,
		ClashUIPath:          "zashboard",
		ClashAPISecret:       mustNewZashboardSecret(),
		SessionTTLMinutes:    10080,
		FinalOutbound:        "Proxy",
		RuleSetBaseURL:       "https://github.com/lyc8503/sing-box-rules/raw/rule-set-geosite",
		AutoApply:            true, // 默认开启自动应用
		SubscriptionInterval: 60,   // 默认 60 分钟更新一次
		GithubProxy:          "",   // 默认不使用代理
		ChainHealthConfig: &ChainHealthConfig{
			Enabled:      false,
			Interval:     300,
			Timeout:      10,
			URL:          "https://www.gstatic.com/generate_204",
			AlertEnabled: false,
			AutoSwitch:   false,
		},
	}
}

func mustNewZashboardSecret() string {
	secret, err := NewZashboardSecret()
	if err != nil {
		panic(err)
	}

	return secret
}

// AppData 应用数据
type AppData struct {
	Subscriptions []Subscription `json:"subscriptions"`
	ManualNodes   []ManualNode   `json:"manual_nodes"`
	Filters       []Filter       `json:"filters"`
	Rules         []Rule         `json:"rules"`
	RuleGroups    []RuleGroup    `json:"rule_groups"`
	Settings      *Settings      `json:"settings"`
	Profiles      []Profile      `json:"profiles,omitempty"`       // Profile 元数据列表
	ActiveProfile string         `json:"active_profile,omitempty"` // 当前激活的 Profile ID
	InboundPorts  []InboundPort  `json:"inbound_ports,omitempty"`  // 自定义入站端口
	ProxyChains   []ProxyChain   `json:"proxy_chains,omitempty"`   // 代理链路配置
}

// DefaultRuleGroups 默认规则组
func DefaultRuleGroups() []RuleGroup {
	return []RuleGroup{
		{ID: "ad-block", Name: "广告拦截", SiteRules: []string{"category-ads-all"}, Outbound: "REJECT", Enabled: true},
		{ID: "ai-services", Name: "AI 服务", SiteRules: []string{"openai", "anthropic", "jetbrains-ai"}, Outbound: "Proxy", Enabled: true},
		{ID: "google", Name: "Google", SiteRules: []string{"google"}, IPRules: []string{"google"}, Outbound: "Proxy", Enabled: true},
		{ID: "youtube", Name: "YouTube", SiteRules: []string{"youtube"}, Outbound: "Proxy", Enabled: true},
		{ID: "github", Name: "GitHub", SiteRules: []string{"github"}, Outbound: "Proxy", Enabled: true},
		{ID: "telegram", Name: "Telegram", SiteRules: []string{"telegram"}, IPRules: []string{"telegram"}, Outbound: "Proxy", Enabled: true},
		{ID: "twitter", Name: "Twitter/X", SiteRules: []string{"twitter"}, Outbound: "Proxy", Enabled: true},
		{ID: "netflix", Name: "Netflix", SiteRules: []string{"netflix"}, Outbound: "Proxy", Enabled: false},
		{ID: "spotify", Name: "Spotify", SiteRules: []string{"spotify"}, Outbound: "Proxy", Enabled: false},
		{ID: "apple", Name: "Apple", SiteRules: []string{"apple"}, Outbound: "DIRECT", Enabled: true},
		{ID: "microsoft", Name: "Microsoft", SiteRules: []string{"microsoft"}, Outbound: "DIRECT", Enabled: true},
		{ID: "cn", Name: "中国地区", SiteRules: []string{"geolocation-cn"}, IPRules: []string{"cn"}, Outbound: "DIRECT", Enabled: true},
		{ID: "private", Name: "私有网络", SiteRules: []string{"private"}, IPRules: []string{"private"}, Outbound: "DIRECT", Enabled: true},
	}
}

// CountryNames 国家代码到中文名称的映射
var CountryNames = map[string]string{
	"HK":    "香港",
	"TW":    "台湾",
	"JP":    "日本",
	"KR":    "韩国",
	"SG":    "新加坡",
	"US":    "美国",
	"GB":    "英国",
	"DE":    "德国",
	"FR":    "法国",
	"NL":    "荷兰",
	"AU":    "澳大利亚",
	"CA":    "加拿大",
	"RU":    "俄罗斯",
	"IN":    "印度",
	"BR":    "巴西",
	"AR":    "阿根廷",
	"TR":    "土耳其",
	"TH":    "泰国",
	"VN":    "越南",
	"MY":    "马来西亚",
	"PH":    "菲律宾",
	"ID":    "印尼",
	"AE":    "阿联酋",
	"ZA":    "南非",
	"CH":    "瑞士",
	"IT":    "意大利",
	"ES":    "西班牙",
	"SE":    "瑞典",
	"NO":    "挪威",
	"FI":    "芬兰",
	"DK":    "丹麦",
	"PL":    "波兰",
	"CZ":    "捷克",
	"AT":    "奥地利",
	"IE":    "爱尔兰",
	"PT":    "葡萄牙",
	"GR":    "希腊",
	"IL":    "以色列",
	"MX":    "墨西哥",
	"CL":    "智利",
	"CO":    "哥伦比亚",
	"PE":    "秘鲁",
	"NZ":    "新西兰",
	"OTHER": "其他",
}

// CountryEmojis 国家代码到 emoji 的映射
var CountryEmojis = map[string]string{
	"HK":    "🇭🇰",
	"TW":    "🇹🇼",
	"JP":    "🇯🇵",
	"KR":    "🇰🇷",
	"SG":    "🇸🇬",
	"US":    "🇺🇸",
	"GB":    "🇬🇧",
	"DE":    "🇩🇪",
	"FR":    "🇫🇷",
	"NL":    "🇳🇱",
	"AU":    "🇦🇺",
	"CA":    "🇨🇦",
	"RU":    "🇷🇺",
	"IN":    "🇮🇳",
	"BR":    "🇧🇷",
	"AR":    "🇦🇷",
	"TR":    "🇹🇷",
	"TH":    "🇹🇭",
	"VN":    "🇻🇳",
	"MY":    "🇲🇾",
	"PH":    "🇵🇭",
	"ID":    "🇮🇩",
	"AE":    "🇦🇪",
	"ZA":    "🇿🇦",
	"CH":    "🇨🇭",
	"IT":    "🇮🇹",
	"ES":    "🇪🇸",
	"SE":    "🇸🇪",
	"NO":    "🇳🇴",
	"FI":    "🇫🇮",
	"DK":    "🇩🇰",
	"PL":    "🇵🇱",
	"CZ":    "🇨🇿",
	"AT":    "🇦🇹",
	"IE":    "🇮🇪",
	"PT":    "🇵🇹",
	"GR":    "🇬🇷",
	"IL":    "🇮🇱",
	"MX":    "🇲🇽",
	"CL":    "🇨🇱",
	"CO":    "🇨🇴",
	"PE":    "🇵🇪",
	"NZ":    "🇳🇿",
	"OTHER": "🌐",
}

// GetCountryName 获取国家名称
func GetCountryName(code string) string {
	if name, ok := CountryNames[code]; ok {
		return name
	}
	return code
}

// GetCountryEmoji 获取国家 emoji
func GetCountryEmoji(code string) string {
	if emoji, ok := CountryEmojis[code]; ok {
		return emoji
	}
	return "🌐"
}
