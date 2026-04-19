package npctest

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
)

// fakeSource 最小化 Source 实现，仅供本子包测试用
type fakeSource struct{}

func (fakeSource) LoadFSMConfig(name string) (*fsm.FSMConfig, error) {
	if name != "guard" {
		return nil, fmt.Errorf("fsm %q not found", name)
	}
	return &fsm.FSMConfig{
		InitialState: "Idle",
		States:       []fsm.StateConfig{{Name: "Idle"}},
	}, nil
}
func (fakeSource) LoadBTTree(name string) ([]byte, error) {
	if name == "guard/idle" {
		return []byte(`{"type":"stub_action","params":{"name":"idle","result":"success"}}`), nil
	}
	return nil, fmt.Errorf("bt %q not found", name)
}
func (fakeSource) LoadEventConfig(string) ([]byte, error)           { return nil, nil }
func (fakeSource) LoadAllEventConfigs() (map[string][]byte, error)  { return nil, nil }
func (fakeSource) LoadNPCTypeConfig(string) ([]byte, error)         { return nil, nil }
func (fakeSource) LoadNPCTemplate(string) ([]byte, error)           { return nil, nil }
func (fakeSource) LoadAllNPCTemplates() (map[string][]byte, error)  { return nil, nil }
func (fakeSource) LoadRegionConfig(string) ([]byte, error)          { return nil, nil }
func (fakeSource) LoadAllRegionConfigs() (map[string][]byte, error) { return nil, nil }

func baseTmpl(fields map[string]any) *npc.ADMINTemplate {
	if fields == nil {
		fields = map[string]any{}
	}
	return &npc.ADMINTemplate{
		Name:        "test_npc",
		TemplateRef: "generic",
		Fields:      fields,
		Behavior: npc.ADMINBehavior{
			FSMRef: "guard",
			BTRefs: map[string]string{"Idle": "guard/idle"},
		},
	}
}

// empty extras 不影响 base instance 行为
func TestNewInstanceWithExtras_EmptyExtras(t *testing.T) {
	inst, err := NewInstanceWithExtras("t1", event.Vec3{}, baseTmpl(nil), nil,
		fakeSource{}, bt.DefaultRegistry(), component.DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"identity", "position", "behavior", "perception", "movement"} {
		if !inst.HasComponent(name) {
			t.Errorf("expected default component %q", name)
		}
	}
	for _, name := range []string{"memory", "emotion", "needs", "personality", "social"} {
		if inst.HasComponent(name) {
			t.Errorf("unexpected opt-in component %q with nil extras + absent opt-in", name)
		}
	}
}

// 注入 memory 单独一个
func TestNewInstanceWithExtras_InjectMemory(t *testing.T) {
	extras := map[string]json.RawMessage{
		"memory": []byte(`{"capacity":5,"memory_types":["threat"],"decay_time":20}`),
	}
	inst, err := NewInstanceWithExtras("t2", event.Vec3{}, baseTmpl(nil), extras,
		fakeSource{}, bt.DefaultRegistry(), component.DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	mem, _ := npc.GetComponent[*component.MemoryComponent](inst, "memory")
	if mem == nil {
		t.Fatal("expected memory component injected")
	}
	if mem.Capacity != 5 {
		t.Errorf("expected capacity=5 from extras, got %d", mem.Capacity)
	}
}

// 注入 memory + emotion
func TestNewInstanceWithExtras_InjectMemoryEmotion(t *testing.T) {
	extras := map[string]json.RawMessage{
		"memory":  []byte(`{"capacity":10,"memory_types":["threat"],"decay_time":30}`),
		"emotion": []byte(`{"emotion_states":[{"name":"fear","value":0,"accumulate_rate":20,"decay_rate":5}]}`),
	}
	inst, err := NewInstanceWithExtras("t3", event.Vec3{}, baseTmpl(nil), extras,
		fakeSource{}, bt.DefaultRegistry(), component.DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if !inst.HasComponent("memory") {
		t.Error("expected memory component")
	}
	if !inst.HasComponent("emotion") {
		t.Error("expected emotion component")
	}
}

// extras 与 opt-in 同名时 extras 覆盖（显式意图优先）
func TestNewInstanceWithExtras_ExtrasOverridesOptIn(t *testing.T) {
	// fields 开启 enable_memory（会装默认 capacity=10 memory），同时 extras 传 capacity=99 memory
	fields := map[string]any{"enable_memory": true}
	extras := map[string]json.RawMessage{
		"memory": []byte(`{"capacity":99,"memory_types":["location"],"decay_time":60}`),
	}
	inst, err := NewInstanceWithExtras("t4", event.Vec3{}, baseTmpl(fields), extras,
		fakeSource{}, bt.DefaultRegistry(), component.DefaultRegistry())
	if err != nil {
		t.Fatal(err)
	}
	mem, _ := npc.GetComponent[*component.MemoryComponent](inst, "memory")
	if mem == nil {
		t.Fatal("expected memory component present")
	}
	if mem.Capacity != 99 {
		t.Errorf("expected extras capacity=99 to override opt-in default, got %d", mem.Capacity)
	}
	if len(mem.MemoryTypes) != 1 || mem.MemoryTypes[0] != "location" {
		t.Errorf("expected extras memory_types=[location] to override, got %v", mem.MemoryTypes)
	}
}

// extras factory 失败时返回错误
func TestNewInstanceWithExtras_ExtrasFactoryError(t *testing.T) {
	extras := map[string]json.RawMessage{
		"memory": []byte(`not-json`),
	}
	_, err := NewInstanceWithExtras("t5", event.Vec3{}, baseTmpl(nil), extras,
		fakeSource{}, bt.DefaultRegistry(), component.DefaultRegistry())
	if err == nil {
		t.Error("expected factory error on malformed extras JSON")
	}
}

// base instance 失败时 extras 被跳过
func TestNewInstanceWithExtras_BaseError(t *testing.T) {
	bad := baseTmpl(nil)
	bad.Behavior.FSMRef = "nonexistent"
	_, err := NewInstanceWithExtras("t6", event.Vec3{}, bad, nil,
		fakeSource{}, bt.DefaultRegistry(), component.DefaultRegistry())
	if err == nil {
		t.Error("expected base instance creation error to propagate")
	}
}
