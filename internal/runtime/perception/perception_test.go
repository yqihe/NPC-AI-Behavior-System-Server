package perception

import (
	"math"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

var defaultCfg = &PerceptionConfig{
	VisualRange:   200.0,
	AuditoryRange: 500.0,
}

func makeEvt(pos event.Vec3) *event.Event {
	return &event.Event{
		ID:       "test_evt",
		Type:     "explosion",
		Position: pos,
		Severity: 80,
		TTL:      10,
	}
}

// === CalcStrength ===

func TestCalcStrength_Global(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "global", Range: 100}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 99999, Y: 0, Z: 99999})
	s := CalcStrength(npc, defaultCfg, evt, evtType)
	if s != 80 {
		t.Errorf("global strength = %f, want 80 (= severity)", s)
	}
}

func TestCalcStrength_Visual_Close(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 10, Y: 0, Z: 0}) // dist=10, maxRange=min(200,300)=200
	s := CalcStrength(npc, defaultCfg, evt, evtType)
	expected := 80 * (1 - 10.0/200.0) // 80 * 0.95 = 76
	if math.Abs(s-expected) > 0.01 {
		t.Errorf("strength = %f, want %f", s, expected)
	}
}

func TestCalcStrength_Visual_Mid(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 100, Y: 0, Z: 0}) // dist=100, maxRange=200
	s := CalcStrength(npc, defaultCfg, evt, evtType)
	expected := 80 * (1 - 100.0/200.0) // 80 * 0.5 = 40
	if math.Abs(s-expected) > 0.01 {
		t.Errorf("strength = %f, want %f", s, expected)
	}
}

func TestCalcStrength_Visual_OutOfRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 250, Y: 0, Z: 0}) // dist=250 > maxRange=200
	s := CalcStrength(npc, defaultCfg, evt, evtType)
	if s != 0 {
		t.Errorf("strength = %f, want 0 (out of range)", s)
	}
}

func TestCalcStrength_Visual_ZeroDistance(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{X: 50, Y: 0, Z: 50}
	evt := makeEvt(event.Vec3{X: 50, Y: 0, Z: 50}) // dist=0
	s := CalcStrength(npc, defaultCfg, evt, evtType)
	if s != 80 {
		t.Errorf("strength at zero distance = %f, want 80 (= severity)", s)
	}
}

func TestCalcStrength_Auditory_InRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "auditory", Range: 500}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 250, Y: 0, Z: 0}) // dist=250, maxRange=500
	s := CalcStrength(npc, defaultCfg, evt, evtType)
	expected := 80 * (1 - 250.0/500.0) // 80 * 0.5 = 40
	if math.Abs(s-expected) > 0.01 {
		t.Errorf("strength = %f, want %f", s, expected)
	}
}

func TestCalcStrength_UnknownMode(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "telepathy", Range: 9999}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 1, Y: 0, Z: 0})
	s := CalcStrength(npc, defaultCfg, evt, evtType)
	if s != 0 {
		t.Errorf("unknown mode strength = %f, want 0", s)
	}
}

// === CanPerceive（v2 兼容）===

func TestCanPerceive_Global_AlwaysTrue(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "global", Range: 100}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 99999, Y: 0, Z: 99999})
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("global mode should always perceive")
	}
}

func TestCanPerceive_Visual_InRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 100, Y: 0, Z: 0})
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive: distance 100 <= visual_range 200")
	}
}

func TestCanPerceive_Visual_OutOfRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 250, Y: 0, Z: 0})
	if CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should not perceive: distance 250 > visual_range 200")
	}
}

func TestCanPerceive_Visual_EventRangeLimits(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 100}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 150, Y: 0, Z: 0})
	if CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should not perceive: distance 150 > event range 100")
	}
}

func TestCanPerceive_Visual_ExactBoundary(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 200, Y: 0, Z: 0}) // dist == visual_range
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive at exact boundary")
	}
}

func TestCanPerceive_Auditory_InRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "auditory", Range: 500}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 400, Y: 0, Z: 0})
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive: distance 400 <= auditory_range 500")
	}
}

func TestCanPerceive_Auditory_OutOfRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "auditory", Range: 600}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 550, Y: 0, Z: 0})
	if CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should not perceive: distance 550 > auditory_range 500")
	}
}

func TestCanPerceive_Auditory_ExactBoundary(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "auditory", Range: 600}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 500, Y: 0, Z: 0})
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive at exact boundary")
	}
}

func TestCanPerceive_UnknownMode(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "telepathy", Range: 9999}
	npc := event.Vec3{X: 0, Y: 0, Z: 0}
	evt := makeEvt(event.Vec3{X: 1, Y: 0, Z: 0})
	if CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("unknown perception mode should return false")
	}
}

func TestCanPerceive_ZeroDistance(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 100}
	npc := event.Vec3{X: 50, Y: 0, Z: 50}
	evt := makeEvt(event.Vec3{X: 50, Y: 0, Z: 50})
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive at zero distance")
	}
}

// === ShouldFilterByZone ===

func TestShouldFilterByZone_SameZone(t *testing.T) {
	if ShouldFilterByZone("meadow", "meadow", "auditory") {
		t.Error("same zone should not filter")
	}
}

func TestShouldFilterByZone_DifferentZone(t *testing.T) {
	if !ShouldFilterByZone("meadow", "mountain", "auditory") {
		t.Error("different zone should filter")
	}
}

func TestShouldFilterByZone_Global_IgnoresZone(t *testing.T) {
	if ShouldFilterByZone("meadow", "mountain", "global") {
		t.Error("global should not filter by zone")
	}
}

func TestShouldFilterByZone_EmptyNPCZone(t *testing.T) {
	if ShouldFilterByZone("", "mountain", "auditory") {
		t.Error("empty NPC zone should not filter (backward compat)")
	}
}

func TestShouldFilterByZone_EmptyEventZone(t *testing.T) {
	if ShouldFilterByZone("meadow", "", "auditory") {
		t.Error("empty event zone should not filter (backward compat)")
	}
}

func TestShouldFilterByZone_BothEmpty(t *testing.T) {
	if ShouldFilterByZone("", "", "visual") {
		t.Error("both empty should not filter")
	}
}
