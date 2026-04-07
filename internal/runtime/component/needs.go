package component

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// NeedConfig 单项需求配置
type NeedConfig struct {
	Name      string  `json:"name"`
	Current   float64 `json:"current"`
	Max       float64 `json:"max"`
	DecayRate float64 `json:"decay_rate"`
}

// NeedsComponent 需求组件（Tickable）
type NeedsComponent struct {
	NeedTypes []NeedConfig `json:"need_types"`
}

func (c *NeedsComponent) Name() string { return "needs" }

// Tick 每项 current 按 decay_rate 衰减，找最低需求写 BB
func (c *NeedsComponent) Tick(bb *blackboard.Blackboard, dt float64) {
	lowestName := ""
	lowestVal := math.MaxFloat64

	for i := range c.NeedTypes {
		n := &c.NeedTypes[i]
		n.Current = math.Max(0, n.Current-n.DecayRate*dt)
		if n.Current < lowestVal {
			lowestVal = n.Current
			lowestName = n.Name
		}
	}

	if lowestName != "" {
		blackboard.Set(bb, blackboard.KeyNeedLowest, lowestName)
		blackboard.Set(bb, blackboard.KeyNeedLowestVal, lowestVal)
	}
}

// NeedsFactory 从 JSON 创建 NeedsComponent
func NeedsFactory(raw json.RawMessage) (Component, error) {
	var c NeedsComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("needs: %w", err)
	}
	if len(c.NeedTypes) == 0 {
		return nil, fmt.Errorf("needs: need_types must have at least one entry")
	}
	for i := range c.NeedTypes {
		n := &c.NeedTypes[i]
		if n.Name == "" {
			return nil, fmt.Errorf("needs: need_types[%d].name is required", i)
		}
		if n.Max <= 0 {
			return nil, fmt.Errorf("needs: need_types[%d].max must be > 0", i)
		}
		// current 未指定时默认等于 max
		if n.Current == 0 {
			n.Current = n.Max
		}
	}
	return &c, nil
}
