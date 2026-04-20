package runtime_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc/npctest"
)

// --- 注意力容量裁剪 ---

func TestPerceptionIntegration_AttentionCapacity(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	// 创建 reactive NPC，attention_capacity=3（通过 extras.perception 覆盖默认 5）
	extras := map[string]json.RawMessage{
		"perception": []byte(`{"visual_range":150,"auditory_range":300,"attention_capacity":3}`),
	}
	inst, err := npctest.NewInstanceWithExtras("wolf_attn", event.Vec3{X: 300, Z: 400},
		wolfADMINTemplate(nil), extras, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	// 验证 attention_capacity
	perc, ok := npc.GetComponent[*component.PerceptionComponent](inst, "perception")
	if !ok {
		t.Fatal("missing perception component")
	}
	if perc.AttentionCapacity != 3 {
		t.Fatalf("attention_capacity = %d, want 3", perc.AttentionCapacity)
	}

	// 发布 5 个事件（都在感知范围内，不同距离）
	bus := event.NewBus()
	explosionCfg := evtTypes["explosion"]
	for i := 0; i < 5; i++ {
		dist := float64(50 + i*30) // 50, 80, 110, 140, 170m
		evt := event.NewEvent(explosionCfg, event.Vec3{X: 300 + dist, Y: 0, Z: 400}, "bomb", 80, "")
		bus.Publish(evt)
	}

	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))
	sched.Tick(0.1)

	// 决策中心应该收到最强的事件（最近的）
	level, _ := blackboard.Get(inst.BB, blackboard.KeyThreatLevel)
	if level <= 0 {
		t.Fatal("threat_level should be > 0")
	}

	// 无法直接验证"只收到 3 个"，但可以验证选的是最强的（最近 50m 的那个）
	// 50m 距离, min(150, 500)=150, strength = 80*(1-50/150) = 53.3
	if level < 50 {
		t.Errorf("expected high threat (closest event), got %f", level)
	}
}

// --- 区域隔离 ---

func TestPerceptionIntegration_ZoneIsolation(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	// 创建 NPC 在 meadow 区域
	inst, err := npctest.NewInstanceWithExtras("wolf_zone", event.Vec3{X: 300, Z: 400},
		wolfADMINTemplate(nil), nil, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	// 设置 NPC 区域
	posComp, _ := npc.GetComponent[*component.PositionComponent](inst, "position")
	posComp.ZoneID = "meadow"

	bus := event.NewBus()
	explosionCfg := evtTypes["explosion"]

	// 事件在不同区域（mountain），auditory 模式
	evtOtherZone := event.NewEvent(explosionCfg, event.Vec3{X: 350, Y: 0, Z: 400}, "bomb_far", 80, "mountain")
	bus.Publish(evtOtherZone)

	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))
	sched.Tick(0.1)

	// 不同区域的 auditory 事件不应被感知
	level, _ := blackboard.Get(inst.BB, blackboard.KeyThreatLevel)
	if level > 0 {
		t.Errorf("cross-zone auditory event should not be perceived, threat_level = %f", level)
	}

	// 发布 global 事件（应无视区域）
	earthquakeCfg := evtTypes["earthquake"]
	evtGlobal := event.NewEvent(earthquakeCfg, event.Vec3{X: 0, Y: 0, Z: 0}, "quake", 100, "mountain")
	bus.Publish(evtGlobal)

	sched.Tick(0.1)

	level2, _ := blackboard.Get(inst.BB, blackboard.KeyThreatLevel)
	if level2 <= 0 {
		t.Error("global event should be perceived regardless of zone")
	}
}

// --- 强度传递到决策中心 ---

func TestPerceptionIntegration_StrengthPassthrough(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	inst, err := npctest.NewInstanceWithExtras("wolf_str", event.Vec3{X: 100, Z: 100},
		wolfADMINTemplate(nil), nil, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	bus := event.NewBus()
	explosionCfg := evtTypes["explosion"]

	// 近距离事件（10m）和远距离事件（140m）— 相对于 NPC 位置 (100,0,100)
	evtClose := event.NewEvent(explosionCfg, event.Vec3{X: 110, Y: 0, Z: 100}, "close", 80, "")
	evtFar := event.NewEvent(explosionCfg, event.Vec3{X: 240, Y: 0, Z: 100}, "far", 80, "")
	bus.Publish(evtClose)
	bus.Publish(evtFar)

	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))
	sched.Tick(0.1)

	// 决策中心应选择近距离事件（强度更高）
	source, _ := blackboard.Get(inst.BB, blackboard.KeyThreatSource)
	if source != evtClose.ID {
		t.Errorf("expected closest event as threat source, got %s (want %s)", source, evtClose.ID)
	}

	level, _ := blackboard.Get(inst.BB, blackboard.KeyThreatLevel)
	// 10m 距离, min(300,500)=300, strength = 80*(1-10/300) ≈ 77.3
	if level < 70 {
		t.Errorf("expected high strength for close event, got %f", level)
	}
}
