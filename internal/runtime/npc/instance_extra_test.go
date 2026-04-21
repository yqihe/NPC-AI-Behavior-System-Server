package npc

import (
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// --- ZeroVec3 ---

func TestZeroVec3(t *testing.T) {
	v := ZeroVec3()
	if v.X != 0 || v.Y != 0 || v.Z != 0 {
		t.Fatalf("ZeroVec3 = %+v, want {0,0,0}", v)
	}
}

// --- NewInstance (v2 legacy path) ---
// 复用 admin_template_test.go 内已定义的 fakeSource（package npc 共享）。

func civilianTypeCfg() *NPCTypeConfig {
	return &NPCTypeConfig{
		TypeName:   "civilian",
		FSMRef:     "guard", // fakeSource 里注册过的 FSM
		BTRefs:     map[string]string{"Idle": "guard/idle"},
		Perception: perception.PerceptionConfig{VisualRange: 100, AuditoryRange: 200},
	}
}

func TestNewInstance_HappyPath(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	typeCfg := civilianTypeCfg()

	inst, err := NewInstance("npc_v2", event.Vec3{X: 5, Z: 10}, typeCfg, src, btReg)
	if err != nil {
		t.Fatalf("NewInstance: %v", err)
	}
	if inst.ID != "npc_v2" || inst.TypeName != "civilian" {
		t.Fatalf("ID/TypeName mismatch: %+v", inst)
	}
	if inst.FSM == nil || inst.BTrees["Idle"] == nil {
		t.Fatal("FSM/BTrees not initialized")
	}
	if x, _ := blackboard.Get(inst.BB, blackboard.KeyNPCPosX); x != 5 {
		t.Fatalf("BB pos_x = %v, want 5", x)
	}
}

func TestNewInstance_MissingFSM(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	typeCfg := civilianTypeCfg()
	typeCfg.FSMRef = "missing_fsm"

	_, err := NewInstance("npc_bad_fsm", event.Vec3{}, typeCfg, src, btReg)
	if err == nil {
		t.Fatal("expected error for missing FSM")
	}
}

func TestNewInstance_MissingBT(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	typeCfg := civilianTypeCfg()
	typeCfg.BTRefs = map[string]string{"Idle": "missing_bt"}

	_, err := NewInstance("npc_bad_bt", event.Vec3{}, typeCfg, src, btReg)
	if err == nil {
		t.Fatal("expected error for missing BT")
	}
}

func TestNewInstance_InvalidBTJSON(t *testing.T) {
	src := newFakeSource()
	src.bts["broken/bt"] = []byte(`{invalid`)
	btReg := bt.DefaultRegistry()
	typeCfg := civilianTypeCfg()
	typeCfg.BTRefs = map[string]string{"Idle": "broken/bt"}

	_, err := NewInstance("npc_bad_btjson", event.Vec3{}, typeCfg, src, btReg)
	if err == nil {
		t.Fatal("expected error for invalid BT JSON")
	}
}

// --- RawComponent ---

func TestInstance_RawComponent_NilComponents(t *testing.T) {
	inst := &Instance{ID: "x"}
	if _, ok := inst.RawComponent("position"); ok {
		t.Fatal("expected false for nil components map")
	}
}

func TestInstance_RawComponent_Found(t *testing.T) {
	inst := &Instance{ID: "x"}
	posComp := &component.PositionComponent{X: 1, Z: 2}
	inst.InjectComponentForTest("position", posComp)

	got, ok := inst.RawComponent("position")
	if !ok {
		t.Fatal("expected true")
	}
	if got != posComp {
		t.Fatal("RawComponent returned different pointer")
	}
}

func TestInstance_RawComponent_NotFound(t *testing.T) {
	inst := &Instance{ID: "x"}
	inst.InjectComponentForTest("position", &component.PositionComponent{})
	if _, ok := inst.RawComponent("nonexistent"); ok {
		t.Fatal("expected false for unregistered name")
	}
}

// --- TickComponents ---

// countingTickable a minimal Tickable component recording Tick calls.
type countingTickable struct {
	name  string
	ticks int
}

func (c *countingTickable) Name() string                  { return c.name }
func (c *countingTickable) Tick(_ *blackboard.Blackboard, _ float64) { c.ticks++ }

func TestInstance_TickComponents_Empty(t *testing.T) {
	inst := &Instance{ID: "x"}
	// 不应 panic；空 tickables 直接 return
	inst.TickComponents(0.1)
}

func TestInstance_TickComponents_Invoked(t *testing.T) {
	inst := &Instance{ID: "x", BB: blackboard.New()}
	c := &countingTickable{name: "counter"}
	inst.InjectComponentForTest("counter", c)

	inst.TickComponents(0.1)
	inst.TickComponents(0.1)

	if c.ticks != 2 {
		t.Fatalf("ticks = %d, want 2", c.ticks)
	}
}

// --- SyncPosition ---

func TestInstance_SyncPosition_UpdatesInstance(t *testing.T) {
	inst := &Instance{ID: "x", BB: blackboard.New(), Position: event.Vec3{X: 0, Z: 0}}
	blackboard.Set(inst.BB, blackboard.KeyNPCPosX, 42.0)
	blackboard.Set(inst.BB, blackboard.KeyNPCPosZ, 17.0)

	inst.SyncPosition()

	if inst.Position.X != 42 || inst.Position.Z != 17 {
		t.Fatalf("Position = %+v, want {42,_,17}", inst.Position)
	}
}

func TestInstance_SyncPosition_MissingBBKeys(t *testing.T) {
	inst := &Instance{ID: "x", BB: blackboard.New(), Position: event.Vec3{X: 7, Z: 8}}
	// BB 里没写 KeyNPCPosX / KeyNPCPosZ → 不改变 Position
	inst.SyncPosition()
	if inst.Position.X != 7 || inst.Position.Z != 8 {
		t.Fatalf("Position changed unexpectedly: %+v", inst.Position)
	}
}

func TestInstance_SyncPosition_UpdatesPositionComponent(t *testing.T) {
	inst := &Instance{ID: "x", BB: blackboard.New()}
	posComp := &component.PositionComponent{}
	inst.InjectComponentForTest("position", posComp)

	blackboard.Set(inst.BB, blackboard.KeyNPCPosX, 11.0)
	blackboard.Set(inst.BB, blackboard.KeyNPCPosZ, 22.0)

	inst.SyncPosition()

	if posComp.X != 11 || posComp.Z != 22 {
		t.Fatalf("PositionComponent = {%v,%v}, want {11,22}", posComp.X, posComp.Z)
	}
}

// --- InjectComponentForTest 行为 ---

func TestInstance_InjectComponentForTest_ReplaceKeepsOnlyLatest(t *testing.T) {
	inst := &Instance{ID: "x", BB: blackboard.New()}
	c1 := &countingTickable{name: "counter"}
	c2 := &countingTickable{name: "counter"}
	inst.InjectComponentForTest("counter", c1)
	inst.InjectComponentForTest("counter", c2)

	inst.TickComponents(0.1)

	if c1.ticks != 0 {
		t.Fatalf("c1 ticked %d times after replacement, want 0", c1.ticks)
	}
	if c2.ticks != 1 {
		t.Fatalf("c2 ticks = %d, want 1", c2.ticks)
	}
}
