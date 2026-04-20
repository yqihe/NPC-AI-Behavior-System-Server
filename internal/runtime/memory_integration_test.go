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

// --- 事件→记忆→情绪链路 ---

func TestMemoryIntegration_EventToMemoryToEmotion(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	extras := map[string]json.RawMessage{
		"memory":  []byte(`{"capacity":10,"memory_types":["threat"],"decay_time":30}`),
		"emotion": []byte(`{"emotion_states":[{"name":"fear","value":0,"accumulate_rate":20,"decay_rate":5}]}`),
	}
	inst, err := npctest.NewInstanceWithExtras("wolf_mem", event.Vec3{X: 100, Z: 100},
		wolfADMINTemplate(nil), extras, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	bus := event.NewBus()
	explosionCfg := evtTypes["explosion"]
	evt := event.NewEvent(explosionCfg, event.Vec3{X: 120, Y: 0, Z: 100}, "bomber_1", 80, "")
	bus.Publish(evt)

	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))

	// Tick 1: 感知→决策→写记忆→memory.Tick(写 BB)→emotion.Tick(读 BB, fear 累积)
	sched.Tick(0.1)

	// 验证记忆已写入
	mem, _ := npc.GetComponent[*component.MemoryComponent](inst, "memory")
	if !mem.HasMemory("threat", "bomber_1") {
		t.Error("should have threat memory for bomber_1")
	}

	// Tick 2: emotion 读到 memory_threat_value > 0，fear 累积
	sched.Tick(0.1)

	emotionVal, _ := blackboard.Get(inst.BB, blackboard.KeyEmotionDominantVal)
	if emotionVal <= 0 {
		t.Errorf("fear should have accumulated with threat memory, got %f", emotionVal)
	}
}

// --- 记忆过期后情绪恢复 ---

func TestMemoryIntegration_MemoryExpiry_EmotionRecovery(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	extras := map[string]json.RawMessage{
		"memory":  []byte(`{"capacity":10,"memory_types":["threat"],"decay_time":2}`), // 短 TTL
		"emotion": []byte(`{"emotion_states":[{"name":"fear","value":50,"accumulate_rate":10,"decay_rate":20}]}`),
	}
	inst, err := npctest.NewInstanceWithExtras("wolf_expire", event.Vec3{X: 100, Z: 100},
		wolfADMINTemplate(nil), extras, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	// 手动添加一条即将过期的威胁记忆
	mem, _ := npc.GetComponent[*component.MemoryComponent](inst, "memory")
	mem.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "old_enemy", Value: 60, Timestamp: 1000, TTL: 1.0})

	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))

	// Tick 多次让记忆过期（dt=0.5 × 3 = 1.5s > TTL 1.0s）
	for i := 0; i < 3; i++ {
		sched.Tick(0.5)
	}

	// 记忆应已过期
	if mem.HasMemory("threat", "old_enemy") {
		t.Error("threat memory should have expired")
	}

	// memory_threat_value 应为 0
	mtv, _ := blackboard.Get(inst.BB, blackboard.KeyMemoryThreatValue)
	if mtv != 0 {
		t.Errorf("memory_threat_value = %f, want 0 after expiry", mtv)
	}

	// 继续 Tick 几次，fear 应该衰减（无威胁记忆 → 只衰减不累积）
	for i := 0; i < 5; i++ {
		sched.Tick(0.5)
	}

	emotionVal, _ := blackboard.Get(inst.BB, blackboard.KeyEmotionDominantVal)
	if emotionVal > 10 {
		t.Errorf("fear should have decayed significantly after memory expiry, got %f", emotionVal)
	}
}

// --- 重复刺激强化 ---

func TestMemoryIntegration_RepeatedStimulus_Reinforcement(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	extras := map[string]json.RawMessage{
		"memory": []byte(`{"capacity":10,"memory_types":["threat"],"decay_time":60}`),
	}
	inst, err := npctest.NewInstanceWithExtras("wolf_reinforce", event.Vec3{X: 100, Z: 100},
		wolfADMINTemplate(nil), extras, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	bus := event.NewBus()
	explosionCfg := evtTypes["explosion"]

	reg := npc.NewRegistry()
	reg.Add(inst)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)

	blackboard.Set(inst.BB, blackboard.KeyCurrentTime, int64(10000))

	// 第一次事件
	evt1 := event.NewEvent(explosionCfg, event.Vec3{X: 120, Y: 0, Z: 100}, "bomber_1", 50, "")
	bus.Publish(evt1)
	sched.Tick(0.1)

	mem, _ := npc.GetComponent[*component.MemoryComponent](inst, "memory")
	e1, ok := mem.GetMemory("threat", "bomber_1")
	if !ok {
		t.Fatal("should have memory after first event")
	}
	firstValue := e1.Value

	// 清除事件总线
	bus.Tick(100) // 让事件过期

	// 第二次事件（更高 severity）
	evt2 := event.NewEvent(explosionCfg, event.Vec3{X: 110, Y: 0, Z: 100}, "bomber_1", 80, "")
	bus.Publish(evt2)
	sched.Tick(0.1)

	e2, ok := mem.GetMemory("threat", "bomber_1")
	if !ok {
		t.Fatal("should still have memory after reinforcement")
	}
	if e2.Value <= firstValue {
		t.Errorf("reinforced value %f should be > first value %f", e2.Value, firstValue)
	}
	if mem.Count() != 1 {
		t.Errorf("should still be 1 memory (reinforced, not duplicated), got %d", mem.Count())
	}
}
