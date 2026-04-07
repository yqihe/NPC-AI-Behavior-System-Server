//go:build experiment

package experiment_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/experiment/modes"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

var behaviorCounts = []int{10, 50, 100, 150, 200}

// 迭代次数：50 万次，保证总耗时 >> 计时精度
const scaleIterations = 500_000

func makeTestEvents() ([]*event.Event, map[string]*event.EventTypeConfig) {
	evtType := &event.EventTypeConfig{Name: "test_explosion", DefaultSeverity: 80, DefaultTTL: 15, PerceptionMode: "auditory", Range: 500}
	evtTypes := map[string]*event.EventTypeConfig{"test_explosion": evtType}
	events := []*event.Event{
		event.NewEvent(evtType, event.Vec3{X: 50}, "src", 80, ""),
	}
	return events, evtTypes
}

// stableNsPerOp 稳定测量：强制 GC + 锁 OS 线程 + 大量迭代
func stableNsPerOp(npc experiment.ExperimentNPC, events []*event.Event, evtTypes map[string]*event.EventTypeConfig, iterations int) int64 {
	// 预热 1000 次
	for i := 0; i < 1000; i++ {
		blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(i)*100)
		npc.Tick(events, evtTypes, 0.1)
	}

	// 强制 GC，避免测量期间触发
	runtime.GC()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	start := time.Now()
	for i := 0; i < iterations; i++ {
		blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(10000+i)*100)
		npc.Tick(events, evtTypes, 0.1)
	}
	total := time.Since(start).Nanoseconds()
	return total / int64(iterations)
}

// --- 图 1：行为数量 vs 单 Tick 耗时 ---

func TestScale_TickLatency(t *testing.T) {
	btReg := bt.DefaultRegistry()
	events, evtTypes := makeTestEvents()

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

		hNs := stableNsPerOp(hybrid, events, evtTypes, scaleIterations)
		fNs := stableNsPerOp(pureFSM, events, evtTypes, scaleIterations)
		bNs := stableNsPerOp(pureBT, events, evtTypes, scaleIterations)

		t.Logf("| %9d | %10d | %10d | %10d |", n, hNs, fNs, bNs)

		// 数据质量检查：不允许 0
		if hNs == 0 || fNs == 0 || bNs == 0 {
			t.Errorf("zero value detected at %d behaviors: h=%d f=%d b=%d", n, hNs, fNs, bNs)
		}
	}
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

	t.Log("\n=== 图 6: 行为数量 vs 单事件响应墙钟时间 (ns) ===")
	t.Log("| Behaviors | Hybrid     | PureFSM    | PureBT     |")
	t.Log("|-----------|------------|------------|------------|")

	for _, n := range behaviorCounts {
		cfg := experiment.GenerateScaleConfig(n)

		hNs := measureEventResponse("h", cfg, btReg, "hybrid")
		fNs := measureEventResponse("f", cfg, btReg, "purefsm")
		bNs := measureEventResponse("b", cfg, btReg, "purebt")

		t.Logf("| %9d | %10d | %10d | %10d |", n, hNs, fNs, bNs)

		if hNs == 0 && fNs == 0 && bNs == 0 {
			t.Errorf("[R12] All response times are 0 at %d behaviors", n)
		}
	}
}

func measureEventResponse(id string, cfg *experiment.ScaleConfig, btReg *bt.Registry, mode string) int64 {
	evtType := &event.EventTypeConfig{Name: "test", DefaultSeverity: 80, DefaultTTL: 15, PerceptionMode: "auditory", Range: 500}
	evtTypes := map[string]*event.EventTypeConfig{"test": evtType}

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

	evt := event.NewEvent(evtType, event.Vec3{X: 50}, "src", 80, "")
	events := []*event.Event{evt}

	return stableNsPerOp(npc, events, evtTypes, scaleIterations)
}
