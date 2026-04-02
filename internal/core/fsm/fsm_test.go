package fsm_test

import (
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/rule"
)

func basicConfig() *fsm.FSMConfig {
	return &fsm.FSMConfig{
		InitialState: "Idle",
		States: []fsm.StateConfig{
			{Name: "Idle"},
			{Name: "Alarmed"},
			{Name: "Flee"},
			{Name: "Dead"},
		},
		Transitions: []fsm.TransitionConfig{
			{
				From:     "Idle",
				To:       "Alarmed",
				Priority: 10,
				Condition: rule.Condition{
					Key: "last_event_type", Op: "!=", Value: mustJSON(`""`),
				},
			},
			{
				From:     "Alarmed",
				To:       "Flee",
				Priority: 10,
				Condition: rule.Condition{
					Key: "threat_level", Op: ">=", Value: mustJSON(`50`),
				},
			},
			{
				From:     "Alarmed",
				To:       "Idle",
				Priority: 5,
				Condition: rule.Condition{
					Key: "last_event_type", Op: "==", Value: mustJSON(`""`),
				},
			},
		},
	}
}

func mustJSON(s string) []byte {
	return []byte(s)
}

// --- 基础测试 ---

func TestNewFSM_InitialState(t *testing.T) {
	bb := blackboard.New()
	f, err := fsm.NewFSM(basicConfig(), bb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Current() != "Idle" {
		t.Fatalf("expected initial state Idle, got %s", f.Current())
	}

	// BB 中也应该写入了初始状态
	state, ok := blackboard.Get(bb, blackboard.KeyFSMState)
	if !ok || state != "Idle" {
		t.Fatalf("expected BB fsm_state=Idle, got %v %v", state, ok)
	}
}

func TestTick_Transition(t *testing.T) {
	bb := blackboard.New()
	f, err := fsm.NewFSM(basicConfig(), bb)
	if err != nil {
		t.Fatal(err)
	}

	// Idle → Alarmed: 需要 last_event_type != ""
	blackboard.Set(bb, blackboard.KeyLastEventType, "E01")
	f.Tick(bb)

	if f.Current() != "Alarmed" {
		t.Fatalf("expected Alarmed, got %s", f.Current())
	}
}

func TestTick_NoTransitionWhenConditionNotMet(t *testing.T) {
	bb := blackboard.New()
	f, err := fsm.NewFSM(basicConfig(), bb)
	if err != nil {
		t.Fatal(err)
	}

	// last_event_type 未设置（零值 ""），不应触发转换
	f.Tick(bb)
	if f.Current() != "Idle" {
		t.Fatalf("expected Idle (no transition), got %s", f.Current())
	}
}

func TestTick_ChainedTransitions(t *testing.T) {
	bb := blackboard.New()
	f, err := fsm.NewFSM(basicConfig(), bb)
	if err != nil {
		t.Fatal(err)
	}

	// Idle → Alarmed
	blackboard.Set(bb, blackboard.KeyLastEventType, "E01")
	f.Tick(bb)
	if f.Current() != "Alarmed" {
		t.Fatalf("expected Alarmed, got %s", f.Current())
	}

	// Alarmed → Flee: 需要 threat_level >= 50
	blackboard.Set(bb, blackboard.KeyThreatLevel, 75.0)
	f.Tick(bb)
	if f.Current() != "Flee" {
		t.Fatalf("expected Flee, got %s", f.Current())
	}
}

func TestTick_PriorityOrdering(t *testing.T) {
	bb := blackboard.New()
	f, err := fsm.NewFSM(basicConfig(), bb)
	if err != nil {
		t.Fatal(err)
	}

	// 进入 Alarmed
	blackboard.Set(bb, blackboard.KeyLastEventType, "E01")
	f.Tick(bb)

	// Alarmed 有两个转换：Flee(priority=10) 和 Idle(priority=5)
	// 当 threat_level >= 50 且 last_event_type == ""，两个都满足
	// priority 高的 Flee 应该优先
	blackboard.Set(bb, blackboard.KeyThreatLevel, 75.0)
	blackboard.Set(bb, blackboard.KeyLastEventType, "")
	f.Tick(bb)

	if f.Current() != "Flee" {
		t.Fatalf("expected Flee (higher priority), got %s", f.Current())
	}
}

// --- 回调测试 ---

func TestCallbacks(t *testing.T) {
	bb := blackboard.New()
	f, err := fsm.NewFSM(basicConfig(), bb)
	if err != nil {
		t.Fatal(err)
	}

	var exitCalled, enterCalled, transCalled string

	f.OnExit(func(state string) { exitCalled = state })
	f.OnEnter(func(state string) { enterCalled = state })
	f.OnTransition(func(from, to string) { transCalled = from + "→" + to })

	blackboard.Set(bb, blackboard.KeyLastEventType, "E01")
	f.Tick(bb)

	if exitCalled != "Idle" {
		t.Fatalf("expected OnExit(Idle), got %q", exitCalled)
	}
	if enterCalled != "Alarmed" {
		t.Fatalf("expected OnEnter(Alarmed), got %q", enterCalled)
	}
	if transCalled != "Idle→Alarmed" {
		t.Fatalf("expected OnTransition(Idle→Alarmed), got %q", transCalled)
	}
}

func TestBBState_UpdatedOnTransition(t *testing.T) {
	bb := blackboard.New()
	f, err := fsm.NewFSM(basicConfig(), bb)
	if err != nil {
		t.Fatal(err)
	}

	blackboard.Set(bb, blackboard.KeyLastEventType, "E01")
	f.Tick(bb)

	state, ok := blackboard.Get(bb, blackboard.KeyFSMState)
	if !ok || state != "Alarmed" {
		t.Fatalf("expected BB fsm_state=Alarmed, got %v %v", state, ok)
	}

	_ = f // suppress unused
}

// --- 配置校验测试 ---

func TestNewFSM_NilConfig(t *testing.T) {
	bb := blackboard.New()
	_, err := fsm.NewFSM(nil, bb)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewFSM_NilBB(t *testing.T) {
	cfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States:       []fsm.StateConfig{{Name: "Idle"}},
	}
	_, err := fsm.NewFSM(cfg, nil)
	if err == nil {
		t.Fatal("expected error for nil blackboard")
	}
}

func TestNewFSM_NoStates(t *testing.T) {
	bb := blackboard.New()
	_, err := fsm.NewFSM(&fsm.FSMConfig{InitialState: "Idle"}, bb)
	if err == nil {
		t.Fatal("expected error for empty states")
	}
}

func TestNewFSM_InvalidInitialState(t *testing.T) {
	bb := blackboard.New()
	cfg := &fsm.FSMConfig{
		InitialState: "NonExistent",
		States:       []fsm.StateConfig{{Name: "Idle"}},
	}
	_, err := fsm.NewFSM(cfg, bb)
	if err == nil {
		t.Fatal("expected error for invalid initial state")
	}
}

func TestNewFSM_DuplicateStateName(t *testing.T) {
	bb := blackboard.New()
	cfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States: []fsm.StateConfig{
			{Name: "Idle"},
			{Name: "Idle"},
		},
	}
	_, err := fsm.NewFSM(cfg, bb)
	if err == nil {
		t.Fatal("expected error for duplicate state name")
	}
}

func TestNewFSM_TransitionFromUnknownState(t *testing.T) {
	bb := blackboard.New()
	cfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States:       []fsm.StateConfig{{Name: "Idle"}},
		Transitions: []fsm.TransitionConfig{
			{From: "Unknown", To: "Idle", Priority: 1},
		},
	}
	_, err := fsm.NewFSM(cfg, bb)
	if err == nil {
		t.Fatal("expected error for transition from unknown state")
	}
}

func TestNewFSM_TransitionToUnknownState(t *testing.T) {
	bb := blackboard.New()
	cfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States:       []fsm.StateConfig{{Name: "Idle"}},
		Transitions: []fsm.TransitionConfig{
			{From: "Idle", To: "Unknown", Priority: 1},
		},
	}
	_, err := fsm.NewFSM(cfg, bb)
	if err == nil {
		t.Fatal("expected error for transition to unknown state")
	}
}

func TestNewFSM_InvalidConditionKey(t *testing.T) {
	bb := blackboard.New()
	cfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States: []fsm.StateConfig{
			{Name: "Idle"},
			{Name: "Alert"},
		},
		Transitions: []fsm.TransitionConfig{
			{
				From: "Idle", To: "Alert", Priority: 1,
				Condition: rule.Condition{Key: "totally_fake_key_xyz", Op: "==", Value: mustJSON(`1`)},
			},
		},
	}
	_, err := fsm.NewFSM(cfg, bb)
	if err == nil {
		t.Fatal("expected error for invalid condition key")
	}
}

func TestStates(t *testing.T) {
	bb := blackboard.New()
	f, err := fsm.NewFSM(basicConfig(), bb)
	if err != nil {
		t.Fatal(err)
	}
	states := f.States()
	if len(states) != 4 {
		t.Fatalf("expected 4 states, got %d", len(states))
	}
}
