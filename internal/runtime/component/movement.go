package component

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"

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

	// 运行时状态
	spawnX, spawnZ   float64
	targetX, targetZ float64
	hasTarget        bool
	waitTimer        float64
	patrolIndex      int
}

func (c *MovementComponent) Name() string { return "movement" }

// SetSpawn 记录 wander 原点（创建时调用）
func (c *MovementComponent) SetSpawn(x, z float64) {
	c.spawnX = x
	c.spawnZ = z
}

// Tick 执行移动逻辑
func (c *MovementComponent) Tick(bb *blackboard.Blackboard, dt float64) {
	posX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	posZ, _ := blackboard.Get(bb, blackboard.KeyNPCPosZ)

	switch c.MoveType {
	case "wander":
		c.tickWander(bb, posX, posZ, dt)
	case "patrol":
		c.tickPatrol(bb, posX, posZ, dt)
	case "follow":
		c.tickFollow(bb, posX, posZ, dt)
	default:
		blackboard.Set(bb, blackboard.KeyMoveState, "idle")
	}
}

func (c *MovementComponent) tickWander(bb *blackboard.Blackboard, posX, posZ, dt float64) {
	// 等待中
	if c.waitTimer > 0 {
		c.waitTimer -= dt
		blackboard.Set(bb, blackboard.KeyMoveState, "arrived")
		return
	}

	// 选目标
	if !c.hasTarget {
		angle := rand.Float64() * 2 * math.Pi
		dist := rand.Float64() * c.WanderRadius
		c.targetX = c.spawnX + math.Cos(angle)*dist
		c.targetZ = c.spawnZ + math.Sin(angle)*dist
		c.hasTarget = true
	}

	// 移动
	dist := Distance2D(posX, posZ, c.targetX, c.targetZ)
	if dist < 1.0 {
		c.hasTarget = false
		c.waitTimer = 1.0 + rand.Float64()*2.0
		blackboard.Set(bb, blackboard.KeyMoveState, "arrived")
		return
	}

	maxStep := c.MoveSpeed * dt
	newX, newZ := MoveToward(posX, posZ, c.targetX, c.targetZ, maxStep)
	c.writePosition(bb, newX, newZ, "moving")
}

func (c *MovementComponent) tickPatrol(bb *blackboard.Blackboard, posX, posZ, dt float64) {
	if len(c.PatrolWaypoints) == 0 {
		blackboard.Set(bb, blackboard.KeyMoveState, "idle")
		return
	}

	target := c.PatrolWaypoints[c.patrolIndex]
	dist := Distance2D(posX, posZ, target.X, target.Z)

	if dist < 1.0 {
		c.patrolIndex = (c.patrolIndex + 1) % len(c.PatrolWaypoints)
		blackboard.Set(bb, blackboard.KeyMoveState, "arrived")
		return
	}

	maxStep := c.MoveSpeed * dt
	newX, newZ := MoveToward(posX, posZ, target.X, target.Z, maxStep)
	c.writePosition(bb, newX, newZ, "moving")
}

func (c *MovementComponent) tickFollow(bb *blackboard.Blackboard, posX, posZ, dt float64) {
	targetX, okX := blackboard.Get(bb, blackboard.KeyFollowTargetX)
	targetZ, okZ := blackboard.Get(bb, blackboard.KeyFollowTargetZ)
	if !okX || !okZ {
		blackboard.Set(bb, blackboard.KeyMoveState, "idle")
		return
	}
	dist := Distance2D(posX, posZ, targetX, targetZ)
	if dist < 2.0 {
		blackboard.Set(bb, blackboard.KeyMoveState, "arrived")
		return
	}
	maxStep := c.MoveSpeed * dt
	newX, newZ := MoveToward(posX, posZ, targetX, targetZ, maxStep)
	c.writePosition(bb, newX, newZ, "moving")
}

func (c *MovementComponent) writePosition(bb *blackboard.Blackboard, x, z float64, state string) {
	blackboard.Set(bb, blackboard.KeyNPCPosX, x)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, z)
	blackboard.Set(bb, blackboard.KeyMoveTargetX, c.targetX)
	blackboard.Set(bb, blackboard.KeyMoveTargetZ, c.targetZ)
	blackboard.Set(bb, blackboard.KeyMoveState, state)
}

// MoveToward 朝目标点移动最多 maxDist 距离。导出供 BT 节点使用。
func MoveToward(posX, posZ, targetX, targetZ, maxDist float64) (float64, float64) {
	dx := targetX - posX
	dz := targetZ - posZ
	dist := math.Sqrt(dx*dx + dz*dz)
	if dist == 0 || dist <= maxDist {
		return targetX, targetZ
	}
	ratio := maxDist / dist
	return posX + dx*ratio, posZ + dz*ratio
}

// Distance2D 计算 XZ 平面距离。导出供 BT 节点使用。
func Distance2D(x1, z1, x2, z2 float64) float64 {
	dx := x2 - x1
	dz := z2 - z1
	return math.Sqrt(dx*dx + dz*dz)
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
