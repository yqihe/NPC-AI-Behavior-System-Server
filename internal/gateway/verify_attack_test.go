package gateway

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

func setupHubCtx(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithCancel(context.Background())
}

func waitForHub() {
	time.Sleep(20 * time.Millisecond)
}

// === Protocol 攻击 ===

func TestAttack_Protocol_NewResponseNilData(t *testing.T) {
	data, err := protocol.NewResponse("req_1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var msg protocol.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if msg.Type != protocol.TypeResponse {
		t.Errorf("expected type response, got %s", msg.Type)
	}
	if msg.ID != "req_1" {
		t.Errorf("expected id req_1, got %s", msg.ID)
	}
}

func TestAttack_Protocol_NewErrorEmptyFields(t *testing.T) {
	data, err := protocol.NewError("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var msg protocol.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if msg.Type != protocol.TypeError {
		t.Errorf("expected type error, got %s", msg.Type)
	}
}

func TestAttack_Protocol_RoundTrip(t *testing.T) {
	// 序列化 → 反序列化 round-trip
	req := protocol.SpawnNPCRequest{
		NpcID:    "npc_1",
		TypeName: "civilian",
		X:        100.5,
		Z:        -200.3,
	}
	reqBytes, _ := json.Marshal(req)

	msg := protocol.Message{
		Type: protocol.TypeSpawnNPC,
		ID:   "req_42",
		Data: json.RawMessage(reqBytes),
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	var decoded protocol.Message
	if err := json.Unmarshal(msgBytes, &decoded); err != nil {
		t.Fatal(err)
	}

	var decodedReq protocol.SpawnNPCRequest
	if err := json.Unmarshal(decoded.Data, &decodedReq); err != nil {
		t.Fatal(err)
	}

	if decodedReq.NpcID != "npc_1" || decodedReq.TypeName != "civilian" {
		t.Errorf("round-trip mismatch: %+v", decodedReq)
	}
	if decodedReq.X != 100.5 || decodedReq.Z != -200.3 {
		t.Errorf("position mismatch: x=%f z=%f", decodedReq.X, decodedReq.Z)
	}
}

func TestAttack_Protocol_WorldSnapshotEmpty(t *testing.T) {
	snap := protocol.WorldSnapshot{Tick: 0, NPCs: nil}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var decoded protocol.WorldSnapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Tick != 0 {
		t.Errorf("expected tick 0, got %d", decoded.Tick)
	}
}

// === Router 攻击 ===

func TestAttack_Router_EmptyType(t *testing.T) {
	router := NewRouter()
	msg := &protocol.Message{Type: "", ID: "req_1"}
	err := router.Dispatch(nil, msg)
	if err == nil {
		t.Fatal("expected error for empty type")
	}
}

func TestAttack_Router_NilConn(t *testing.T) {
	router := NewRouter()
	called := false
	router.Register("test", func(conn *Conn, msg *protocol.Message) error {
		called = true
		if conn != nil {
			t.Error("expected nil conn")
		}
		return nil
	})
	router.Dispatch(nil, &protocol.Message{Type: "test"})
	if !called {
		t.Fatal("handler not called with nil conn")
	}
}

// === Hub 并发攻击 ===

func TestAttack_Hub_ConcurrentBroadcast(t *testing.T) {
	hub := NewHub()
	ctx, cancel := setupHubCtx(t)
	defer cancel()
	go hub.Run(ctx)

	// 注册 10 个连接
	conns := make([]*Conn, 10)
	for i := range conns {
		conns[i] = newTestConn(hub)
		hub.register <- conns[i]
	}
	waitForHub()

	// 50 个 goroutine 并发广播
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			hub.Broadcast([]byte(`{"n":` + string(rune('0'+n%10)) + `}`))
		}(i)
	}
	wg.Wait()
	waitForHub()

	// 不 panic 即通过
}

func TestAttack_Hub_RegisterUnregisterRace(t *testing.T) {
	hub := NewHub()
	ctx, cancel := setupHubCtx(t)
	defer cancel()
	go hub.Run(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := newTestConn(hub)
			hub.register <- c
			hub.unregister <- c
		}()
	}
	wg.Wait()
	waitForHub()
}
