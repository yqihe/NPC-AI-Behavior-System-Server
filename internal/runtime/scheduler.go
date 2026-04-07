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
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/metrics"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/social"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/zone"
)

// npcTickState 单个 NPC 在 Tick 中的暂存状态
type npcTickState struct {
	inst      *npc.Instance
	perceived []perception.PerceiveResult
}

// Scheduler 驱动整个 Runtime 的 Tick 循环
type Scheduler struct {
	EventBus     *event.Bus
	Registry     *npc.Registry
	Decision     *decision.Center
	EvtTypes     map[string]*event.EventTypeConfig
	TickRate     time.Duration
	GroupManager *social.GroupManager // 可选，nil 时不做群组处理
	ZoneManager  *zone.ZoneManager  // 可选，nil 时不做区域过滤
	Metrics      *metrics.Metrics   // 可选，nil 时不采集指标
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
	tickStart := time.Now()

	// 1. 事件 TTL 衰减
	s.EventBus.Tick(dt)

	// 2. 获取当前活跃事件快照
	activeEvents := s.EventBus.Active()

	// 3. 更新当前时间（毫秒）
	now := time.Now().UnixMilli()

	// --- 第一遍：感知 ---
	var states []npcTickState

	s.Registry.ForEach(func(inst *npc.Instance) {
		// 跳过休眠区域的 NPC
		if s.ZoneManager != nil {
			zoneID := ""
			if pos, ok := npc.GetComponent[*component.PositionComponent](inst, "position"); ok {
				zoneID = pos.ZoneID
			}
			if !s.ZoneManager.IsActive(zoneID) {
				return
			}
		}

		blackboard.Set(inst.BB, blackboard.KeyCurrentTime, now)

		var perceived []perception.PerceiveResult
		if perc, ok := npc.GetComponent[*component.PerceptionComponent](inst, "perception"); ok {
			perceived = s.filterPerception(inst, perc, activeEvents)
		} else if inst.Perception != nil {
			for _, evt := range activeEvents {
				typeCfg, ok := s.EvtTypes[evt.Type]
				if !ok {
					continue
				}
				if perception.CanPerceive(inst.Position, inst.Perception, evt, typeCfg) {
					strength := decision.CalcThreat(evt.Severity, inst.Position, evt.Position, typeCfg.Range)
					perceived = append(perceived, perception.PerceiveResult{Event: evt, Strength: strength})
				}
			}
		}

		states = append(states, npcTickState{inst: inst, perceived: perceived})
	})

	// --- 群组感知共享 ---
	if s.GroupManager != nil {
		s.shareGroupPerception(states)
		s.updateFollowerTargets()
	}

	// --- 第二遍：决策 + 行为 ---
	for i := range states {
		st := &states[i]
		inst := st.inst

		if beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior"); ok {
			input := s.buildDecisionInput(inst, st.perceived)
			s.Decision.Evaluate(inst.BB, inst.Position, input, s.EvtTypes, dt)
			s.writeThreatMemory(inst, st.perceived, now)

			beh.FSM.Tick(inst.BB)
			state := beh.FSM.Current()
			if tree, ok := beh.BTrees[state]; ok {
				ctx := &bt.Context{BB: inst.BB, DeltaTime: dt}
				tree.Tick(ctx)
			}
		} else if inst.FSM != nil {
			input := decision.DecisionInput{NPCID: inst.ID, Perceived: st.perceived, Weights: decision.DefaultWeights}
			s.Decision.Evaluate(inst.BB, inst.Position, input, s.EvtTypes, dt)
			inst.Tick()
		}

		inst.TickComponents(dt)
		inst.SyncPosition()
	}

	// --- 群体状态传播 ---
	if s.GroupManager != nil {
		s.propagateGroupState()
	}

	// --- 指标采集 ---
	if s.Metrics != nil {
		duration := time.Since(tickStart).Seconds()
		zoneCounts := make(map[string]int)
		sleepingCount := 0
		for _, st := range states {
			zoneID := ""
			if pos, ok := npc.GetComponent[*component.PositionComponent](st.inst, "position"); ok {
				zoneID = pos.ZoneID
			}
			if zoneID == "" {
				zoneID = "global"
			}
			zoneCounts[zoneID]++
		}
		if s.ZoneManager != nil {
			for _, z := range s.ZoneManager.AllZones() {
				if !z.Active {
					sleepingCount += len(z.NPCIDs())
				}
			}
		}
		s.Metrics.RecordTick(duration, len(states), zoneCounts, sleepingCount)
	}
}

// shareGroupPerception 群组感知共享：同组成员的 perceived 合并去重
func (s *Scheduler) shareGroupPerception(states []npcTickState) {
	// 按 group_id 分组收集 perceived
	groupPerceived := make(map[string][]perception.PerceiveResult)

	for i := range states {
		social, ok := npc.GetComponent[*component.SocialComponent](states[i].inst, "social")
		if !ok || social.GroupID == "" {
			continue
		}
		for _, pr := range states[i].perceived {
			groupPerceived[social.GroupID] = append(groupPerceived[social.GroupID], pr)
		}
	}

	// 去重（同 Event.ID 保留 Strength 最高）并分发回每个成员
	for groupID, pool := range groupPerceived {
		deduped := deduplicatePerceived(pool)
		// 分发给该组每个成员
		for i := range states {
			social, ok := npc.GetComponent[*component.SocialComponent](states[i].inst, "social")
			if !ok || social.GroupID != groupID {
				continue
			}
			states[i].perceived = deduped
		}
	}
}

// deduplicatePerceived 按 Event.ID 去重，保留 Strength 最高
func deduplicatePerceived(pool []perception.PerceiveResult) []perception.PerceiveResult {
	best := make(map[string]perception.PerceiveResult)
	for _, pr := range pool {
		if existing, ok := best[pr.Event.ID]; !ok || pr.Strength > existing.Strength {
			best[pr.Event.ID] = pr
		}
	}
	result := make([]perception.PerceiveResult, 0, len(best))
	for _, pr := range best {
		result = append(result, pr)
	}
	return result
}

// updateFollowerTargets 从 GroupManager 查找 leader 位置，写入 follower BB
func (s *Scheduler) updateFollowerTargets() {
	s.Registry.ForEach(func(inst *npc.Instance) {
		sc, ok := npc.GetComponent[*component.SocialComponent](inst, "social")
		if !ok || sc.Role != "follower" || sc.GroupID == "" {
			return
		}
		leader := s.GroupManager.GetLeader(sc.GroupID)
		if leader == nil {
			return
		}
		blackboard.Set(inst.BB, blackboard.KeyFollowTargetX, leader.Position.X)
		blackboard.Set(inst.BB, blackboard.KeyFollowTargetZ, leader.Position.Z)
	})
}

// propagateGroupState 群体状态传播：有成员 Flee → group_alert=true
func (s *Scheduler) propagateGroupState() {
	for _, members := range s.GroupManager.AllGroups() {
		hasFlee := false
		for _, inst := range members {
			if beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior"); ok {
				if beh.FSM != nil && beh.FSM.Current() == "Flee" {
					hasFlee = true
					break
				}
			}
		}
		for _, inst := range members {
			blackboard.Set(inst.BB, blackboard.KeyGroupAlert, hasFlee)
		}
	}
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
		NPCID:     inst.ID,
		Perceived: perceived,
		Weights:   decision.DefaultWeights,
	}
	if pers, ok := npc.GetComponent[*component.PersonalityComponent](inst, "personality"); ok {
		input.Weights = decision.DecisionWeights{
			Threat:  pers.DecisionWeights.Threat,
			Needs:   pers.DecisionWeights.Needs,
			Emotion: pers.DecisionWeights.Emotion,
		}
	}
	if needs, ok := npc.GetComponent[*component.NeedsComponent](inst, "needs"); ok {
		input.NeedUrgency = calcNeedUrgency(inst.BB, needs)
	}
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
		if perception.ShouldFilterByZone(npcZoneID, evt.ZoneID, typeCfg.PerceptionMode) {
			continue
		}
		strength := perception.CalcStrength(inst.Position, cfg, evt, typeCfg)
		if strength > 0 {
			results = append(results, perception.PerceiveResult{Event: evt, Strength: strength})
		}
	}

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
