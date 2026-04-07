package runtime

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// Scheduler 驱动整个 Runtime 的 Tick 循环
type Scheduler struct {
	EventBus *event.Bus
	Registry *npc.Registry
	Decision *decision.Center
	EvtTypes map[string]*event.EventTypeConfig
	TickRate time.Duration // Tick 间隔
}

// NewScheduler 创建 Tick 调度器
func NewScheduler(bus *event.Bus, reg *npc.Registry, dec *decision.Center, evtTypes map[string]*event.EventTypeConfig, tickRate time.Duration) *Scheduler {
	return &Scheduler{
		EventBus: bus,
		Registry: reg,
		Decision: dec,
		EvtTypes: evtTypes,
		TickRate: tickRate,
	}
}

// Tick 执行一次完整的 Tick 循环
func (s *Scheduler) Tick(dt float64) {
	// 1. 事件 TTL 衰减
	s.EventBus.Tick(dt)

	// 2. 获取当前活跃事件快照
	activeEvents := s.EventBus.Active()

	// 3. 更新当前时间（毫秒）
	now := time.Now().UnixMilli()

	// 4. 遍历所有 NPC
	s.Registry.ForEach(func(inst *npc.Instance) {
		// 更新时间
		blackboard.Set(inst.BB, blackboard.KeyCurrentTime, now)

		// --- AI 管线（仅有对应组件的 NPC 执行）---

		// 4a. 感知过滤（需要 perception 组件）
		var perceived []perception.PerceiveResult
		if perc, ok := npc.GetComponent[*component.PerceptionComponent](inst, "perception"); ok {
			perceived = s.filterPerception(inst, perc, activeEvents)
		} else if inst.Perception != nil {
			// v2 兼容：旧路径创建的 NPC，用 CanPerceive + CalcThreat 包装
			for _, evt := range activeEvents {
				typeCfg, ok := s.EvtTypes[evt.Type]
				if !ok {
					slog.Warn("scheduler.event_type_not_found", "event_type", evt.Type)
					continue
				}
				if perception.CanPerceive(inst.Position, inst.Perception, evt, typeCfg) {
					strength := decision.CalcThreat(evt.Severity, inst.Position, evt.Position, typeCfg.Range)
					perceived = append(perceived, perception.PerceiveResult{Event: evt, Strength: strength})
				}
			}
		}

		// 4b. 决策 + FSM + BT（需要 behavior 组件或旧字段）
		if beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior"); ok {
			s.Decision.Evaluate(inst.BB, inst.Position, perceived, s.EvtTypes, dt)
			beh.FSM.Tick(inst.BB)
			state := beh.FSM.Current()
			if tree, ok := beh.BTrees[state]; ok {
				ctx := &bt.Context{BB: inst.BB}
				tree.Tick(ctx)
			}
		} else if inst.FSM != nil {
			// v2 兼容：旧路径创建的 NPC
			s.Decision.Evaluate(inst.BB, inst.Position, perceived, s.EvtTypes, dt)
			inst.Tick()
		}

		// --- 通用组件 Tick ---
		inst.TickComponents(dt)
	})
}

// filterPerception 感知过滤：区域隔离 → 强度计算 → 注意力裁剪
func (s *Scheduler) filterPerception(inst *npc.Instance, perc *component.PerceptionComponent, events []*event.Event) []perception.PerceiveResult {
	// 获取 NPC 所在区域
	npcZoneID := ""
	if pos, ok := npc.GetComponent[*component.PositionComponent](inst, "position"); ok {
		npcZoneID = pos.ZoneID
	}

	cfg := &perception.PerceptionConfig{
		VisualRange:   perc.VisualRange,
		AuditoryRange: perc.AuditoryRange,
	}

	var results []perception.PerceiveResult
	for _, evt := range events {
		typeCfg, ok := s.EvtTypes[evt.Type]
		if !ok {
			slog.Warn("scheduler.event_type_not_found", "event_type", evt.Type)
			continue
		}
		// 区域过滤
		if perception.ShouldFilterByZone(npcZoneID, evt.ZoneID, typeCfg.PerceptionMode) {
			continue
		}
		// 强度计算
		strength := perception.CalcStrength(inst.Position, cfg, evt, typeCfg)
		if strength > 0 {
			results = append(results, perception.PerceiveResult{Event: evt, Strength: strength})
		}
	}

	// 注意力裁剪：按强度降序，保留前 AttentionCapacity 个
	if perc.AttentionCapacity > 0 && len(results) > perc.AttentionCapacity {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Strength > results[j].Strength
		})
		results = results[:perc.AttentionCapacity]
	}

	return results
}

// Run 启动 Tick 循环（阻塞，直到 ctx 取消）
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.TickRate)
	defer ticker.Stop()

	lastTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			dt := now.Sub(lastTime).Seconds()
			lastTime = now
			s.Tick(dt)
		}
	}
}
