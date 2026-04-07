package component

import (
	"encoding/json"
	"fmt"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

// PositionComponent 位置组件，所有 NPC 必有
type PositionComponent struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Z           float64 `json:"z"`
	Orientation float64 `json:"orientation"`
	ZoneID      string  `json:"zone_id"`
}

func (c *PositionComponent) Name() string { return "position" }

// ToVec3 转换为 event.Vec3，供 Scheduler 和广播使用
func (c *PositionComponent) ToVec3() event.Vec3 {
	return event.Vec3{X: c.X, Y: c.Y, Z: c.Z}
}

// PositionFactory 从 JSON 创建 PositionComponent
func PositionFactory(raw json.RawMessage) (Component, error) {
	var c PositionComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("position: %w", err)
	}
	return &c, nil
}
