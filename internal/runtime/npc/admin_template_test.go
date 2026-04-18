package npc

import (
	"fmt"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
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

// --- NewInstanceFromADMIN ---

func TestNewInstanceFromADMIN_HappyPath(t *testing.T) {
	src := newFakeSource()
	reg := bt.DefaultRegistry()

	tmpl := &ADMINTemplate{
		Name:        "guard_basic",
		TemplateRef: "admin-uuid-1",
		Fields: map[string]any{
			"hp":              100.0,
			"attack":          15.0,
			"visual_range":    80.0,
			"auditory_range":  150.0,
		},
		Behavior: ADMINBehavior{
			FSMRef: "guard",
			BTRefs: map[string]string{"Idle": "guard/idle", "Alert": "guard/alert"},
		},
	}

	inst, err := NewInstanceFromADMIN("npc_1", event.Vec3{X: 5, Z: 10}, tmpl, src, reg)
	if err != nil {
		t.Fatal(err)
	}
	if inst.ID != "npc_1" {
		t.Errorf("expected id=npc_1, got %q", inst.ID)
	}
	if inst.TypeName != "guard_basic" {
		t.Errorf("expected typename=guard_basic, got %q", inst.TypeName)
	}
	if inst.FSM.Current() != "Idle" {
		t.Errorf("expected initial state=Idle, got %q", inst.FSM.Current())
	}
	if len(inst.BTrees) != 2 {
		t.Errorf("expected 2 BTs, got %d", len(inst.BTrees))
	}
	// 感知距离从 fields 读取而非默认
	if inst.Perception.VisualRange != 80.0 {
		t.Errorf("expected visual_range=80, got %v", inst.Perception.VisualRange)
	}
	// fields 写入 BB
	hp, ok := inst.BB.GetRaw("hp")
	if !ok || hp.(float64) != 100.0 {
		t.Errorf("expected hp=100 in BB, got %v (ok=%v)", hp, ok)
	}
}

func TestNewInstanceFromADMIN_DefaultsWhenFieldsMissing(t *testing.T) {
	src := newFakeSource()
	reg := bt.DefaultRegistry()

	tmpl := &ADMINTemplate{
		Name:   "minimal",
		Fields: map[string]any{}, // 空 fields
		Behavior: ADMINBehavior{
			FSMRef: "guard",
			BTRefs: map[string]string{"Idle": "guard/idle"},
		},
	}

	inst, err := NewInstanceFromADMIN("npc_2", event.Vec3{}, tmpl, src, reg)
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
	reg := bt.DefaultRegistry()

	tmpl := &ADMINTemplate{
		Name:   "merged",
		Fields: map[string]any{"perception_range": 75.0}, // ADMIN 合并字段
		Behavior: ADMINBehavior{
			FSMRef: "guard",
			BTRefs: map[string]string{"Idle": "guard/idle"},
		},
	}

	inst, err := NewInstanceFromADMIN("npc_merged", event.Vec3{}, tmpl, src, reg)
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
	reg := bt.DefaultRegistry()

	tmpl := &ADMINTemplate{
		Name: "mixed",
		Fields: map[string]any{
			"perception_range": 75.0, // 合并值
			"visual_range":     120.0, // 专用值应胜出
			// 无 auditory_range，应 fallback 到 perception_range
		},
		Behavior: ADMINBehavior{
			FSMRef: "guard",
			BTRefs: map[string]string{"Idle": "guard/idle"},
		},
	}

	inst, err := NewInstanceFromADMIN("npc_mixed", event.Vec3{}, tmpl, src, reg)
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
	reg := bt.DefaultRegistry()

	tmpl := &ADMINTemplate{
		Name:     "bad",
		Fields:   map[string]any{},
		Behavior: ADMINBehavior{FSMRef: "nonexistent"},
	}
	_, err := NewInstanceFromADMIN("npc_3", event.Vec3{}, tmpl, src, reg)
	if err == nil {
		t.Error("expected error for missing FSM")
	}
}

func TestNewInstanceFromADMIN_BTNotFound(t *testing.T) {
	src := newFakeSource()
	reg := bt.DefaultRegistry()

	tmpl := &ADMINTemplate{
		Name:   "bad_bt",
		Fields: map[string]any{},
		Behavior: ADMINBehavior{
			FSMRef: "guard",
			BTRefs: map[string]string{"Idle": "nonexistent/tree"},
		},
	}
	_, err := NewInstanceFromADMIN("npc_4", event.Vec3{}, tmpl, src, reg)
	if err == nil {
		t.Error("expected error for missing BT")
	}
}

func TestNewInstanceFromADMIN_NilTemplate(t *testing.T) {
	src := newFakeSource()
	reg := bt.DefaultRegistry()
	_, err := NewInstanceFromADMIN("npc", event.Vec3{}, nil, src, reg)
	if err == nil {
		t.Error("expected error for nil template")
	}
}
