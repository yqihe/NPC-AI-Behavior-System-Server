package component

import (
	"encoding/json"
	"fmt"
)

// PerceptionComponent 感知组件
type PerceptionComponent struct {
	VisualRange       float64 `json:"visual_range"`
	AuditoryRange     float64 `json:"auditory_range"`
	AttentionCapacity int     `json:"attention_capacity"`
}

func (c *PerceptionComponent) Name() string { return "perception" }

// PerceptionFactory 从 JSON 创建 PerceptionComponent
func PerceptionFactory(raw json.RawMessage) (Component, error) {
	c := PerceptionComponent{
		AttentionCapacity: 5, // 默认值
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("perception: %w", err)
	}
	if c.VisualRange < 0 {
		return nil, fmt.Errorf("perception: visual_range must be >= 0")
	}
	if c.AuditoryRange < 0 {
		return nil, fmt.Errorf("perception: auditory_range must be >= 0")
	}
	if c.AttentionCapacity < 1 {
		return nil, fmt.Errorf("perception: attention_capacity must be >= 1")
	}
	return &c, nil
}
