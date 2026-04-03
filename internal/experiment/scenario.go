//go:build experiment

package experiment

import (
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

// ScenarioEvent 场景中的一个事件
type ScenarioEvent struct {
	AtTick   int
	Type     string
	Position event.Vec3
	Severity float64
}

// ExpectedState 某个 Tick 的期望状态检查点
type ExpectedState struct {
	AtTick        int
	ExpectedState string
}

// BBCheckpoint 某个 Tick 的 BB 值检查点
type BBCheckpoint struct {
	AtTick   int
	Key      string // BB key 名称
	Expected string // 期望值（空字符串 = 该 key 应不存在或为空）
	NonEmpty bool   // true = 只要非空就通过
}

// Scenario 一个完整的实验场景
type Scenario struct {
	Name          string
	NPCType       string
	NPCPosition   event.Vec3
	Events        []ScenarioEvent
	Expected      []ExpectedState
	BBCheckpoints []BBCheckpoint
	TotalTicks    int
	DeltaTime     float64
}

// --- 定性场景 ---

// ScenarioDistanceTrap 距离陷阱——证明 DC 不可替代
func ScenarioDistanceTrap() *Scenario {
	return &Scenario{
		Name: "distance_trap", NPCType: "civilian",
		NPCPosition: event.Vec3{}, TotalTicks: 30, DeltaTime: 0.1,
		Events: []ScenarioEvent{
			{AtTick: 3, Type: "gunshot", Position: event.Vec3{X: 290, Z: 0}, Severity: 90},
		},
		Expected: []ExpectedState{
			{AtTick: 0, ExpectedState: "Idle"},
			{AtTick: 8, ExpectedState: "Alarmed"}, // 有 DC: threat=3 → Alarmed; 无 DC: threat=90 → Flee
		},
	}
}

// ScenarioMultiStepBehavior 多步骤行为——证明 BT 不可替代
func ScenarioMultiStepBehavior() *Scenario {
	return &Scenario{
		Name: "multi_step_behavior", NPCType: "civilian",
		NPCPosition: event.Vec3{}, TotalTicks: 30, DeltaTime: 0.1,
		Events: []ScenarioEvent{
			{AtTick: 3, Type: "explosion", Position: event.Vec3{X: 50, Z: 0}, Severity: 80},
		},
		Expected: []ExpectedState{
			{AtTick: 0, ExpectedState: "Idle"},
			{AtTick: 8, ExpectedState: "Flee"},
		},
		BBCheckpoints: []BBCheckpoint{
			{AtTick: 10, Key: "current_action", Expected: "run_away"},
		},
	}
}

// ScenarioStateLifecycle 状态生命周期——证明 FSM 不可替代
func ScenarioStateLifecycle() *Scenario {
	return &Scenario{
		Name: "state_lifecycle", NPCType: "civilian",
		NPCPosition: event.Vec3{}, TotalTicks: 80, DeltaTime: 0.5,
		Events: []ScenarioEvent{
			{AtTick: 3, Type: "shout", Position: event.Vec3{X: 30, Z: 0}, Severity: 30},
			{AtTick: 15, Type: "gunshot", Position: event.Vec3{X: 30, Z: 0}, Severity: 90},
		},
		Expected: []ExpectedState{
			{AtTick: 0, ExpectedState: "Idle"},
			{AtTick: 6, ExpectedState: "Alarmed"},
			{AtTick: 20, ExpectedState: "Flee"},
		},
		BBCheckpoints: []BBCheckpoint{
			{AtTick: 8, Key: "alert_start_tick", NonEmpty: true},
			{AtTick: 22, Key: "exit_cleanup_done", Expected: "alarmed_cleaned"},
		},
	}
}

// ScenarioCivilian3Events 基线对照
func ScenarioCivilian3Events() *Scenario {
	return &Scenario{
		Name: "civilian_3events", NPCType: "civilian",
		NPCPosition: event.Vec3{}, TotalTicks: 200, DeltaTime: 0.5,
		Events: []ScenarioEvent{
			{AtTick: 5, Type: "explosion", Position: event.Vec3{X: 100, Z: 0}, Severity: 80},
			{AtTick: 100, Type: "shout", Position: event.Vec3{X: 50, Z: 0}, Severity: 30},
			{AtTick: 110, Type: "gunshot", Position: event.Vec3{X: 50, Z: 0}, Severity: 90},
		},
		Expected: []ExpectedState{
			{AtTick: 0, ExpectedState: "Idle"},
			{AtTick: 8, ExpectedState: "Flee"},
			{AtTick: 80, ExpectedState: "Idle"},
			{AtTick: 115, ExpectedState: "Flee"},
		},
	}
}
