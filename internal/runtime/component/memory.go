package component

import (
	"encoding/json"
	"fmt"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// MemoryComponent 记忆组件（Tickable）
type MemoryComponent struct {
	Capacity    int      `json:"capacity"`
	MemoryTypes []string `json:"memory_types"`
	DecayTime   float64  `json:"decay_time"`
}

func (c *MemoryComponent) Name() string { return "memory" }

// Tick 最小实现：写 memory_count 到 BB。深入逻辑在需求 4。
func (c *MemoryComponent) Tick(bb *blackboard.Blackboard, dt float64) {
	blackboard.Set(bb, blackboard.KeyMemoryCount, int64(0))
}

// MemoryFactory 从 JSON 创建 MemoryComponent
func MemoryFactory(raw json.RawMessage) (Component, error) {
	var c MemoryComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("memory: %w", err)
	}
	if c.Capacity < 1 {
		return nil, fmt.Errorf("memory: capacity must be >= 1")
	}
	if len(c.MemoryTypes) == 0 {
		return nil, fmt.Errorf("memory: memory_types must have at least one entry")
	}
	if c.DecayTime <= 0 {
		return nil, fmt.Errorf("memory: decay_time must be > 0")
	}
	return &c, nil
}
