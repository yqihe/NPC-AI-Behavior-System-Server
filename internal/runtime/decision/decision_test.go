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

// pr 辅助：从事件和强度构建 PerceiveResult
func pr(evt *event.Event, strength float64) perception.PerceiveResult {
	return perception.PerceiveResult{Event: evt, Strength: strength}
}

// --- CalcThreat（保留的工具函数）---

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

func TestCalcThreat_BeyondRange(t *testing.T) {
	threat := CalcThreat(80, event.Vec3{0, 0, 0}, event.Vec3{600, 0, 0}, 500)
	if threat != 0 {
		t.Errorf("expected 0, got %f", threat)
	}
}

func TestCalcThreat_ZeroRange_Global(t *testing.T) {
	threat := CalcThreat(80, event.Vec3{0, 0, 0}, event.Vec3{100, 0, 100}, 0)
	if threat != 80 {
		t.Errorf("expected 80 (severity) for global event, got %f", threat)
	}
}

// --- Evaluate: 单事件 ---

func TestEvaluate_SingleEvent(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	npcPos := event.Vec3{0, 0, 0}
	evt := &event.Event{ID: "evt_1", Type: "explosion", Position: event.Vec3{100, 0, 0}, Severity: 80, TTL: 10}

	// 感知强度 = 80 * (1 - 100/500) = 64（由感知层计算后传入）
	center.Evaluate(bb, npcPos, []perception.PerceiveResult{pr(evt, 64)}, evtTypes(), 0.1)

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	source, _ := blackboard.Get(bb, blackboard.KeyThreatSource)
	evtType, _ := blackboard.Get(bb, blackboard.KeyLastEventType)

	if level != 64 {
		t.Errorf("expected threat_level 64, got %f", level)
	}
	if source != "evt_1" {
		t.Errorf("expected source evt_1, got %s", source)
	}
	if evtType != "explosion" {
		t.Errorf("expected event type explosion, got %s", evtType)
	}
}

// --- Evaluate: 多事件仲裁 ---

func TestEvaluate_MultipleEvents_HighestWins(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	npcPos := event.Vec3{0, 0, 0}

	shout := &event.Event{ID: "evt_shout", Type: "shout", Position: event.Vec3{50, 0, 0}, Severity: 30, TTL: 8}
	gunshot := &event.Event{ID: "evt_gunshot", Type: "gunshot", Position: event.Vec3{100, 0, 0}, Severity: 90, TTL: 10}

	// 感知强度：shout=30*(1-50/200)=22.5, gunshot=90*(1-100/300)=60
	perceived := []perception.PerceiveResult{pr(shout, 22.5), pr(gunshot, 60)}
	center.Evaluate(bb, npcPos, perceived, evtTypes(), 0.1)

	source, _ := blackboard.Get(bb, blackboard.KeyThreatSource)
	if source != "evt_gunshot" {
		t.Errorf("expected highest threat (gunshot), got source %s", source)
	}

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if level != 60 {
		t.Errorf("expected threat_level 60, got %f", level)
	}
}

// --- Evaluate: 事件抢占 ---

func TestEvaluate_EventPreemption(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	npcPos := event.Vec3{0, 0, 0}

	lowEvt := &event.Event{ID: "evt_shout", Type: "shout", Position: event.Vec3{50, 0, 0}, Severity: 30, TTL: 8}
	center.Evaluate(bb, npcPos, []perception.PerceiveResult{pr(lowEvt, 22.5)}, evtTypes(), 0.1)

	levelBefore, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)

	highEvt := &event.Event{ID: "evt_gunshot", Type: "gunshot", Position: event.Vec3{50, 0, 0}, Severity: 90, TTL: 10}
	center.Evaluate(bb, npcPos, []perception.PerceiveResult{pr(lowEvt, 22.5), pr(highEvt, 75)}, evtTypes(), 0.1)

	levelAfter, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	sourceAfter, _ := blackboard.Get(bb, blackboard.KeyThreatSource)

	if sourceAfter != "evt_gunshot" {
		t.Errorf("expected preemption to gunshot, got %s", sourceAfter)
	}
	if levelAfter <= levelBefore {
		t.Errorf("expected higher threat after preemption: before=%f after=%f", levelBefore, levelAfter)
	}
}

// --- Evaluate: 威胁衰减 ---

func TestEvaluate_Decay_NoEvents(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)
	blackboard.Set(bb, blackboard.KeyThreatSource, "old_evt")
	blackboard.Set(bb, blackboard.KeyLastEventType, "explosion")

	center := NewCenter(10.0)
	npcPos := event.Vec3{0, 0, 0}

	center.Evaluate(bb, npcPos, nil, evtTypes(), 1.0)

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if level != 40 {
		t.Errorf("expected 40 after decay, got %f", level)
	}
}

func TestEvaluate_Decay_ToZero(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 5.0)
	blackboard.Set(bb, blackboard.KeyThreatSource, "old_evt")
	blackboard.Set(bb, blackboard.KeyLastEventType, "explosion")

	center := NewCenter(10.0)
	npcPos := event.Vec3{0, 0, 0}

	center.Evaluate(bb, npcPos, nil, evtTypes(), 1.0)

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if level != 0 {
		t.Errorf("expected 0 after full decay, got %f", level)
	}
	source, _ := blackboard.Get(bb, blackboard.KeyThreatSource)
	if source != "" {
		t.Errorf("expected empty source after decay to zero, got %s", source)
	}
}

func TestEvaluate_Decay_AlreadyZero(t *testing.T) {
	bb := blackboard.New()
	center := NewCenter(10.0)
	center.Evaluate(bb, event.Vec3{}, nil, evtTypes(), 1.0)
}
