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

func TestEmotionComponent_Tick(t *testing.T) {
	raw := json.RawMessage(`{"emotion_states":[
		{"name":"fear","value":50,"accumulate_rate":10,"decay_rate":5},
		{"name":"anger","value":30,"accumulate_rate":8,"decay_rate":3}
	]}`)
	comp, _ := component.EmotionFactory(raw)
	tickable := comp.(component.Tickable)
	board := blackboard.New()

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
