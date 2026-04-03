package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/gateway"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

const (
	recvTimeout = 2 * time.Second
	tickRate    = 50 * time.Millisecond // e2e 用更快的 tick
)

// startTestServer 启动完整服务（随机端口），返回 WS URL 和 cleanup 函数
func startTestServer(t *testing.T) (wsURL string, cleanup func()) {
	t.Helper()

	// 找到 configs 目录
	wd, _ := os.Getwd()
	configsDir := filepath.Join(wd, "..", "..", "configs")

	src := config.NewJSONSource(configsDir)
	btReg := bt.DefaultRegistry()

	// 加载事件类型
	rawConfigs, err := src.LoadAllEventConfigs()
	if err != nil {
		t.Fatalf("load event configs: %v", err)
	}
	evtTypes := make(map[string]*event.EventTypeConfig, len(rawConfigs))
	for name, data := range rawConfigs {
		var cfg event.EventTypeConfig
		if err := json.Unmarshal(data, &cfg); err != nil {
			t.Fatalf("parse event config %s: %v", name, err)
		}
		evtTypes[cfg.Name] = &cfg
	}

	// 初始化 Runtime
	bus := event.NewBus()
	reg := npc.NewRegistry()
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, tickRate)

	// 初始化 Gateway
	hub := gateway.NewHub()
	router := gateway.NewRouter()
	gateway.RegisterHandlers(router, reg, bus, src, btReg, evtTypes)

	// 随机端口
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	srv := gateway.NewServer(":"+fmt.Sprint(port), hub, router)

	ctx, cancel := context.WithCancel(context.Background())

	go sched.Run(ctx)
	go hub.Run(ctx)
	go broadcastLoop(ctx, hub, reg, tickRate)
	go srv.Start(ctx)

	// 等待服务就绪
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	wsURL = fmt.Sprintf("ws://%s/ws", addr)
	cleanup = func() { cancel() }
	return
}

// broadcastLoop 复用 main.go 的广播逻辑
func broadcastLoop(ctx context.Context, hub *gateway.Hub, reg *npc.Registry, tick time.Duration) {
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	var tickCount uint64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tickCount++
			if hub.Count() == 0 {
				continue
			}
			snapshot := buildSnapshot(tickCount, reg)
			snapshotJSON, _ := json.Marshal(snapshot)
			msg, _ := json.Marshal(protocol.Message{
				Type: protocol.TypeWorldSnapshot,
				Data: json.RawMessage(snapshotJSON),
			})
			hub.Broadcast(msg)
		}
	}
}

func buildSnapshot(tick uint64, reg *npc.Registry) protocol.WorldSnapshot {
	npcs := make([]protocol.NPCState, 0)
	reg.ForEach(func(inst *npc.Instance) {
		currentAction, _ := blackboard.Get(inst.BB, blackboard.KeyCurrentAction)
		threatLevel, _ := blackboard.Get(inst.BB, blackboard.KeyThreatLevel)
		npcs = append(npcs, protocol.NPCState{
			NpcID:         inst.ID,
			TypeName:      inst.TypeName,
			X:             inst.Position.X,
			Z:             inst.Position.Z,
			FSMState:      inst.FSM.Current(),
			CurrentAction: currentAction,
			ThreatLevel:   threatLevel,
		})
	})
	return protocol.WorldSnapshot{Tick: tick, NPCs: npcs}
}

// dial 连接 WS 并注册 cleanup
func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// sendMsg 发送 protocol.Message
func sendMsg(t *testing.T, conn *websocket.Conn, msg protocol.Message) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// recvMsg 接收一条消息（带超时）
func recvMsg(t *testing.T, conn *websocket.Conn) protocol.Message {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(recvTimeout))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var msg protocol.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return msg
}

// sendAndRecv 发送请求并等待 response/error 响应（跳过 world_snapshot 广播）
func sendAndRecv(t *testing.T, conn *websocket.Conn, req protocol.Message) protocol.Message {
	t.Helper()
	sendMsg(t, conn, req)
	// 循环读取直到收到非 world_snapshot 消息
	for {
		msg := recvMsg(t, conn)
		if msg.Type != protocol.TypeWorldSnapshot {
			return msg
		}
	}
}

// waitForSnapshot 等待下一个 world_snapshot 广播
func waitForSnapshot(t *testing.T, conn *websocket.Conn) protocol.WorldSnapshot {
	t.Helper()
	for {
		msg := recvMsg(t, conn)
		if msg.Type == protocol.TypeWorldSnapshot {
			var snap protocol.WorldSnapshot
			if err := json.Unmarshal(msg.Data, &snap); err != nil {
				t.Fatalf("unmarshal snapshot: %v", err)
			}
			return snap
		}
	}
}

// makeSpawnMsg 构造 spawn_npc 请求
func makeSpawnMsg(id, typeName string, x, z float64) protocol.Message {
	data, _ := json.Marshal(protocol.SpawnNPCRequest{
		NpcID: id, TypeName: typeName, X: x, Z: z,
	})
	return protocol.Message{Type: protocol.TypeSpawnNPC, ID: "spawn_" + id, Data: data}
}

// makeRemoveMsg 构造 remove_npc 请求
func makeRemoveMsg(id string) protocol.Message {
	data, _ := json.Marshal(protocol.RemoveNPCRequest{NpcID: id})
	return protocol.Message{Type: protocol.TypeRemoveNPC, ID: "remove_" + id, Data: data}
}

// makeQueryMsg 构造 query_npc 请求
func makeQueryMsg(id string) protocol.Message {
	data, _ := json.Marshal(protocol.QueryNPCRequest{NpcID: id})
	return protocol.Message{Type: protocol.TypeQueryNPC, ID: "query_" + id, Data: data}
}

// makePublishEventMsg 构造 publish_event 请求
func makePublishEventMsg(evtType string, x, z, severity float64) protocol.Message {
	data, _ := json.Marshal(protocol.PublishEventRequest{
		EventType: evtType, X: x, Z: z, Severity: severity,
	})
	return protocol.Message{Type: protocol.TypePublishEvent, ID: "evt_" + evtType, Data: data}
}
