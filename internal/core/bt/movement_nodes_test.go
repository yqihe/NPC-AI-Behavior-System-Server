package bt_test

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
)

func buildMoveToNode(t *testing.T, params string) bt.Node {
	t.Helper()
	reg := bt.DefaultRegistry()
	f, err := reg.Get("move_to")
	if err != nil {
		t.Fatalf("registry.Get(move_to): %v", err)
	}
	node, err := f(json.RawMessage(params))
	if err != nil {
		t.Fatalf("moveToFactory(%s): %v", params, err)
	}
	return node
}

func buildFleeFromNode(t *testing.T, params string) bt.Node {
	t.Helper()
	reg := bt.DefaultRegistry()
	f, err := reg.Get("flee_from")
	if err != nil {
		t.Fatalf("registry.Get(flee_from): %v", err)
	}
	node, err := f(json.RawMessage(params))
	if err != nil {
		t.Fatalf("fleeFromFactory(%s): %v", params, err)
	}
	return node
}

// --- move_to factory ---

func TestMoveToFactory_MissingTargetKey(t *testing.T) {
	reg := bt.DefaultRegistry()
	f, _ := reg.Get("move_to")
	_, err := f(json.RawMessage(`{"speed": 3.0}`))
	if err == nil {
		t.Fatal("expected error when target_key_x/z missing")
	}
}

func TestMoveToFactory_InvalidJSON(t *testing.T) {
	reg := bt.DefaultRegistry()
	f, _ := reg.Get("move_to")
	_, err := f(json.RawMessage(`{not json`))
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestMoveToFactory_DefaultSpeed(t *testing.T) {
	node := buildMoveToNode(t, `{"target_key_x":"tx","target_key_z":"tz"}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "tx", 100.0)
	blackboard.SetDynamic(bb, "tz", 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 1.0}

	// 默认 speed=3.0，dt=1s，一步移 3 单位
	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running, got %v", got)
	}
	x, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if math.Abs(x-3.0) > 1e-6 {
		t.Fatalf("expected x=3.0 after one tick at default speed, got %v", x)
	}
}

// --- move_to Tick ---

func TestMoveTo_TargetKeyMissing(t *testing.T) {
	node := buildMoveToNode(t, `{"target_key_x":"tx","target_key_z":"tz","speed":5}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}

	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure when target keys missing, got %v", got)
	}
}

func TestMoveTo_TargetNonNumeric(t *testing.T) {
	node := buildMoveToNode(t, `{"target_key_x":"tx","target_key_z":"tz","speed":5}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "tx", "not_a_number")
	blackboard.SetDynamic(bb, "tz", 10.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}

	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure on non-numeric target X, got %v", got)
	}

	// swap: Z 非数字
	blackboard.SetDynamic(bb, "tx", 10.0)
	blackboard.SetDynamic(bb, "tz", "nope")
	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure on non-numeric target Z, got %v", got)
	}
}

func TestMoveTo_Arrived(t *testing.T) {
	node := buildMoveToNode(t, `{"target_key_x":"tx","target_key_z":"tz","speed":5}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 10.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 10.0)
	blackboard.SetDynamic(bb, "tx", 10.3)
	blackboard.SetDynamic(bb, "tz", 10.2)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}

	if got := node.Tick(ctx); got != bt.Success {
		t.Fatalf("expected Success when dist<1, got %v", got)
	}
	state, _ := blackboard.Get(bb, blackboard.KeyMoveState)
	if state != "arrived" {
		t.Fatalf("expected move_state=arrived, got %q", state)
	}
}

func TestMoveTo_Moving(t *testing.T) {
	node := buildMoveToNode(t, `{"target_key_x":"tx","target_key_z":"tz","speed":10}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "tx", 100.0)
	blackboard.SetDynamic(bb, "tz", 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.5}

	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running while far, got %v", got)
	}
	x, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	// speed 10 × dt 0.5 = 5 单位沿 X 向靠近
	if math.Abs(x-5.0) > 1e-6 {
		t.Fatalf("expected x=5.0 after one tick, got %v", x)
	}
	state, _ := blackboard.Get(bb, blackboard.KeyMoveState)
	if state != "moving" {
		t.Fatalf("expected move_state=moving, got %q", state)
	}
}

func TestMoveTo_NonPositiveDeltaFallback(t *testing.T) {
	node := buildMoveToNode(t, `{"target_key_x":"tx","target_key_z":"tz","speed":10}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "tx", 100.0)
	blackboard.SetDynamic(bb, "tz", 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0}

	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running, got %v", got)
	}
	x, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	// 0.1 fallback × speed 10 = 1 单位
	if math.Abs(x-1.0) > 1e-6 {
		t.Fatalf("expected x=1.0 with dt fallback, got %v", x)
	}
}

// --- flee_from factory ---

func TestFleeFromFactory_MissingSourceKey(t *testing.T) {
	reg := bt.DefaultRegistry()
	f, _ := reg.Get("flee_from")
	_, err := f(json.RawMessage(`{"distance": 50}`))
	if err == nil {
		t.Fatal("expected error when source_key_x/z missing")
	}
}

func TestFleeFromFactory_InvalidJSON(t *testing.T) {
	reg := bt.DefaultRegistry()
	f, _ := reg.Get("flee_from")
	_, err := f(json.RawMessage(`{bad`))
	if err == nil {
		t.Fatal("expected JSON parse error")
	}
}

func TestFleeFromFactory_Defaults(t *testing.T) {
	// distance/speed 缺省
	node := buildFleeFromNode(t, `{"source_key_x":"sx","source_key_z":"sz"}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "sx", 5.0)
	blackboard.SetDynamic(bb, "sz", 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 1.0}

	// 默认 distance=100，当前 dist=5 < 100 → Running
	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running with default distance, got %v", got)
	}
}

// --- flee_from Tick ---

func TestFleeFrom_SourceKeyMissing(t *testing.T) {
	node := buildFleeFromNode(t, `{"source_key_x":"sx","source_key_z":"sz","distance":50,"speed":5}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}

	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure on missing source keys, got %v", got)
	}
}

func TestFleeFrom_SourceNonNumeric(t *testing.T) {
	node := buildFleeFromNode(t, `{"source_key_x":"sx","source_key_z":"sz","distance":50,"speed":5}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "sx", "bad")
	blackboard.SetDynamic(bb, "sz", 10.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}

	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure on non-numeric source X, got %v", got)
	}

	blackboard.SetDynamic(bb, "sx", 10.0)
	blackboard.SetDynamic(bb, "sz", "bad")
	if got := node.Tick(ctx); got != bt.Failure {
		t.Fatalf("expected Failure on non-numeric source Z, got %v", got)
	}
}

func TestFleeFrom_FarEnough(t *testing.T) {
	node := buildFleeFromNode(t, `{"source_key_x":"sx","source_key_z":"sz","distance":50,"speed":5}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 100.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "sx", 0.0)
	blackboard.SetDynamic(bb, "sz", 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}

	if got := node.Tick(ctx); got != bt.Success {
		t.Fatalf("expected Success when dist>=distance, got %v", got)
	}
	state, _ := blackboard.Get(bb, blackboard.KeyMoveState)
	if state != "arrived" {
		t.Fatalf("expected move_state=arrived, got %q", state)
	}
}

func TestFleeFrom_Moving(t *testing.T) {
	node := buildFleeFromNode(t, `{"source_key_x":"sx","source_key_z":"sz","distance":50,"speed":10}`)
	bb := blackboard.New()
	// NPC 在 (10, 0)，威胁源在 (0, 0)，距离 10 < distance 50 → 朝 +X 逃
	blackboard.Set(bb, blackboard.KeyNPCPosX, 10.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "sx", 0.0)
	blackboard.SetDynamic(bb, "sz", 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.2}

	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running while close, got %v", got)
	}
	x, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	// step = 10 × 0.2 = 2，沿 +X 逃 → 新 x = 12
	if math.Abs(x-12.0) > 1e-6 {
		t.Fatalf("expected x=12.0, got %v", x)
	}
	state, _ := blackboard.Get(bb, blackboard.KeyMoveState)
	if state != "moving" {
		t.Fatalf("expected move_state=moving, got %q", state)
	}
}

func TestFleeFrom_CoincideFallbackX(t *testing.T) {
	node := buildFleeFromNode(t, `{"source_key_x":"sx","source_key_z":"sz","distance":50,"speed":10}`)
	bb := blackboard.New()
	// NPC 与威胁源重合 → fallback 向 +X
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "sx", 0.0)
	blackboard.SetDynamic(bb, "sz", 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}

	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running on coincide, got %v", got)
	}
	x, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if x <= 0 {
		t.Fatalf("expected positive X after coincide-fallback (toward +X), got %v", x)
	}
}

func TestFleeFrom_NonPositiveDeltaFallback(t *testing.T) {
	node := buildFleeFromNode(t, `{"source_key_x":"sx","source_key_z":"sz","distance":50,"speed":10}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 10.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "sx", 0.0)
	blackboard.SetDynamic(bb, "sz", 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 0}

	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running with dt fallback, got %v", got)
	}
	x, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	// fallback dt=0.1, step=1, 新 x=11
	if math.Abs(x-11.0) > 1e-6 {
		t.Fatalf("expected x=11.0 with dt fallback, got %v", x)
	}
}

// --- btMoveToward 覆盖（通过 moveTo 间接触发三分支） ---

func TestBtMoveToward_ExactTargetWhenWithinStep(t *testing.T) {
	// maxStep > dist → 一步抵达目标
	node := buildMoveToNode(t, `{"target_key_x":"tx","target_key_z":"tz","speed":100}`)
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)
	blackboard.SetDynamic(bb, "tx", 5.0)
	blackboard.SetDynamic(bb, "tz", 0.0)
	ctx := &bt.Context{BB: bb, DeltaTime: 1.0}

	// speed 100 × dt 1 = 100 远超距离 5 → btMoveToward 走 dist<=maxDist 分支，位置 = 目标
	// moveTo 先检查 dist<1（5.0 不满足）→ 调用 btMoveToward 后位置变 5.0
	// 但下一 Tick 才会判 arrived——本 Tick 仍 Running
	if got := node.Tick(ctx); got != bt.Running {
		t.Fatalf("expected Running on first tick, got %v", got)
	}
	x, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if math.Abs(x-5.0) > 1e-6 {
		t.Fatalf("expected x snapped to 5.0, got %v", x)
	}
}
