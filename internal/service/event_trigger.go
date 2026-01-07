package service

import (
	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/logger"
)

// EventTrigger 事件触发器
type EventTrigger struct {
	store         *database.Store
	taskManager   *TaskManager
	configBuilder func() error
	tagEngine     *TagEngine
}

// NewEventTrigger 创建事件触发器
func NewEventTrigger(store *database.Store, taskManager *TaskManager) *EventTrigger {
	return &EventTrigger{
		store:       store,
		taskManager: taskManager,
	}
}

// SetConfigBuilder 设置配置生成器
func (e *EventTrigger) SetConfigBuilder(builder func() error) {
	e.configBuilder = builder
}

// SetTagEngine 设置标签引擎
func (e *EventTrigger) SetTagEngine(engine *TagEngine) {
	e.tagEngine = engine
}

// OnSubscriptionUpdate 订阅更新后触发
func (e *EventTrigger) OnSubscriptionUpdate(subID string, nodeIDs []uint) {
	logger.Info("事件触发: 订阅更新 [%s], 节点数: %d", subID, len(nodeIDs))

	// 应用标签规则
	if e.tagEngine != nil && len(nodeIDs) > 0 {
		if err := e.tagEngine.ApplyRulesAfterSubscriptionUpdate(nodeIDs); err != nil {
			logger.Warn("应用标签规则失败: %v", err)
		}
	}

	// 重新生成配置
	e.rebuildConfig("订阅更新")
}

// OnNodeChange 节点变更后触发
func (e *EventTrigger) OnNodeChange(nodeID uint) {
	logger.Info("事件触发: 节点变更 [%d]", nodeID)
	e.rebuildConfig("节点变更")
}

// OnRuleChange 规则变更后触发
func (e *EventTrigger) OnRuleChange(ruleID string) {
	logger.Info("事件触发: 规则变更 [%s]", ruleID)
	e.rebuildConfig("规则变更")
}

// OnChainChange 链路变更后触发
func (e *EventTrigger) OnChainChange(chainID string) {
	logger.Info("事件触发: 链路变更 [%s]", chainID)
	e.rebuildConfig("链路变更")
}

// OnSpeedTestComplete 测速完成后触发
func (e *EventTrigger) OnSpeedTestComplete(nodeIDs []uint) {
	logger.Info("事件触发: 测速完成, 节点数: %d", len(nodeIDs))

	// 应用标签规则（测速触发类型）
	if e.tagEngine != nil && len(nodeIDs) > 0 {
		if err := e.tagEngine.ApplyRulesAfterSpeedTest(nodeIDs); err != nil {
			logger.Warn("应用标签规则失败: %v", err)
		}
	}
}

// rebuildConfig 重新生成配置
func (e *EventTrigger) rebuildConfig(reason string) {
	if e.configBuilder == nil {
		return
	}

	if err := e.configBuilder(); err != nil {
		logger.Error("重新生成配置失败 (%s): %v", reason, err)
	} else {
		logger.Info("配置已重新生成 (%s)", reason)
	}
}
