package core_test

// 攻击性测试：验证 core-engine 在边界和异常输入下的鲁棒性
// 这个文件是 /verify 阶段产出，验证通过后可保留或删除

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/rule"
)

// --- Blackboard 攻击 ---

func TestAttack_BB_ConcurrentHeavy(t *testing.T) {
	bb := blackboard.New()
	var wg sync.WaitGroup
	const goroutines = 100
	const ops = 1000

	// 100 goroutine 同时读写
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < ops; j++ {
				blackboard.Set(bb, blackboard.KeyThreatLevel, float64(j))
				blackboard.Get(bb, blackboard.KeyThreatLevel)
				blackboard.Set(bb, blackboard.KeyThreatSource, "enemy")
				blackboard.Get(bb, blackboard.KeyThreatSource)
				blackboard.Has(bb, blackboard.KeyFSMState)
			}
		}()
	}
	wg.Wait()
}

func TestAttack_BB_GetRawEmptyString(t *testing.T) {
	bb := blackboard.New()
	// 空字符串 key
	val, ok := bb.GetRaw("")
	if ok {
		t.Errorf("expected not found for empty key, got %v", val)
	}
}

func TestAttack_BB_ZeroValueFloat(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 0.0)
	val, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if !ok {
		t.Fatal("expected key to exist after set to zero value")
	}
	if val != 0.0 {
		t.Errorf("expected 0.0, got %v", val)
	}
}

func TestAttack_BB_NegativeFloat(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, -999.99)
	val, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if !ok || val != -999.99 {
		t.Errorf("expected -999.99, got %v (ok=%v)", val, ok)
	}
}

func TestAttack_BB_EmptyStringValue(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatSource, "")
	val, ok := blackboard.Get(bb, blackboard.KeyThreatSource)
	if !ok {
		t.Fatal("expected key to exist after set to empty string")
	}
	if val != "" {
		t.Errorf("expected empty string, got %q", val)
	}
}

// --- Rule 攻击 ---

func TestAttack_Rule_EmptyCondition(t *testing.T) {
	bb := blackboard.New()
	cond := rule.Condition{}
	// 空条件应返回 true
	if !cond.Evaluate(bb) {
		t.Error("empty condition should return true")
	}
}

func TestAttack_Rule_DeeplyNestedAnd(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 80.0)

	// 构造 50 层深嵌套 AND
	inner := rule.Condition{Key: "threat_level", Op: ">=", Value: json.RawMessage(`50`)}
	for i := 0; i < 50; i++ {
		inner = rule.Condition{And: []rule.Condition{inner}}
	}
	if !inner.Evaluate(bb) {
		t.Error("deeply nested AND with true leaf should be true")
	}
}

func TestAttack_Rule_MissingBBKey(t *testing.T) {
	bb := blackboard.New()
	// BB 中没有 threat_level，求值应返回 false
	cond := rule.Condition{Key: "threat_level", Op: ">=", Value: json.RawMessage(`50`)}
	if cond.Evaluate(bb) {
		t.Error("condition on missing BB key should return false")
	}
}

func TestAttack_Rule_TypeMismatch(t *testing.T) {
	bb := blackboard.New()
	// threat_level 是 float64，但设置为 string
	blackboard.Set(bb, blackboard.KeyThreatSource, "not_a_number")
	cond := rule.Condition{Key: "threat_source", Op: ">=", Value: json.RawMessage(`50`)}
	// string 和 number 比较应返回 false，不 panic
	if cond.Evaluate(bb) {
		t.Error("type mismatch comparison should return false")
	}
}

func TestAttack_Rule_InEmptyArray(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)
	cond := rule.Condition{Key: "threat_level", Op: "in", Value: json.RawMessage(`[]`)}
	if cond.Evaluate(bb) {
		t.Error("in with empty array should return false")
	}
}

func TestAttack_Rule_InvalidJSON(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)
	cond := rule.Condition{Key: "threat_level", Op: ">=", Value: json.RawMessage(`{invalid`)}
	// 无效 JSON 值应返回 false，不 panic
	if cond.Evaluate(bb) {
		t.Error("invalid JSON value should cause false")
	}
}

// --- FSM 攻击 ---

func TestAttack_FSM_EmptyStates(t *testing.T) {
	bb := blackboard.New()
	cfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States:       []fsm.StateConfig{},
	}
	_, err := fsm.NewFSM(cfg, bb)
	if err == nil {
		t.Error("expected error for empty states")
	}
}

func TestAttack_FSM_NilConfig(t *testing.T) {
	bb := blackboard.New()
	_, err := fsm.NewFSM(nil, bb)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestAttack_FSM_NilBB(t *testing.T) {
	cfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States:       []fsm.StateConfig{{Name: "Idle"}},
	}
	_, err := fsm.NewFSM(cfg, nil)
	if err == nil {
		t.Error("expected error for nil bb")
	}
}

func TestAttack_FSM_TickNoTransitions(t *testing.T) {
	bb := blackboard.New()
	cfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States:       []fsm.StateConfig{{Name: "Idle"}},
		Transitions:  []fsm.TransitionConfig{},
	}
	f, err := fsm.NewFSM(cfg, bb)
	if err != nil {
		t.Fatal(err)
	}
	// Tick without any transitions should not panic
	f.Tick(bb)
	if f.Current() != "Idle" {
		t.Errorf("expected Idle, got %s", f.Current())
	}
}

func TestAttack_FSM_TickWithUnmetConditions(t *testing.T) {
	bb := blackboard.New()
	cfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States: []fsm.StateConfig{
			{Name: "Idle"},
			{Name: "Alert"},
		},
		Transitions: []fsm.TransitionConfig{
			{
				From:     "Idle",
				To:       "Alert",
				Priority: 10,
				Condition: rule.Condition{
					Key: "threat_level", Op: ">=",
					Value: json.RawMessage(`999`),
				},
			},
		},
	}
	f, err := fsm.NewFSM(cfg, bb)
	if err != nil {
		t.Fatal(err)
	}
	// 多次 Tick，条件永不满足
	for i := 0; i < 100; i++ {
		f.Tick(bb)
	}
	if f.Current() != "Idle" {
		t.Errorf("expected Idle after 100 ticks, got %s", f.Current())
	}
}

func TestAttack_FSM_RapidTransitions(t *testing.T) {
	bb := blackboard.New()
	cfg := &fsm.FSMConfig{
		InitialState: "A",
		States: []fsm.StateConfig{
			{Name: "A"},
			{Name: "B"},
		},
		Transitions: []fsm.TransitionConfig{
			{From: "A", To: "B", Priority: 10, Condition: rule.Condition{}}, // 无条件
			{From: "B", To: "A", Priority: 10, Condition: rule.Condition{}}, // 无条件
		},
	}
	f, err := fsm.NewFSM(cfg, bb)
	if err != nil {
		t.Fatal(err)
	}
	// 快速来回切换 1000 次
	for i := 0; i < 1000; i++ {
		f.Tick(bb)
	}
	// 1000 次切换，每次 A→B 或 B→A，最终取决于偶数/奇数
	current := f.Current()
	if current != "A" && current != "B" {
		t.Errorf("unexpected state: %s", current)
	}
}

// --- BT 攻击 ---

func TestAttack_BT_NilConfig(t *testing.T) {
	reg := bt.DefaultRegistry()
	_, err := bt.Build(nil, reg)
	if err == nil {
		t.Error("expected error for nil config")
	}
}

func TestAttack_BT_UnknownType(t *testing.T) {
	reg := bt.DefaultRegistry()
	cfg := &bt.TreeConfig{Type: "nonexistent_node_type_xyz"}
	_, err := bt.Build(cfg, reg)
	if err == nil {
		t.Error("expected error for unknown node type")
	}
}

func TestAttack_BT_EmptySequence(t *testing.T) {
	bb := blackboard.New()
	ctx := &bt.Context{BB: bb}
	seq := &bt.Sequence{Children: []bt.Node{}}
	// 空 Sequence 应返回 Success（无子节点全部成功）
	status := seq.Tick(ctx)
	if status != bt.Success {
		t.Errorf("empty sequence should return Success, got %s", status)
	}
}

func TestAttack_BT_EmptySelector(t *testing.T) {
	bb := blackboard.New()
	ctx := &bt.Context{BB: bb}
	sel := &bt.Selector{Children: []bt.Node{}}
	// 空 Selector 应返回 Failure（无子节点全部失败）
	status := sel.Tick(ctx)
	if status != bt.Failure {
		t.Errorf("empty selector should return Failure, got %s", status)
	}
}

func TestAttack_BT_InvalidJSON(t *testing.T) {
	reg := bt.DefaultRegistry()
	_, err := bt.BuildFromJSON([]byte(`{invalid json`), reg)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestAttack_BT_DeeplyNestedTree(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 80.0)
	ctx := &bt.Context{BB: bb}

	// 构造 100 层嵌套 sequence → sequence → ... → check_bb_float
	leaf := &checkFloat{key: "threat_level", op: ">=", val: 50}
	var node bt.Node = leaf
	for i := 0; i < 100; i++ {
		node = &bt.Sequence{Children: []bt.Node{node}}
	}
	status := node.Tick(ctx)
	if status != bt.Success {
		t.Errorf("deeply nested tree should succeed, got %s", status)
	}
}

// helper: inline check node for deep nesting test
type checkFloat struct {
	key string
	op  string
	val float64
}

func (c *checkFloat) Tick(ctx *bt.Context) bt.Status {
	raw, ok := ctx.BB.GetRaw(c.key)
	if !ok {
		return bt.Failure
	}
	f, isNum := raw.(float64)
	if !isNum {
		return bt.Failure
	}
	switch c.op {
	case ">=":
		if f >= c.val {
			return bt.Success
		}
	}
	return bt.Failure
}

func TestAttack_BT_SetRawUnregisteredKey(t *testing.T) {
	// set_bb_value 对未注册 Key 应在构建期拒绝（R3 保证）
	treeJSON := `{
		"type": "set_bb_value",
		"params": {"key": "totally_unregistered_key_abc", "value": 42}
	}`
	reg := bt.DefaultRegistry()
	_, err := bt.BuildFromJSON([]byte(treeJSON), reg)
	if err == nil {
		t.Fatal("expected build error for unregistered key in set_bb_value")
	}
}

// --- Config 攻击 ---

func testConfigsDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..", "configs")
}

func TestAttack_Config_Concurrent(t *testing.T) {
	// 并发加载同一配置文件不应出问题
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			src := config.NewJSONSource(testConfigsDir(t))
			cfg, err := src.LoadFSMConfig("civilian")
			if err != nil {
				t.Errorf("concurrent load failed: %v", err)
				return
			}
			if cfg.InitialState != "Idle" {
				t.Errorf("unexpected initial state: %s", cfg.InitialState)
			}
		}()
	}
	wg.Wait()
}
