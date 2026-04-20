package npc

import (
	"fmt"
	"sync"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/rule"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// --- ParseNPCTypeConfig ---

func TestParseNPCTypeConfig(t *testing.T) {
	data := []byte(`{
		"type_name": "civilian",
		"fsm_ref": "civilian",
		"bt_refs": {"Idle": "civilian/idle", "Flee": "civilian/flee"},
		"perception": {"visual_range": 200, "auditory_range": 500}
	}`)
	cfg, err := ParseNPCTypeConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TypeName != "civilian" {
		t.Errorf("expected civilian, got %s", cfg.TypeName)
	}
	if cfg.FSMRef != "civilian" {
		t.Errorf("expected fsm_ref civilian, got %s", cfg.FSMRef)
	}
	if len(cfg.BTRefs) != 2 {
		t.Errorf("expected 2 bt_refs, got %d", len(cfg.BTRefs))
	}
	if cfg.Perception.VisualRange != 200 {
		t.Errorf("expected visual_range 200, got %f", cfg.Perception.VisualRange)
	}
}

func TestParseNPCTypeConfig_InvalidJSON(t *testing.T) {
	_, err := ParseNPCTypeConfig([]byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- Instance (手动构建，不走 Config) ---

func makeTestInstance(id string) *Instance {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))

	fsmCfg := &fsm.FSMConfig{
		InitialState: "Idle",
		States:       []fsm.StateConfig{{Name: "Idle"}, {Name: "Flee"}},
		Transitions: []fsm.TransitionConfig{
			{From: "Idle", To: "Flee", Priority: 10, Condition: rule.Condition{
				Key: "threat_level", Op: ">=", Value: mustRawJSON("50"),
			}},
			{From: "Flee", To: "Idle", Priority: 10, Condition: rule.Condition{
				Key: "threat_level", Op: "<", Value: mustRawJSON("20"),
			}},
		},
	}
	f, err := fsm.NewFSM(fsmCfg, bb)
	if err != nil {
		panic(err)
	}

	// 空 BT（stub）
	stubNode := &stubBTNode{status: bt.Success}
	btrees := map[string]bt.Node{
		"Idle": stubNode,
		"Flee": stubNode,
	}

	percCfg := perception.PerceptionConfig{VisualRange: 200, AuditoryRange: 500}

	return &Instance{
		ID:         id,
		TypeName:   "civilian",
		Position:   event.Vec3{X: 0, Y: 0, Z: 0},
		BB:         bb,
		FSM:        f,
		BTrees:     btrees,
		Perception: &percCfg,
	}
}

type stubBTNode struct {
	status bt.Status
}

func (s *stubBTNode) Tick(ctx *bt.Context) bt.Status {
	return s.status
}

func mustRawJSON(s string) []byte {
	return []byte(s)
}

func TestInstance_Tick_NoTransition(t *testing.T) {
	inst := makeTestInstance("npc_1")
	inst.Tick()
	if inst.FSM.Current() != "Idle" {
		t.Errorf("expected Idle, got %s", inst.FSM.Current())
	}
}

func TestInstance_Tick_Transition(t *testing.T) {
	inst := makeTestInstance("npc_1")
	blackboard.Set(inst.BB, blackboard.KeyThreatLevel, 75.0)
	blackboard.Set(inst.BB, blackboard.KeyThreatExpireAt, int64(9999))

	inst.Tick()
	if inst.FSM.Current() != "Flee" {
		t.Errorf("expected Flee after high threat, got %s", inst.FSM.Current())
	}
}

func TestInstance_Tick_NoTreeForState(t *testing.T) {
	inst := makeTestInstance("npc_1")
	delete(inst.BTrees, "Idle") // 移除 Idle 状态的 BT
	// Tick 不应 panic
	inst.Tick()
	if inst.FSM.Current() != "Idle" {
		t.Errorf("expected Idle, got %s", inst.FSM.Current())
	}
}

// --- Registry ---

func TestRegistry_AddGetRemove(t *testing.T) {
	reg := NewRegistry()
	inst := makeTestInstance("npc_1")

	reg.Add(inst)
	if reg.Count() != 1 {
		t.Fatalf("expected 1, got %d", reg.Count())
	}

	got, ok := reg.Get("npc_1")
	if !ok || got.ID != "npc_1" {
		t.Fatal("expected to find npc_1")
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("should not find nonexistent NPC")
	}

	reg.Remove("npc_1")
	if reg.Count() != 0 {
		t.Errorf("expected 0 after remove, got %d", reg.Count())
	}
}

func TestRegistry_ForEach(t *testing.T) {
	reg := NewRegistry()
	reg.Add(makeTestInstance("npc_1"))
	reg.Add(makeTestInstance("npc_2"))
	reg.Add(makeTestInstance("npc_3"))

	count := 0
	reg.ForEach(func(inst *Instance) {
		count++
	})
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

func TestRegistry_ConcurrentAddRemove(t *testing.T) {
	reg := NewRegistry()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		id := fmt.Sprintf("npc_%d", i)
		go func() {
			defer wg.Done()
			inst := makeTestInstance(id)
			reg.Add(inst)
		}()
	}
	wg.Wait()

	if reg.Count() != 50 {
		t.Errorf("expected 50, got %d", reg.Count())
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		id := fmt.Sprintf("npc_%d", i)
		go func() {
			defer wg.Done()
			reg.Remove(id)
		}()
	}
	wg.Wait()

	if reg.Count() != 0 {
		t.Errorf("expected 0 after concurrent remove, got %d", reg.Count())
	}
}
