//go:build experiment

package experiment

import (
	"fmt"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

// ExperimentNPC 五种模式的统一接口
type ExperimentNPC interface {
	Tick(events []*event.Event, evtTypes map[string]*event.EventTypeConfig, dt float64) string
	State() string
	BB() *blackboard.Blackboard
}

// ModeEntry 一个模式的名称和实例
type ModeEntry struct {
	Name string
	NPC  ExperimentNPC
}

// Runner 驱动模式跑场景
type Runner struct {
	Scenario *Scenario
	EvtTypes map[string]*event.EventTypeConfig
}

// NewRunner 创建 Runner
func NewRunner(scenario *Scenario, evtTypes map[string]*event.EventTypeConfig) *Runner {
	return &Runner{Scenario: scenario, EvtTypes: evtTypes}
}

// RunMode 跑一种模式，返回结果
func (r *Runner) RunMode(npc ExperimentNPC, name string) *ModeResult {
	result := &ModeResult{
		ModeName: name,
		Records:  make([]TickRecord, 0, r.Scenario.TotalTicks),
	}

	prevState := npc.State()

	// 事件索引
	eventsByTick := make(map[int][]ScenarioEvent)
	for _, se := range r.Scenario.Events {
		eventsByTick[se.AtTick] = append(eventsByTick[se.AtTick], se)
	}

	// BB 检查点索引
	bbChecksByTick := make(map[int][]int)
	for i, chk := range r.Scenario.BBCheckpoints {
		bbChecksByTick[chk.AtTick] = append(bbChecksByTick[chk.AtTick], i)
	}
	bbCheckResults := make([]BBCheckResult, len(r.Scenario.BBCheckpoints))

	// 活跃事件
	var activeEvents []*event.Event

	for tick := 0; tick < r.Scenario.TotalTicks; tick++ {
		blackboard.Set(npc.BB(), blackboard.KeyCurrentTime, int64(tick)*100)

		// TTL 衰减
		alive := activeEvents[:0]
		for _, evt := range activeEvents {
			evt.TTL -= r.Scenario.DeltaTime
			if evt.TTL > 0 {
				alive = append(alive, evt)
			}
		}
		activeEvents = alive

		// 发布事件
		var arrivedType string
		if ses, ok := eventsByTick[tick]; ok {
			for _, se := range ses {
				typeCfg, ok := r.EvtTypes[se.Type]
				if !ok {
					continue
				}
				activeEvents = append(activeEvents, event.NewEvent(typeCfg, se.Position, "scenario", se.Severity, ""))
				arrivedType = se.Type
			}
		}

		// Tick
		state := npc.Tick(activeEvents, r.EvtTypes, r.Scenario.DeltaTime)
		transitioned := state != prevState

		threatLevel, _ := blackboard.Get(npc.BB(), blackboard.KeyThreatLevel)
		result.Records = append(result.Records, TickRecord{
			Tick: tick, State: state, ThreatLevel: threatLevel,
			EventArrived: arrivedType, Transitioned: transitioned,
		})

		// BB 检查点（在正确的 Tick 读取）
		if indices, ok := bbChecksByTick[tick]; ok {
			for _, idx := range indices {
				chk := r.Scenario.BBCheckpoints[idx]
				actual, ok := npc.BB().GetRaw(chk.Key)
				actualStr := ""
				if ok && actual != nil {
					actualStr = fmt.Sprintf("%v", actual)
				}
				pass := false
				if chk.NonEmpty {
					pass = actualStr != "" && actualStr != "0"
				} else {
					pass = actualStr == chk.Expected
				}
				bbCheckResults[idx] = BBCheckResult{
					Key: chk.Key, Expected: chk.Expected, Actual: actualStr, Pass: pass,
				}
			}
		}

		prevState = state
	}

	result.CalcMetrics(r.Scenario.Expected)
	r.evalExpressiveness(result)
	result.BBCheckResults = bbCheckResults
	return result
}

// RunAll 跑所有模式
func (r *Runner) RunAll(modes []ModeEntry) *ComparisonReport {
	report := &ComparisonReport{Scenario: r.Scenario.Name, Results: make([]*ModeResult, 0, len(modes))}
	for _, m := range modes {
		report.Results = append(report.Results, r.RunMode(m.NPC, m.Name))
	}
	return report
}

// evalExpressiveness 评估 M5
func (r *Runner) evalExpressiveness(result *ModeResult) {
	records := result.Records
	if len(records) == 0 {
		return
	}

	// Recovery: Flee → Idle
	inFlee := false
	for _, rec := range records {
		if rec.State == "Flee" {
			inFlee = true
		}
		if inFlee && rec.State == "Idle" {
			result.RecoveryOK = true
			break
		}
	}

	// Preemption: Alarmed → Flee (transition)
	inAlarmed := false
	for _, rec := range records {
		if rec.State == "Alarmed" {
			inAlarmed = true
		}
		if inAlarmed && rec.State == "Flee" && rec.Transitioned {
			result.PreemptionOK = true
			break
		}
	}

	// Arbitration: multi-event tick → high threat wins
	eventsByTick := make(map[int][]ScenarioEvent)
	for _, se := range r.Scenario.Events {
		eventsByTick[se.AtTick] = append(eventsByTick[se.AtTick], se)
	}
	result.ArbitrationOK = true
	for tick, evts := range eventsByTick {
		if len(evts) <= 1 {
			continue
		}
		maxSev := evts[0].Severity
		for _, e := range evts[1:] {
			if e.Severity > maxSev {
				maxSev = e.Severity
			}
		}
		for i := tick; i < len(records) && i < tick+10; i++ {
			if records[i].ThreatLevel >= maxSev*0.5 {
				break
			}
			if i == tick+9 {
				result.ArbitrationOK = false
			}
		}
	}
}
