package bt

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// --- move_to ---

type moveTo struct {
	targetKeyX string
	targetKeyZ string
	speed      float64
}

func (n *moveTo) Tick(ctx *Context) Status {
	// 读取目标坐标
	rawX, okX := ctx.BB.GetRaw(n.targetKeyX)
	rawZ, okZ := ctx.BB.GetRaw(n.targetKeyZ)
	if !okX || !okZ {
		return Failure
	}
	targetX, isNum := toFloat64(rawX)
	if !isNum {
		return Failure
	}
	targetZ, isNum := toFloat64(rawZ)
	if !isNum {
		return Failure
	}

	npcX, _ := blackboard.Get(ctx.BB, blackboard.KeyNPCPosX)
	npcZ, _ := blackboard.Get(ctx.BB, blackboard.KeyNPCPosZ)

	dist := math.Sqrt((targetX-npcX)*(targetX-npcX) + (targetZ-npcZ)*(targetZ-npcZ))
	if dist < 1.0 {
		blackboard.Set(ctx.BB, blackboard.KeyMoveState, "arrived")
		return Success
	}

	dt := ctx.DeltaTime
	if dt <= 0 {
		dt = 0.1
	}
	maxStep := n.speed * dt
	newX, newZ := btMoveToward(npcX, npcZ, targetX, targetZ, maxStep)
	blackboard.Set(ctx.BB, blackboard.KeyNPCPosX, newX)
	blackboard.Set(ctx.BB, blackboard.KeyNPCPosZ, newZ)
	blackboard.Set(ctx.BB, blackboard.KeyMoveState, "moving")
	return Running
}

func moveToFactory(params json.RawMessage) (Node, error) {
	var cfg struct {
		TargetKeyX string  `json:"target_key_x"`
		TargetKeyZ string  `json:"target_key_z"`
		Speed      float64 `json:"speed"`
	}
	if err := json.Unmarshal(params, &cfg); err != nil {
		return nil, fmt.Errorf("move_to: %w", err)
	}
	if cfg.TargetKeyX == "" || cfg.TargetKeyZ == "" {
		return nil, fmt.Errorf("move_to: target_key_x and target_key_z are required")
	}
	if cfg.Speed <= 0 {
		cfg.Speed = 3.0
	}
	return &moveTo{targetKeyX: cfg.TargetKeyX, targetKeyZ: cfg.TargetKeyZ, speed: cfg.Speed}, nil
}

// --- flee_from ---

type fleeFrom struct {
	sourceKeyX string
	sourceKeyZ string
	distance   float64
	speed      float64
}

func (n *fleeFrom) Tick(ctx *Context) Status {
	// 读取威胁源坐标
	rawX, okX := ctx.BB.GetRaw(n.sourceKeyX)
	rawZ, okZ := ctx.BB.GetRaw(n.sourceKeyZ)
	if !okX || !okZ {
		return Failure
	}
	srcX, isNum := toFloat64(rawX)
	if !isNum {
		return Failure
	}
	srcZ, isNum := toFloat64(rawZ)
	if !isNum {
		return Failure
	}

	npcX, _ := blackboard.Get(ctx.BB, blackboard.KeyNPCPosX)
	npcZ, _ := blackboard.Get(ctx.BB, blackboard.KeyNPCPosZ)

	dist := math.Sqrt((srcX-npcX)*(srcX-npcX) + (srcZ-npcZ)*(srcZ-npcZ))
	if dist >= n.distance {
		blackboard.Set(ctx.BB, blackboard.KeyMoveState, "arrived")
		return Success
	}

	// 计算反方向
	dx := npcX - srcX
	dz := npcZ - srcZ
	if dx == 0 && dz == 0 {
		dx = 1 // 重合时默认向 X 正方向逃
	}
	norm := math.Sqrt(dx*dx + dz*dz)
	dx /= norm
	dz /= norm

	dt := ctx.DeltaTime
	if dt <= 0 {
		dt = 0.1
	}
	step := n.speed * dt
	newX := npcX + dx*step
	newZ := npcZ + dz*step

	blackboard.Set(ctx.BB, blackboard.KeyNPCPosX, newX)
	blackboard.Set(ctx.BB, blackboard.KeyNPCPosZ, newZ)
	blackboard.Set(ctx.BB, blackboard.KeyMoveState, "moving")
	return Running
}

func fleeFromFactory(params json.RawMessage) (Node, error) {
	var cfg struct {
		SourceKeyX string  `json:"source_key_x"`
		SourceKeyZ string  `json:"source_key_z"`
		Distance   float64 `json:"distance"`
		Speed      float64 `json:"speed"`
	}
	if err := json.Unmarshal(params, &cfg); err != nil {
		return nil, fmt.Errorf("flee_from: %w", err)
	}
	if cfg.SourceKeyX == "" || cfg.SourceKeyZ == "" {
		return nil, fmt.Errorf("flee_from: source_key_x and source_key_z are required")
	}
	if cfg.Distance <= 0 {
		cfg.Distance = 100
	}
	if cfg.Speed <= 0 {
		cfg.Speed = 5.0
	}
	return &fleeFrom{sourceKeyX: cfg.SourceKeyX, sourceKeyZ: cfg.SourceKeyZ, distance: cfg.Distance, speed: cfg.Speed}, nil
}

// btMoveToward 朝目标点移动（BT 内部用，避免 import runtime/component）
func btMoveToward(posX, posZ, targetX, targetZ, maxDist float64) (float64, float64) {
	dx := targetX - posX
	dz := targetZ - posZ
	dist := math.Sqrt(dx*dx + dz*dz)
	if dist == 0 || dist <= maxDist {
		return targetX, targetZ
	}
	ratio := maxDist / dist
	return posX + dx*ratio, posZ + dz*ratio
}
