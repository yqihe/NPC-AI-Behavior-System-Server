package component_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	bb "github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
)

func TestMovementFactory_Wander(t *testing.T) {
	raw := json.RawMessage(`{"move_type":"wander","move_speed":3.0,"wander_radius":50}`)
	comp, err := component.MovementFactory(raw)
	if err != nil {
		t.Fatalf("MovementFactory failed: %v", err)
	}
	m := comp.(*component.MovementComponent)
	if m.MoveType != "wander" {
		t.Errorf("MoveType = %q, want %q", m.MoveType, "wander")
	}
}

func TestMovementFactory_PatrolMissingWaypoints(t *testing.T) {
	raw := json.RawMessage(`{"move_type":"patrol","move_speed":3.0}`)
	_, err := component.MovementFactory(raw)
	if err == nil {
		t.Fatal("expected error for patrol without waypoints")
	}
}

func TestMovement_Wander_Moves(t *testing.T) {
	raw := json.RawMessage(`{"move_type":"wander","move_speed":10.0,"wander_radius":50}`)
	comp, _ := component.MovementFactory(raw)
	m := comp.(*component.MovementComponent)
	m.SetSpawn(100, 200)

	board := blackboard.New()
	blackboard.Set(board, bb.KeyNPCPosX, 100.0)
	blackboard.Set(board, bb.KeyNPCPosZ, 200.0)

	// 多次 Tick，位置应变化
	for i := 0; i < 10; i++ {
		m.Tick(board, 0.1)
	}

	x, _ := bb.Get(board, bb.KeyNPCPosX)
	z, _ := bb.Get(board, bb.KeyNPCPosZ)

	// 位置不应该还在 spawn 点（概率极低除非随机到原点）
	moved := math.Abs(x-100) > 0.01 || math.Abs(z-200) > 0.01
	if !moved {
		t.Error("NPC should have moved from spawn point after 10 ticks")
	}

	state, _ := bb.Get(board, bb.KeyMoveState)
	if state != "moving" && state != "arrived" {
		t.Errorf("move_state = %q, want moving or arrived", state)
	}
}

func TestMovement_Patrol_Cycles(t *testing.T) {
	raw := json.RawMessage(`{"move_type":"patrol","move_speed":100.0,"patrol_waypoints":[{"x":10,"z":0},{"x":20,"z":0},{"x":30,"z":0}]}`)
	comp, _ := component.MovementFactory(raw)
	m := comp.(*component.MovementComponent)

	board := blackboard.New()
	blackboard.Set(board, bb.KeyNPCPosX, 0.0)
	blackboard.Set(board, bb.KeyNPCPosZ, 0.0)

	// 快速到达每个路点（speed=100, dt=0.5 → step=50m >> 距离）
	arrivedCount := 0
	for i := 0; i < 20; i++ {
		m.Tick(board, 0.5)
		state, _ := bb.Get(board, bb.KeyMoveState)
		if state == "arrived" {
			arrivedCount++
		}
	}

	// 应至少到达 3 个路点（完成一个循环）
	if arrivedCount < 3 {
		t.Errorf("arrivedCount = %d, want >= 3 (one full cycle)", arrivedCount)
	}
}

func TestMovement_MoveToward(t *testing.T) {
	// 移动一半距离
	x, z := component.MoveToward(0, 0, 10, 0, 5)
	if math.Abs(x-5) > 0.01 || math.Abs(z) > 0.01 {
		t.Errorf("MoveToward(0,0 → 10,0, step=5) = (%f,%f), want (5,0)", x, z)
	}

	// 步长 > 距离 → 到达目标
	x2, z2 := component.MoveToward(0, 0, 3, 4, 100)
	if math.Abs(x2-3) > 0.01 || math.Abs(z2-4) > 0.01 {
		t.Errorf("MoveToward(0,0 → 3,4, step=100) = (%f,%f), want (3,4)", x2, z2)
	}

	// 零距离
	x3, z3 := component.MoveToward(5, 5, 5, 5, 10)
	if x3 != 5 || z3 != 5 {
		t.Errorf("MoveToward same point = (%f,%f), want (5,5)", x3, z3)
	}
}
