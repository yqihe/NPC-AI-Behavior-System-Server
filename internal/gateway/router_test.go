package gateway

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

func TestRouter_DispatchRegistered(t *testing.T) {
	router := NewRouter()
	called := false

	router.Register("test_type", func(conn *Conn, msg *protocol.Message) error {
		called = true
		if msg.Type != "test_type" {
			t.Errorf("expected type test_type, got %s", msg.Type)
		}
		if msg.ID != "req_1" {
			t.Errorf("expected id req_1, got %s", msg.ID)
		}
		return nil
	})

	msg := &protocol.Message{
		Type: "test_type",
		ID:   "req_1",
		Data: json.RawMessage(`{}`),
	}

	err := router.Dispatch(nil, msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestRouter_DispatchUnknownType(t *testing.T) {
	router := NewRouter()

	msg := &protocol.Message{
		Type: "nonexistent",
		ID:   "req_1",
	}

	err := router.Dispatch(nil, msg)
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unknown message type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRouter_RegisterOverwrite(t *testing.T) {
	router := NewRouter()
	callCount := 0

	router.Register("dup", func(conn *Conn, msg *protocol.Message) error {
		callCount = 1
		return nil
	})
	// 覆盖注册
	router.Register("dup", func(conn *Conn, msg *protocol.Message) error {
		callCount = 2
		return nil
	})

	msg := &protocol.Message{Type: "dup"}
	router.Dispatch(nil, msg)

	if callCount != 2 {
		t.Fatalf("expected second handler (2), got %d", callCount)
	}
}
