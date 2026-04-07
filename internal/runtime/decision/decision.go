package decision

import (
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
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

// Evaluate 对单个 NPC 评估所有感知结果，仲裁并写入 BB
// perceived: 已经过感知过滤的结果列表（含感知强度）
// dt: 本次 Tick 的时间间隔（秒）
func (c *Center) Evaluate(bb *blackboard.Blackboard, npcPos event.Vec3, perceived []perception.PerceiveResult, evtTypes map[string]*event.EventTypeConfig, dt float64) {
	if len(perceived) == 0 {
		c.decay(bb, dt)
		return
	}

	// 直接使用感知强度作为威胁值（感知层已计算距离衰减）
	var maxThreat float64
	var maxEvent *event.Event

	for _, pr := range perceived {
		if pr.Strength > maxThreat {
			maxThreat = pr.Strength
			maxEvent = pr.Event
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

// CalcThreat 计算单个事件对 NPC 的威胁值（保留，供外部直接计算用）
// range > 0: threat = severity × max(0, 1 - distance/range)
// range <= 0: global 事件，threat = severity（无距离衰减）
func CalcThreat(severity float64, npcPos, evtPos event.Vec3, evtRange float64) float64 {
	if evtRange <= 0 {
		return severity
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
