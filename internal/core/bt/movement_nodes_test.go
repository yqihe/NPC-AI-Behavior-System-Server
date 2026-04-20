package bt_test

import (
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
)

// --- stub_action ---

func TestStubAction_Success(t *testing.T) {
	node, err := bt.BuildFromJSON([]byte(`{"type":"stub_action","params":{"name":"idle","result":"success"}}`), defaultReg())
	if err != nil {
		t.Fatal(err)
	}
	if node.Tick(newCtx()) != bt.Success {
		t.Fatal("expected Success")
	}
}

func TestStubAction_Failure(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"stub_action","params":{"name":"x","result":"failure"}}`), defaultReg())
	if node.Tick(newCtx()) != bt.Failure {
		t.Fatal("expected Failure")
	}
}

func TestStubAction_Running(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"stub_action","params":{"name":"x","result":"running"}}`), defaultReg())
	if node.Tick(newCtx()) != bt.Running {
		t.Fatal("expected Running")
	}
}

func TestStubAction_DefaultResultIsSuccess(t *testing.T) {
	node, err := bt.BuildFromJSON([]byte(`{"type":"stub_action","params":{"name":"x"}}`), defaultReg())
	if err != nil {
		t.Fatal(err)
	}
	if node.Tick(newCtx()) != bt.Success {
		t.Fatal("empty result should default to Success")
	}
}

func TestStubAction_MissingName(t *testing.T) {
	if _, err := bt.BuildFromJSON([]byte(`{"type":"stub_action","params":{"name":""}}`), defaultReg()); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestStubAction_UnknownResult(t *testing.T) {
	if _, err := bt.BuildFromJSON([]byte(`{"type":"stub_action","params":{"name":"x","result":"maybe"}}`), defaultReg()); err == nil {
		t.Fatal("expected error for unknown result")
	}
}

// --- move_to ---

func TestMoveTo_TargetReached(t *testing.T) {
	node, err := bt.BuildFromJSON([]byte(`{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":3}}`), defaultReg())
	if err != nil {
		t.Fatal(err)
	}
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyMoveTargetX, 10.0)
	blackboard.Set(bb, blackboard.KeyMoveTargetZ, 20.0)
	blackboard.Set(bb, blackboard.KeyNPCPosX, 10.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 20.0)

	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}
	if got := node.Tick(ctx); got != bt.Success {
		t.Fatalf("expected Success (already at target), got %v", got)
	}
	state, _ := blackboard.Get(bb, blackboard.KeyMoveState)
	if state != "arrived" {
		t.Errorf("expected move_state=arrived, got %q", state)
	}
}

func TestMoveTo_Running(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":3}}`), defaultReg())
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyMoveTargetX, 100.0)
	blackboard.Set(bb, blackboard.KeyMoveTargetZ, 100.0)
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)

	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}
	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running, got %v", got)
	}
	newX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if newX == 0.0 {
		t.Error("expected X to advance")
	}
	state, _ := blackboard.Get(bb, blackboard.KeyMoveState)
	if state != "moving" {
		t.Errorf("expected move_state=moving, got %q", state)
	}
}

func TestMoveTo_MissingTargetKeys(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":3}}`), defaultReg())
	bb := blackboard.New()
	// 不设置 target keys，让 GetRaw 返回 !ok
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}
	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure when target keys absent, got %v", got)
	}
}

func TestMoveTo_BadTargetType(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"move_to","params":{"target_key_x":"threat_source","target_key_z":"move_target_z","speed":3}}`), defaultReg())
	bb := blackboard.New()
	// threat_source 是字符串键，用它当 target_key_x 会触发 toFloat64 失败路径
	blackboard.Set(bb, blackboard.KeyThreatSource, "npc_1")
	blackboard.Set(bb, blackboard.KeyMoveTargetZ, 10.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}
	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure on non-numeric target, got %v", got)
	}
}

func TestMoveTo_RequiredFieldsValidation(t *testing.T) {
	if _, err := bt.BuildFromJSON([]byte(`{"type":"move_to","params":{"speed":3}}`), defaultReg()); err == nil {
		t.Fatal("expected error when target keys missing")
	}
}

func TestMoveTo_DefaultSpeed(t *testing.T) {
	if _, err := bt.BuildFromJSON([]byte(`{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z"}}`), defaultReg()); err != nil {
		t.Fatalf("omitting speed should default, got error: %v", err)
	}
}

func TestMoveTo_DeltaTimeFallback(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"move_to","params":{"target_key_x":"move_target_x","target_key_z":"move_target_z","speed":3}}`), defaultReg())
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyMoveTargetX, 100.0)
	blackboard.Set(bb, blackboard.KeyMoveTargetZ, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0}
	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running with dt=0 fallback, got %v", got)
	}
	newX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	// dt=0 应回退到 0.1 → speed*0.1 = 0.3 步长
	if newX <= 0 || newX > 1 {
		t.Errorf("expected small positive progress, got %f", newX)
	}
}

// --- flee_from ---

func TestFleeFrom_AlreadySafe(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"flee_from","params":{"source_key_x":"follow_target_x","source_key_z":"follow_target_z","distance":50,"speed":5}}`), defaultReg())
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyFollowTargetX, 0.0)
	blackboard.Set(bb, blackboard.KeyFollowTargetZ, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosX, 100.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)

	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}
	if got := node.Tick(ctx); got != bt.Success {
		t.Fatalf("expected Success (already safe), got %v", got)
	}
}

func TestFleeFrom_Running(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"flee_from","params":{"source_key_x":"follow_target_x","source_key_z":"follow_target_z","distance":100,"speed":5}}`), defaultReg())
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyFollowTargetX, 0.0)
	blackboard.Set(bb, blackboard.KeyFollowTargetZ, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosX, 10.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)

	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}
	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running, got %v", got)
	}
	newX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if newX <= 10.0 {
		t.Errorf("expected X>10 after fleeing from (0,0), got %f", newX)
	}
}

func TestFleeFrom_OverlapFallback(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"flee_from","params":{"source_key_x":"follow_target_x","source_key_z":"follow_target_z","distance":100,"speed":5}}`), defaultReg())
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyFollowTargetX, 5.0)
	blackboard.Set(bb, blackboard.KeyFollowTargetZ, 5.0)
	blackboard.Set(bb, blackboard.KeyNPCPosX, 5.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 5.0)

	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}
	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running, got %v", got)
	}
	newX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if newX <= 5.0 {
		t.Errorf("overlap fallback should push X positive, got %f", newX)
	}
}

func TestFleeFrom_MissingSourceKeys(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"flee_from","params":{"source_key_x":"follow_target_x","source_key_z":"follow_target_z","distance":100,"speed":5}}`), defaultReg())
	ctx := &bt.Context{BB: blackboard.New(), DeltaTime: 0.1}
	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure on missing keys, got %v", got)
	}
}

func TestFleeFrom_BadSourceType(t *testing.T) {
	node, _ := bt.BuildFromJSON([]byte(`{"type":"flee_from","params":{"source_key_x":"follow_target_x","source_key_z":"threat_source","distance":100,"speed":5}}`), defaultReg())
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyFollowTargetX, 0.0)
	blackboard.Set(bb, blackboard.KeyThreatSource, "npc_1")
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}
	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure, got %v", got)
	}
}

func TestFleeFrom_RequiredFields(t *testing.T) {
	if _, err := bt.BuildFromJSON([]byte(`{"type":"flee_from","params":{"distance":100,"speed":5}}`), defaultReg()); err == nil {
		t.Fatal("expected error for missing source keys")
	}
}

func TestFleeFrom_DefaultDistanceAndSpeed(t *testing.T) {
	if _, err := bt.BuildFromJSON([]byte(`{"type":"flee_from","params":{"source_key_x":"follow_target_x","source_key_z":"follow_target_z"}}`), defaultReg()); err != nil {
		t.Fatalf("omitting distance/speed should default, got error: %v", err)
	}
}
