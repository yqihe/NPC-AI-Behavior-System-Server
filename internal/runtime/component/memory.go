package component

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// MemoryEntry 一条记忆
type MemoryEntry struct {
	Type      string  // "threat" / "location" / "social"
	TargetID  string  // 记忆目标标识
	Value     float64 // 记忆强度
	Timestamp int64   // 创建/最后强化的时间戳（毫秒）
	TTL       float64 // 剩余生存时间（秒）
}

// MemoryComponent 记忆组件（Tickable）
type MemoryComponent struct {
	Capacity    int      `json:"capacity"`
	MemoryTypes []string `json:"memory_types"`
	DecayTime   float64  `json:"decay_time"`

	entries []MemoryEntry // 运行时状态
}

func (c *MemoryComponent) Name() string { return "memory" }

// SupportsType 检查是否支持指定记忆类型
func (c *MemoryComponent) SupportsType(t string) bool {
	for _, mt := range c.MemoryTypes {
		if mt == t {
			return true
		}
	}
	return false
}

// AddMemory 写入记忆。同 Type+TargetID 已存在则强化（Value 取 max，TTL 重置），否则新增。满时淘汰最旧。
func (c *MemoryComponent) AddMemory(entry MemoryEntry) {
	// 1. 查找已有记忆 → 强化
	for i := range c.entries {
		if c.entries[i].Type == entry.Type && c.entries[i].TargetID == entry.TargetID {
			if entry.Value > c.entries[i].Value {
				c.entries[i].Value = entry.Value
			}
			c.entries[i].TTL = c.DecayTime
			c.entries[i].Timestamp = entry.Timestamp
			slog.Debug("memory.reinforced", "type", entry.Type, "target", entry.TargetID, "value", c.entries[i].Value)
			return
		}
	}

	// 2. 容量检查 → 淘汰最旧
	if len(c.entries) >= c.Capacity {
		oldestIdx := 0
		for i := 1; i < len(c.entries); i++ {
			if c.entries[i].Timestamp < c.entries[oldestIdx].Timestamp {
				oldestIdx = i
			}
		}
		slog.Debug("memory.evicted", "type", c.entries[oldestIdx].Type, "target", c.entries[oldestIdx].TargetID)
		c.entries[oldestIdx] = entry
		return
	}

	// 3. 新增
	c.entries = append(c.entries, entry)
	slog.Debug("memory.added", "type", entry.Type, "target", entry.TargetID, "value", entry.Value)
}

// GetMemories 按类型查询所有记忆
func (c *MemoryComponent) GetMemories(memType string) []MemoryEntry {
	var result []MemoryEntry
	for _, e := range c.entries {
		if e.Type == memType {
			result = append(result, e)
		}
	}
	return result
}

// HasMemory 检查是否存在指定类型和目标的记忆
func (c *MemoryComponent) HasMemory(memType, targetID string) bool {
	for _, e := range c.entries {
		if e.Type == memType && e.TargetID == targetID {
			return true
		}
	}
	return false
}

// GetMemory 获取指定类型和目标的记忆
func (c *MemoryComponent) GetMemory(memType, targetID string) (MemoryEntry, bool) {
	for _, e := range c.entries {
		if e.Type == memType && e.TargetID == targetID {
			return e, true
		}
	}
	return MemoryEntry{}, false
}

// Count 当前记忆条目数
func (c *MemoryComponent) Count() int {
	return len(c.entries)
}

// Tick 清理过期记忆，更新 BB
func (c *MemoryComponent) Tick(bb *blackboard.Blackboard, dt float64) {
	alive := c.entries[:0]
	for i := range c.entries {
		c.entries[i].TTL -= dt
		if c.entries[i].TTL > 0 {
			alive = append(alive, c.entries[i])
		}
	}
	c.entries = alive

	// 写 BB
	blackboard.Set(bb, blackboard.KeyMemoryCount, int64(len(c.entries)))

	// 写最高威胁记忆 value
	maxThreatVal := 0.0
	for _, e := range c.entries {
		if e.Type == "threat" && e.Value > maxThreatVal {
			maxThreatVal = e.Value
		}
	}
	blackboard.Set(bb, blackboard.KeyMemoryThreatValue, maxThreatVal)
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
	c.entries = make([]MemoryEntry, 0)
	return &c, nil
}
