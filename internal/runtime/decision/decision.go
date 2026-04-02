package decision

import (
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

// Center 决策中心，负责威胁评估、优先级仲裁、威胁衰减
// 无状态——所有 NPC 状态存储在各自的 Blackboard 中
type Center struct {
	DecayRate float64 // 威胁衰减速率（每秒），无事件时 threat_level 按此速率下降
}

// NewCenter 创建决策中心
func NewCenter(decayRate float64) *Center {
	return &Center{DecayRate: decayRate}
}

// Evaluate 对单个 NPC 评估所有可感知事件，仲裁并写入 BB
// events: 已经过感知过滤的事件列表
// dt: 本次 Tick 的时间间隔（秒）
func (c *Center) Evaluate(bb *blackboard.Blackboard, npcPos event.Vec3, events []*event.Event, evtTypes map[string]*event.EventTypeConfig, dt float64) {
	if len(events) == 0 {
		c.decay(bb, dt)
		return
	}

	// 计算每个事件的威胁值，取最高
	var maxThreat float64
	var maxEvent *event.Event

	for _, evt := range events {
		typeCfg, ok := evtTypes[evt.Type]
		if !ok {
			continue
		}
		threat := CalcThreat(evt.Severity, npcPos, evt.Position, typeCfg.Range)
		if threat > maxThreat {
			maxThreat = threat
			maxEvent = evt
		}
	}

	if maxEvent == nil {
		c.decay(bb, dt)
		return
	}

	// 写入 BB
	blackboard.Set(bb, blackboard.KeyThreatLevel, maxThreat)
	blackboard.Set(bb, blackboard.KeyThreatSource, maxEvent.ID)
	blackboard.Set(bb, blackboard.KeyLastEventType, maxEvent.Type)

	// 威胁过期时间 = 当前时间 + 事件剩余 TTL（转为毫秒）
	currentTime, _ := blackboard.Get(bb, blackboard.KeyCurrentTime)
	expireAt := currentTime + int64(maxEvent.TTL*1000)
	blackboard.Set(bb, blackboard.KeyThreatExpireAt, expireAt)
}

// CalcThreat 计算单个事件对 NPC 的威胁值
// threat = severity × max(0, 1 - distance/range)
func CalcThreat(severity float64, npcPos, evtPos event.Vec3, evtRange float64) float64 {
	if evtRange <= 0 {
		return 0
	}
	dist := event.Distance(npcPos, evtPos)
	factor := math.Max(0, 1-dist/evtRange)
	return severity * factor
}

// decay 无可感知事件时，威胁衰减
func (c *Center) decay(bb *blackboard.Blackboard, dt float64) {
	current, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if !ok || current <= 0 {
		return
	}

	newLevel := math.Max(0, current-c.DecayRate*dt)
	blackboard.Set(bb, blackboard.KeyThreatLevel, newLevel)

	if newLevel == 0 {
		blackboard.Set(bb, blackboard.KeyThreatSource, "")
		blackboard.Set(bb, blackboard.KeyLastEventType, "")
	}
}
