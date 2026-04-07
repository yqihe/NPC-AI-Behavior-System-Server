package decision

import (
	"log/slog"
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// DecisionWeights 三维决策权重（从 personality 组件读取）
type DecisionWeights struct {
	Threat  float64
	Needs   float64
	Emotion float64
}

// DefaultWeights v2 兼容默认权重：纯威胁决策
var DefaultWeights = DecisionWeights{Threat: 1.0, Needs: 0, Emotion: 0}

// DecisionInput 决策中心输入，由 Scheduler 从组件数据组装
type DecisionInput struct {
	NPCID        string                      // NPC ID（用于日志）
	Perceived    []perception.PerceiveResult // 感知结果（威胁维度）
	NeedUrgency  float64                     // 需求紧迫度 0~100
	EmotionValue float64                     // 主导情绪强度
	Weights      DecisionWeights             // 性格权重
}

// Center 决策中心，负责多维评分、加权仲裁、威胁衰减
// 无状态——所有 NPC 状态存储在各自的 Blackboard 中
type Center struct {
	DecayRate float64 // 威胁衰减速率（每秒），无事件时 threat_level 按此速率下降
}

// NewCenter 创建决策中心
func NewCenter(decayRate float64) *Center {
	return &Center{DecayRate: decayRate}
}

// Evaluate 对单个 NPC 进行多维决策评估
// 三维评分（威胁/需求/情绪）→ 加权仲裁 → 写 BB
func (c *Center) Evaluate(bb *blackboard.Blackboard, npcPos event.Vec3, input DecisionInput, evtTypes map[string]*event.EventTypeConfig, dt float64) {
	// 1. 威胁维度：取 perceived 最高 Strength
	threatScore, maxEvent := c.calcThreatScore(input.Perceived)

	// 2. 需求维度
	needScore := input.NeedUrgency

	// 3. 情绪维度
	emotionScore := input.EmotionValue

	// 4. 写三维原始分到 BB
	blackboard.Set(bb, blackboard.KeyThreatScore, threatScore)
	blackboard.Set(bb, blackboard.KeyNeedScore, needScore)
	blackboard.Set(bb, blackboard.KeyEmotionScore, emotionScore)

	// 5. 加权仲裁
	w := input.Weights
	weightedThreat := threatScore * w.Threat
	weightedNeed := needScore * w.Needs
	weightedEmotion := emotionScore * w.Emotion

	winner := "threat"
	maxWeighted := weightedThreat
	if weightedNeed > maxWeighted {
		winner = "needs"
		maxWeighted = weightedNeed
	}
	if weightedEmotion > maxWeighted {
		winner = "emotion"
	}
	blackboard.Set(bb, blackboard.KeyDecisionWinner, winner)

	// 决策日志
	maxEventID := ""
	if maxEvent != nil {
		maxEventID = maxEvent.ID
	}
	slog.Debug("decision.evaluated",
		"npc_id", input.NPCID,
		"threat_score", threatScore,
		"need_score", needScore,
		"emotion_score", emotionScore,
		"winner", winner,
		"threat_source", maxEventID,
	)

	// 6. 威胁维度 BB 写入（保持 v2 兼容，不受仲裁结果影响）
	if maxEvent != nil {
		blackboard.Set(bb, blackboard.KeyThreatLevel, threatScore)
		blackboard.Set(bb, blackboard.KeyThreatSource, maxEvent.ID)
		blackboard.Set(bb, blackboard.KeyLastEventType, maxEvent.Type)

		currentTime, _ := blackboard.Get(bb, blackboard.KeyCurrentTime)
		expireAt := currentTime + int64(maxEvent.TTL*1000)
		blackboard.Set(bb, blackboard.KeyThreatExpireAt, expireAt)
	} else if len(input.Perceived) == 0 {
		c.decay(bb, dt)
	}
}

// calcThreatScore 从感知结果中取最高强度事件
func (c *Center) calcThreatScore(perceived []perception.PerceiveResult) (float64, *event.Event) {
	var maxThreat float64
	var maxEvent *event.Event

	for _, pr := range perceived {
		if pr.Strength > maxThreat {
			maxThreat = pr.Strength
			maxEvent = pr.Event
		}
	}

	return maxThreat, maxEvent
}

// CalcThreat 计算单个事件对 NPC 的威胁值（保留，供外部直接计算用）
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
