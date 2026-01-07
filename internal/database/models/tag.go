package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Tag 标签表
type Tag struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string `gorm:"uniqueIndex;not null;type:text" json:"name"` // 标签名称
	Color       string `gorm:"default:'#1976d2';type:text" json:"color"`   // 颜色
	TagGroup    string `gorm:"type:text" json:"tag_group,omitempty"`       // 互斥组名称
	Description string `gorm:"type:text" json:"description,omitempty"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	// 关联
	Nodes []Node `gorm:"many2many:node_tags;" json:"nodes,omitempty"`
	Rules []TagRule `gorm:"foreignKey:TagID" json:"rules,omitempty"`
}

func (Tag) TableName() string {
	return "tags"
}

// TagConditions 标签条件结构
type TagConditions struct {
	Logic      string         `json:"logic"`      // AND / OR
	Conditions []TagCondition `json:"conditions"`
}

// TagCondition 单个条件
type TagCondition struct {
	Field    string      `json:"field"`    // delay / speed / country / type / name / source
	Operator string      `json:"operator"` // eq / ne / gt / lt / gte / lte / contains / regex / in
	Value    interface{} `json:"value"`
}

// Scan 实现 sql.Scanner 接口
func (c *TagConditions) Scan(value interface{}) error {
	if value == nil {
		*c = TagConditions{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		str, ok := value.(string)
		if ok {
			bytes = []byte(str)
		} else {
			return nil
		}
	}
	return json.Unmarshal(bytes, c)
}

// Value 实现 driver.Valuer 接口
func (c TagConditions) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// TagRule 标签规则表
type TagRule struct {
	ID          uint          `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string        `gorm:"not null;type:text" json:"name"`
	TagID       uint          `gorm:"not null;index" json:"tag_id"`
	Enabled     bool          `gorm:"default:true" json:"enabled"`
	TriggerType string        `gorm:"not null;type:text" json:"trigger_type"` // subscription_update / speed_test / manual
	Conditions  TagConditions `gorm:"type:text" json:"conditions"`            // JSON: TagConditions

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	// 关联
	Tag *Tag `gorm:"foreignKey:TagID" json:"tag,omitempty"`
}

func (TagRule) TableName() string {
	return "tag_rules"
}
