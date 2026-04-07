package component

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// EmotionConfig 单项情绪配置
type EmotionConfig struct {
	Name           string  `json:"name"`
	Value          float64 `json:"value"`
	AccumulateRate float64 `json:"accumulate_rate"`
	DecayRate      float64 `json:"decay_rate"`
}

// EmotionComponent 情绪组件（Tickable）
type EmotionComponent struct {
	EmotionStates []EmotionConfig `json:"emotion_states"`
}

func (c *EmotionComponent) Name() string { return "emotion" }

// Tick 情绪更新：有威胁记忆时恐惧累积，否则衰减。找最高情绪写 BB
func (c *EmotionComponent) Tick(bb *blackboard.Blackboard, dt float64) {
	// 读取威胁记忆值（由 memory.Tick 在本帧先写入）
	threatMemVal, _ := blackboard.Get(bb, blackboard.KeyMemoryThreatValue)

	dominantName := ""
	dominantVal := -1.0

	for i := range c.EmotionStates {
		e := &c.EmotionStates[i]
		if e.Name == "fear" && threatMemVal > 0 {
			// 有威胁记忆 → 恐惧累积
			e.Value += e.AccumulateRate * dt
		} else {
			// 无威胁记忆或非 fear 情绪 → 正常衰减
			e.Value = math.Max(0, e.Value-e.DecayRate*dt)
		}
		if e.Value > dominantVal {
			dominantVal = e.Value
			dominantName = e.Name
		}
	}

	if dominantName != "" {
		blackboard.Set(bb, blackboard.KeyEmotionDominant, dominantName)
		blackboard.Set(bb, blackboard.KeyEmotionDominantVal, dominantVal)
	}
}

// EmotionFactory 从 JSON 创建 EmotionComponent
func EmotionFactory(raw json.RawMessage) (Component, error) {
	var c EmotionComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("emotion: %w", err)
	}
	if len(c.EmotionStates) == 0 {
		return nil, fmt.Errorf("emotion: emotion_states must have at least one entry")
	}
	for i := range c.EmotionStates {
		e := &c.EmotionStates[i]
		if e.Name == "" {
			return nil, fmt.Errorf("emotion: emotion_states[%d].name is required", i)
		}
	}
	return &c, nil
}
