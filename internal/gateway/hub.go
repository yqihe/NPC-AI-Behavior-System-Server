package gateway

import (
	"context"
	"log/slog"
	"sync"
)

// Hub 管理所有活跃 WebSocket 连接
type Hub struct {
	conns      map[*Conn]struct{} // 活跃连接集合（仅 Run goroutine 读写）
	register   chan *Conn         // 注册通道
	unregister chan *Conn         // 注销通道
	broadcast  chan []byte        // 广播通道（已序列化的 JSON）
	mu         sync.RWMutex      // 保护 count（供 Count 外部读取）
	count      int               // 当前连接数
}

// NewHub 创建 Hub
func NewHub() *Hub {
	return &Hub{
		conns:      make(map[*Conn]struct{}),
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
		broadcast:  make(chan []byte, 256),
	}
}

// Run 主循环，处理注册/注销/广播。阻塞直到 ctx 取消
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case conn := <-h.register:
			h.conns[conn] = struct{}{}
			h.mu.Lock()
			h.count = len(h.conns)
			h.mu.Unlock()
			slog.Info("hub.register", "clients", h.count)
		case conn := <-h.unregister:
			if _, ok := h.conns[conn]; ok {
				delete(h.conns, conn)
				close(conn.send)
				h.mu.Lock()
				h.count = len(h.conns)
				h.mu.Unlock()
				slog.Info("hub.unregister", "clients", h.count)
			}
		case data := <-h.broadcast:
			for conn := range h.conns {
				select {
				case conn.send <- data:
				default:
					// send channel 满，关闭慢客户端
					delete(h.conns, conn)
					close(conn.send)
					h.mu.Lock()
					h.count = len(h.conns)
					h.mu.Unlock()
					slog.Warn("hub.slow_client", "clients", h.count)
				}
			}
		}
	}
}

// Broadcast 向所有连接广播数据
func (h *Hub) Broadcast(data []byte) {
	h.broadcast <- data
}

// Count 返回当前连接数
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.count
}
