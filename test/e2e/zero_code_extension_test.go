package e2e

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

// TestZeroCodeExtension_TheftAlarm 验证"零代码扩展"承诺：
// Admin 后台新增一条 event_type + 新 FSM + 新 BT + update 现有 NPC 的 fsm_ref/bt_refs，
// Server 侧 internal/ cmd/ pkg/ 0 改动的前提下，
// 一条 theft_alarm 事件能同时驱动 villager_merchant（被改配成 thief）和 villager_guard（原战斗 FSM）按各自 FSM 分叉。
func TestZeroCodeExtension_TheftAlarm(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	// --- 1. Spawn villager_merchant（被 Admin update 为 thief_fsm + bt/thief/*）---
	resp := sendAndRecv(t, conn, makeSpawnMsg("merchant_1", "villager_merchant", 0, 0))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("spawn merchant: expected response, got %s", resp.Type)
	}

	// --- 2. Spawn guard_basic（绑 guard FSM，initial_state=Patrol，
	//        Patrol→Alert 条件 last_event_type!=""，Alert→Defend 条件 threat>=60）---
	// 注：选 guard_basic 而非 villager_guard 是因为 guard FSM 的跨 NPC
	// 分叉断言更直白（Alert/Defend vs thief 的 flee）；villager_guard 绑的
	// fsm_combat_basic 经 Admin 侧 Finding-1 修复后也已可用（Idle→Patrol
	// 无条件转换），该路径由 TestAdminFix_FSMCombatBasic 覆盖。
	resp = sendAndRecv(t, conn, makeSpawnMsg("guard_1", "guard_basic", 0, 0))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("spawn guard: expected response, got %s", resp.Type)
	}

	// --- 3. Query 初始 FSM state ---
	time.Sleep(tickRate * 2)

	merchantBefore := queryNPC(t, conn, "merchant_1")
	guardBefore := queryNPC(t, conn, "guard_1")

	if merchantBefore.FSMState != "steal" {
		t.Fatalf("thief-villager_merchant initial state: want steal, got %s", merchantBefore.FSMState)
	}
	t.Logf("[before] merchant=%s(threat=%.0f) guard=%s(threat=%.0f)",
		merchantBefore.FSMState, merchantBefore.ThreatLevel,
		guardBefore.FSMState, guardBefore.ThreatLevel)

	// --- 4. Inject theft_alarm（global 模式，位置/severity 传默认，severity=0 → 用 default_severity=80）---
	resp = sendAndRecv(t, conn, makePublishEventMsg("theft_alarm", 0, 0, 0))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("publish theft_alarm: expected response, got %s data=%s", resp.Type, string(resp.Data))
	}

	// --- 5. 等若干 tick，让 perception → decision → FSM 跳转 ---
	time.Sleep(tickRate * 8)

	// --- 6. 断言 ---
	merchantAfter := queryNPC(t, conn, "merchant_1")
	guardAfter := queryNPC(t, conn, "guard_1")

	t.Logf("[after]  merchant=%s(threat=%.0f) guard=%s(threat=%.0f)",
		merchantAfter.FSMState, merchantAfter.ThreatLevel,
		guardAfter.FSMState, guardAfter.ThreatLevel)

	// 6.1 thief_fsm: steal → flee (threat_level > 50 AND threat_expire_at > current_time)
	if merchantAfter.FSMState != "flee" {
		t.Errorf("thief should transition steal→flee, got %s", merchantAfter.FSMState)
	}
	if merchantAfter.ThreatLevel <= 50 {
		t.Errorf("thief threat_level should exceed 50 (global severity=80), got %.1f", merchantAfter.ThreatLevel)
	}

	// 6.2 guard FSM: Patrol → Alert (last_event_type!="") → Defend (threat>=60)
	if guardAfter.FSMState == guardBefore.FSMState {
		t.Errorf("guard should leave %s after theft_alarm, still %s", guardBefore.FSMState, guardAfter.FSMState)
	}
	if guardAfter.FSMState != "Alert" && guardAfter.FSMState != "Defend" {
		t.Errorf("guard FSM should be Alert or Defend, got %s", guardAfter.FSMState)
	}

	// 6.3 同一事件，不同 FSM，不同响应 — 这是"加配置不改代码"的核心证据
	if merchantAfter.FSMState == guardAfter.FSMState {
		t.Errorf("merchant and guard should diverge: merchant=%s guard=%s",
			merchantAfter.FSMState, guardAfter.FSMState)
	}
}

func queryNPC(t *testing.T, conn *websocket.Conn, npcID string) protocol.QueryNPCResponse {
	t.Helper()
	resp := sendAndRecv(t, conn, makeQueryMsg(npcID))
	if resp.Type != protocol.TypeResponse {
		t.Fatalf("query %s: expected response, got %s data=%s", npcID, resp.Type, string(resp.Data))
	}
	var q protocol.QueryNPCResponse
	if err := json.Unmarshal(resp.Data, &q); err != nil {
		t.Fatalf("unmarshal query %s: %v", npcID, err)
	}
	return q
}
