package perception

import (
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

// --- Global ---

func TestCanPerceive_Global_AlwaysTrue(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "global", Range: 100}
	// 即使距离很远也能感知
	npc := event.Vec3{0, 0, 0}
	evt := makeEvt(event.Vec3{99999, 0, 99999})
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("global mode should always perceive")
	}
}

// --- Visual ---

func TestCanPerceive_Visual_InRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{0, 0, 0}
	evt := makeEvt(event.Vec3{100, 0, 0}) // 距离 100, visual_range=200, event_range=300 → min=200 → 100<=200
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive: distance 100 <= visual_range 200")
	}
}

func TestCanPerceive_Visual_OutOfRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{0, 0, 0}
	evt := makeEvt(event.Vec3{250, 0, 0}) // 距离 250, min(200,300)=200 → 250>200
	if CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should not perceive: distance 250 > visual_range 200")
	}
}

func TestCanPerceive_Visual_EventRangeLimits(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 100} // event range < visual range
	npc := event.Vec3{0, 0, 0}
	evt := makeEvt(event.Vec3{150, 0, 0}) // 距离 150, min(200,100)=100 → 150>100
	if CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should not perceive: distance 150 > event range 100")
	}
}

func TestCanPerceive_Visual_ExactBoundary(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 300}
	npc := event.Vec3{0, 0, 0}
	evt := makeEvt(event.Vec3{200, 0, 0}) // 距离刚好等于 visual_range
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive at exact boundary")
	}
}

// --- Auditory ---

func TestCanPerceive_Auditory_InRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "auditory", Range: 500}
	npc := event.Vec3{0, 0, 0}
	evt := makeEvt(event.Vec3{400, 0, 0}) // 距离 400, min(500,500)=500 → 400<=500
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive: distance 400 <= auditory_range 500")
	}
}

func TestCanPerceive_Auditory_OutOfRange(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "auditory", Range: 600}
	npc := event.Vec3{0, 0, 0}
	evt := makeEvt(event.Vec3{550, 0, 0}) // 距离 550, min(500,600)=500 → 550>500
	if CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should not perceive: distance 550 > auditory_range 500")
	}
}

func TestCanPerceive_Auditory_ExactBoundary(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "auditory", Range: 600}
	npc := event.Vec3{0, 0, 0}
	evt := makeEvt(event.Vec3{500, 0, 0}) // 距离 500 == auditory_range
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive at exact boundary")
	}
}

// --- Unknown mode ---

func TestCanPerceive_UnknownMode(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "telepathy", Range: 9999}
	npc := event.Vec3{0, 0, 0}
	evt := makeEvt(event.Vec3{1, 0, 0})
	if CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("unknown perception mode should return false")
	}
}

// --- Zero distance ---

func TestCanPerceive_ZeroDistance(t *testing.T) {
	evtType := &event.EventTypeConfig{PerceptionMode: "visual", Range: 100}
	npc := event.Vec3{50, 0, 50}
	evt := makeEvt(event.Vec3{50, 0, 50}) // 同一位置
	if !CanPerceive(npc, defaultCfg, evt, evtType) {
		t.Error("should perceive at zero distance")
	}
}
