package gateway

import (
	"fmt"
	"log/slog"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

// HandlerFunc 消息处理函数签名
type HandlerFunc func(conn *Conn, msg *protocol.Message) error

// Router 消息路由器（注册表模式，非 switch-case）
type Router struct {
	handlers map[string]HandlerFunc // type → handler
}

// NewRouter 创建路由器
func NewRouter() *Router {
	return &Router{
		handlers: make(map[string]HandlerFunc),
	}
}

// Register 注册消息处理器
func (r *Router) Register(msgType string, handler HandlerFunc) {
	r.handlers[msgType] = handler
}

// Dispatch 按消息类型分发到对应 handler
func (r *Router) Dispatch(conn *Conn, msg *protocol.Message) error {
	handler, ok := r.handlers[msg.Type]
	if !ok {
		slog.Warn("router.unknown_type", "type", msg.Type, "id", msg.ID)
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
	slog.Debug("router.dispatch", "type", msg.Type, "id", msg.ID)
	return handler(conn, msg)
}
