package gateway

import (
	"encoding/json"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

const (
	sendBufSize = 256          // send channel 容量
	writeWait   = 10 * time.Second // 写超时
	pongWait    = 60 * time.Second // 等待 pong 的超时
	pingPeriod  = 54 * time.Second // ping 间隔（必须小于 pongWait）
)

// Conn 封装单个 WebSocket 连接
type Conn struct {
	hub    *Hub
	ws     *websocket.Conn
	send   chan []byte // 出站消息缓冲
	router *Router
	ZoneID string // 客户端所在区域，空=全局（收全部 NPC）
}

// NewConn 创建连接
func NewConn(hub *Hub, ws *websocket.Conn, router *Router) *Conn {
	return &Conn{
		hub:    hub,
		ws:     ws,
		send:   make(chan []byte, sendBufSize),
		router: router,
	}
}

// ReadPump 读循环：解码 → 路由 → 响应写入 send。阻塞直到连接关闭。
// panic 兜底：router/handler panic 不拖垮其他连接，本连接清理退出。
func (c *Conn) ReadPump() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic.recovered",
				"where", "conn.read_pump",
				"err", r,
				"stack", string(debug.Stack()),
			)
		}
		c.hub.unregister <- c
		c.ws.Close()
	}()

	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("conn.read_error", "err", err)
			}
			return
		}

		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			resp, _ := protocol.NewError("", "invalid_json", "failed to parse message")
			c.sendMsg(resp)
			continue
		}

		if err := c.router.Dispatch(c, &msg); err != nil {
			resp, _ := protocol.NewError(msg.ID, "unknown_message_type", err.Error())
			c.sendMsg(resp)
		}
	}
}

// WritePump 写循环：从 send channel 取数据发送。阻塞直到 send 关闭。
// panic 兜底：确保 ws 关闭 + 不影响其他连接。
func (c *Conn) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic.recovered",
				"where", "conn.write_pump",
				"err", r,
				"stack", string(debug.Stack()),
			)
		}
		ticker.Stop()
		c.ws.Close()
	}()

	for {
		select {
		case data, ok := <-c.send:
			if !ok {
				// send channel 已关闭（Hub 注销时关闭）
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.Warn("conn.write_error", "err", err)
				return
			}
		case <-ticker.C:
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendMsg 写入 send channel，不阻塞
func (c *Conn) sendMsg(data []byte) {
	select {
	case c.send <- data:
	default:
		// channel 满，由 Hub 处理慢客户端
	}
}
