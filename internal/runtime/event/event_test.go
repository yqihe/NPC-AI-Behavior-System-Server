package event

import (
	"sync"
	"testing"
)

var testExplosion = &EventTypeConfig{
	Name:            "explosion",
	DefaultSeverity: 80,
	DefaultTTL:      15.0,
	PerceptionMode:  "auditory",
	Range:           500.0,
}

var testGunshot = &EventTypeConfig{
	Name:            "gunshot",
	DefaultSeverity: 90,
	DefaultTTL:      10.0,
	PerceptionMode:  "auditory",
	Range:           300.0,
}

// --- Vec3 + Distance ---

func TestDistance_SamePoint(t *testing.T) {
	d := Distance(Vec3{1, 2, 3}, Vec3{1, 2, 3})
	if d != 0 {
		t.Errorf("expected 0, got %f", d)
	}
}

func TestDistance_XZOnly(t *testing.T) {
	// Y 轴差异应该被忽略
	d := Distance(Vec3{0, 0, 0}, Vec3{3, 999, 4})
	expected := 5.0 // 3-4-5 三角形
	if diff := d - expected; diff > 0.001 || diff < -0.001 {
		t.Errorf("expected %f, got %f", expected, d)
	}
}

// --- NewEvent ---

func TestNewEvent_Defaults(t *testing.T) {
	evt := NewEvent(testExplosion, Vec3{100, 0, 100}, "src_1", 0)
	if evt.Type != "explosion" {
		t.Errorf("expected type explosion, got %s", evt.Type)
	}
	if evt.Severity != 80 {
		t.Errorf("expected severity 80, got %f", evt.Severity)
	}
	if evt.TTL != 15.0 {
		t.Errorf("expected TTL 15, got %f", evt.TTL)
	}
	if evt.SourceID != "src_1" {
		t.Errorf("expected source src_1, got %s", evt.SourceID)
	}
	if evt.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestNewEvent_SeverityOverride(t *testing.T) {
	evt := NewEvent(testExplosion, Vec3{}, "src_1", 95)
	if evt.Severity != 95 {
		t.Errorf("expected severity 95, got %f", evt.Severity)
	}
}

func TestNewEvent_UniqueIDs(t *testing.T) {
	e1 := NewEvent(testExplosion, Vec3{}, "", 0)
	e2 := NewEvent(testExplosion, Vec3{}, "", 0)
	if e1.ID == e2.ID {
		t.Errorf("expected unique IDs, got %s and %s", e1.ID, e2.ID)
	}
}

// --- Bus ---

func TestBus_PublishAndActive(t *testing.T) {
	bus := NewBus()
	evt := NewEvent(testExplosion, Vec3{100, 0, 100}, "src_1", 0)
	bus.Publish(evt)

	active := bus.Active()
	if len(active) != 1 {
		t.Fatalf("expected 1 active, got %d", len(active))
	}
	if active[0].ID != evt.ID {
		t.Errorf("expected event %s, got %s", evt.ID, active[0].ID)
	}
}

func TestBus_ActiveReturnsSnapshot(t *testing.T) {
	bus := NewBus()
	bus.Publish(NewEvent(testExplosion, Vec3{}, "", 0))

	snapshot := bus.Active()
	// 修改快照不影响总线
	snapshot[0] = nil
	if bus.ActiveCount() != 1 {
		t.Error("modifying snapshot should not affect bus")
	}
}

func TestBus_TickTTLDecay(t *testing.T) {
	bus := NewBus()
	evt := NewEvent(testExplosion, Vec3{}, "", 0) // TTL=15
	bus.Publish(evt)

	bus.Tick(5.0) // TTL: 15 → 10
	if bus.ActiveCount() != 1 {
		t.Fatal("event should still be active")
	}

	active := bus.Active()
	if active[0].TTL != 10.0 {
		t.Errorf("expected TTL 10, got %f", active[0].TTL)
	}
}

func TestBus_TickRemovesExpired(t *testing.T) {
	bus := NewBus()
	bus.Publish(NewEvent(testExplosion, Vec3{}, "", 0)) // TTL=15
	bus.Publish(NewEvent(testGunshot, Vec3{}, "", 0))   // TTL=10

	bus.Tick(12.0) // explosion TTL=3, gunshot TTL=-2

	if bus.ActiveCount() != 1 {
		t.Fatalf("expected 1 active after tick, got %d", bus.ActiveCount())
	}
	active := bus.Active()
	if active[0].Type != "explosion" {
		t.Errorf("expected explosion to survive, got %s", active[0].Type)
	}
}

func TestBus_TickRemovesAllExpired(t *testing.T) {
	bus := NewBus()
	bus.Publish(NewEvent(testExplosion, Vec3{}, "", 0)) // TTL=15
	bus.Publish(NewEvent(testGunshot, Vec3{}, "", 0))   // TTL=10

	bus.Tick(20.0) // 全部过期

	if bus.ActiveCount() != 0 {
		t.Errorf("expected 0 active, got %d", bus.ActiveCount())
	}
}

func TestBus_TickExactTTLZero(t *testing.T) {
	bus := NewBus()
	bus.Publish(NewEvent(testGunshot, Vec3{}, "", 0)) // TTL=10

	bus.Tick(10.0) // TTL=0，应该被移除

	if bus.ActiveCount() != 0 {
		t.Errorf("TTL=0 event should be removed, got %d active", bus.ActiveCount())
	}
}

func TestBus_EmptyTick(t *testing.T) {
	bus := NewBus()
	// 空总线 Tick 不 panic
	bus.Tick(1.0)
	if bus.ActiveCount() != 0 {
		t.Error("expected 0 active")
	}
}

// --- 并发安全 ---

func TestBus_ConcurrentPublish(t *testing.T) {
	bus := NewBus()
	var wg sync.WaitGroup
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(NewEvent(testExplosion, Vec3{}, "", 0))
		}()
	}
	wg.Wait()

	if bus.ActiveCount() != goroutines {
		t.Errorf("expected %d active, got %d", goroutines, bus.ActiveCount())
	}
}

func TestBus_ConcurrentPublishAndRead(t *testing.T) {
	bus := NewBus()
	var wg sync.WaitGroup

	// 并发写
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(NewEvent(testExplosion, Vec3{}, "", 0))
		}()
	}

	// 并发读
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = bus.Active()
			_ = bus.ActiveCount()
		}()
	}

	wg.Wait()
}
