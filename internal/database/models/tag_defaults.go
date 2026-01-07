package models

// DefaultTags 内置默认标签
// 参考 sublinkPro 项目的标签管理设计
var DefaultTags = []Tag{
	// ========== 延迟分级（互斥组: delay_level）==========
	{Name: "极速", Color: "#4CAF50", TagGroup: "delay_level", Description: "延迟 < 100ms"},
	{Name: "正常", Color: "#2196F3", TagGroup: "delay_level", Description: "延迟 100-300ms"},
	{Name: "较慢", Color: "#FF9800", TagGroup: "delay_level", Description: "延迟 > 300ms"},
	{Name: "超时", Color: "#F44336", TagGroup: "delay_level", Description: "延迟测试超时"},

	// ========== 速度分级（互斥组: speed_level）==========
	{Name: "高速", Color: "#8BC34A", TagGroup: "speed_level", Description: "速度 > 10 MB/s"},
	{Name: "中速", Color: "#03A9F4", TagGroup: "speed_level", Description: "速度 1-10 MB/s"},
	{Name: "低速", Color: "#FFC107", TagGroup: "speed_level", Description: "速度 < 1 MB/s"},

	// ========== 地区分类（互斥组: region）==========
	{Name: "香港", Color: "#E91E63", TagGroup: "region", Description: "香港节点"},
	{Name: "台湾", Color: "#9C27B0", TagGroup: "region", Description: "台湾节点"},
	{Name: "日本", Color: "#673AB7", TagGroup: "region", Description: "日本节点"},
	{Name: "新加坡", Color: "#3F51B5", TagGroup: "region", Description: "新加坡节点"},
	{Name: "美国", Color: "#00BCD4", TagGroup: "region", Description: "美国节点"},
	{Name: "韩国", Color: "#009688", TagGroup: "region", Description: "韩国节点"},
	{Name: "其他地区", Color: "#607D8B", TagGroup: "region", Description: "其他地区节点"},

	// ========== 协议分类（互斥组: protocol）==========
	{Name: "Shadowsocks", Color: "#795548", TagGroup: "protocol", Description: "SS 协议"},
	{Name: "VMess", Color: "#FF5722", TagGroup: "protocol", Description: "VMess 协议"},
	{Name: "VLESS", Color: "#E040FB", TagGroup: "protocol", Description: "VLESS 协议"},
	{Name: "Trojan", Color: "#00E676", TagGroup: "protocol", Description: "Trojan 协议"},
	{Name: "Hysteria2", Color: "#FFAB00", TagGroup: "protocol", Description: "Hysteria2 协议"},
	{Name: "TUIC", Color: "#18FFFF", TagGroup: "protocol", Description: "TUIC 协议"},

	// ========== 功能标签（无互斥组）==========
	{Name: "流媒体", Color: "#FF4081", Description: "适合流媒体观看"},
	{Name: "游戏加速", Color: "#7C4DFF", Description: "适合游戏加速"},
	{Name: "稳定节点", Color: "#00E5FF", Description: "稳定可靠的节点"},
}

// DefaultTagRules 内置默认规则
// 规则会在测速完成后或订阅更新后自动应用
var DefaultTagRules = []struct {
	Name        string
	TagName     string // 关联的标签名称
	Enabled     bool
	TriggerType string        // speed_test / subscription_update
	Conditions  TagConditions // 条件
}{
	// ========== 延迟分级规则（测速后触发）==========
	{
		Name:        "极速节点标记",
		TagName:     "极速",
		Enabled:     true,
		TriggerType: "speed_test",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "delay", Operator: "gt", Value: 0},
				{Field: "delay", Operator: "lt", Value: 100},
				{Field: "delay_status", Operator: "eq", Value: "success"},
			},
		},
	},
	{
		Name:        "正常延迟标记",
		TagName:     "正常",
		Enabled:     true,
		TriggerType: "speed_test",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "delay", Operator: "gte", Value: 100},
				{Field: "delay", Operator: "lte", Value: 300},
				{Field: "delay_status", Operator: "eq", Value: "success"},
			},
		},
	},
	{
		Name:        "较慢节点标记",
		TagName:     "较慢",
		Enabled:     true,
		TriggerType: "speed_test",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "delay", Operator: "gt", Value: 300},
				{Field: "delay_status", Operator: "eq", Value: "success"},
			},
		},
	},
	{
		Name:        "超时节点标记",
		TagName:     "超时",
		Enabled:     true,
		TriggerType: "speed_test",
		Conditions: TagConditions{
			Logic: "OR",
			Conditions: []TagCondition{
				{Field: "delay_status", Operator: "eq", Value: "timeout"},
				{Field: "delay_status", Operator: "eq", Value: "error"},
			},
		},
	},

	// ========== 速度分级规则（测速后触发）==========
	{
		Name:        "高速节点标记",
		TagName:     "高速",
		Enabled:     true,
		TriggerType: "speed_test",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "speed", Operator: "gt", Value: 10},
				{Field: "speed_status", Operator: "eq", Value: "success"},
			},
		},
	},
	{
		Name:        "中速节点标记",
		TagName:     "中速",
		Enabled:     true,
		TriggerType: "speed_test",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "speed", Operator: "gte", Value: 1},
				{Field: "speed", Operator: "lte", Value: 10},
				{Field: "speed_status", Operator: "eq", Value: "success"},
			},
		},
	},
	{
		Name:        "低速节点标记",
		TagName:     "低速",
		Enabled:     true,
		TriggerType: "speed_test",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "speed", Operator: "gt", Value: 0},
				{Field: "speed", Operator: "lt", Value: 1},
				{Field: "speed_status", Operator: "eq", Value: "success"},
			},
		},
	},

	// ========== 地区分类规则（订阅更新后触发）==========
	{
		Name:        "香港节点标记",
		TagName:     "香港",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "OR",
			Conditions: []TagCondition{
				{Field: "country", Operator: "eq", Value: "HK"},
				{Field: "country", Operator: "eq", Value: "CN-HK"},
			},
		},
	},
	{
		Name:        "台湾节点标记",
		TagName:     "台湾",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "OR",
			Conditions: []TagCondition{
				{Field: "country", Operator: "eq", Value: "TW"},
				{Field: "country", Operator: "eq", Value: "CN-TW"},
			},
		},
	},
	{
		Name:        "日本节点标记",
		TagName:     "日本",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "country", Operator: "eq", Value: "JP"},
			},
		},
	},
	{
		Name:        "新加坡节点标记",
		TagName:     "新加坡",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "country", Operator: "eq", Value: "SG"},
			},
		},
	},
	{
		Name:        "美国节点标记",
		TagName:     "美国",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "country", Operator: "eq", Value: "US"},
			},
		},
	},
	{
		Name:        "韩国节点标记",
		TagName:     "韩国",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "country", Operator: "eq", Value: "KR"},
			},
		},
	},

	// ========== 协议分类规则（订阅更新后触发）==========
	{
		Name:        "Shadowsocks 节点标记",
		TagName:     "Shadowsocks",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "OR",
			Conditions: []TagCondition{
				{Field: "type", Operator: "eq", Value: "ss"},
				{Field: "type", Operator: "eq", Value: "shadowsocks"},
			},
		},
	},
	{
		Name:        "VMess 节点标记",
		TagName:     "VMess",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "type", Operator: "eq", Value: "vmess"},
			},
		},
	},
	{
		Name:        "VLESS 节点标记",
		TagName:     "VLESS",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "type", Operator: "eq", Value: "vless"},
			},
		},
	},
	{
		Name:        "Trojan 节点标记",
		TagName:     "Trojan",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "type", Operator: "eq", Value: "trojan"},
			},
		},
	},
	{
		Name:        "Hysteria2 节点标记",
		TagName:     "Hysteria2",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "OR",
			Conditions: []TagCondition{
				{Field: "type", Operator: "eq", Value: "hysteria2"},
				{Field: "type", Operator: "eq", Value: "hy2"},
			},
		},
	},
	{
		Name:        "TUIC 节点标记",
		TagName:     "TUIC",
		Enabled:     true,
		TriggerType: "subscription_update",
		Conditions: TagConditions{
			Logic: "AND",
			Conditions: []TagCondition{
				{Field: "type", Operator: "eq", Value: "tuic"},
			},
		},
	},
}
