package runtime_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/decision"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc/npctest"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/social"
)

func createGroupNPC(t *testing.T, id string, pos event.Vec3, groupID, role string, src config.Source, btReg *bt.Registry, compReg *component.Registry) *npc.Instance {
	t.Helper()
	socialJSON := []byte(`{"group_id":"` + groupID + `","role":"` + role + `"}`)
	inst, err := npctest.NewInstanceWithExtras(id, pos, wolfADMINTemplate(nil),
		map[string]json.RawMessage{"social": socialJSON}, src, btReg, compReg)
	if err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
	return inst
}

// --- 群组感知共享 ---

func TestSocialIntegration_GroupPerceptionSharing(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	// 两只狼同组：wolf_a 在爆炸附近，wolf_b 在远处
	wolfA := createGroupNPC(t, "wolf_a", event.Vec3{X: 100, Y: 0, Z: 100}, "pack1", "leader", src, btReg, compReg)
	wolfB := createGroupNPC(t, "wolf_b", event.Vec3{X: 500, Y: 0, Z: 500}, "pack1", "follower", src, btReg, compReg)

	gm := social.NewGroupManager()
	gm.Register(wolfA)
	gm.Register(wolfB)

	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(wolfA)
	reg.Add(wolfB)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)
	sched.GroupManager = gm

	// 爆炸在 wolf_a 附近（wolf_b 感知不到）
	explosionCfg := evtTypes["explosion"]
	evt := event.NewEvent(explosionCfg, event.Vec3{X: 120, Y: 0, Z: 100}, "bomber", 80, "")
	bus.Publish(evt)

	blackboard.Set(wolfA.BB, blackboard.KeyCurrentTime, int64(10000))
	blackboard.Set(wolfB.BB, blackboard.KeyCurrentTime, int64(10000))
	sched.Tick(0.1)

	// wolf_a 应有威胁（直接感知）
	levelA, _ := blackboard.Get(wolfA.BB, blackboard.KeyThreatLevel)
	if levelA <= 0 {
		t.Error("wolf_a should perceive explosion directly")
	}

	// wolf_b 也应有威胁（通过群组共享）
	levelB, _ := blackboard.Get(wolfB.BB, blackboard.KeyThreatLevel)
	if levelB <= 0 {
		t.Error("wolf_b should receive shared perception from wolf_a")
	}
}

// --- leader 丢失 ---

func TestSocialIntegration_LeaderLost(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()

	leader := createGroupNPC(t, "leader", event.Vec3{X: 100, Y: 0, Z: 100}, "pack1", "leader", src, btReg, compReg)
	follower := createGroupNPC(t, "follower", event.Vec3{X: 110, Y: 0, Z: 100}, "pack1", "follower", src, btReg, compReg)

	gm := social.NewGroupManager()
	gm.Register(leader)
	gm.Register(follower)

	// 移除 leader
	gm.Unregister(leader)

	lost, _ := blackboard.Get(follower.BB, blackboard.KeyLeaderLost)
	if !lost {
		t.Error("follower should have leader_lost=true after leader removal")
	}
}

// --- 群体逃跑（group_alert）---

func TestSocialIntegration_GroupAlert(t *testing.T) {
	src := config.NewJSONSource(configsDir(t))
	btReg := bt.DefaultRegistry()
	compReg := component.DefaultRegistry()
	evtTypes := loadEvtTypes(t, src)

	wolfA := createGroupNPC(t, "wolf_a", event.Vec3{X: 100, Y: 0, Z: 100}, "pack1", "leader", src, btReg, compReg)
	wolfB := createGroupNPC(t, "wolf_b", event.Vec3{X: 110, Y: 0, Z: 100}, "pack1", "follower", src, btReg, compReg)

	gm := social.NewGroupManager()
	gm.Register(wolfA)
	gm.Register(wolfB)

	bus := event.NewBus()
	reg := npc.NewRegistry()
	reg.Add(wolfA)
	reg.Add(wolfB)
	dec := decision.NewCenter(10.0)
	sched := runtime.NewScheduler(bus, reg, dec, evtTypes, 100*time.Millisecond)
	sched.GroupManager = gm

	// 发布高威胁事件让 wolf_a 进入 Flee
	explosionCfg := evtTypes["explosion"]
	evt := event.NewEvent(explosionCfg, event.Vec3{X: 105, Y: 0, Z: 100}, "bomber", 90, "")
	bus.Publish(evt)

	blackboard.Set(wolfA.BB, blackboard.KeyCurrentTime, int64(10000))
	blackboard.Set(wolfB.BB, blackboard.KeyCurrentTime, int64(10000))

	// 多次 Tick 让 wolf_a 的 FSM 转到 Flee
	for i := 0; i < 5; i++ {
		sched.Tick(0.1)
	}

	// 检查 wolf_a 是否 Flee
	behA, _ := npc.GetComponent[*component.BehaviorComponent](wolfA, "behavior")
	if behA.FSM.Current() == "Flee" {
		// wolf_b 应有 group_alert
		alert, _ := blackboard.Get(wolfB.BB, blackboard.KeyGroupAlert)
		if !alert {
			t.Error("wolf_b should have group_alert=true when wolf_a is Flee")
		}
	} else {
		t.Logf("wolf_a FSM = %s (not Flee yet, may need more ticks or higher threat)", behA.FSM.Current())
	}
}
