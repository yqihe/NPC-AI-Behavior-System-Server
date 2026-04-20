package component_test

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
)

func newTestMemory(capacity int) *component.MemoryComponent {
	raw := []byte(`{"capacity":` + itoa(capacity) + `,"memory_types":["threat","location","social"],"decay_time":60}`)
	comp, err := component.MemoryFactory(json.RawMessage(raw))
	if err != nil {
		panic(err)
	}
	return comp.(*component.MemoryComponent)
}

func itoa(n int) string {
	return string(rune('0'+n)) // works for 1-9
}

func TestMemoryFactory(t *testing.T) {
	raw := json.RawMessage(`{"capacity":5,"memory_types":["threat","location"],"decay_time":60}`)
	comp, err := component.MemoryFactory(raw)
	if err != nil {
		t.Fatalf("MemoryFactory failed: %v", err)
	}
	m := comp.(*component.MemoryComponent)
	if m.Capacity != 5 {
		t.Errorf("Capacity = %d, want 5", m.Capacity)
	}
	if m.Count() != 0 {
		t.Errorf("Count = %d, want 0", m.Count())
	}
}

func TestMemory_AddAndGet(t *testing.T) {
	m := newTestMemory(5)
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "enemy_1", Value: 50, Timestamp: 1000, TTL: 60})

	if m.Count() != 1 {
		t.Fatalf("Count = %d, want 1", m.Count())
	}
	if !m.HasMemory("threat", "enemy_1") {
		t.Error("HasMemory should be true")
	}
	e, ok := m.GetMemory("threat", "enemy_1")
	if !ok {
		t.Fatal("GetMemory should return true")
	}
	if e.Value != 50 {
		t.Errorf("Value = %f, want 50", e.Value)
	}
}

func TestMemory_Reinforce(t *testing.T) {
	m := newTestMemory(5)
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "enemy_1", Value: 50, Timestamp: 1000, TTL: 60})
	// 强化：同 Type+TargetID，更高 Value
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "enemy_1", Value: 80, Timestamp: 2000, TTL: 60})

	if m.Count() != 1 {
		t.Fatalf("Count = %d, want 1 (reinforced, not duplicated)", m.Count())
	}
	e, _ := m.GetMemory("threat", "enemy_1")
	if e.Value != 80 {
		t.Errorf("Value = %f, want 80 (reinforced to max)", e.Value)
	}
	if e.Timestamp != 2000 {
		t.Errorf("Timestamp = %d, want 2000 (updated)", e.Timestamp)
	}
}

func TestMemory_Reinforce_LowerValue(t *testing.T) {
	m := newTestMemory(5)
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "enemy_1", Value: 80, Timestamp: 1000, TTL: 60})
	// 强化：更低 Value → Value 不降
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "enemy_1", Value: 30, Timestamp: 2000, TTL: 60})

	e, _ := m.GetMemory("threat", "enemy_1")
	if e.Value != 80 {
		t.Errorf("Value = %f, want 80 (should keep max)", e.Value)
	}
}

func TestMemory_Evict_Oldest(t *testing.T) {
	m := newTestMemory(3)
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "a", Value: 10, Timestamp: 100, TTL: 60})
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "b", Value: 20, Timestamp: 200, TTL: 60})
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "c", Value: 30, Timestamp: 300, TTL: 60})

	// 满了，新增应淘汰 Timestamp 最小的 "a"
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "d", Value: 40, Timestamp: 400, TTL: 60})

	if m.Count() != 3 {
		t.Fatalf("Count = %d, want 3", m.Count())
	}
	if m.HasMemory("threat", "a") {
		t.Error("oldest entry 'a' should be evicted")
	}
	if !m.HasMemory("threat", "d") {
		t.Error("new entry 'd' should exist")
	}
}

func TestMemory_GetMemories_ByType(t *testing.T) {
	m := newTestMemory(5)
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "e1", Value: 10, Timestamp: 100, TTL: 60})
	m.AddMemory(component.MemoryEntry{Type: "location", TargetID: "l1", Value: 3, Timestamp: 200, TTL: 60})
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "e2", Value: 20, Timestamp: 300, TTL: 60})

	threats := m.GetMemories("threat")
	if len(threats) != 2 {
		t.Errorf("GetMemories(threat) = %d, want 2", len(threats))
	}
	locations := m.GetMemories("location")
	if len(locations) != 1 {
		t.Errorf("GetMemories(location) = %d, want 1", len(locations))
	}
}

func TestMemory_SupportsType(t *testing.T) {
	m := newTestMemory(5)
	if !m.SupportsType("threat") {
		t.Error("should support threat")
	}
	if m.SupportsType("unknown") {
		t.Error("should not support unknown")
	}
}

func TestMemory_Tick_TTLDecay(t *testing.T) {
	m := newTestMemory(5)
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "e1", Value: 50, Timestamp: 100, TTL: 10})
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "e2", Value: 30, Timestamp: 200, TTL: 5})

	board := blackboard.New()

	// dt=3 → e1.TTL=7, e2.TTL=2
	m.Tick(board, 3.0)
	if m.Count() != 2 {
		t.Fatalf("Count = %d, want 2 after dt=3", m.Count())
	}

	// dt=3 → e1.TTL=4, e2.TTL=-1 → e2 removed
	m.Tick(board, 3.0)
	if m.Count() != 1 {
		t.Fatalf("Count = %d, want 1 after dt=6", m.Count())
	}
	if m.HasMemory("threat", "e2") {
		t.Error("e2 should be expired")
	}

	count, _ := blackboard.Get(board, blackboard.KeyMemoryCount)
	if count != 1 {
		t.Errorf("BB memory_count = %d, want 1", count)
	}
}

func TestMemory_Tick_ThreatValue(t *testing.T) {
	m := newTestMemory(5)
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "e1", Value: 50, Timestamp: 100, TTL: 60})
	m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "e2", Value: 80, Timestamp: 200, TTL: 60})
	m.AddMemory(component.MemoryEntry{Type: "location", TargetID: "l1", Value: 99, Timestamp: 300, TTL: 60})

	board := blackboard.New()
	m.Tick(board, 0.1)

	tv, _ := blackboard.Get(board, blackboard.KeyMemoryThreatValue)
	if tv != 80 {
		t.Errorf("memory_threat_value = %f, want 80 (max threat memory)", tv)
	}
}

func TestMemory_Tick_NoThreatMemory(t *testing.T) {
	m := newTestMemory(5)
	m.AddMemory(component.MemoryEntry{Type: "location", TargetID: "l1", Value: 3, Timestamp: 100, TTL: 60})

	board := blackboard.New()
	m.Tick(board, 0.1)

	tv, _ := blackboard.Get(board, blackboard.KeyMemoryThreatValue)
	if tv != 0 {
		t.Errorf("memory_threat_value = %f, want 0 (no threat memories)", tv)
	}
}

func BenchmarkMemory_Tick_1000Entries(b *testing.B) {
	raw := json.RawMessage(`{"capacity":1000,"memory_types":["threat"],"decay_time":60}`)
	comp, _ := component.MemoryFactory(raw)
	m := comp.(*component.MemoryComponent)
	for i := 0; i < 1000; i++ {
		m.AddMemory(component.MemoryEntry{Type: "threat", TargetID: "e" + string(rune(i)), Value: float64(i), Timestamp: int64(i), TTL: 60})
	}
	board := blackboard.New()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Tick(board, 0.001)
	}
}
