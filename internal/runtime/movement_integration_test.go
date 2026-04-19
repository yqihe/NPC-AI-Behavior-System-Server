package runtime_test

import (
	"fmt"
	"math"
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

func TestMovementIntegration_WanderPositionChanges(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	// 蝴蝶：wander 模式
	inst := createFromTemplate(t, "b1", event.Vec3{100, 5, 200}, "butterfly_01", src, btReg, compReg)
	spawnX, spawnZ := 100.0, 200.0

	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	// 多次 Tick
	for i := 0; i < 50; i++ {
		sched.Tick(0.1)
	}

	// 位置应变化
	x := inst.Position.X
	z := inst.Position.Z
	moved := math.Abs(x-spawnX) > 0.01 || math.Abs(z-spawnZ) > 0.01
	if !moved {
		t.Error("wander NPC should have moved after 50 ticks")
	}

	// BB 位置应与 Instance.Position 同步
	bbX, _ := blackboard.Get(inst.BB, blackboard.KeyNPCPosX)
	bbZ, _ := blackboard.Get(inst.BB, blackboard.KeyNPCPosZ)
	if math.Abs(bbX-x) > 0.001 || math.Abs(bbZ-z) > 0.001 {
		t.Errorf("BB pos (%f,%f) != Instance pos (%f,%f)", bbX, bbZ, x, z)
	}
}

func TestMovementIntegration_PatrolCycles(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	// 狼：ADMIN shape 走 wander 模式（T1 翻译层推断 move_type=wander；
	// 原 wolf_common.json 的 patrol 语义由 BT 层承担，不影响 arrived 次数断言）
	inst, err := npctest.NewInstanceWithExtras("w1", event.Vec3{X: 300, Z: 400},
		wolfADMINTemplate(nil), nil, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create wolf: %v", err)
	}

	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	// 多次 Tick 让巡逻进行
	arrivedCount := 0
	for i := 0; i < 200; i++ {
		sched.Tick(0.1)
		state, _ := blackboard.Get(inst.BB, blackboard.KeyMoveState)
		if state == "arrived" {
			arrivedCount++
		}
	}

	// 应至少到达过几个路点
	if arrivedCount < 2 {
		t.Errorf("patrol should have arrived at waypoints, arrivedCount = %d", arrivedCount)
	}
}

func TestMovementIntegration_MoveToNode(t *testing.T) {
	// 直接测试 move_to BT 节点
	btReg := bt.DefaultRegistry()
	bb := blackboard.New()

	blackboard.Set(bb, blackboard.KeyNPCPosX, 0.0)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, 0.0)

	// 注册目标坐标 key
	targetKeyX := blackboard.NewKey[float64]("test_target_x")
	targetKeyZ := blackboard.NewKey[float64]("test_target_z")
	blackboard.Set(bb, targetKeyX, 10.0)
	blackboard.Set(bb, targetKeyZ, 0.0)

	// 构建 move_to 节点
	nodeJSON := []byte(`{"type":"move_to","params":{"target_key_x":"test_target_x","target_key_z":"test_target_z","speed":50}}`)
	node, err := bt.BuildFromJSON(nodeJSON, btReg)
	if err != nil {
		t.Fatalf("build move_to: %v", err)
	}

	ctx := &bt.Context{BB: bb, DeltaTime: 0.1}

	// Tick 直到 Success
	var lastStatus bt.Status
	for i := 0; i < 100; i++ {
		lastStatus = node.Tick(ctx)
		if lastStatus == bt.Success {
			break
		}
	}

	if lastStatus != bt.Success {
		t.Errorf("move_to should reach target, last status = %d", lastStatus)
	}

	x, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
	if math.Abs(x-10) > 1.0 {
		t.Errorf("NPC should be at target x=10, got %f", x)
	}
}

func BenchmarkMovement_500NPCs(b *testing.B) {
	src := config.NewJSONSource(benchConfigsDir(b))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	raw, err := src.LoadNPCTemplate("butterfly_01")
	if err != nil {
		b.Fatal(err)
	}
	tmpl, err := npc.ParseNPCTemplate(raw)
	if err != nil {
		b.Fatal(err)
	}

	var instances []*npc.Instance
	for i := 0; i < 500; i++ {
		inst, err := npc.NewInstanceFromTemplate(
			fmt.Sprintf("b_%d", i),
			event.Vec3{X: float64(i * 10), Z: float64(i * 10)},
			tmpl, compReg, src, btReg,
		)
		if err != nil {
			b.Fatal(err)
		}
		instances = append(instances, inst)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, inst := range instances {
			inst.TickComponents(0.1)
		}
	}
}
