package component

import (
	"encoding/json"
	"fmt"
)

// DecisionWeights 决策中心对各维度的加权系数
type DecisionWeights struct {
	Threat  float64 `json:"threat"`
	Needs   float64 `json:"needs"`
	Emotion float64 `json:"emotion"`
}

// PersonalityComponent 性格组件，影响决策权重
type PersonalityComponent struct {
	PersonalityType string          `json:"personality_type"`
	DecisionWeights DecisionWeights `json:"decision_weights"`
	AggroRange      float64         `json:"aggro_range,omitempty"`
	FleeThreshold   float64         `json:"flee_threshold,omitempty"`
}

func (c *PersonalityComponent) Name() string { return "personality" }

// PersonalityFactory 从 JSON 创建 PersonalityComponent
func PersonalityFactory(raw json.RawMessage) (Component, error) {
	var c PersonalityComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("personality: %w", err)
	}
	if c.PersonalityType == "" {
		return nil, fmt.Errorf("personality: personality_type is required")
	}
	validTypes := map[string]bool{"timid": true, "aggressive": true, "docile": true, "curious": true}
	if !validTypes[c.PersonalityType] {
		return nil, fmt.Errorf("personality: unknown type %q", c.PersonalityType)
	}
	return &c, nil
}
