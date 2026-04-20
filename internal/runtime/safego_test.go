package runtime

import "testing"

// TestRecover_NoPanic 没发生 panic 时不影响正常流
func TestRecover_NoPanic(t *testing.T) {
	ran := false
	func() {
		defer Recover("test.no_panic")
		ran = true
	}()
	if !ran {
		t.Fatal("body should run normally when no panic")
	}
}

// TestRecover_CatchesPanic panic 被吞掉，caller 继续执行
func TestRecover_CatchesPanic(t *testing.T) {
	func() {
		defer Recover("test.catch_panic")
		panic("boom")
	}()
	// 能走到这说明 panic 被 Recover 吃掉
}

// TestRecover_OuterDeferStillRuns Recover 吞 panic 后，外层 defer 仍然执行
func TestRecover_OuterDeferStillRuns(t *testing.T) {
	outerRan := false
	func() {
		defer func() { outerRan = true }()
		defer Recover("test.outer_defer")
		panic("inner")
	}()
	if !outerRan {
		t.Fatal("outer defer should run after Recover absorbs panic")
	}
}

// TestScheduler_SafeTick_RecoversFromPanic 空 Scheduler 的 Tick 会 nil-deref
// panic；safeTick 必须吞掉，不传播到 Run 循环。
func TestScheduler_SafeTick_RecoversFromPanic(t *testing.T) {
	s := &Scheduler{} // 所有字段 nil → Tick 内 s.EventBus.Tick 会 panic
	// 不崩 = 过
	s.safeTick(0.1)
	s.safeTick(0.1) // 连续 panic 仍不累积
}
