package gateway

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestServer_GracefulShutdown 验证 ctx 取消后 Start 返回且 HTTP 端口被释放
// （用于 R5 验收项：SIGTERM 后优雅关闭）。
func TestServer_GracefulShutdown(t *testing.T) {
	// 0 端口让 OS 分配可用端口
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close() // 关了，把端口让给 Server

	hub := NewHub()
	router := NewRouter()
	srv := NewServer(addr, hub, router)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- srv.Start(ctx)
	}()

	// 等 Server 监听起来
	waitListening(t, addr, 500*time.Millisecond)

	// 取消 ctx → 触发 Shutdown 路径
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start returned error: %v", err)
		}
	case <-time.After(ShutdownTimeout + 2*time.Second):
		t.Fatalf("Start did not return within %s after ctx cancel", ShutdownTimeout)
	}

	// 端口应已释放 — 能再次 Listen 即证明
	l2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("port not released after shutdown: %v", err)
	}
	l2.Close()
}

func waitListening(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server did not start listening on %s within %s", addr, timeout)
}
