package zone_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/social"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/zone"
)

// configsDir returns the repo-relative configs path from internal/runtime/zone/.
func configsDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(wd, "..", "..", "..", "configs")
}

// --- AllZones / Count / NPCIDs ---

func TestZoneManager_AllZones(t *testing.T) {
	zm := zone.NewZoneManager()
	if got := zm.AllZones(); len(got) != 0 {
		t.Fatalf("empty manager AllZones len = %d, want 0", len(got))
	}

	zm.AddZone(&zone.Zone{ID: "a", Active: true})
	zm.AddZone(&zone.Zone{ID: "b", Active: false})

	all := zm.AllZones()
	if len(all) != 2 {
		t.Fatalf("AllZones len = %d, want 2", len(all))
	}
	seen := map[string]bool{}
	for _, z := range all {
		seen[z.ID] = true
	}
	if !seen["a"] || !seen["b"] {
		t.Fatalf("AllZones missing ids: %v", seen)
	}
}

func TestZone_NPCIDs_Empty(t *testing.T) {
	z := &zone.Zone{ID: "x"}
	if ids := z.NPCIDs(); len(ids) != 0 {
		t.Fatalf("new zone NPCIDs len = %d, want 0", len(ids))
	}
}

// --- Spawn ---

func loadMeadow(t *testing.T, src config.Source) *zone.Zone {
	t.Helper()
	raw, err := src.LoadRegionConfig("meadow")
	if err != nil {
		t.Fatalf("load region meadow: %v", err)
	}
	var z zone.Zone
	if err := json.Unmarshal(raw, &z); err != nil {
		t.Fatalf("parse meadow: %v", err)
	}
	return &z
}

func TestZone_Spawn_HappyPath(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	z := loadMeadow(t, src)
	z.Active = true

	reg := npc.NewRegistry()
	if err := z.Spawn(compReg, src, btReg, reg, nil); err != nil {
		t.Fatalf("spawn: %v", err)
	}

	// meadow.json: butterfly_01 count=3
	if reg.Count() != 3 {
		t.Fatalf("NPC count = %d, want 3", reg.Count())
	}
	ids := z.NPCIDs()
	if len(ids) != 3 {
		t.Fatalf("NPCIDs len = %d, want 3", len(ids))
	}

	// zone_id 注入到 position 组件
	reg.ForEach(func(inst *npc.Instance) {
		pos, ok := npc.GetComponent[*component.PositionComponent](inst, "position")
		if !ok {
			t.Fatalf("NPC %s missing position component", inst.ID)
		}
		if pos.ZoneID != "meadow" {
			t.Fatalf("NPC %s zone_id = %q, want meadow", inst.ID, pos.ZoneID)
		}
	})
}

func TestZone_Spawn_WithGroupManager(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	gm := social.NewGroupManager()

	z := loadMeadow(t, src)
	z.Active = true

	reg := npc.NewRegistry()
	if err := z.Spawn(compReg, src, btReg, reg, gm); err != nil {
		t.Fatalf("spawn with gm: %v", err)
	}
	// butterfly 无 social 组件，gm.Register 内部 early-return；
	// 本测试仅断言非 nil gm 路径不崩溃
	if reg.Count() != 3 {
		t.Fatalf("NPC count = %d, want 3", reg.Count())
	}
}

func TestZone_Spawn_EmptySpawnPoints(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	// SpawnPoints 为空 → warn + continue，Spawn 返回 nil 且不产生 NPC
	z := &zone.Zone{
		ID: "empty_points",
		SpawnTable: []zone.SpawnEntry{
			{TemplateRef: "butterfly_01", Count: 3, SpawnPoints: nil},
		},
	}
	reg := npc.NewRegistry()
	if err := z.Spawn(compReg, src, btReg, reg, nil); err != nil {
		t.Fatalf("spawn should not error on empty spawn_points, got: %v", err)
	}
	if reg.Count() != 0 {
		t.Fatalf("NPC count = %d, want 0 (no spawn_points)", reg.Count())
	}
	if len(z.NPCIDs()) != 0 {
		t.Fatalf("NPCIDs len = %d, want 0", len(z.NPCIDs()))
	}
}

func TestZone_Spawn_TemplateNotFound(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	z := &zone.Zone{
		ID: "bad_tmpl",
		SpawnTable: []zone.SpawnEntry{
			{
				TemplateRef: "nonexistent_template_xxx",
				Count:       1,
				SpawnPoints: []zone.Position{{X: 0, Z: 0}},
			},
		},
	}
	reg := npc.NewRegistry()
	err := z.Spawn(compReg, src, btReg, reg, nil)
	if err == nil {
		t.Fatal("expected error for missing template_ref")
	}
	if reg.Count() != 0 {
		t.Fatalf("NPC count = %d, want 0 on error", reg.Count())
	}
}
