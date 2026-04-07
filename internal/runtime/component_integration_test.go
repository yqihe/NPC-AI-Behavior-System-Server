package runtime_test

import (
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
)

// createFromTemplate 从组件化模板创建 NPC 实例
func createFromTemplate(t *testing.T, id string, pos event.Vec3, templateName string, src config.Source, btReg *bt.Registry, compReg *component.Registry) *npc.Instance {
	t.Helper()
	raw, err := src.LoadNPCTemplate(templateName)
	if err != nil {
		t.Fatalf("load template %q: %v", templateName, err)
	}
	tmpl, err := npc.ParseNPCTemplate(raw)
	if err != nil {
		t.Fatalf("parse template %q: %v", templateName, err)
	}
	inst, err := npc.NewInstanceFromTemplate(id, pos, tmpl, compReg, src, btReg)
	if err != nil {
		t.Fatalf("create from template %q: %v", templateName, err)
	}
	return inst
}

// --- 场景：simple NPC 不走 AI 管线 ---

func TestComponentIntegration_SimpleNPC_NoAIPipeline(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	// 创建 simple NPC（蝴蝶，无 behavior/perception）
	inst := createFromTemplate(t, "butterfly_1", event.Vec3{100, 5, 200}, "butterfly_01", src, btReg, compReg)

	// 验证没有 behavior 和 perception 组件
	if inst.HasComponent("behavior") {
		t.Error("simple NPC should not have behavior component")
	}
	if inst.HasComponent("perception") {
		t.Error("simple NPC should not have perception component")
	}
	if !inst.HasComponent("movement") {
		t.Error("simple NPC should have movement component")
	}

	// 创建 Scheduler 并 Tick
	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	// 发布事件
	explosionCfg := evtTypes["explosion"]
	evt := event.NewEvent(explosionCfg, event.Vec3{100, 0, 200}, "bomb_1", 80, "")
	bus.Publish(evt)

	// Tick
	sched.Tick(0.1)

	// simple NPC 不应有威胁（跳过了感知和决策）
	level, _ := blackboard.Get(inst.BB, blackboard.KeyThreatLevel)
	if level != 0 {
		t.Errorf("simple NPC should have threat_level 0, got %f", level)
	}

	// movement 组件应该被 Tick（写了 move_state）
	moveState, ok := blackboard.Get(inst.BB, blackboard.KeyMoveState)
	if !ok {
		t.Error("move_state should be set by movement Tick")
	}
	if moveState != "moving" && moveState != "arrived" {
		t.Errorf("move_state = %q, want moving or arrived", moveState)
	}
}

// --- 场景：reactive NPC 完整 AI 管线 ---

func TestComponentIntegration_ReactiveNPC_FullPipeline(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	// 创建 reactive NPC（灰狼）
	inst := createFromTemplate(t, "wolf_1", event.Vec3{300, 0, 400}, "wolf_common", src, btReg, compReg)

	// 验证有 behavior 和 perception 组件
	if !inst.HasComponent("behavior") {
		t.Fatal("reactive NPC should have behavior component")
	}
	if !inst.HasComponent("perception") {
		t.Fatal("reactive NPC should have perception component")
	}

	// 验证 FSM 可用
	beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior")
	if !ok {
		t.Fatal("GetComponent behavior failed")
	}
	if beh.FSM == nil {
		t.Fatal("behavior FSM should be initialized")
	}
	if beh.FSM.Current() != "Idle" {
		t.Fatalf("expected Idle, got %s", beh.FSM.Current())
	}

	// 创建 Scheduler 并驱动完整管线
	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	// 发布爆炸事件（近距离）
	explosionCfg := evtTypes["explosion"]
	evt := event.NewEvent(explosionCfg, event.Vec3{350, 0, 400}, "bomb_1", 80, "")
	bus.Publish(evt)

	// Tick → 应走完整管线
	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))
	sched.Tick(0.1)

	// 应有威胁（走了感知+决策）
	level, _ := blackboard.Get(inst.BB, blackboard.KeyThreatLevel)
	if level <= 0 {
		t.Errorf("reactive NPC should have threat_level > 0, got %f", level)
	}

	// FSM 应转换（走了 behavior Tick）
	if beh.FSM.Current() == "Idle" {
		t.Error("reactive NPC FSM should have transitioned from Idle")
	}

	// movement 组件也应被 Tick
	moveState, ok := blackboard.Get(inst.BB, blackboard.KeyMoveState)
	if !ok {
		t.Error("move_state should be set")
	}
	if moveState != "moving" && moveState != "arrived" {
		t.Errorf("move_state = %q, want moving or arrived", moveState)
	}
}

// --- 场景：混合 Tick（simple + reactive 共存）---

func TestComponentIntegration_MixedTick(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	butterfly := createFromTemplate(t, "b1", event.Vec3{100, 5, 200}, "butterfly_01", src, btReg, compReg)
	wolf := createFromTemplate(t, "w1", event.Vec3{300, 0, 400}, "wolf_common", src, btReg, compReg)

	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(butterfly)
	reg.Add(wolf)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	// 发布事件（在狼附近）
	explosionCfg := evtTypes["explosion"]
	evt := event.NewEvent(explosionCfg, event.Vec3{350, 0, 400}, "bomb_1", 80, "")
	bus.Publish(evt)

	blackboard.Set(butterfly.BB, blackboard.KeyCurrentTime, int64(10000))
	blackboard.Set(wolf.BB, blackboard.KeyCurrentTime, int64(10000))
	sched.Tick(0.1)

	// 蝴蝶：无威胁（无感知组件）
	bLevel, _ := blackboard.Get(butterfly.BB, blackboard.KeyThreatLevel)
	if bLevel != 0 {
		t.Errorf("butterfly threat_level = %f, want 0", bLevel)
	}

	// 狼：有威胁（有感知+决策）
	wLevel, _ := blackboard.Get(wolf.BB, blackboard.KeyThreatLevel)
	if wLevel <= 0 {
		t.Errorf("wolf threat_level = %f, want > 0", wLevel)
	}

	// 两者都有 move_state（都有 movement 组件）
	bMove, _ := blackboard.Get(butterfly.BB, blackboard.KeyMoveState)
	wMove, _ := blackboard.Get(wolf.BB, blackboard.KeyMoveState)
	if bMove != "moving" && bMove != "arrived" {
		t.Errorf("butterfly move_state = %q, want moving or arrived", bMove)
	}
	if wMove != "moving" && wMove != "arrived" {
		t.Errorf("wolf move_state = %q, want moving or arrived", wMove)
	}
}
