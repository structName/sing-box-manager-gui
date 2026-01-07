package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/xiaobei/singbox-manager/internal/database"
	"github.com/xiaobei/singbox-manager/internal/database/models"
	"github.com/xiaobei/singbox-manager/internal/service"
)

// TagHandler 标签 API 处理器
type TagHandler struct {
	store     *database.Store
	tagEngine *service.TagEngine
}

// NewTagHandler 创建标签处理器
func NewTagHandler(store *database.Store, tagEngine *service.TagEngine) *TagHandler {
	return &TagHandler{
		store:     store,
		tagEngine: tagEngine,
	}
}

// ==================== 标签 API ====================

// GetTags 获取所有标签
func (h *TagHandler) GetTags(c *gin.Context) {
	tags, err := h.store.GetTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// GetTag 获取单个标签
func (h *TagHandler) GetTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	tag, err := h.store.GetTag(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		return
	}
	c.JSON(http.StatusOK, tag)
}

// CreateTagRequest 创建标签请求
type CreateTagRequest struct {
	Name        string `json:"name" binding:"required"`
	Color       string `json:"color"`
	TagGroup    string `json:"tag_group"`
	Description string `json:"description"`
}

// CreateTag 创建标签
func (h *TagHandler) CreateTag(c *gin.Context) {
	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 检查名称是否已存在
	if existing, _ := h.store.GetTagByName(req.Name); existing != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签名称已存在"})
		return
	}

	tag := &models.Tag{
		Name:        req.Name,
		Color:       req.Color,
		TagGroup:    req.TagGroup,
		Description: req.Description,
	}

	// 设置默认颜色
	if tag.Color == "" {
		tag.Color = "#1976d2"
	}

	if err := h.store.CreateTag(tag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tag)
}

// UpdateTag 更新标签
func (h *TagHandler) UpdateTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	tag, err := h.store.GetTag(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		return
	}

	var req CreateTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 如果名称变更，检查新名称是否已存在
	if req.Name != tag.Name {
		if existing, _ := h.store.GetTagByName(req.Name); existing != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "标签名称已存在"})
			return
		}
	}

	tag.Name = req.Name
	tag.Color = req.Color
	tag.TagGroup = req.TagGroup
	tag.Description = req.Description

	if err := h.store.UpdateTag(tag); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tag)
}

// DeleteTag 删除标签
func (h *TagHandler) DeleteTag(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if _, err := h.store.GetTag(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "标签不存在"})
		return
	}

	if err := h.store.DeleteTag(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetTagGroups 获取所有标签组
func (h *TagHandler) GetTagGroups(c *gin.Context) {
	groups, err := h.store.GetTagGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, groups)
}

// ==================== 标签规则 API ====================

// GetTagRules 获取所有标签规则
func (h *TagHandler) GetTagRules(c *gin.Context) {
	rules, err := h.store.GetTagRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

// GetTagRule 获取单个标签规则
func (h *TagHandler) GetTagRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	rule, err := h.store.GetTagRule(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}
	c.JSON(http.StatusOK, rule)
}

// CreateTagRuleRequest 创建标签规则请求
type CreateTagRuleRequest struct {
	Name        string               `json:"name" binding:"required"`
	TagID       uint                 `json:"tag_id" binding:"required"`
	Enabled     bool                 `json:"enabled"`
	TriggerType string               `json:"trigger_type" binding:"required"` // speed_test / subscription_update / manual
	Conditions  models.TagConditions `json:"conditions"`
}

// CreateTagRule 创建标签规则
func (h *TagHandler) CreateTagRule(c *gin.Context) {
	var req CreateTagRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证标签存在
	if _, err := h.store.GetTag(req.TagID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签不存在"})
		return
	}

	// 验证触发类型
	validTriggers := map[string]bool{
		"speed_test":          true,
		"subscription_update": true,
		"manual":              true,
	}
	if !validTriggers[req.TriggerType] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的触发类型"})
		return
	}

	rule := &models.TagRule{
		Name:        req.Name,
		TagID:       req.TagID,
		Enabled:     req.Enabled,
		TriggerType: req.TriggerType,
		Conditions:  req.Conditions,
	}

	if err := h.store.CreateTagRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

// UpdateTagRule 更新标签规则
func (h *TagHandler) UpdateTagRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	rule, err := h.store.GetTagRule(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}

	var req CreateTagRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule.Name = req.Name
	rule.TagID = req.TagID
	rule.Enabled = req.Enabled
	rule.TriggerType = req.TriggerType
	rule.Conditions = req.Conditions

	if err := h.store.UpdateTagRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// DeleteTagRule 删除标签规则
func (h *TagHandler) DeleteTagRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 ID"})
		return
	}

	if _, err := h.store.GetTagRule(uint(id)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "规则不存在"})
		return
	}

	if err := h.store.DeleteTagRule(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ==================== 节点标签 API ====================

// GetNodeTags 获取节点的标签
func (h *TagHandler) GetNodeTags(c *gin.Context) {
	nodeIDStr := c.Param("nodeId")
	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	tags, err := h.store.GetNodeTags(uint(nodeID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}
	c.JSON(http.StatusOK, tags)
}

// SetNodeTagsRequest 设置节点标签请求
type SetNodeTagsRequest struct {
	TagIDs []uint `json:"tag_ids"`
}

// SetNodeTags 设置节点标签
func (h *TagHandler) SetNodeTags(c *gin.Context) {
	nodeIDStr := c.Param("nodeId")
	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var req SetNodeTagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.store.SetNodeTags(uint(nodeID), req.TagIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "设置成功"})
}

// AddNodeTagRequest 添加节点标签请求
type AddNodeTagRequest struct {
	TagID uint `json:"tag_id" binding:"required"`
}

// AddNodeTag 为节点添加标签
func (h *TagHandler) AddNodeTag(c *gin.Context) {
	nodeIDStr := c.Param("nodeId")
	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	var req AddNodeTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 处理互斥标签
	tag, err := h.store.GetTag(req.TagID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "标签不存在"})
		return
	}

	// 如果标签属于某个组，移除同组的其他标签
	if tag.TagGroup != "" {
		sameGroupTags, _ := h.store.GetTagsByGroup(tag.TagGroup)
		for _, t := range sameGroupTags {
			if t.ID != req.TagID {
				h.store.RemoveNodeTag(uint(nodeID), t.ID)
			}
		}
	}

	if err := h.store.AddNodeTag(uint(nodeID), req.TagID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "添加成功"})
}

// RemoveNodeTag 移除节点标签
func (h *TagHandler) RemoveNodeTag(c *gin.Context) {
	nodeIDStr := c.Param("nodeId")
	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点 ID"})
		return
	}

	tagIDStr := c.Param("tagId")
	tagID, err := strconv.ParseUint(tagIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的标签 ID"})
		return
	}

	if err := h.store.RemoveNodeTag(uint(nodeID), uint(tagID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "移除成功"})
}

// ==================== 规则执行 API ====================

// ApplyTagRulesRequest 应用标签规则请求
type ApplyTagRulesRequest struct {
	TriggerType string `json:"trigger_type" binding:"required"` // speed_test / subscription_update / manual
	NodeIDs     []uint `json:"node_ids,omitempty"`              // 可选，指定节点
}

// ApplyTagRules 手动应用标签规则
func (h *TagHandler) ApplyTagRules(c *gin.Context) {
	var req ApplyTagRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.tagEngine.ApplyRules(req.TriggerType, req.NodeIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
