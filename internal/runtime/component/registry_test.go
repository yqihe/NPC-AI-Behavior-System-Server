package component_test

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
)

// stubComponent 测试用组件
type stubComponent struct {
	Val string
}

func (s *stubComponent) Name() string { return "stub" }

func stubFactory(raw json.RawMessage) (component.Component, error) {
	var cfg struct {
		Val string `json:"val"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return &stubComponent{Val: cfg.Val}, nil
}

func TestRegistry_RegisterAndCreate(t *testing.T) {
	reg := component.NewRegistry()
	reg.Register("stub", stubFactory)

	raw := json.RawMessage(`{"val":"hello"}`)
	comp, err := reg.Create("stub", raw)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if comp.Name() != "stub" {
		t.Errorf("Name() = %q, want %q", comp.Name(), "stub")
	}
	s, ok := comp.(*stubComponent)
	if !ok {
		t.Fatal("type assertion failed")
	}
	if s.Val != "hello" {
		t.Errorf("Val = %q, want %q", s.Val, "hello")
	}
}

func TestRegistry_CreateUnknown(t *testing.T) {
	reg := component.NewRegistry()
	_, err := reg.Create("nonexistent", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for unknown component, got nil")
	}
}

func TestRegistry_DuplicateRegisterPanics(t *testing.T) {
	reg := component.NewRegistry()
	reg.Register("stub", stubFactory)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	reg.Register("stub", stubFactory)
}

func TestRegistry_Has(t *testing.T) {
	reg := component.NewRegistry()
	if reg.Has("stub") {
		t.Error("Has(stub) should be false before registration")
	}
	reg.Register("stub", stubFactory)
	if !reg.Has("stub") {
		t.Error("Has(stub) should be true after registration")
	}
}
