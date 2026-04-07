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
			input := s.buildDecisionInput(inst, perceived)
			s.Decision.Evaluate(inst.BB, inst.Position, input, s.EvtTypes, dt)

			// 4c. 写入威胁记忆
			s.writeThreatMemory(inst, perceived, now)

			beh.FSM.Tick(inst.BB)
			state := beh.FSM.Current()
			if tree, ok := beh.BTrees[state]; ok {
				ctx := &bt.Context{BB: inst.BB}
				tree.Tick(ctx)
			}
		} else if inst.FSM != nil {
			// v2 兼容：旧路径创建的 NPC
			input := decision.DecisionInput{Perceived: perceived, Weights: decision.DefaultWeights}
			s.Decision.Evaluate(inst.BB, inst.Position, input, s.EvtTypes, dt)
			inst.Tick()
		}

		// --- 通用组件 Tick ---
		inst.TickComponents(dt)
	})
}

// writeThreatMemory 将最高威胁事件写入 NPC 记忆
func (s *Scheduler) writeThreatMemory(inst *npc.Instance, perceived []perception.PerceiveResult, now int64) {
	if len(perceived) == 0 {
		return
	}
	mem, ok := npc.GetComponent[*component.MemoryComponent](inst, "memory")
	if !ok || !mem.SupportsType("threat") {
		return
	}
	// 取最高强度事件
	best := perceived[0]
	for _, pr := range perceived[1:] {
		if pr.Strength > best.Strength {
			best = pr
		}
	}
	if best.Event.SourceID == "" {
		return
	}
	mem.AddMemory(component.MemoryEntry{
		Type:      "threat",
		TargetID:  best.Event.SourceID,
		Value:     best.Strength,
		Timestamp: now,
		TTL:       mem.DecayTime,
	})
}

// buildDecisionInput 从 NPC 组件组装决策输入
func (s *Scheduler) buildDecisionInput(inst *npc.Instance, perceived []perception.PerceiveResult) decision.DecisionInput {
	input := decision.DecisionInput{
		Perceived: perceived,
		Weights:   decision.DefaultWeights,
	}

	// 读取 personality 权重
	if pers, ok := npc.GetComponent[*component.PersonalityComponent](inst, "personality"); ok {
		input.Weights = decision.DecisionWeights{
			Threat:  pers.DecisionWeights.Threat,
			Needs:   pers.DecisionWeights.Needs,
			Emotion: pers.DecisionWeights.Emotion,
		}
	}

	// 读取需求紧迫度
	if needs, ok := npc.GetComponent[*component.NeedsComponent](inst, "needs"); ok {
		input.NeedUrgency = calcNeedUrgency(inst.BB, needs)
	}

	// 读取情绪强度
	emotionVal, _ := blackboard.Get(inst.BB, blackboard.KeyEmotionDominantVal)
	input.EmotionValue = emotionVal

	return input
}

// calcNeedUrgency 从 BB 读取最低需求值，计算紧迫度 (max-current)/max*100
func calcNeedUrgency(bb *blackboard.Blackboard, needs *component.NeedsComponent) float64 {
	lowestName, ok := blackboard.Get(bb, blackboard.KeyNeedLowest)
	if !ok || lowestName == "" {
		return 0
	}
	lowestVal, _ := blackboard.Get(bb, blackboard.KeyNeedLowestVal)
	for _, n := range needs.NeedTypes {
		if n.Name == lowestName {
			if n.Max <= 0 {
				return 0
			}
			return (n.Max - lowestVal) / n.Max * 100
		}
	}
	return 0
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
