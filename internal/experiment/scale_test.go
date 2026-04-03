//go:build experiment

package experiment_test

import (
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment/modes"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

var behaviorCounts = []int{10, 50, 100, 150, 200}

func makeTestEvents() ([]*event.Event, map[string]*event.EventTypeConfig) {
	evtType := &event.EventTypeConfig{Name: "test_explosion", DefaultSeverity: 80, DefaultTTL: 15, PerceptionMode: "auditory", Range: 500}
	evtTypes := map[string]*event.EventTypeConfig{"test_explosion": evtType}
	events := []*event.Event{
		event.NewEvent(evtType, event.Vec3{X: 50}, "src", 80),
	}
	return events, evtTypes
}

// --- 图 1：行为数量 vs 单 Tick 耗时 ---

func TestScale_TickLatency(t *testing.T) {
	btReg := bt.DefaultRegistry()
	events, evtTypes := makeTestEvents()
	const iterations = 10000

	t.Log("\n=== 图 1: 行为数量 vs 单 Tick 耗时 (ns) ===")
	t.Log("| Behaviors | Hybrid     | PureFSM    | PureBT     |")
	t.Log("|-----------|------------|------------|------------|")

	for _, n := range behaviorCounts {
		cfg := experiment.GenerateScaleConfig(n)

		hybrid, err := modes.NewHybridFromScale("h", cfg, btReg)
		if err != nil {
			t.Fatalf("hybrid scale %d: %v", n, err)
		}
		pureFSM, err := modes.NewPureFSMFromScale("f", cfg)
		if err != nil {
			t.Fatalf("purefsm scale %d: %v", n, err)
		}
		pureBT, err := modes.NewPureBTFromScale("b", cfg, btReg)
		if err != nil {
			t.Fatalf("purebt scale %d: %v", n, err)
		}

		hNs := measureTickLatency(hybrid, events, evtTypes, iterations)
		fNs := measureTickLatency(pureFSM, events, evtTypes, iterations)
		bNs := measureTickLatency(pureBT, events, evtTypes, iterations)

		t.Logf("| %9d | %10d | %10d | %10d |", n, hNs, fNs, bNs)
	}
}

func measureTickLatency(npc experiment.ExperimentNPC, events []*event.Event, evtTypes map[string]*event.EventTypeConfig, iterations int) int64 {
	// 预热
	for i := 0; i < 10; i++ {
		blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(i)*100)
		npc.Tick(events, evtTypes, 0.1)
	}
	start := time.Now()
	for i := 0; i < iterations; i++ {
		blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(1000+i)*100)
		npc.Tick(events, evtTypes, 0.1)
	}
	return time.Since(start).Nanoseconds() / int64(iterations)
}

// --- 图 3：行为数量 vs 配置复杂度 ---

func TestScale_ConfigComplexity(t *testing.T) {
	t.Log("\n=== 图 3: 行为数量 vs 配置复杂度 ===")
	t.Log("| Behaviors | FSM Trans | BT Nodes | Hybrid FSM Trans | Hybrid BT Nodes |")
	t.Log("|-----------|-----------|----------|-----------------|-----------------|")

	for _, n := range behaviorCounts {
		cfg := experiment.GenerateScaleConfig(n)
		t.Logf("| %9d | %9d | %8d | %15d | %15d |",
			n, cfg.FSMTransCount, cfg.BTNodeCount, cfg.HybridFSMTrans, cfg.HybridBTTotal)
	}
}

// --- 图 4：边际扩展成本 ---

func TestScale_MarginalCost(t *testing.T) {
	t.Log("\n=== 图 4: 边际扩展成本 (加 1 个行为的增量) ===")
	t.Log("| From→To   | ΔFSM Trans | ΔBT Nodes | ΔHybrid FSM | ΔHybrid BT |")
	t.Log("|-----------|------------|-----------|-------------|------------|")

	for _, n := range []int{10, 50, 100, 200} {
		cfgN := experiment.GenerateScaleConfig(n)
		cfgN1 := experiment.GenerateScaleConfig(n + 1)
		t.Logf("| %3d→%3d   | %10d | %9d | %11d | %10d |",
			n, n+1,
			cfgN1.FSMTransCount-cfgN.FSMTransCount,
			cfgN1.BTNodeCount-cfgN.BTNodeCount,
			cfgN1.HybridFSMTrans-cfgN.HybridFSMTrans,
			cfgN1.HybridBTTotal-cfgN.HybridBTTotal)
	}
}

// --- 图 6：行为数量 vs 单事件响应墙钟时间 ---

func TestScale_EventResponseTime(t *testing.T) {
	btReg := bt.DefaultRegistry()
	const iterations = 10000

	t.Log("\n=== 图 6: 行为数量 vs 单事件响应墙钟时间 (ns) ===")
	t.Log("| Behaviors | Hybrid     | PureFSM    | PureBT     |")
	t.Log("|-----------|------------|------------|------------|")

	for _, n := range behaviorCounts {
		cfg := experiment.GenerateScaleConfig(n)

		hNs := measureEventResponse("h", cfg, btReg, "hybrid", iterations)
		fNs := measureEventResponse("f", cfg, btReg, "purefsm", iterations)
		bNs := measureEventResponse("b", cfg, btReg, "purebt", iterations)

		t.Logf("| %9d | %10d | %10d | %10d |", n, hNs, fNs, bNs)

		// R12: 不允许全零
		if hNs == 0 && fNs == 0 && bNs == 0 {
			t.Errorf("[R12] All response times are 0 at %d behaviors — metric broken", n)
		}
	}
}

func measureEventResponse(id string, cfg *experiment.ScaleConfig, btReg *bt.Registry, mode string, iterations int) int64 {
	evtType := &event.EventTypeConfig{Name: "test", DefaultSeverity: 80, DefaultTTL: 15, PerceptionMode: "auditory", Range: 500}
	evtTypes := map[string]*event.EventTypeConfig{"test": evtType}

	// 创建一个 NPC，跑 iterations 次 Tick，累积总时间再除
	var npc experiment.ExperimentNPC
	var err error
	switch mode {
	case "hybrid":
		npc, err = modes.NewHybridFromScale(id, cfg, btReg)
	case "purefsm":
		npc, err = modes.NewPureFSMFromScale(id, cfg)
	case "purebt":
		npc, err = modes.NewPureBTFromScale(id, cfg, btReg)
	}
	if err != nil {
		return -1
	}

	evt := event.NewEvent(evtType, event.Vec3{X: 50}, "src", 80)
	events := []*event.Event{evt}

	// 预热
	for i := 0; i < 100; i++ {
		blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(i)*100)
		npc.Tick(events, evtTypes, 0.1)
	}

	// 累积测量
	start := time.Now()
	for i := 0; i < iterations; i++ {
		blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(1000+i)*100)
		npc.Tick(events, evtTypes, 0.1)
	}
	return time.Since(start).Nanoseconds() / int64(iterations)
}
