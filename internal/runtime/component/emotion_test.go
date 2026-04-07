package component_test

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	bb "github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
)

func TestEmotionFactory(t *testing.T) {
	raw := json.RawMessage(`{"emotion_states":[{"name":"fear","value":50,"accumulate_rate":10,"decay_rate":5}]}`)
	comp, err := component.EmotionFactory(raw)
	if err != nil {
		t.Fatalf("EmotionFactory failed: %v", err)
	}
	e := comp.(*component.EmotionComponent)
	if len(e.EmotionStates) != 1 {
		t.Fatalf("EmotionStates len = %d, want 1", len(e.EmotionStates))
	}
}

func TestEmotionComponent_Tick_Decay(t *testing.T) {
	raw := json.RawMessage(`{"emotion_states":[
		{"name":"fear","value":50,"accumulate_rate":10,"decay_rate":5},
		{"name":"anger","value":30,"accumulate_rate":8,"decay_rate":3}
	]}`)
	comp, _ := component.EmotionFactory(raw)
	tickable := comp.(component.Tickable)
	board := blackboard.New()
	// 无威胁记忆 → fear 衰减

	// dt=1.0 → fear: 50-5=45, anger: 30-3=27 → dominant=fear(45)
	tickable.Tick(board, 1.0)

	name, ok := bb.Get(board, bb.KeyEmotionDominant)
	if !ok {
		t.Fatal("emotion_dominant not set")
	}
	if name != "fear" {
		t.Errorf("emotion_dominant = %q, want %q", name, "fear")
	}
	val, ok := bb.Get(board, bb.KeyEmotionDominantVal)
	if !ok {
		t.Fatal("emotion_dominant_val not set")
	}
	if val != 45 {
		t.Errorf("emotion_dominant_val = %v, want 45", val)
	}
}

func TestEmotionComponent_Tick_FearAccumulate_WithThreatMemory(t *testing.T) {
	raw := json.RawMessage(`{"emotion_states":[{"name":"fear","value":10,"accumulate_rate":20,"decay_rate":5}]}`)
	comp, _ := component.EmotionFactory(raw)
	tickable := comp.(component.Tickable)
	board := blackboard.New()

	// 设置威胁记忆 → fear 应累积而非衰减
	blackboard.Set(board, bb.KeyMemoryThreatValue, 50.0)

	// dt=1.0 → fear: 10+20=30（累积，不衰减）
	tickable.Tick(board, 1.0)

	val, _ := bb.Get(board, bb.KeyEmotionDominantVal)
	if val != 30 {
		t.Errorf("fear should accumulate to 30 with threat memory, got %f", val)
	}
}

func TestEmotionComponent_Tick_FearDecay_NoThreatMemory(t *testing.T) {
	raw := json.RawMessage(`{"emotion_states":[{"name":"fear","value":50,"accumulate_rate":20,"decay_rate":10}]}`)
	comp, _ := component.EmotionFactory(raw)
	tickable := comp.(component.Tickable)
	board := blackboard.New()

	// memory_threat_value=0 → fear 衰减
	blackboard.Set(board, bb.KeyMemoryThreatValue, 0.0)

	// dt=1.0 → fear: 50-10=40
	tickable.Tick(board, 1.0)

	val, _ := bb.Get(board, bb.KeyEmotionDominantVal)
	if val != 40 {
		t.Errorf("fear should decay to 40 without threat memory, got %f", val)
	}
}

func TestEmotionComponent_Tick_NonFearDecays_WithThreatMemory(t *testing.T) {
	raw := json.RawMessage(`{"emotion_states":[
		{"name":"fear","value":10,"accumulate_rate":20,"decay_rate":5},
		{"name":"anger","value":50,"accumulate_rate":15,"decay_rate":10}
	]}`)
	comp, _ := component.EmotionFactory(raw)
	tickable := comp.(component.Tickable)
	board := blackboard.New()

	blackboard.Set(board, bb.KeyMemoryThreatValue, 50.0)

	// dt=1.0 → fear: 10+20=30（累积），anger: 50-10=40（衰减，非 fear 不受记忆影响）
	tickable.Tick(board, 1.0)

	name, _ := bb.Get(board, bb.KeyEmotionDominant)
	// anger=40 > fear=30 → dominant=anger
	if name != "anger" {
		t.Errorf("emotion_dominant = %q, want %q", name, "anger")
	}
}
