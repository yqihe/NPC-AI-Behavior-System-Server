package decision

import (
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

func evtTypes() map[string]*event.EventTypeConfig {
	return map[string]*event.EventTypeConfig{
		"explosion": {Name: "explosion", DefaultSeverity: 80, DefaultTTL: 15, PerceptionMode: "auditory", Range: 500},
		"gunshot":   {Name: "gunshot", DefaultSeverity: 90, DefaultTTL: 10, PerceptionMode: "auditory", Range: 300},
		"shout":     {Name: "shout", DefaultSeverity: 30, DefaultTTL: 8, PerceptionMode: "auditory", Range: 200},
	}
}

func pr(evt *event.Event, strength float64) perception.PerceiveResult {
	return perception.PerceiveResult{Event: evt, Strength: strength}
}

func defaultInput(perceived []perception.PerceiveResult) DecisionInput {
	return DecisionInput{Perceived: perceived, Weights: DefaultWeights}
}

// --- CalcThreat ---

func TestCalcThreat_ZeroDistance(t *testing.T) {
	threat := CalcThreat(80, event.Vec3{0, 0, 0}, event.Vec3{0, 0, 0}, 500)
	if threat != 80 {
		t.Errorf("expected 80, got %f", threat)
	}
}

func TestCalcThreat_HalfRange(t *testing.T) {
	threat := CalcThreat(80, event.Vec3{0, 0, 0}, event.Vec3{250, 0, 0}, 500)
	if threat != 40 {
		t.Errorf("expected 40, got %f", threat)
	}
}

func TestCalcThreat_AtRange(t *testing.T) {
	threat := CalcThreat(80, event.Vec3{0, 0, 0}, event.Vec3{500, 0, 0}, 500)
	if threat != 0 {
		t.Errorf("expected 0, got %f", threat)
	}
}

// --- Evaluate: 纯威胁（v2 兼容）---

func TestEvaluate_ThreatOnly(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	evt := &event.Event{ID: "evt_1", Type: "explosion", Position: event.Vec3{100, 0, 0}, Severity: 80, TTL: 10}

	center.Evaluate(bb, event.Vec3{}, defaultInput([]perception.PerceiveResult{pr(evt, 64)}), evtTypes(), 0.1)

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if level != 64 {
		t.Errorf("threat_level = %f, want 64", level)
	}

	winner, _ := blackboard.Get(bb, blackboard.KeyDecisionWinner)
	if winner != "threat" {
		t.Errorf("decision_winner = %q, want %q", winner, "threat")
	}

	ts, _ := blackboard.Get(bb, blackboard.KeyThreatScore)
	if ts != 64 {
		t.Errorf("threat_score = %f, want 64", ts)
	}
}

// --- Evaluate: 需求优先 ---

func TestEvaluate_NeedsPriority(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	input := DecisionInput{
		Perceived:    []perception.PerceiveResult{pr(&event.Event{ID: "e1", Type: "shout", Severity: 30, TTL: 8}, 20)},
		NeedUrgency:  80, // 饥饿紧迫
		EmotionValue: 10,
		Weights:      DecisionWeights{Threat: 0.3, Needs: 0.5, Emotion: 0.2},
	}

	center.Evaluate(bb, event.Vec3{}, input, evtTypes(), 0.1)

	winner, _ := blackboard.Get(bb, blackboard.KeyDecisionWinner)
	// 加权：threat=20*0.3=6, needs=80*0.5=40, emotion=10*0.2=2 → needs
	if winner != "needs" {
		t.Errorf("decision_winner = %q, want %q", winner, "needs")
	}

	ns, _ := blackboard.Get(bb, blackboard.KeyNeedScore)
	if ns != 80 {
		t.Errorf("need_score = %f, want 80", ns)
	}
}

// --- Evaluate: 情绪优先（timid NPC）---

func TestEvaluate_EmotionPriority_Timid(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	input := DecisionInput{
		Perceived:    []perception.PerceiveResult{pr(&event.Event{ID: "e1", Type: "shout", Severity: 30, TTL: 8}, 25)},
		NeedUrgency:  30,
		EmotionValue: 70, // 高恐惧
		Weights:      DecisionWeights{Threat: 0.2, Needs: 0.2, Emotion: 0.6}, // timid
	}

	center.Evaluate(bb, event.Vec3{}, input, evtTypes(), 0.1)

	winner, _ := blackboard.Get(bb, blackboard.KeyDecisionWinner)
	// 加权：threat=25*0.2=5, needs=30*0.2=6, emotion=70*0.6=42 → emotion
	if winner != "emotion" {
		t.Errorf("decision_winner = %q, want %q", winner, "emotion")
	}
}

// --- Evaluate: 高威胁压制 ---

func TestEvaluate_ThreatOverride(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	input := DecisionInput{
		Perceived:    []perception.PerceiveResult{pr(&event.Event{ID: "e1", Type: "explosion", Severity: 80, TTL: 10}, 75)},
		NeedUrgency:  60,
		EmotionValue: 40,
		Weights:      DecisionWeights{Threat: 0.5, Needs: 0.3, Emotion: 0.2},
	}

	center.Evaluate(bb, event.Vec3{}, input, evtTypes(), 0.1)

	winner, _ := blackboard.Get(bb, blackboard.KeyDecisionWinner)
	// 加权：threat=75*0.5=37.5, needs=60*0.3=18, emotion=40*0.2=8 → threat
	if winner != "threat" {
		t.Errorf("decision_winner = %q, want %q", winner, "threat")
	}
}

// --- Evaluate: 默认权重始终 threat ---

func TestEvaluate_DefaultWeights_AlwaysThreat(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	input := DecisionInput{
		Perceived:    []perception.PerceiveResult{pr(&event.Event{ID: "e1", Type: "shout", Severity: 30, TTL: 8}, 5)},
		NeedUrgency:  90,
		EmotionValue: 80,
		Weights:      DefaultWeights, // {1, 0, 0}
	}

	center.Evaluate(bb, event.Vec3{}, input, evtTypes(), 0.1)

	winner, _ := blackboard.Get(bb, blackboard.KeyDecisionWinner)
	// 加权：threat=5*1=5, needs=90*0=0, emotion=80*0=0 → threat
	if winner != "threat" {
		t.Errorf("decision_winner = %q, want %q (default weights)", winner, "threat")
	}
}

// --- Evaluate: 三维分正确写入 ---

func TestEvaluate_ScoresWrittenToBB(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	input := DecisionInput{
		Perceived:    []perception.PerceiveResult{pr(&event.Event{ID: "e1", Type: "explosion", Severity: 80, TTL: 10}, 55)},
		NeedUrgency:  42,
		EmotionValue: 33,
		Weights:      DefaultWeights,
	}

	center.Evaluate(bb, event.Vec3{}, input, evtTypes(), 0.1)

	ts, _ := blackboard.Get(bb, blackboard.KeyThreatScore)
	ns, _ := blackboard.Get(bb, blackboard.KeyNeedScore)
	es, _ := blackboard.Get(bb, blackboard.KeyEmotionScore)

	if ts != 55 {
		t.Errorf("threat_score = %f, want 55", ts)
	}
	if ns != 42 {
		t.Errorf("need_score = %f, want 42", ns)
	}
	if es != 33 {
		t.Errorf("emotion_score = %f, want 33", es)
	}
}

// --- Evaluate: 衰减不变 ---

func TestEvaluate_Decay_NoEvents(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)
	blackboard.Set(bb, blackboard.KeyThreatSource, "old_evt")

	center := NewCenter(10.0)
	center.Evaluate(bb, event.Vec3{}, defaultInput(nil), evtTypes(), 1.0)

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if level != 40 {
		t.Errorf("expected 40 after decay, got %f", level)
	}
}

func TestEvaluate_Decay_ToZero(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 5.0)
	blackboard.Set(bb, blackboard.KeyThreatSource, "old")
	blackboard.Set(bb, blackboard.KeyLastEventType, "explosion")

	center := NewCenter(10.0)
	center.Evaluate(bb, event.Vec3{}, defaultInput(nil), evtTypes(), 1.0)

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if level != 0 {
		t.Errorf("expected 0, got %f", level)
	}
	source, _ := blackboard.Get(bb, blackboard.KeyThreatSource)
	if source != "" {
		t.Errorf("expected empty source, got %q", source)
	}
}
