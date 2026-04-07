package component_test

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
)

func TestBehaviorFactory(t *testing.T) {
	raw := json.RawMessage(`{"fsm_ref": "wolf", "bt_refs": {"Idle": "wolf/idle", "Chase": "wolf/chase"}}`)
	comp, err := component.BehaviorFactory(raw)
	if err != nil {
		t.Fatalf("BehaviorFactory failed: %v", err)
	}
	if comp.Name() != "behavior" {
		t.Errorf("Name() = %q, want %q", comp.Name(), "behavior")
	}
	beh := comp.(*component.BehaviorComponent)
	if beh.FSMRef != "wolf" {
		t.Errorf("FSMRef = %q, want %q", beh.FSMRef, "wolf")
	}
	if len(beh.BTRefs) != 2 {
		t.Errorf("BTRefs len = %d, want 2", len(beh.BTRefs))
	}
	if beh.FSM != nil {
		t.Error("FSM should be nil before BuildRuntime")
	}
}

func TestBehaviorFactory_MissingFSMRef(t *testing.T) {
	raw := json.RawMessage(`{"bt_refs": {"Idle": "wolf/idle"}}`)
	_, err := component.BehaviorFactory(raw)
	if err == nil {
		t.Fatal("expected error for missing fsm_ref")
	}
}

func TestBehaviorFactory_EmptyBTRefs(t *testing.T) {
	raw := json.RawMessage(`{"fsm_ref": "wolf", "bt_refs": {}}`)
	_, err := component.BehaviorFactory(raw)
	if err == nil {
		t.Fatal("expected error for empty bt_refs")
	}
}
