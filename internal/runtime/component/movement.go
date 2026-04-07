package component

import (
	"encoding/json"
	"fmt"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// Waypoint 巡逻路点
type Waypoint struct {
	X float64 `json:"x"`
	Z float64 `json:"z"`
}

// MovementComponent 移动组件（Tickable）
type MovementComponent struct {
	MoveType        string     `json:"move_type"`
	MoveSpeed       float64    `json:"move_speed"`
	WanderRadius    float64    `json:"wander_radius,omitempty"`
	PatrolWaypoints []Waypoint `json:"patrol_waypoints,omitempty"`
}

func (c *MovementComponent) Name() string { return "movement" }

// Tick 最小实现：写 move_state 到 BB。真实移动逻辑在需求 5。
func (c *MovementComponent) Tick(bb *blackboard.Blackboard, dt float64) {
	blackboard.Set(bb, blackboard.KeyMoveState, "idle")
}

// MovementFactory 从 JSON 创建 MovementComponent
func MovementFactory(raw json.RawMessage) (Component, error) {
	var c MovementComponent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("movement: %w", err)
	}
	if c.MoveType == "" {
		return nil, fmt.Errorf("movement: move_type is required")
	}
	validTypes := map[string]bool{"wander": true, "patrol": true, "follow": true}
	if !validTypes[c.MoveType] {
		return nil, fmt.Errorf("movement: unknown move_type %q", c.MoveType)
	}
	if c.MoveSpeed < 0 {
		return nil, fmt.Errorf("movement: move_speed must be >= 0")
	}
	if c.MoveType == "wander" && c.WanderRadius <= 0 {
		return nil, fmt.Errorf("movement: wander_radius is required for wander mode")
	}
	if c.MoveType == "patrol" && len(c.PatrolWaypoints) == 0 {
		return nil, fmt.Errorf("movement: patrol_waypoints is required for patrol mode")
	}
	return &c, nil
}
