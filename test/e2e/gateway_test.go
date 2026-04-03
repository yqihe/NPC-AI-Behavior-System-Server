package e2e

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

// --- R1: 连接/断开 ---

func TestConnect(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)
	// 正常关闭
	conn.WriteMessage(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	time.Sleep(50 * time.Millisecond)
}

// --- R2, R6: spawn + query ---

func TestSpawnAndQuery(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	// spawn
	resp := sendAndRecv(t, conn, makeSpawnMsg("npc_1", "civilian", 100, 200))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("expected response, got %s", resp.Type)
	}
	var spawnResp protocol.SpawnNPCResponse
	json.Unmarshal(resp.Data, &spawnResp)
	if spawnResp.NpcID != "npc_1" || spawnResp.TypeName != "civilian" {
		t.Fatalf("unexpected spawn response: %+v", spawnResp)
	}

	// query
	resp = sendAndRecv(t, conn, makeQueryMsg("npc_1"))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("expected response, got %s", resp.Type)
	}
	var queryResp protocol.QueryNPCResponse
	json.Unmarshal(resp.Data, &queryResp)

	if queryResp.NpcID != "npc_1" {
		t.Errorf("npc_id: got %s, want npc_1", queryResp.NpcID)
	}
	if queryResp.TypeName != "civilian" {
		t.Errorf("type_name: got %s, want civilian", queryResp.TypeName)
	}
	if queryResp.X != 100 || queryResp.Z != 200 {
		t.Errorf("position: got (%f, %f), want (100, 200)", queryResp.X, queryResp.Z)
	}
	if queryResp.FSMState != "Idle" {
		t.Errorf("fsm_state: got %s, want Idle", queryResp.FSMState)
	}
}

// --- R3: remove ---

func TestRemoveNPC(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	// spawn
	sendAndRecv(t, conn, makeSpawnMsg("npc_1", "civilian", 0, 0))

	// remove
	resp := sendAndRecv(t, conn, makeRemoveMsg("npc_1"))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("expected response, got %s", resp.Type)
	}

	// query → not_found
	resp = sendAndRecv(t, conn, makeQueryMsg("npc_1"))
	if resp.Type != protocol.TypeError {
		t.Fatalf("expected error, got %s", resp.Type)
	}
	var errResp protocol.ErrorResponse
	json.Unmarshal(resp.Data, &errResp)
	if errResp.Code != "npc_not_found" {
		t.Errorf("error code: got %s, want npc_not_found", errResp.Code)
	}
}

// --- R4: publish_event → NPC 状态变化 ---

func TestPublishEvent(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	// spawn civilian at (100, 0)
	sendAndRecv(t, conn, makeSpawnMsg("npc_1", "civilian", 100, 0))

	// publish explosion near NPC
	resp := sendAndRecv(t, conn, makePublishEventMsg("explosion", 100, 0, 80))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("expected response, got %s", resp.Type)
	}
	var evtResp protocol.PublishEventResponse
	json.Unmarshal(resp.Data, &evtResp)
	if evtResp.EventID == "" {
		t.Fatal("expected non-empty event_id")
	}

	// 等待几个 Tick 让 NPC 处理事件
	time.Sleep(tickRate * 5)

	// query → 状态应该不再是 Idle
	resp = sendAndRecv(t, conn, makeQueryMsg("npc_1"))
	var queryResp protocol.QueryNPCResponse
	json.Unmarshal(resp.Data, &queryResp)

	if queryResp.FSMState == "Idle" {
		t.Errorf("expected NPC to leave Idle after explosion, still Idle")
	}
	if queryResp.ThreatLevel <= 0 {
		t.Errorf("expected threat_level > 0, got %f", queryResp.ThreatLevel)
	}
}

// --- R5: world_snapshot 广播 ---

func TestWorldSnapshot(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	// spawn
	sendAndRecv(t, conn, makeSpawnMsg("npc_1", "civilian", 50, 60))

	// 等待 snapshot
	snap := waitForSnapshot(t, conn)

	if snap.Tick == 0 {
		t.Error("expected tick > 0")
	}

	found := false
	for _, n := range snap.NPCs {
		if n.NpcID == "npc_1" {
			found = true
			if n.TypeName != "civilian" {
				t.Errorf("type_name: got %s, want civilian", n.TypeName)
			}
			if n.X != 50 || n.Z != 60 {
				t.Errorf("position: got (%f, %f), want (50, 60)", n.X, n.Z)
			}
		}
	}
	if !found {
		t.Errorf("npc_1 not found in snapshot, npcs: %+v", snap.NPCs)
	}
}

// --- R7, R8: 多客户端 ---

func TestMultiClient(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	connA := dial(t, url)
	connB := dial(t, url)

	// A spawns NPC
	sendAndRecv(t, connA, makeSpawnMsg("npc_1", "civilian", 0, 0))

	// 两个都应收到 snapshot
	snapA := waitForSnapshot(t, connA)
	snapB := waitForSnapshot(t, connB)

	findNPC := func(snap protocol.WorldSnapshot, id string) bool {
		for _, n := range snap.NPCs {
			if n.NpcID == id {
				return true
			}
		}
		return false
	}

	if !findNPC(snapA, "npc_1") {
		t.Error("connA did not receive npc_1 in snapshot")
	}
	if !findNPC(snapB, "npc_1") {
		t.Error("connB did not receive npc_1 in snapshot")
	}

	// 断开 A
	connA.Close()
	time.Sleep(100 * time.Millisecond)

	// B 发送 spawn，继续正常工作
	resp := sendAndRecv(t, connB, makeSpawnMsg("npc_2", "civilian", 10, 10))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("expected response after A disconnect, got %s", resp.Type)
	}

	// B 继续收 snapshot
	snapB2 := waitForSnapshot(t, connB)
	if !findNPC(snapB2, "npc_2") {
		t.Error("connB did not receive npc_2 in snapshot after A disconnect")
	}
}

// --- 边界：未知消息类型 ---

func TestUnknownMessage(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	msg := protocol.Message{Type: "invalid_type", ID: "req_1", Data: json.RawMessage(`{}`)}
	resp := sendAndRecv(t, conn, msg)

	if resp.Type != protocol.TypeError {
		t.Fatalf("expected error, got %s", resp.Type)
	}
	var errResp protocol.ErrorResponse
	json.Unmarshal(resp.Data, &errResp)
	if errResp.Code != "unknown_message_type" {
		t.Errorf("error code: got %s, want unknown_message_type", errResp.Code)
	}
}

// --- 边界：重复 ID ---

func TestSpawnDuplicateID(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	// 第一次 spawn 成功
	resp := sendAndRecv(t, conn, makeSpawnMsg("dup_1", "civilian", 0, 0))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("first spawn: expected response, got %s", resp.Type)
	}

	// 第二次同 ID → 错误
	resp = sendAndRecv(t, conn, makeSpawnMsg("dup_1", "civilian", 10, 10))
	if resp.Type != protocol.TypeError {
		t.Fatalf("second spawn: expected error, got %s", resp.Type)
	}
	var errResp protocol.ErrorResponse
	json.Unmarshal(resp.Data, &errResp)
	if errResp.Code != "npc_already_exists" {
		t.Errorf("error code: got %s, want npc_already_exists", errResp.Code)
	}
}
