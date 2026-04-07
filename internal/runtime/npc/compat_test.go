package npc_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
)

func v2ConfigsDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..", "..", "configs", "npc_types")
}

func TestParseNPCTemplate_V2Format_Civilian(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(v2ConfigsDir(t), "civilian.json"))
	if err != nil {
		t.Fatalf("read civilian.json: %v", err)
	}

	tmpl, err := npc.ParseNPCTemplate(data)
	if err != nil {
		t.Fatalf("ParseNPCTemplate failed: %v", err)
	}

	if tmpl.Name != "civilian" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "civilian")
	}
	if tmpl.Preset != "reactive" {
		t.Errorf("Preset = %q, want %q", tmpl.Preset, "reactive")
	}

	// 必须有 identity, position, behavior, perception
	for _, name := range []string{"identity", "position", "behavior", "perception"} {
		if _, ok := tmpl.Components[name]; !ok {
			t.Errorf("missing component %q after v2 conversion", name)
		}
	}

	// 验证 behavior 内容
	var beh struct {
		FSMRef string            `json:"fsm_ref"`
		BTRefs map[string]string `json:"bt_refs"`
	}
	if err := json.Unmarshal(tmpl.Components["behavior"], &beh); err != nil {
		t.Fatalf("unmarshal behavior: %v", err)
	}
	if beh.FSMRef != "civilian" {
		t.Errorf("behavior.fsm_ref = %q, want %q", beh.FSMRef, "civilian")
	}
	if len(beh.BTRefs) == 0 {
		t.Error("behavior.bt_refs is empty")
	}
}

func TestParseNPCTemplate_V2Format_Guard(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(v2ConfigsDir(t), "guard.json"))
	if err != nil {
		t.Fatalf("read guard.json: %v", err)
	}

	tmpl, err := npc.ParseNPCTemplate(data)
	if err != nil {
		t.Fatalf("ParseNPCTemplate failed: %v", err)
	}
	if tmpl.Name != "guard" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "guard")
	}
}

func TestParseNPCTemplate_V2Format_Police(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(v2ConfigsDir(t), "police.json"))
	if err != nil {
		t.Fatalf("read police.json: %v", err)
	}

	tmpl, err := npc.ParseNPCTemplate(data)
	if err != nil {
		t.Fatalf("ParseNPCTemplate failed: %v", err)
	}
	if tmpl.Name != "police" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "police")
	}
}

func TestParseNPCTemplate_NewFormat(t *testing.T) {
	data := []byte(`{
		"name": "wolf_01",
		"preset": "reactive",
		"components": {
			"identity": {"name": "灰狼", "model_id": "wolf_gray"},
			"position": {"x": 100, "z": 200}
		}
	}`)

	tmpl, err := npc.ParseNPCTemplate(data)
	if err != nil {
		t.Fatalf("ParseNPCTemplate failed: %v", err)
	}
	if tmpl.Name != "wolf_01" {
		t.Errorf("Name = %q, want %q", tmpl.Name, "wolf_01")
	}
	if tmpl.Preset != "reactive" {
		t.Errorf("Preset = %q, want %q", tmpl.Preset, "reactive")
	}
}

func TestParseNPCTemplate_UnrecognizedFormat(t *testing.T) {
	data := []byte(`{"foo": "bar"}`)
	_, err := npc.ParseNPCTemplate(data)
	if err == nil {
		t.Fatal("expected error for unrecognized format")
	}
}
