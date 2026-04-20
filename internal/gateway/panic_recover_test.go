package gateway

import (
	"context"
	"testing"
	"time"
)

// TestHub_RunOnce_PanicRecoversAndContinues 模拟"广播遇到已关闭 send channel"
// 场景：正常路径会 panic（send to closed channel），Run 被 recover 吞掉后
// 后续广播仍能处理新注册的连接。没有 recover 时 Run goroutine 直接死，
// 下一条 Broadcast 永远不会被消费。
func TestHub_RunOnce_PanicRecoversAndContinues(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// 1. 注册一个 conn，手动 close 掉它的 send chan（绕过 unregister 流程）
	bad := newTestConn(hub)
	hub.register <- bad
	time.Sleep(10 * time.Millisecond)
	close(bad.send)

	// 2. 触发广播 — 路径里 `conn.send <- data` 打向已关闭 chan → panic
	hub.Broadcast([]byte("payload"))
	time.Sleep(20 * time.Millisecond)

	// 3. Run goroutine 应当仍存活：注册一个新 conn 能被处理 + Count 更新
	good := newTestConn(hub)
	hub.register <- good
	time.Sleep(20 * time.Millisecond)

	// 关键断言：Count() > 0 说明 register 被消费 = Run 没死
	if hub.Count() == 0 {
		t.Fatal("Hub.Run died after panic — register not processed")
	}
}
