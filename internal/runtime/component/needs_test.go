package component_test

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
)

func TestNeedsFactory(t *testing.T) {
	raw := json.RawMessage(`{"need_types":[{"name":"hunger","max":100,"decay_rate":5}]}`)
	comp, err := component.NeedsFactory(raw)
	if err != nil {
		t.Fatalf("NeedsFactory failed: %v", err)
	}
	n := comp.(*component.NeedsComponent)
	if len(n.NeedTypes) != 1 {
		t.Fatalf("NeedTypes len = %d, want 1", len(n.NeedTypes))
	}
	// current 未指定，应默认为 max
	if n.NeedTypes[0].Current != 100 {
		t.Errorf("Current = %v, want 100 (default to max)", n.NeedTypes[0].Current)
	}
}

func TestNeedsComponent_Tick(t *testing.T) {
	raw := json.RawMessage(`{"need_types":[
		{"name":"hunger","current":80,"max":100,"decay_rate":10},
		{"name":"fatigue","current":90,"max":100,"decay_rate":5}
	]}`)
	comp, err := component.NeedsFactory(raw)
	if err != nil {
		t.Fatalf("NeedsFactory failed: %v", err)
	}
	tickable := comp.(component.Tickable)
	board := blackboard.New()

	// dt=1.0 → hunger: 80-10=70, fatigue: 90-5=85 → lowest=hunger(70)
	tickable.Tick(board, 1.0)

	name, ok := blackboard.Get(board, blackboard.KeyNeedLowest)
	if !ok {
		t.Fatal("need_lowest not set")
	}
	if name != "hunger" {
		t.Errorf("need_lowest = %q, want %q", name, "hunger")
	}
	val, ok := blackboard.Get(board, blackboard.KeyNeedLowestVal)
	if !ok {
		t.Fatal("need_lowest_val not set")
	}
	if val != 70 {
		t.Errorf("need_lowest_val = %v, want 70", val)
	}
}

func TestNeedsComponent_DecayClampZero(t *testing.T) {
	raw := json.RawMessage(`{"need_types":[{"name":"hunger","current":3,"max":100,"decay_rate":10}]}`)
	comp, _ := component.NeedsFactory(raw)
	tickable := comp.(component.Tickable)
	board := blackboard.New()

	// dt=1.0 → 3-10 = clamp 0
	tickable.Tick(board, 1.0)

	val, _ := blackboard.Get(board, blackboard.KeyNeedLowestVal)
	if val != 0 {
		t.Errorf("need_lowest_val = %v, want 0 (clamped)", val)
	}
}
