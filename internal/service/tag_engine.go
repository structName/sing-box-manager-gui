package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/database/models"
	"github.com/xiaobei/singbox-manager/internal/logger"
)

// TagEngine 标签规则引擎
type TagEngine struct {
	store       *database.Store
	taskManager *TaskManager
}

// NewTagEngine 创建标签引擎
func NewTagEngine(store *database.Store) *TagEngine {
	return &TagEngine{store: store}
}

// Rebind 更新内部 store 引用（用于 Profile 切换）
func (e *TagEngine) Rebind(store *database.Store) {
	e.store = store
}

// SetTaskManager 设置任务管理器
func (e *TagEngine) SetTaskManager(tm *TaskManager) {
	e.taskManager = tm
}

// ApplyRulesResult 应用规则结果
type ApplyRulesResult struct {
	ProcessedNodes int            `json:"processed_nodes"`
	AppliedTags    map[string]int `json:"applied_tags"` // tag_name -> count
	Errors         []string       `json:"errors,omitempty"`
}

// ApplyRules 应用标签规则
func (e *TagEngine) ApplyRules(triggerType string, nodeIDs []uint) (*ApplyRulesResult, error) {
	result := &ApplyRulesResult{
		AppliedTags: make(map[string]int),
	}

	// 获取启用的规则
	rules, err := e.store.GetEnabledTagRulesByTrigger(triggerType)
	if err != nil {
		return nil, err
	}

	if len(rules) == 0 {
		logger.Info("没有找到触发类型为 %s 的启用规则", triggerType)
		return result, nil
	}

	// 获取要处理的节点
	var nodes []models.Node
	if len(nodeIDs) > 0 {
		for _, id := range nodeIDs {
			node, err := e.store.GetNode(id)
			if err == nil {
				nodes = append(nodes, *node)
			}
		}
	} else {
		nodes, err = e.store.GetNodes()
		if err != nil {
			return nil, err
		}
	}

	result.ProcessedNodes = len(nodes)

	// 创建任务记录
	var task *models.Task
	if e.taskManager != nil && len(nodes) > 0 {
		triggerName, taskTrigger := "手动", models.TaskTriggerManual
		switch triggerType {
		case "subscription_update":
			triggerName, taskTrigger = "订阅更新", models.TaskTriggerEvent
		case "speed_test":
			triggerName, taskTrigger = "测速完成", models.TaskTriggerEvent
		case "scheduled":
			triggerName, taskTrigger = "定时", models.TaskTriggerScheduled
		}
		task, _, _ = e.taskManager.CreateTask(
			models.TaskTypeTagRule,
			fmt.Sprintf("应用标签规则 (%s)", triggerName),
			taskTrigger,
			len(nodes),
		)
		e.taskManager.StartTask(task.ID)
	}

	logger.Info("开始应用标签规则，处理 %d 个节点，%d 条规则", len(nodes), len(rules))

	// 对每个节点应用规则
	for i, node := range nodes {
		// 更新进度
		if task != nil && i%10 == 0 {
			e.taskManager.UpdateProgress(task.ID, i, node.Tag, "")
		}

		for _, rule := range rules {
			if e.matchConditions(&node, rule.Conditions) {
				// 获取标签信息
				tag := rule.Tag
				if tag == nil {
					tag, _ = e.store.GetTag(rule.TagID)
				}
				if tag == nil {
					continue
				}

				// 处理互斥标签
				if tag.TagGroup != "" {
					sameGroupTags, _ := e.store.GetTagsByGroup(tag.TagGroup)
					for _, t := range sameGroupTags {
						if t.ID != tag.ID {
							e.store.RemoveNodeTag(node.ID, t.ID)
						}
					}
				}

				// 添加标签
				if err := e.store.AddNodeTag(node.ID, rule.TagID); err != nil {
					result.Errors = append(result.Errors, err.Error())
				} else {
					result.AppliedTags[tag.Name]++
				}
			}
		}
	}

	// 完成任务
	if task != nil {
		totalApplied := 0
		for _, count := range result.AppliedTags {
			totalApplied += count
		}
		e.taskManager.CompleteTask(task.ID, fmt.Sprintf("应用完成，共打标 %d 次", totalApplied), map[string]any{
			"processed_nodes": result.ProcessedNodes,
			"applied_tags":    result.AppliedTags,
		})
	}

	logger.Info("标签规则应用完成，共打标 %d 次", len(result.AppliedTags))
	return result, nil
}

// matchConditions 检查节点是否匹配条件
func (e *TagEngine) matchConditions(node *models.Node, conditions models.TagConditions) bool {
	if len(conditions.Conditions) == 0 {
		return true // 无条件则匹配所有
	}

	logic := strings.ToUpper(conditions.Logic)
	if logic == "" {
		logic = "AND"
	}

	for _, cond := range conditions.Conditions {
		matched := e.matchCondition(node, cond)
		if logic == "OR" && matched {
			return true
		}
		if logic == "AND" && !matched {
			return false
		}
	}

	return logic == "AND"
}

// matchCondition 检查单个条件
func (e *TagEngine) matchCondition(node *models.Node, cond models.TagCondition) bool {
	var value any

	// 获取字段值（参考 sublinkPro 扩展更多字段）
	switch cond.Field {
	// 测速相关
	case "delay":
		value = node.Delay
	case "speed":
		value = node.Speed
	case "delay_status":
		value = node.DelayStatus
	case "speed_status":
		value = node.SpeedStatus
	// 地理信息
	case "country":
		value = node.Country
	case "country_emoji":
		value = node.CountryEmoji
	case "landing_ip":
		value = node.LandingIP
	// 节点属性
	case "name", "tag":
		value = node.Tag
	case "type", "protocol":
		value = node.Type
	case "server", "server_address":
		value = node.Server
	case "server_port", "port":
		value = node.ServerPort
	case "source":
		value = node.Source
	case "source_name":
		value = node.SourceName
	case "enabled":
		value = node.Enabled
	case "link":
		value = node.Link
	default:
		return false
	}

	// 比较值
	return e.compareValues(value, cond.Operator, cond.Value)
}

// compareValues 比较值
func (e *TagEngine) compareValues(actual any, operator string, expected any) bool {
	switch operator {
	case "eq", "==", "=":
		return e.equals(actual, expected)
	case "ne", "!=":
		return !e.equals(actual, expected)
	case "gt", ">":
		return e.compare(actual, expected) > 0
	case "lt", "<":
		return e.compare(actual, expected) < 0
	case "gte", ">=":
		return e.compare(actual, expected) >= 0
	case "lte", "<=":
		return e.compare(actual, expected) <= 0
	case "contains":
		return e.contains(actual, expected)
	case "not_contains":
		return !e.contains(actual, expected)
	case "regex":
		return e.matchRegex(actual, expected)
	case "in":
		return e.inList(actual, expected)
	case "not_in":
		return !e.inList(actual, expected)
	default:
		return false
	}
}

// equals 相等比较
func (e *TagEngine) equals(a, b any) bool {
	// 转换为字符串比较
	aStr := e.toString(a)
	bStr := e.toString(b)
	return strings.EqualFold(aStr, bStr)
}

// compare 数值比较
func (e *TagEngine) compare(a, b any) int {
	aNum := e.toFloat64(a)
	bNum := e.toFloat64(b)
	if aNum < bNum {
		return -1
	}
	if aNum > bNum {
		return 1
	}
	return 0
}

// contains 包含检查
func (e *TagEngine) contains(a, b any) bool {
	aStr := strings.ToLower(e.toString(a))
	bStr := strings.ToLower(e.toString(b))
	return strings.Contains(aStr, bStr)
}

// matchRegex 正则匹配
func (e *TagEngine) matchRegex(a, pattern any) bool {
	aStr := e.toString(a)
	patternStr := e.toString(pattern)
	matched, err := regexp.MatchString(patternStr, aStr)
	if err != nil {
		return false
	}
	return matched
}

// inList 列表包含
func (e *TagEngine) inList(a, list any) bool {
	aStr := strings.ToLower(e.toString(a))

	// 尝试解析列表
	switch v := list.(type) {
	case []any:
		for _, item := range v {
			if strings.EqualFold(e.toString(item), aStr) {
				return true
			}
		}
	case []string:
		for _, item := range v {
			if strings.EqualFold(item, aStr) {
				return true
			}
		}
	case string:
		// 以逗号分隔的字符串
		items := strings.Split(v, ",")
		for _, item := range items {
			if strings.EqualFold(strings.TrimSpace(item), aStr) {
				return true
			}
		}
	}
	return false
}

// toString 转换为字符串
func (e *TagEngine) toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32)
	case bool:
		return strconv.FormatBool(val)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

// toFloat64 转换为浮点数
func (e *TagEngine) toFloat64(v any) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case float32:
		return float64(val)
	case string:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}

// ApplyRulesAfterSpeedTest 测速后应用规则
func (e *TagEngine) ApplyRulesAfterSpeedTest(nodeIDs []uint) error {
	_, err := e.ApplyRules("speed_test", nodeIDs)
	return err
}

// ApplyRulesAfterSubscriptionUpdate 订阅更新后应用规则
func (e *TagEngine) ApplyRulesAfterSubscriptionUpdate(nodeIDs []uint) error {
	_, err := e.ApplyRules("subscription_update", nodeIDs)
	return err
}
