package bt_test

import (
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
)

func defaultReg() *bt.Registry {
	return bt.DefaultRegistry()
}

// --- Registry ---

func TestRegistry_UnknownType(t *testing.T) {
	reg := defaultReg()
	_, err := reg.Get("nonexistent_node_xyz")
	if err == nil {
		t.Fatal("expected error for unknown node type")
	}
}

// --- Builder ---

func TestBuildFromJSON_SimpleSequence(t *testing.T) {
	jsonData := []byte(`{
		"type": "sequence",
		"children": [
			{"type": "check_bb_float", "params": {"key": "threat_level", "op": ">=", "value": 50}},
			{"type": "check_bb_string", "params": {"key": "last_event_type", "op": "==", "value": "E01"}}
		]
	}`)

	reg := defaultReg()
	node, err := bt.BuildFromJSON(jsonData, reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 75.0)
	blackboard.Set(bb, blackboard.KeyLastEventType, "E01")

	ctx := &bt.Context{BB: bb}
	if node.Tick(ctx) != bt.Success {
		t.Fatal("expected Success")
	}
}

func TestBuildFromJSON_FailingCondition(t *testing.T) {
	jsonData := []byte(`{
		"type": "check_bb_float",
		"params": {"key": "threat_level", "op": ">=", "value": 90}
	}`)

	reg := defaultReg()
	node, err := bt.BuildFromJSON(jsonData, reg)
	if err != nil {
		t.Fatal(err)
	}

	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)

	if node.Tick(&bt.Context{BB: bb}) != bt.Failure {
		t.Fatal("expected Failure for threat_level 50 >= 90")
	}
}

func TestBuildFromJSON_Inverter(t *testing.T) {
	jsonData := []byte(`{
		"type": "inverter",
		"child": {
			"type": "check_bb_float",
			"params": {"key": "threat_level", "op": "<", "value": 10}
		}
	}`)

	reg := defaultReg()
	node, err := bt.BuildFromJSON(jsonData, reg)
	if err != nil {
		t.Fatal(err)
	}

	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)

	// threat_level(50) < 10 → Failure → Inverter → Success
	if node.Tick(&bt.Context{BB: bb}) != bt.Success {
		t.Fatal("expected Success (inverted Failure)")
	}
}

func TestBuildFromJSON_SetBBValue(t *testing.T) {
	jsonData := []byte(`{
		"type": "sequence",
		"children": [
			{"type": "set_bb_value", "params": {"key": "threat_level", "value": 99.9}},
			{"type": "check_bb_float", "params": {"key": "threat_level", "op": ">=", "value": 99}}
		]
	}`)

	reg := defaultReg()
	node, err := bt.BuildFromJSON(jsonData, reg)
	if err != nil {
		t.Fatal(err)
	}

	bb := blackboard.New()
	ctx := &bt.Context{BB: bb}
	if node.Tick(ctx) != bt.Success {
		t.Fatal("expected Success after set_bb_value + check")
	}

	val, ok := bb.GetRaw("threat_level")
	if !ok {
		t.Fatal("expected threat_level to be set")
	}
	if val.(float64) != 99.9 {
		t.Fatalf("expected 99.9, got %v", val)
	}
}

func TestBuildFromJSON_Parallel(t *testing.T) {
	jsonData := []byte(`{
		"type": "parallel",
		"params": {"policy": "require_one"},
		"children": [
			{"type": "check_bb_float", "params": {"key": "threat_level", "op": ">=", "value": 90}},
			{"type": "check_bb_string", "params": {"key": "last_event_type", "op": "==", "value": "E01"}}
		]
	}`)

	reg := defaultReg()
	node, err := bt.BuildFromJSON(jsonData, reg)
	if err != nil {
		t.Fatal(err)
	}

	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0) // 不满足 >= 90
	blackboard.Set(bb, blackboard.KeyLastEventType, "E01") // 满足 == E01

	// require_one: 任一 Success → Success
	if node.Tick(&bt.Context{BB: bb}) != bt.Success {
		t.Fatal("expected Success with require_one policy")
	}
}

func TestBuildFromJSON_UnknownType(t *testing.T) {
	jsonData := []byte(`{"type": "nonexistent_xyz"}`)
	reg := defaultReg()
	_, err := bt.BuildFromJSON(jsonData, reg)
	if err == nil {
		t.Fatal("expected error for unknown node type")
	}
}

func TestBuildFromJSON_InvalidJSON(t *testing.T) {
	reg := defaultReg()
	_, err := bt.BuildFromJSON([]byte(`{invalid`), reg)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestBuildFromJSON_NilConfig(t *testing.T) {
	reg := defaultReg()
	_, err := bt.Build(nil, reg)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestBuildFromJSON_InverterWithoutChild(t *testing.T) {
	jsonData := []byte(`{"type": "inverter"}`)
	reg := defaultReg()
	_, err := bt.BuildFromJSON(jsonData, reg)
	if err == nil {
		t.Fatal("expected error for inverter without child")
	}
}

func TestBuildFromJSON_MissingBBKey(t *testing.T) {
	jsonData := []byte(`{
		"type": "check_bb_float",
		"params": {"key": "threat_level", "op": ">=", "value": 50}
	}`)

	reg := defaultReg()
	node, err := bt.BuildFromJSON(jsonData, reg)
	if err != nil {
		t.Fatal(err)
	}

	// 空 BB，key 不存在应该返回 Failure 而非 panic
	bb := blackboard.New()
	if node.Tick(&bt.Context{BB: bb}) != bt.Failure {
		t.Fatal("expected Failure when BB key is missing")
	}
}
