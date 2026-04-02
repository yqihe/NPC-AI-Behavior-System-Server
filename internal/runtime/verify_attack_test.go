package runtime_test

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// === EventBus 攻击 ===

func TestAttack_Bus_MassivePublish(t *testing.T) {
	bus := event.NewBus()
	cfg := &event.EventTypeConfig{Name: "test", DefaultSeverity: 50, DefaultTTL: 5, PerceptionMode: "global", Range: 100}
	for i := 0; i < 10000; i++ {
		bus.Publish(event.NewEvent(cfg, event.Vec3{}, "", 0))
	}
	if bus.ActiveCount() != 10000 {
		t.Errorf("expected 10000, got %d", bus.ActiveCount())
	}
	bus.Tick(10) // 全部过期
	if bus.ActiveCount() != 0 {
		t.Errorf("expected 0 after mass expire, got %d", bus.ActiveCount())
	}
}

func TestAttack_Bus_NegativeDT(t *testing.T) {
	bus := event.NewBus()
	cfg := &event.EventTypeConfig{Name: "test", DefaultSeverity: 50, DefaultTTL: 5, PerceptionMode: "global", Range: 100}
	bus.Publish(event.NewEvent(cfg, event.Vec3{}, "", 0))
	// 负 dt 不应 panic，TTL 会增加
	bus.Tick(-1.0)
	if bus.ActiveCount() != 1 {
		t.Error("negative dt should not remove events")
	}
}

func TestAttack_Bus_ZeroDT(t *testing.T) {
	bus := event.NewBus()
	cfg := &event.EventTypeConfig{Name: "test", DefaultSeverity: 50, DefaultTTL: 5, PerceptionMode: "global", Range: 100}
	bus.Publish(event.NewEvent(cfg, event.Vec3{}, "", 0))
	bus.Tick(0)
	if bus.ActiveCount() != 1 {
		t.Error("zero dt should not remove events")
	}
}

func TestAttack_Bus_ConcurrentPublishAndTick(t *testing.T) {
	bus := event.NewBus()
	cfg := &event.EventTypeConfig{Name: "test", DefaultSeverity: 50, DefaultTTL: 100, PerceptionMode: "global", Range: 100}
	var wg sync.WaitGroup
	// 并发 Publish + Tick + Active
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			bus.Publish(event.NewEvent(cfg, event.Vec3{}, "", 0))
		}()
		go func() {
			defer wg.Done()
			bus.Tick(0.001)
		}()
		go func() {
			defer wg.Done()
			_ = bus.Active()
		}()
	}
	wg.Wait()
}

// === Perception 攻击 ===

func TestAttack_Perception_ZeroRange(t *testing.T) {
	cfg := &perception.PerceptionConfig{VisualRange: 0, AuditoryRange: 0}
	evtCfg := &event.EventTypeConfig{PerceptionMode: "visual", Range: 100}
	evt := &event.Event{Position: event.Vec3{0, 0, 0}}
	// visual_range=0, min(0,100)=0, distance=0 → 0<=0 → true
	result := perception.CanPerceive(event.Vec3{0, 0, 0}, cfg, evt, evtCfg)
	if !result {
		t.Error("zero range at zero distance should perceive")
	}
}

func TestAttack_Perception_NegativeRange(t *testing.T) {
	cfg := &perception.PerceptionConfig{VisualRange: -100, AuditoryRange: -100}
	evtCfg := &event.EventTypeConfig{PerceptionMode: "auditory", Range: 100}
	evt := &event.Event{Position: event.Vec3{0, 0, 0}}
	// min(-100, 100)=-100, distance=0 → 0<=-100 → false
	result := perception.CanPerceive(event.Vec3{0, 0, 0}, cfg, evt, evtCfg)
	if result {
		t.Error("negative range should not perceive")
	}
}

func TestAttack_Perception_VeryLargeDistance(t *testing.T) {
	cfg := &perception.PerceptionConfig{VisualRange: 1e10, AuditoryRange: 1e10}
	evtCfg := &event.EventTypeConfig{PerceptionMode: "visual", Range: 1e10}
	evt := &event.Event{Position: event.Vec3{1e9, 0, 1e9}}
	// 不应 panic
	_ = perception.CanPerceive(event.Vec3{0, 0, 0}, cfg, evt, evtCfg)
}

// === Decision 攻击 ===

func TestAttack_Decision_EmptyEvtTypes(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))
	center := decision.NewCenter(10.0)
	evt := &event.Event{ID: "e1", Type: "unknown", Position: event.Vec3{}, Severity: 80, TTL: 10}
	// 空 evtTypes map
	center.Evaluate(bb, event.Vec3{}, []*event.Event{evt}, map[string]*event.EventTypeConfig{}, 0.1)
	// 不应 panic，不应写入 BB
}

func TestAttack_Decision_ZeroSeverity(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))
	center := decision.NewCenter(10.0)
	evtTypes := map[string]*event.EventTypeConfig{
		"whisper": {Name: "whisper", Range: 100},
	}
	evt := &event.Event{ID: "e1", Type: "whisper", Position: event.Vec3{}, Severity: 0, TTL: 10}
	center.Evaluate(bb, event.Vec3{}, []*event.Event{evt}, evtTypes, 0.1)

	level, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if ok && level > 0 {
		t.Errorf("zero severity should produce zero threat, got %f", level)
	}
}

func TestAttack_Decision_NegativeSeverity(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))
	center := decision.NewCenter(10.0)
	evtTypes := map[string]*event.EventTypeConfig{
		"heal": {Name: "heal", Range: 100},
	}
	evt := &event.Event{ID: "e1", Type: "heal", Position: event.Vec3{}, Severity: -50, TTL: 10}
	// 负 severity 不应 panic
	center.Evaluate(bb, event.Vec3{}, []*event.Event{evt}, evtTypes, 0.1)
}

func TestAttack_Decision_MassiveDecay(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 1000.0)
	blackboard.Set(bb, blackboard.KeyThreatSource, "old")
	blackboard.Set(bb, blackboard.KeyLastEventType, "old")
	center := decision.NewCenter(1e6) // 极高衰减率
	center.Evaluate(bb, event.Vec3{}, nil, nil, 1.0)

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if level != 0 {
		t.Errorf("massive decay should clamp to 0, got %f", level)
	}
}

// === NPC 攻击 ===

func TestAttack_Registry_RemoveNonexistent(t *testing.T) {
	reg := npc.NewRegistry()
	reg.Remove("nonexistent") // 不应 panic
	if reg.Count() != 0 {
		t.Error("expected 0")
	}
}

func TestAttack_Registry_DoubleAdd(t *testing.T) {
	reg := npc.NewRegistry()
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	inst := createCivilian(t, "npc_1", event.Vec3{}, src, btReg)
	reg.Add(inst)
	reg.Add(inst) // 同 ID 覆盖
	if reg.Count() != 1 {
		t.Errorf("expected 1 after double add, got %d", reg.Count())
	}
}

func TestAttack_Registry_ForEachEmpty(t *testing.T) {
	reg := npc.NewRegistry()
	count := 0
	reg.ForEach(func(inst *npc.Instance) { count++ })
	if count != 0 {
		t.Error("expected 0 iterations on empty registry")
	}
}

// === Scheduler 攻击 ===

func TestAttack_Scheduler_NoNPCs(t *testing.T) {
	bus := event.NewBus()
	reg := npc.NewRegistry()
	center := decision.NewCenter(10.0)
	evtTypes := map[string]*event.EventTypeConfig{}
	scheduler := runtime.NewScheduler(bus, reg, center, evtTypes, 100*time.Millisecond)
	// Tick with no NPCs should not panic
	scheduler.Tick(0.1)
}

func TestAttack_Scheduler_NoEvents(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	bus := event.NewBus()
	reg := npc.NewRegistry()
	inst := createCivilian(t, "npc_1", event.Vec3{}, src, btReg)
	reg.Add(inst)
	center := decision.NewCenter(10.0)
	scheduler := runtime.NewScheduler(bus, reg, center, evtTypes, 100*time.Millisecond)
	// Tick with NPCs but no events should not panic
	scheduler.Tick(0.1)
	if inst.FSM.Current() != "Idle" {
		t.Errorf("expected Idle with no events, got %s", inst.FSM.Current())
	}
}

func TestAttack_Scheduler_RapidTicks(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	bus := event.NewBus()
	reg := npc.NewRegistry()
	for i := 0; i < 10; i++ {
		inst := createCivilian(t, fmt.Sprintf("npc_%d", i), event.Vec3{X: float64(i * 10)}, src, btReg)
		reg.Add(inst)
	}
	explosionCfg := evtTypes["explosion"]
	bus.Publish(event.NewEvent(explosionCfg, event.Vec3{50, 0, 0}, "bomb", 80))

	center := decision.NewCenter(10.0)
	scheduler := runtime.NewScheduler(bus, reg, center, evtTypes, 100*time.Millisecond)

	// 1000 次快速 Tick 不 panic
	for i := 0; i < 1000; i++ {
		scheduler.Tick(0.001)
	}
}

// === 事件重复发送 ===

func TestAttack_DuplicateEvents(t *testing.T) {
	bus := event.NewBus()
	cfg := &event.EventTypeConfig{Name: "test", DefaultSeverity: 50, DefaultTTL: 10, PerceptionMode: "global", Range: 100}
	evt := event.NewEvent(cfg, event.Vec3{}, "", 0)
	// 同一个 event 指针 Publish 两次
	bus.Publish(evt)
	bus.Publish(evt)
	if bus.ActiveCount() != 2 {
		t.Errorf("expected 2 (duplicate allowed), got %d", bus.ActiveCount())
	}
}

// === Config 攻击 ===

func TestAttack_ParseNPCTypeConfig_EmptyBTRefs(t *testing.T) {
	data := []byte(`{"type_name": "test", "fsm_ref": "civilian", "bt_refs": {}, "perception": {"visual_range": 100, "auditory_range": 200}}`)
	cfg, err := npc.ParseNPCTypeConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.BTRefs) != 0 {
		t.Error("expected empty bt_refs")
	}
}

func TestAttack_ParseEventTypeConfig_AllZero(t *testing.T) {
	data := []byte(`{"name": "zero", "default_severity": 0, "default_ttl": 0, "perception_mode": "global", "range": 0}`)
	var cfg event.EventTypeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "zero" {
		t.Error("expected name zero")
	}
}
