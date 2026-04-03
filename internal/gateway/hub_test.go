package gateway

import (
	"context"
	"testing"
	"time"
)

// newTestConn 创建用于测试的最小 Conn
func newTestConn(hub *Hub) *Conn {
	return &Conn{
		hub:  hub,
		send: make(chan []byte, 256),
	}
}

func TestHub_RegisterAndCount(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	if hub.Count() != 0 {
		t.Fatalf("expected 0, got %d", hub.Count())
	}

	conn := newTestConn(hub)
	hub.register <- conn

	// 等待 Run goroutine 处理
	time.Sleep(10 * time.Millisecond)

	if hub.Count() != 1 {
		t.Fatalf("expected 1, got %d", hub.Count())
	}
}

func TestHub_Unregister(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	conn := newTestConn(hub)
	hub.register <- conn
	time.Sleep(10 * time.Millisecond)

	hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)

	if hub.Count() != 0 {
		t.Fatalf("expected 0 after unregister, got %d", hub.Count())
	}

	// send channel 应已关闭
	_, ok := <-conn.send
	if ok {
		t.Fatal("expected send channel to be closed")
	}
}

func TestHub_UnregisterNonexistent(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	conn := newTestConn(hub)
	// 不注册直接注销，不应 panic
	hub.unregister <- conn
	time.Sleep(10 * time.Millisecond)

	if hub.Count() != 0 {
		t.Fatalf("expected 0, got %d", hub.Count())
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	conn1 := newTestConn(hub)
	conn2 := newTestConn(hub)
	hub.register <- conn1
	hub.register <- conn2
	time.Sleep(10 * time.Millisecond)

	msg := []byte(`{"type":"world_snapshot"}`)
	hub.Broadcast(msg)
	time.Sleep(10 * time.Millisecond)

	// 两个连接都应收到
	select {
	case data := <-conn1.send:
		if string(data) != string(msg) {
			t.Fatalf("conn1 got %s, want %s", data, msg)
		}
	default:
		t.Fatal("conn1 did not receive broadcast")
	}

	select {
	case data := <-conn2.send:
		if string(data) != string(msg) {
			t.Fatalf("conn2 got %s, want %s", data, msg)
		}
	default:
		t.Fatal("conn2 did not receive broadcast")
	}
}

func TestHub_BroadcastNoClients(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// 无连接时广播不应阻塞或 panic
	hub.Broadcast([]byte(`{}`))
	time.Sleep(10 * time.Millisecond)
}

func TestHub_BroadcastSlowClient(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	// 创建 send 容量为 1 的慢客户端
	slow := &Conn{hub: hub, send: make(chan []byte, 1)}
	fast := newTestConn(hub)
	hub.register <- slow
	hub.register <- fast
	time.Sleep(10 * time.Millisecond)

	// 填满慢客户端的 send channel
	slow.send <- []byte("fill")

	// 广播——慢客户端 send 已满，应被踢出
	hub.Broadcast([]byte(`{"type":"test"}`))
	time.Sleep(10 * time.Millisecond)

	if hub.Count() != 1 {
		t.Fatalf("expected 1 (slow kicked), got %d", hub.Count())
	}

	// fast 仍应收到
	select {
	case <-fast.send:
	default:
		t.Fatal("fast client did not receive broadcast")
	}
}

func TestHub_CtxCancel(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		hub.Run(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Hub.Run did not exit after context cancel")
	}
}
