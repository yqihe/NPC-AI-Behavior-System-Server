package runtime

import (
	"context"
	"log/slog"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
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

		// 4a. 感知过滤
		perceived := make([]*event.Event, 0, len(activeEvents))
		for _, evt := range activeEvents {
			typeCfg, ok := s.EvtTypes[evt.Type]
			if !ok {
				slog.Warn("scheduler.event_type_not_found", "event_type", evt.Type)
				continue
			}
			if perception.CanPerceive(inst.Position, inst.Perception, evt, typeCfg) {
				perceived = append(perceived, evt)
			}
		}

		// 4b. 决策中心评估
		s.Decision.Evaluate(inst.BB, inst.Position, perceived, s.EvtTypes, dt)

		// 4c. NPC Tick（FSM + BT）
		inst.Tick()
	})
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
