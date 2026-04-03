package e2e

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

// --- 扩展轴 1：加事件源 fire，civilian 自动响应 ---

func TestExtension_NewEventType_Fire(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	// spawn civilian near fire position
	sendAndRecv(t, conn, makeSpawnMsg("npc_1", "civilian", 50, 0))

	// publish fire event at (50, 0) — 距离 0，severity 60，range 150
	// threat = 60 * (1 - 0/150) = 60 → 应触发 Alarmed → Flee
	resp := sendAndRecv(t, conn, makePublishEventMsg("fire", 50, 0, 0))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("expected response, got %s", resp.Type)
	}

	// 等待 Tick 处理
	time.Sleep(tickRate * 5)

	// query — NPC 应该离开 Idle
	resp = sendAndRecv(t, conn, makeQueryMsg("npc_1"))
	var q protocol.QueryNPCResponse
	json.Unmarshal(resp.Data, &q)

	if q.FSMState == "Idle" {
		t.Errorf("civilian should respond to fire event, still Idle")
	}

	// 关键验证：没有修改任何 Go 代码，只加了 fire.json
	t.Logf("civilian responded to fire: fsm_state=%s, threat_level=%f", q.FSMState, q.ThreatLevel)
}

// --- 扩展轴 2：加 police NPC 类型，响应已有事件 ---

func TestExtension_NewNPCType_Police(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	// spawn police
	resp := sendAndRecv(t, conn, makeSpawnMsg("police_1", "police", 100, 0))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("expected response, got %s", resp.Type)
	}
	var spawnResp protocol.SpawnNPCResponse
	json.Unmarshal(resp.Data, &spawnResp)
	if spawnResp.TypeName != "police" {
		t.Fatalf("expected police, got %s", spawnResp.TypeName)
	}

	// query — 初始状态 Idle
	resp = sendAndRecv(t, conn, makeQueryMsg("police_1"))
	var q protocol.QueryNPCResponse
	json.Unmarshal(resp.Data, &q)
	if q.FSMState != "Idle" {
		t.Fatalf("expected Idle, got %s", q.FSMState)
	}

	// publish explosion near police
	sendAndRecv(t, conn, makePublishEventMsg("explosion", 100, 0, 80))
	time.Sleep(tickRate * 5)

	// query — police 应进入 Engage（不是 Flee）
	resp = sendAndRecv(t, conn, makeQueryMsg("police_1"))
	json.Unmarshal(resp.Data, &q)

	if q.FSMState == "Idle" {
		t.Errorf("police should respond to explosion, still Idle")
	}
	// police FSM: Alarmed → Engage（threat >= 30），不是 Flee
	if q.FSMState == "Flee" {
		t.Errorf("police should Engage, not Flee — got %s", q.FSMState)
	}

	t.Logf("police responded to explosion: fsm_state=%s, threat=%f, action=%s", q.FSMState, q.ThreatLevel, q.CurrentAction)
}

// --- 扩展轴 1+2：多类型 × 多事件自动交叉 ---

func TestExtension_CrossTypeEvent_CivilianFlees_PoliceEngages(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	// spawn both types at same position
	sendAndRecv(t, conn, makeSpawnMsg("civ_1", "civilian", 50, 0))
	sendAndRecv(t, conn, makeSpawnMsg("pol_1", "police", 50, 0))

	// publish fire — 新事件源 × 新 NPC 类型，零改动
	sendAndRecv(t, conn, makePublishEventMsg("fire", 50, 0, 0))
	time.Sleep(tickRate * 5)

	// civilian 应该 Flee
	resp := sendAndRecv(t, conn, makeQueryMsg("civ_1"))
	var civQ protocol.QueryNPCResponse
	json.Unmarshal(resp.Data, &civQ)

	// police 应该 Engage
	resp = sendAndRecv(t, conn, makeQueryMsg("pol_1"))
	var polQ protocol.QueryNPCResponse
	json.Unmarshal(resp.Data, &polQ)

	// 关键验证：同一事件，不同 NPC 类型，不同行为
	if civQ.FSMState == "Idle" {
		t.Errorf("civilian should not be Idle after fire")
	}
	if polQ.FSMState == "Idle" {
		t.Errorf("police should not be Idle after fire")
	}
	if civQ.FSMState == polQ.FSMState {
		t.Errorf("civilian and police should have different responses: civilian=%s, police=%s", civQ.FSMState, polQ.FSMState)
	}

	t.Logf("civilian: fsm=%s | police: fsm=%s — different behaviors, zero code change", civQ.FSMState, polQ.FSMState)
}
