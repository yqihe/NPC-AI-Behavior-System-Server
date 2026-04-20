package gateway

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// ShutdownTimeout 优雅关闭最长等待时间：排空 in-flight HTTP 请求
const ShutdownTimeout = 5 * time.Second

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 开发阶段允许所有来源
}

// Server WebSocket 网关服务
type Server struct {
	hub            *Hub
	router         *Router
	addr           string
	httpSrv        *http.Server
	MetricsHandler http.HandlerFunc // /metrics 端点（可选）
}

// NewServer 创建网关服务
func NewServer(addr string, hub *Hub, router *Router) *Server {
	return &Server{
		hub:    hub,
		router: router,
		addr:   addr,
	}
}

// Start 启动 HTTP/WS 服务，阻塞直到 ctx 取消或监听失败
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWS)
	if s.MetricsHandler != nil {
		mux.HandleFunc("/metrics", s.MetricsHandler)
	}

	s.httpSrv = &http.Server{
		Addr:    s.addr,
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	slog.Info("server.start", "addr", s.addr)

	// 监听 ctx 取消，优雅关闭：排空 in-flight 请求，超时兜底
	go func() {
		<-ctx.Done()
		slog.Info("server.shutdown", "timeout", ShutdownTimeout.String())
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()
		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			slog.Warn("server.shutdown_error", "err", err)
			s.httpSrv.Close()
		}
	}()

	err := s.httpSrv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// handleWS 处理 WebSocket 升级请求
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Warn("server.upgrade_error", "err", err)
		return
	}

	conn := NewConn(s.hub, ws, s.router)
	s.hub.register <- conn

	go conn.WritePump()
	go conn.ReadPump()
}
