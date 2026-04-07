package zone_test

import (
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/zone"
)

func TestZoneManager_AddAndGet(t *testing.T) {
	zm := zone.NewZoneManager()
	z := &zone.Zone{ID: "meadow", Name: "草原", Active: true}
	zm.AddZone(z)

	got, ok := zm.GetZone("meadow")
	if !ok {
		t.Fatal("GetZone should return true")
	}
	if got.Name != "草原" {
		t.Errorf("Name = %q, want %q", got.Name, "草原")
	}
}

func TestZoneManager_IsActive(t *testing.T) {
	zm := zone.NewZoneManager()
	z := &zone.Zone{ID: "meadow", Active: true}
	zm.AddZone(z)

	if !zm.IsActive("meadow") {
		t.Error("active zone should return true")
	}

	zm.Sleep("meadow")
	if zm.IsActive("meadow") {
		t.Error("sleeping zone should return false")
	}

	zm.Wake("meadow")
	if !zm.IsActive("meadow") {
		t.Error("woken zone should return true")
	}
}

func TestZoneManager_IsActive_EmptyZoneID(t *testing.T) {
	zm := zone.NewZoneManager()
	if !zm.IsActive("") {
		t.Error("empty zoneID should return true (v2 compat)")
	}
}

func TestZoneManager_IsActive_UnregisteredZone(t *testing.T) {
	zm := zone.NewZoneManager()
	if !zm.IsActive("nonexistent") {
		t.Error("unregistered zone should return true")
	}
}

func TestZoneManager_Count(t *testing.T) {
	zm := zone.NewZoneManager()
	if zm.Count() != 0 {
		t.Errorf("Count = %d, want 0", zm.Count())
	}
	zm.AddZone(&zone.Zone{ID: "a", Active: true})
	zm.AddZone(&zone.Zone{ID: "b", Active: true})
	if zm.Count() != 2 {
		t.Errorf("Count = %d, want 2", zm.Count())
	}
}
