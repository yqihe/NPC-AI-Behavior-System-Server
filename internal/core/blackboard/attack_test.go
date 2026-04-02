package blackboard_test

import (
	"sync"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
)

// 攻击性测试：尝试搞崩 Blackboard

func TestAttack_GetBeforeSet(t *testing.T) {
	// 从未写入的 BB 读取每个 Key，应该返回零值和 false，不 panic
	bb := blackboard.New()

	if val, ok := blackboard.Get(bb, blackboard.KeyThreatLevel); ok || val != 0 {
		t.Fatalf("expected (0, false), got (%v, %v)", val, ok)
	}
	if val, ok := blackboard.Get(bb, blackboard.KeyThreatSource); ok || val != "" {
		t.Fatalf("expected ('', false), got (%v, %v)", val, ok)
	}
	if val, ok := blackboard.Get(bb, blackboard.KeyThreatExpireAt); ok || val != 0 {
		t.Fatalf("expected (0, false), got (%v, %v)", val, ok)
	}
}

func TestAttack_SetZeroValues(t *testing.T) {
	// 写入零值应该能正常读回，且 Has 返回 true
	bb := blackboard.New()

	blackboard.Set(bb, blackboard.KeyThreatLevel, 0.0)
	blackboard.Set(bb, blackboard.KeyThreatSource, "")
	blackboard.Set(bb, blackboard.KeyThreatExpireAt, int64(0))

	if val, ok := blackboard.Get(bb, blackboard.KeyThreatLevel); !ok || val != 0.0 {
		t.Fatalf("expected (0, true), got (%v, %v)", val, ok)
	}
	if !blackboard.Has(bb, blackboard.KeyThreatSource) {
		t.Fatal("expected Has to return true for zero-value string")
	}
	if val, _ := blackboard.Get(bb, blackboard.KeyThreatSource); val != "" {
		t.Fatalf("expected empty string, got %q", val)
	}
}

func TestAttack_NegativeValues(t *testing.T) {
	bb := blackboard.New()

	blackboard.Set(bb, blackboard.KeyThreatLevel, -999.99)
	val, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if !ok || val != -999.99 {
		t.Fatalf("expected (-999.99, true), got (%v, %v)", val, ok)
	}

	blackboard.Set(bb, blackboard.KeyThreatExpireAt, int64(-1))
	ival, ok := blackboard.Get(bb, blackboard.KeyThreatExpireAt)
	if !ok || ival != -1 {
		t.Fatalf("expected (-1, true), got (%v, %v)", ival, ok)
	}
}

func TestAttack_OverwriteValue(t *testing.T) {
	bb := blackboard.New()

	blackboard.Set(bb, blackboard.KeyThreatLevel, 10.0)
	blackboard.Set(bb, blackboard.KeyThreatLevel, 90.0)

	val, _ := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if val != 90.0 {
		t.Fatalf("expected overwritten value 90.0, got %v", val)
	}
}

func TestAttack_DeleteThenGet(t *testing.T) {
	bb := blackboard.New()

	blackboard.Set(bb, blackboard.KeyThreatLevel, 50.0)
	blackboard.Delete(bb, blackboard.KeyThreatLevel)

	val, ok := blackboard.Get(bb, blackboard.KeyThreatLevel)
	if ok {
		t.Fatal("expected false after delete")
	}
	if val != 0 {
		t.Fatalf("expected zero value after delete, got %v", val)
	}
}

func TestAttack_DeleteNonExistent(t *testing.T) {
	// 删除从未写入的 Key，不应 panic
	bb := blackboard.New()
	blackboard.Delete(bb, blackboard.KeyThreatLevel)
}

func TestAttack_GetRawNonExistent(t *testing.T) {
	bb := blackboard.New()
	val, ok := bb.GetRaw("threat_level")
	if ok || val != nil {
		t.Fatalf("expected (nil, false), got (%v, %v)", val, ok)
	}
}

func TestAttack_HeavyConcurrency(t *testing.T) {
	bb := blackboard.New()
	var wg sync.WaitGroup

	// 1000 个 goroutine 同时读写不同的 Key
	for i := 0; i < 500; i++ {
		wg.Add(2)
		go func(v float64) {
			defer wg.Done()
			blackboard.Set(bb, blackboard.KeyThreatLevel, v)
			blackboard.Get(bb, blackboard.KeyThreatLevel)
			blackboard.Has(bb, blackboard.KeyThreatLevel)
		}(float64(i))
		go func(s string) {
			defer wg.Done()
			blackboard.Set(bb, blackboard.KeyThreatSource, s)
			blackboard.Get(bb, blackboard.KeyThreatSource)
			bb.GetRaw("threat_source")
		}("player_" + s(i))
	}

	wg.Wait()
}

func s(i int) string {
	return string(rune('0' + i%10))
}

func TestAttack_SetRawUnregisteredKey_Panics(t *testing.T) {
	bb := blackboard.New()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for SetRaw with unregistered key")
		}
		t.Logf("correctly panicked: %v", r)
	}()
	bb.SetRaw("totally_fake_key_xyz", 999)
}

func TestAttack_SetRawRegisteredKey_OK(t *testing.T) {
	bb := blackboard.New()
	// 已注册的 key 应正常写入，不 panic
	bb.SetRaw("threat_level", 42.0)
	val, ok := bb.GetRaw("threat_level")
	if !ok || val != 42.0 {
		t.Fatalf("expected (42.0, true), got (%v, %v)", val, ok)
	}
}

func TestAttack_ValidateEmptyString(t *testing.T) {
	// 空字符串不是合法的 Key 名称
	err := blackboard.ValidateKeyName("")
	if err == nil {
		t.Fatal("expected empty string key name to fail validation")
	}
}
