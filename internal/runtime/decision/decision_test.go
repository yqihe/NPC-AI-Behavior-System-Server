package decision

import (
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

func evtTypes() map[string]*event.EventTypeConfig {
	return map[string]*event.EventTypeConfig{
		"explosion": {Name: "explosion", DefaultSeverity: 80, DefaultTTL: 15, PerceptionMode: "auditory", Range: 500},
		"gunshot":   {Name: "gunshot", DefaultSeverity: 90, DefaultTTL: 10, PerceptionMode: "auditory", Range: 300},
		"shout":     {Name: "shout", DefaultSeverity: 30, DefaultTTL: 8, PerceptionMode: "auditory", Range: 200},
	}
}

// --- CalcThreat ---

func TestCalcThreat_ZeroDistance(t *testing.T) {
	// 距离 0，factor=1，threat=severity
	threat := CalcThreat(80, event.Vec3{0, 0, 0}, event.Vec3{0, 0, 0}, 500)
	if threat != 80 {
		t.Errorf("expected 80, got %f", threat)
	}
}

func TestCalcThreat_HalfRange(t *testing.T) {
	// 距离 250，range 500，factor=0.5，threat=40
	threat := CalcThreat(80, event.Vec3{0, 0, 0}, event.Vec3{250, 0, 0}, 500)
	if threat != 40 {
		t.Errorf("expected 40, got %f", threat)
	}
}

func TestCalcThreat_AtRange(t *testing.T) {
	// 距离等于 range，factor=0，threat=0
	threat := CalcThreat(80, event.Vec3{0, 0, 0}, event.Vec3{500, 0, 0}, 500)
	if threat != 0 {
		t.Errorf("expected 0, got %f", threat)
	}
}

func TestCalcThreat_BeyondRange(t *testing.T) {
	// 距离超出 range，factor=0，threat=0
	threat := CalcThreat(80, event.Vec3{0, 0, 0}, event.Vec3{600, 0, 0}, 500)
	if threat != 0 {
		t.Errorf("expected 0, got %f", threat)
	}
}

func TestCalcThreat_ZeroRange_Global(t *testing.T) {
	// range=0 表示 global 事件，威胁值直接等于 severity，无距离衰减
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

	center.Evaluate(bb, npcPos, []*event.Event{evt}, evtTypes(), 0.1)

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	source, _ := blackboard.Get(bb, blackboard.KeyThreatSource)
	evtType, _ := blackboard.Get(bb, blackboard.KeyLastEventType)

	// threat = 80 * (1 - 100/500) = 80 * 0.8 = 64
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

	events := []*event.Event{
		{ID: "evt_shout", Type: "shout", Position: event.Vec3{50, 0, 0}, Severity: 30, TTL: 8},
		{ID: "evt_gunshot", Type: "gunshot", Position: event.Vec3{100, 0, 0}, Severity: 90, TTL: 10},
	}

	center.Evaluate(bb, npcPos, events, evtTypes(), 0.1)

	source, _ := blackboard.Get(bb, blackboard.KeyThreatSource)
	if source != "evt_gunshot" {
		t.Errorf("expected highest threat (gunshot), got source %s", source)
	}

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	// gunshot: 90 * (1 - 100/300) = 90 * 0.667 = 60
	if level < 59 || level > 61 {
		t.Errorf("expected threat_level ~60, got %f", level)
	}
}

// --- Evaluate: 事件抢占（R9）---

func TestEvaluate_EventPreemption(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	npcPos := event.Vec3{0, 0, 0}

	// 第一次：低威胁事件
	lowEvt := []*event.Event{
		{ID: "evt_shout", Type: "shout", Position: event.Vec3{50, 0, 0}, Severity: 30, TTL: 8},
	}
	center.Evaluate(bb, npcPos, lowEvt, evtTypes(), 0.1)

	levelBefore, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	sourceBefore, _ := blackboard.Get(bb, blackboard.KeyThreatSource)

	// 第二次：高威胁事件到达，应覆写
	highEvt := []*event.Event{
		{ID: "evt_shout", Type: "shout", Position: event.Vec3{50, 0, 0}, Severity: 30, TTL: 7},
		{ID: "evt_gunshot", Type: "gunshot", Position: event.Vec3{50, 0, 0}, Severity: 90, TTL: 10},
	}
	center.Evaluate(bb, npcPos, highEvt, evtTypes(), 0.1)

	levelAfter, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	sourceAfter, _ := blackboard.Get(bb, blackboard.KeyThreatSource)

	if sourceAfter != "evt_gunshot" {
		t.Errorf("expected preemption to gunshot, got %s", sourceAfter)
	}
	if levelAfter <= levelBefore {
		t.Errorf("expected higher threat after preemption: before=%f after=%f (source before=%s)", levelBefore, levelAfter, sourceBefore)
	}
}

// --- Evaluate: 威胁衰减（R8）---

func TestEvaluate_Decay_NoEvents(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)
	blackboard.Set(bb, blackboard.KeyThreatSource, "old_evt")
	blackboard.Set(bb, blackboard.KeyLastEventType, "explosion")

	center := NewCenter(10.0) // 10 per second
	npcPos := event.Vec3{0, 0, 0}

	// 无事件，dt=1s → 衰减 10
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

	// 无事件，dt=1s → 衰减 10，但不低于 0
	center.Evaluate(bb, npcPos, nil, evtTypes(), 1.0)

	level, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if level != 0 {
		t.Errorf("expected 0 after full decay, got %f", level)
	}

	// 衰减到 0 后应清空 source 和 event_type
	source, _ := blackboard.Get(bb, blackboard.KeyThreatSource)
	if source != "" {
		t.Errorf("expected empty source after decay to zero, got %s", source)
	}
	evtType, _ := blackboard.Get(bb, blackboard.KeyLastEventType)
	if evtType != "" {
		t.Errorf("expected empty event type after decay to zero, got %s", evtType)
	}
}

func TestEvaluate_Decay_AlreadyZero(t *testing.T) {
	bb := blackboard.New()
	// 不设置 threat_level → 不存在
	center := NewCenter(10.0)
	// 不应 panic
	center.Evaluate(bb, event.Vec3{}, nil, evtTypes(), 1.0)
}

// --- Evaluate: 未知事件类型 ---

func TestEvaluate_UnknownEventType(t *testing.T) {
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(10000))

	center := NewCenter(10.0)
	evt := &event.Event{ID: "evt_1", Type: "unknown_type", Position: event.Vec3{0, 0, 0}, Severity: 80, TTL: 10}

	// 未知类型事件应被跳过，触发衰减
	center.Evaluate(bb, event.Vec3{}, []*event.Event{evt}, evtTypes(), 0.1)

	_, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if ok {
		t.Error("unknown event type should not write threat_level")
	}
}
