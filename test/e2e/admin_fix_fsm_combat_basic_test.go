package e2e

import (
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/pkg/protocol"
)

// TestAdminFix_FSMCombatBasic 验证 Admin 侧 Finding-1 修复：
// fsm_combat_basic 的 Idle→Patrol transition condition 从
// {"key":"last_event_type","op":"==","value":""} 改为 {}（空条件 = 无条件），
// 避免因 BB 缺键导致的 absent→false 卡死。
// 4 个绑定 fsm_combat_basic 的 NPC 应在数个 tick 内离开 Idle 进入 Patrol。
func TestAdminFix_FSMCombatBasic(t *testing.T) {
	url, cleanup := startTestServer(t)
	defer cleanup()

	conn := dial(t, url)

	npcs := []struct {
		id       string
		typeName string
	}{
		{"wolf_common_1", "wolf_common"},
		{"wolf_alpha_1", "wolf_alpha"},
		{"goblin_archer_1", "goblin_archer"},
		{"villager_guard_1", "villager_guard"},
	}

	for _, n := range npcs {
		resp := sendAndRecv(t, conn, makeSpawnMsg(n.id, n.typeName, 0, 0))
		if resp.Type != protocol.TypeResponse {
			t.Fatalf("spawn %s (%s): expected response, got %s data=%s",
				n.id, n.typeName, resp.Type, string(resp.Data))
		}
	}

	time.Sleep(tickRate * 4)

	for _, n := range npcs {
		q := queryNPC(t, conn, n.id)
		t.Logf("%s(%s) fsm_state=%s", n.id, n.typeName, q.FSMState)
		if q.FSMState == "Idle" {
			t.Errorf("%s (%s) stuck in Idle after Admin fix, expected Patrol",
				n.id, n.typeName)
		}
		if q.FSMState != "Patrol" {
			t.Errorf("%s (%s) unexpected state %s, want Patrol",
				n.id, n.typeName, q.FSMState)
		}
	}
}
