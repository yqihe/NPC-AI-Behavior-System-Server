package component_test

import (
	"encoding/json"
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
	if m.WanderRadius != 50 {
		t.Errorf("WanderRadius = %v, want 50", m.WanderRadius)
	}
}

func TestMovementFactory_PatrolMissingWaypoints(t *testing.T) {
	raw := json.RawMessage(`{"move_type":"patrol","move_speed":3.0}`)
	_, err := component.MovementFactory(raw)
	if err == nil {
		t.Fatal("expected error for patrol without waypoints")
	}
}

func TestMovementComponent_Tick(t *testing.T) {
	raw := json.RawMessage(`{"move_type":"wander","move_speed":3.0,"wander_radius":50}`)
	comp, err := component.MovementFactory(raw)
	if err != nil {
		t.Fatalf("MovementFactory failed: %v", err)
	}
	tickable := comp.(component.Tickable)
	board := blackboard.New()
	tickable.Tick(board, 0.1)

	state, ok := bb.Get(board, bb.KeyMoveState)
	if !ok {
		t.Fatal("move_state not set in BB")
	}
	if state != "idle" {
		t.Errorf("move_state = %q, want %q", state, "idle")
	}
}
