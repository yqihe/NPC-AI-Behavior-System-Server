package npc

import (
	"fmt"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
)

// --- fakeSource 最小化 Source 实现，供 ADMIN template 单测用 ---

type fakeSource struct {
	fsms  map[string]*fsm.FSMConfig
	bts   map[string][]byte
	tmpls map[string][]byte
}

func (f *fakeSource) LoadFSMConfig(name string) (*fsm.FSMConfig, error) {
	c, ok := f.fsms[name]
	if !ok {
		return nil, fmt.Errorf("fsm %q not found", name)
	}
	return c, nil
}
func (f *fakeSource) LoadBTTree(name string) ([]byte, error) {
	d, ok := f.bts[name]
	if !ok {
		return nil, fmt.Errorf("bt %q not found", name)
	}
	return d, nil
}
func (f *fakeSource) LoadEventConfig(string) ([]byte, error)                { return nil, nil }
func (f *fakeSource) LoadAllEventConfigs() (map[string][]byte, error)       { return nil, nil }
func (f *fakeSource) LoadNPCTypeConfig(string) ([]byte, error)              { return nil, nil }
func (f *fakeSource) LoadNPCTemplate(name string) ([]byte, error)           { return f.tmpls[name], nil }
func (f *fakeSource) LoadAllNPCTemplates() (map[string][]byte, error)       { return f.tmpls, nil }
func (f *fakeSource) LoadRegionConfig(string) ([]byte, error)               { return nil, nil }
func (f *fakeSource) LoadAllRegionConfigs() (map[string][]byte, error)      { return nil, nil }

func newFakeSource() *fakeSource {
	return &fakeSource{
		fsms: map[string]*fsm.FSMConfig{
			"guard": {
				InitialState: "Idle",
				States:       []fsm.StateConfig{{Name: "Idle"}, {Name: "Alert"}},
			},
		},
		bts: map[string][]byte{
			"guard/idle":  []byte(`{"type":"stub_action","params":{"name":"idle","result":"success"}}`),
			"guard/alert": []byte(`{"type":"stub_action","params":{"name":"alert","result":"success"}}`),
		},
	}
}

// --- ParseADMINTemplate ---

func TestParseADMINTemplate_Valid(t *testing.T) {
	data := []byte(`{
		"template_ref": "admin-uuid-abc",
		"fields": {"hp": 100, "attack": 15, "visual_range": 30.0},
		"behavior": {"fsm_ref": "guard", "bt_refs": {"Idle": "guard/idle"}}
	}`)
	tmpl, err := ParseADMINTemplate("guard_basic", data)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Name != "guard_basic" {
		t.Errorf("expected name=guard_basic, got %q", tmpl.Name)
	}
	if tmpl.TemplateRef != "admin-uuid-abc" {
		t.Errorf("expected template_ref=admin-uuid-abc, got %q", tmpl.TemplateRef)
	}
	if tmpl.Fields["hp"].(float64) != 100 {
		t.Errorf("expected hp=100, got %v", tmpl.Fields["hp"])
	}
	if tmpl.Behavior.FSMRef != "guard" {
		t.Errorf("expected fsm_ref=guard, got %q", tmpl.Behavior.FSMRef)
	}
}

func TestParseADMINTemplate_MissingFSMRef(t *testing.T) {
	data := []byte(`{"template_ref":"x","fields":{},"behavior":{}}`)
	_, err := ParseADMINTemplate("broken", data)
	if err == nil {
		t.Error("expected error for missing fsm_ref")
	}
}

func TestParseADMINTemplate_InvalidJSON(t *testing.T) {
	_, err := ParseADMINTemplate("x", []byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseADMINTemplate_NilFieldsNormalized(t *testing.T) {
	data := []byte(`{"template_ref":"x","behavior":{"fsm_ref":"guard","bt_refs":{"Idle":"guard/idle"}}}`)
	tmpl, err := ParseADMINTemplate("x", data)
	if err != nil {
		t.Fatal(err)
	}
	if tmpl.Fields == nil {
		t.Error("expected Fields to be initialized to empty map, got nil")
	}
}

// --- NewInstanceFromADMIN ---

func adminTemplateFixture(fields map[string]any) *ADMINTemplate {
	if fields == nil {
		fields = map[string]any{}
	}
	return &ADMINTemplate{
		Name:        "guard_basic",
		TemplateRef: "admin-uuid-1",
		Fields:      fields,
		Behavior: ADMINBehavior{
			FSMRef: "guard",
			BTRefs: map[string]string{"Idle": "guard/idle", "Alert": "guard/alert"},
		},
	}
}

func TestNewInstanceFromADMIN_HappyPath(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := adminTemplateFixture(map[string]any{
		"hp":             100.0,
		"attack":         15.0,
		"visual_range":   80.0,
		"auditory_range": 150.0,
	})

	inst, err := NewInstanceFromADMIN("npc_1", event.Vec3{X: 5, Z: 10}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	if inst.ID != "npc_1" {
		t.Errorf("expected id=npc_1, got %q", inst.ID)
	}
	if inst.TypeName != "guard_basic" {
		t.Errorf("expected typename=guard_basic, got %q", inst.TypeName)
	}
	if inst.FSM == nil || inst.FSM.Current() != "Idle" {
		t.Errorf("expected initial state=Idle, got %v", inst.FSM)
	}
	if len(inst.BTrees) != 2 {
		t.Errorf("expected 2 BTs, got %d", len(inst.BTrees))
	}
	if inst.Perception == nil || inst.Perception.VisualRange != 80.0 {
		t.Errorf("expected visual_range=80, got %v", inst.Perception)
	}
	hp, ok := inst.BB.GetRaw("hp")
	if !ok || hp.(float64) != 100.0 {
		t.Errorf("expected hp=100 in BB, got %v (ok=%v)", hp, ok)
	}
}

// T1 acceptance: 5 默认组件全装（identity/position/behavior/perception/movement）
func TestNewInstanceFromADMIN_AllDefaultComponentsPresent(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := adminTemplateFixture(map[string]any{"move_speed": 5.0, "perception_range": 20.0})

	inst, err := NewInstanceFromADMIN("npc_default", event.Vec3{X: 100, Z: 200}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"identity", "position", "behavior", "perception", "movement"} {
		if !inst.HasComponent(name) {
			t.Errorf("expected default component %q present", name)
		}
	}
}

// T1 acceptance: opt-in 全 absent → 5 能力组件全部不创建（R17 absent ≡ false）
func TestNewInstanceFromADMIN_OptInAllAbsent(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := adminTemplateFixture(nil) // 无 enable_* 字段

	inst, err := NewInstanceFromADMIN("npc_absent", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"memory", "emotion", "needs", "personality", "social"} {
		if inst.HasComponent(name) {
			t.Errorf("opt-in absent: component %q should not be instantiated", name)
		}
	}
}

// T1 acceptance: opt-in 混合启用 - memory+emotion=true, 其余 false
func TestNewInstanceFromADMIN_OptInMixedEnabled(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := adminTemplateFixture(map[string]any{
		"enable_memory":  true,
		"enable_emotion": true,
	})

	inst, err := NewInstanceFromADMIN("npc_mixed", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	if !inst.HasComponent("memory") {
		t.Error("expected memory component (enable_memory=true)")
	}
	if !inst.HasComponent("emotion") {
		t.Error("expected emotion component (enable_emotion=true)")
	}
	for _, name := range []string{"needs", "personality", "social"} {
		if inst.HasComponent(name) {
			t.Errorf("expected no %q component (absent)", name)
		}
	}
}

// T1 acceptance: opt-in false 与 absent 等价（都不创建）
func TestNewInstanceFromADMIN_OptInExplicitFalseEquivalentToAbsent(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := adminTemplateFixture(map[string]any{
		"enable_memory":      false,
		"enable_emotion":     false,
		"enable_needs":       false,
		"enable_personality": false,
		"enable_social":      false,
	})

	inst, err := NewInstanceFromADMIN("npc_false", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"memory", "emotion", "needs", "personality", "social"} {
		if inst.HasComponent(name) {
			t.Errorf("opt-in explicit false: component %q should not be instantiated", name)
		}
	}
}

// T1 acceptance: 所有 opt-in 全 true → 5 能力组件全部装配
func TestNewInstanceFromADMIN_OptInAllEnabled(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := adminTemplateFixture(map[string]any{
		"enable_memory":      true,
		"enable_emotion":     true,
		"enable_needs":       true,
		"enable_personality": true,
		"enable_social":      true,
		"aggression":         "aggressive",
		"group_id":           "pack_alpha",
		"social_role":        "leader",
	})

	inst, err := NewInstanceFromADMIN("npc_full", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"memory", "emotion", "needs", "personality", "social"} {
		if !inst.HasComponent(name) {
			t.Errorf("expected opt-in component %q (all enabled)", name)
		}
	}
	// Personality 应用 aggression → aggressive type
	pers, _ := GetComponent[*component.PersonalityComponent](inst, "personality")
	if pers == nil || pers.PersonalityType != "aggressive" {
		t.Errorf("expected personality_type=aggressive from fields.aggression, got %v", pers)
	}
	// Social 应读 group_id / social_role
	soc, _ := GetComponent[*component.SocialComponent](inst, "social")
	if soc == nil || soc.GroupID != "pack_alpha" || soc.Role != "leader" {
		t.Errorf("expected social GroupID=pack_alpha Role=leader, got %+v", soc)
	}
}

// T1 acceptance: 未知 fields SetDynamic 透传（含 hp 孤儿字段、is_boss、loot_table 等）
func TestNewInstanceFromADMIN_UnknownFieldsTransparent(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := adminTemplateFixture(map[string]any{
		"hp":          100.0, // guard_basic 孤儿字段
		"is_boss":     true,
		"loot_table":  "wolf_alpha_loot",
		"attack_power": 45.0,
	})

	inst, err := NewInstanceFromADMIN("npc_orphan", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		key  string
		want any
	}{
		{"hp", 100.0},
		{"is_boss", true},
		{"loot_table", "wolf_alpha_loot"},
		{"attack_power", 45.0},
	}
	for _, tt := range tests {
		got, ok := inst.BB.GetRaw(tt.key)
		if !ok {
			t.Errorf("field %q not written to BB", tt.key)
			continue
		}
		if got != tt.want {
			t.Errorf("field %q: expected %v, got %v", tt.key, tt.want, got)
		}
	}
}

func TestNewInstanceFromADMIN_DefaultsWhenFieldsMissing(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := &ADMINTemplate{
		Name:   "minimal",
		Fields: map[string]any{},
		Behavior: ADMINBehavior{
			FSMRef: "guard",
			BTRefs: map[string]string{"Idle": "guard/idle"},
		},
	}

	inst, err := NewInstanceFromADMIN("npc_2", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Perception.VisualRange != defaultVisualRange {
		t.Errorf("expected default visual_range=%v, got %v", defaultVisualRange, inst.Perception.VisualRange)
	}
	if inst.Perception.AuditoryRange != defaultAuditoryRange {
		t.Errorf("expected default auditory_range=%v, got %v", defaultAuditoryRange, inst.Perception.AuditoryRange)
	}
}

// ADMIN 当前只有合并的 perception_range，visual/auditory 都应回落到它
func TestNewInstanceFromADMIN_PerceptionRangeFallback(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := adminTemplateFixture(map[string]any{"perception_range": 75.0})

	inst, err := NewInstanceFromADMIN("npc_merged", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Perception.VisualRange != 75.0 {
		t.Errorf("expected visual_range=75 via perception_range fallback, got %v", inst.Perception.VisualRange)
	}
	if inst.Perception.AuditoryRange != 75.0 {
		t.Errorf("expected auditory_range=75 via perception_range fallback, got %v", inst.Perception.AuditoryRange)
	}
}

// 专用字段存在时优先于合并字段
func TestNewInstanceFromADMIN_SpecificOverridesMerged(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := adminTemplateFixture(map[string]any{
		"perception_range": 75.0,
		"visual_range":     120.0,
	})

	inst, err := NewInstanceFromADMIN("npc_mixed", event.Vec3{}, tmpl, src, btReg, compReg)
	if err != nil {
		t.Fatal(err)
	}
	if inst.Perception.VisualRange != 120.0 {
		t.Errorf("expected visual_range=120 (specific wins), got %v", inst.Perception.VisualRange)
	}
	if inst.Perception.AuditoryRange != 75.0 {
		t.Errorf("expected auditory_range=75 (fallback to merged), got %v", inst.Perception.AuditoryRange)
	}
}

func TestNewInstanceFromADMIN_FSMNotFound(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := &ADMINTemplate{
		Name:   "bad",
		Fields: map[string]any{},
		Behavior: ADMINBehavior{
			FSMRef: "nonexistent",
			BTRefs: map[string]string{"Idle": "guard/idle"},
		},
	}
	_, err := NewInstanceFromADMIN("npc_3", event.Vec3{}, tmpl, src, btReg, compReg)
	if err == nil {
		t.Error("expected error for missing FSM")
	}
}

func TestNewInstanceFromADMIN_BTNotFound(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	tmpl := &ADMINTemplate{
		Name:   "bad_bt",
		Fields: map[string]any{},
		Behavior: ADMINBehavior{
			FSMRef: "guard",
			BTRefs: map[string]string{"Idle": "nonexistent/tree"},
		},
	}
	_, err := NewInstanceFromADMIN("npc_4", event.Vec3{}, tmpl, src, btReg, compReg)
	if err == nil {
		t.Error("expected error for missing BT")
	}
}

func TestNewInstanceFromADMIN_NilTemplate(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	_, err := NewInstanceFromADMIN("npc", event.Vec3{}, nil, src, btReg, compReg)
	if err == nil {
		t.Error("expected error for nil template")
	}
}

func TestNewInstanceFromADMIN_NilCompRegistry(t *testing.T) {
	src := newFakeSource()
	btReg := bt.DefaultRegistry()
	tmpl := adminTemplateFixture(nil)
	_, err := NewInstanceFromADMIN("npc", event.Vec3{}, tmpl, src, btReg, nil)
	if err == nil {
		t.Error("expected error for nil component registry")
	}
}
