package gateway

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
)

// zonedBroadcast 区域级广播数据
type zonedBroadcast struct {
	zoneData map[string][]byte // zone_id → serialized snapshot; "" key = global
}

// Hub 管理所有活跃 WebSocket 连接
type Hub struct {
	conns         map[*Conn]struct{} // 活跃连接集合（仅 Run goroutine 读写）
	register      chan *Conn         // 注册通道
	unregister    chan *Conn         // 注销通道
	broadcast     chan []byte        // 全局广播通道
	zoneBroadcast chan zonedBroadcast // 区域级广播通道
	mu            sync.RWMutex       // 保护 count
	count         int
}

// NewHub 创建 Hub
func NewHub() *Hub {
	return &Hub{
		conns:         make(map[*Conn]struct{}),
		register:      make(chan *Conn),
		unregister:    make(chan *Conn),
		broadcast:     make(chan []byte, 256),
		zoneBroadcast: make(chan zonedBroadcast, 64),
	}
}

// Run 主循环，处理注册/注销/广播。阻塞直到 ctx 取消。
// panic 兜底：单次事件 panic 后记录 + 继续循环，不拖垮整个 WS 层。
func (h *Hub) Run(ctx context.Context) {
	for {
		if !h.runOnce(ctx) {
			return
		}
	}
}

// runOnce 处理一个事件并 recover。返回 false 表示 ctx 已取消。
// keepRunning 默认 true：panic 被 defer 吞后函数会提前返回默认值，
// 这里的默认值必须是 true 才不会导致 Run 误以为要退出。
func (h *Hub) runOnce(ctx context.Context) (keepRunning bool) {
	keepRunning = true
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic.recovered",
				"where", "hub.run",
				"err", r,
				"stack", string(debug.Stack()),
			)
			// panic 已吞掉：保持 keepRunning=true，让 Run 继续下一轮
		}
	}()
	select {
	case <-ctx.Done():
		keepRunning = false
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
				delete(h.conns, conn)
				close(conn.send)
				h.mu.Lock()
				h.count = len(h.conns)
				h.mu.Unlock()
				slog.Warn("hub.slow_client", "clients", h.count)
			}
		}
	case zb := <-h.zoneBroadcast:
		for conn := range h.conns {
			var data []byte
			if conn.ZoneID != "" {
				data = zb.zoneData[conn.ZoneID]
			} else {
				data = zb.zoneData[""]
			}
			if len(data) == 0 {
				continue
			}
			select {
			case conn.send <- data:
			default:
				delete(h.conns, conn)
				close(conn.send)
				h.mu.Lock()
				h.count = len(h.conns)
				h.mu.Unlock()
				slog.Warn("hub.slow_client", "clients", h.count)
			}
		}
	}
	return
}

// Broadcast 向所有连接广播数据（非阻塞，channel 满时丢弃）
func (h *Hub) Broadcast(data []byte) {
	select {
	case h.broadcast <- data:
	default:
		slog.Warn("hub.broadcast_full")
	}
}

// BroadcastByZone 按区域广播。zoneData key 为 zone_id，"" 表示全局（给未设 zone 的连接）。
func (h *Hub) BroadcastByZone(zoneData map[string][]byte) {
	select {
	case h.zoneBroadcast <- zonedBroadcast{zoneData: zoneData}:
	default:
		slog.Warn("hub.zone_broadcast_full")
	}
}

// Count 返回当前连接数
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.count
}
