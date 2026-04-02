package blackboard_test

import (
	"sync"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

func TestGetSet_TypeSafety(t *testing.T) {
	bb := blackboard.New()

	// Set float64, Get float64
	blackboard.Set(bb, blackboard.KeyThreatLevel, 75.0)
	val, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != 75.0 {
		t.Fatalf("expected 75.0, got %v", val)
	}

	// Set string, Get string
	blackboard.Set(bb, blackboard.KeyThreatSource, "player_1")
	sval, ok := blackboard.Get(bb, blackboard.KeyThreatSource)
	if !ok {
		t.Fatal("expected key to exist")
	}
	if sval != "player_1" {
		t.Fatalf("expected player_1, got %v", sval)
	}
}

func TestGet_MissingKey(t *testing.T) {
	bb := blackboard.New()

	val, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if ok {
		t.Fatal("expected key to not exist")
	}
	if val != 0 {
		t.Fatalf("expected zero value, got %v", val)
	}
}

func TestHas(t *testing.T) {
	bb := blackboard.New()

	if blackboard.Has(bb, blackboard.KeyThreatLevel) {
		t.Fatal("expected Has to return false for missing key")
	}

	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)

	if !blackboard.Has(bb, blackboard.KeyThreatLevel) {
		t.Fatal("expected Has to return true after Set")
	}
}

func TestDelete(t *testing.T) {
	bb := blackboard.New()

	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)
	blackboard.Delete(bb, blackboard.KeyThreatLevel)

	if blackboard.Has(bb, blackboard.KeyThreatLevel) {
		t.Fatal("expected key to be deleted")
	}
}

func TestGetRaw(t *testing.T) {
	bb := blackboard.New()

	blackboard.Set(bb, blackboard.KeyThreatLevel, 80.0)

	val, ok := bb.GetRaw("threat_level")
	if !ok {
		t.Fatal("expected raw key to exist")
	}
	fval, fok := val.(float64)
	if !fok {
		t.Fatalf("expected float64, got %T", val)
	}
	if fval != 80.0 {
		t.Fatalf("expected 80.0, got %v", fval)
	}

	_, ok = bb.GetRaw("nonexistent")
	if ok {
		t.Fatal("expected raw get of nonexistent key to return false")
	}
}

func TestValidateKeyName(t *testing.T) {
	// 已注册的 Key 应该校验通过
	if err := blackboard.ValidateKeyName("threat_level"); err != nil {
		t.Fatalf("expected registered key to pass validation, got: %v", err)
	}

	// 未注册的 Key 应该校验失败
	if err := blackboard.ValidateKeyName("nonexistent_key"); err == nil {
		t.Fatal("expected unregistered key to fail validation")
	}
}

func TestIsRegistered(t *testing.T) {
	if !blackboard.IsRegistered("threat_level") {
		t.Fatal("expected threat_level to be registered")
	}
	if blackboard.IsRegistered("totally_fake_key") {
		t.Fatal("expected totally_fake_key to not be registered")
	}
}

func TestRegisteredKeys(t *testing.T) {
	keys := blackboard.RegisteredKeys()
	if len(keys) == 0 {
		t.Fatal("expected at least one registered key")
	}

	// 验证 keys.go 中定义的 Key 都在列表中
	expected := map[string]bool{
		"threat_level":     false,
		"threat_source":    false,
		"threat_expire_at": false,
		"last_event_type":  false,
		"current_time":     false,
		"fsm_state":        false,
	}
	for _, k := range keys {
		if _, ok := expected[k]; ok {
			expected[k] = true
		}
	}
	for k, found := range expected {
		if !found {
			t.Errorf("expected key %q to be in RegisteredKeys()", k)
		}
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	bb := blackboard.New()
	var wg sync.WaitGroup

	// 多个 goroutine 并发写
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v float64) {
			defer wg.Done()
			blackboard.Set(bb, blackboard.KeyThreatLevel, v)
		}(float64(i))
	}

	// 多个 goroutine 并发读
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			blackboard.Get(bb, blackboard.KeyThreatLevel)
		}()
	}

	wg.Wait()

	// 只要不 panic/race 就算通过
	_, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if !ok {
		t.Fatal("expected key to exist after concurrent writes")
	}
}

func TestDuplicateKeyRegistration_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate key registration")
		}
	}()

	// 尝试注册一个已存在的 Key 名称，应该 panic
	blackboard.NewKey[string]("threat_level")
}
