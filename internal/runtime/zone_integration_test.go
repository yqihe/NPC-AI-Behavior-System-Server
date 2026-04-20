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
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/zone"
)

// butterflyInstance 为 zone 测试构造 butterfly NPC（走 npctest 路径，
// 不再依赖 configs/npc_templates/butterfly_01.json）
func butterflyInstance(t *testing.T, id string, pos event.Vec3, src config.Source, btReg *bt.Registry, compReg *component.Registry) *npc.Instance {
	t.Helper()
	inst, err := npctest.NewInstanceWithExtras(id, pos, butterflyADMINTemplate(nil),
		nil, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create butterfly %s: %v", id, err)
	}
	return inst
}

// --- 休眠区域 NPC 不 Tick ---

func TestZoneIntegration_SleepSkipsTick(t *testing.T) {
	src, btReg, compReg, evtTypes := newTestEnv(t)

	// 创建两个 NPC：一个在 meadow（将休眠），一个在 forest（活跃）
	meadowNPC := butterflyInstance(t, "m1", event.Vec3{X: 100, Z: 200}, src, btReg, compReg)
	forestNPC := butterflyInstance(t, "f1", event.Vec3{X: 300, Z: 400}, src, btReg, compReg)

	// 手动设 zone_id
	if pos, ok := npc.GetComponent[*component.PositionComponent](meadowNPC, "position"); ok {
		pos.ZoneID = "meadow"
	}
	if pos, ok := npc.GetComponent[*component.PositionComponent](forestNPC, "position"); ok {
		pos.ZoneID = "forest"
	}

	zm := zone.NewZoneManager()
	zm.AddZone(&zone.Zone{ID: "meadow", Name: "草原", Active: true})
	zm.AddZone(&zone.Zone{ID: "forest", Name: "森林", Active: true})

	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(meadowNPC)
	reg.Add(forestNPC)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)
	sched.ZoneManager = zm

	// 记录初始位置
	_ = meadowNPC.Position.X // used after sleep
	forestStartX := forestNPC.Position.X

	// Tick 几次，两个都应移动
	for i := 0; i < 10; i++ {
		sched.Tick(0.1)
	}

	// 休眠 meadow
	zm.Sleep("meadow")

	// 记录休眠后的 meadow 位置
	meadowAfterSleep := meadowNPC.Position.X

	// 再 Tick 几次
	for i := 0; i < 10; i++ {
		sched.Tick(0.1)
	}

	// meadow NPC 位置不应该变（被跳过了）
	if meadowNPC.Position.X != meadowAfterSleep {
		t.Errorf("sleeping zone NPC should not move: before=%f after=%f", meadowAfterSleep, meadowNPC.Position.X)
	}

	// forest NPC 位置应该变
	if forestNPC.Position.X == forestStartX {
		t.Error("active zone NPC should have moved")
	}
}

// --- 唤醒后恢复 ---

func TestZoneIntegration_WakeResumes(t *testing.T) {
	src, btReg, compReg, evtTypes := newTestEnv(t)

	inst := butterflyInstance(t, "b1", event.Vec3{X: 100, Z: 200}, src, btReg, compReg)
	if pos, ok := npc.GetComponent[*component.PositionComponent](inst, "position"); ok {
		pos.ZoneID = "meadow"
	}

	zm := zone.NewZoneManager()
	zm.AddZone(&zone.Zone{ID: "meadow", Active: false}) // 起始休眠

	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)
	sched.ZoneManager = zm

	posBeforeWake := inst.Position.X

	// Tick 时 meadow 休眠，NPC 不动
	for i := 0; i < 5; i++ {
		sched.Tick(0.1)
	}
	if inst.Position.X != posBeforeWake {
		t.Error("sleeping NPC should not move before wake")
	}

	// 唤醒
	zm.Wake("meadow")

	// Tick 后应移动
	for i := 0; i < 20; i++ {
		sched.Tick(0.1)
	}

	moveState, _ := blackboard.Get(inst.BB, blackboard.KeyMoveState)
	if moveState != "moving" && moveState != "arrived" {
		t.Errorf("woken NPC should be moving, got %q", moveState)
	}
}

// --- 从配置批量生成 ---

func TestZoneIntegration_SpawnFromConfig(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	// 从 meadow.json 加载区域
	raw, err := src.LoadRegionConfig("meadow")
	if err != nil {
		t.Fatalf("load region: %v", err)
	}

	var z zone.Zone
	if err := json.Unmarshal(raw, &z); err != nil {
		t.Fatalf("parse region: %v", err)
	}
	z.Active = true

	reg := npc.NewRegistry()
	if err := z.Spawn(compReg, src, btReg, reg, nil); err != nil {
		t.Fatalf("spawn: %v", err)
	}

	// meadow.json 有 butterfly_01 count=3
	if reg.Count() != 3 {
		t.Errorf("NPC count = %d, want 3", reg.Count())
	}

	// 验证 zone_id 设置正确
	reg.ForEach(func(inst *npc.Instance) {
		if pos, ok := npc.GetComponent[*component.PositionComponent](inst, "position"); ok {
			if pos.ZoneID != "meadow" {
				t.Errorf("NPC %s zone_id = %q, want %q", inst.ID, pos.ZoneID, "meadow")
			}
		}
	})
}
