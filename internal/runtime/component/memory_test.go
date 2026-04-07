package component_test

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	bb "github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
)

func TestMemoryFactory(t *testing.T) {
	raw := json.RawMessage(`{"capacity":10,"memory_types":["threat","location"],"decay_time":60}`)
	comp, err := component.MemoryFactory(raw)
	if err != nil {
		t.Fatalf("MemoryFactory failed: %v", err)
	}
	m := comp.(*component.MemoryComponent)
	if m.Capacity != 10 {
		t.Errorf("Capacity = %d, want 10", m.Capacity)
	}
	if len(m.MemoryTypes) != 2 {
		t.Errorf("MemoryTypes len = %d, want 2", len(m.MemoryTypes))
	}
}

func TestMemoryFactory_InvalidCapacity(t *testing.T) {
	raw := json.RawMessage(`{"capacity":0,"memory_types":["threat"],"decay_time":60}`)
	_, err := component.MemoryFactory(raw)
	if err == nil {
		t.Fatal("expected error for capacity < 1")
	}
}

func TestMemoryComponent_Tick(t *testing.T) {
	raw := json.RawMessage(`{"capacity":10,"memory_types":["threat"],"decay_time":60}`)
	comp, _ := component.MemoryFactory(raw)
	tickable := comp.(component.Tickable)
	board := blackboard.New()

	tickable.Tick(board, 0.1)

	count, ok := bb.Get(board, bb.KeyMemoryCount)
	if !ok {
		t.Fatal("memory_count not set")
	}
	if count != 0 {
		t.Errorf("memory_count = %d, want 0", count)
	}
}
