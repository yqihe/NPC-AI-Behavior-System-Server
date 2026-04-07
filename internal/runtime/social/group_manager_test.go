package social_test

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/social"
)

func makeTestNPC(t *testing.T, id, groupID, role string, compReg *component.Registry) *npc.Instance {
	t.Helper()
	comps := map[string]json.RawMessage{
		"identity": json.RawMessage(`{"name":"test","model_id":"test"}`),
		"position": json.RawMessage(`{"x":0,"z":0}`),
	}
	socialJSON := `{"group_id":"` + groupID + `","role":"` + role + `"}`
	if groupID != "" {
		comps["social"] = json.RawMessage(socialJSON)
	}
	tmpl := &npc.TemplateConfig{Name: "test", Components: comps}
	inst, err := npc.NewInstanceFromTemplate(id, npc.ZeroVec3(), tmpl, compReg, nil, nil)
	if err != nil {
		t.Fatalf("create NPC %s: %v", id, err)
	}
	return inst
}

func TestGroupManager_RegisterAndGetGroup(t *testing.T) {
	compReg := component.DefaultRegistry()
	gm := social.NewGroupManager()

	a := makeTestNPC(t, "a", "pack1", "leader", compReg)
	b := makeTestNPC(t, "b", "pack1", "follower", compReg)
	c := makeTestNPC(t, "c", "pack2", "leader", compReg)

	gm.Register(a)
	gm.Register(b)
	gm.Register(c)

	pack1 := gm.GetGroup("pack1")
	if len(pack1) != 2 {
		t.Errorf("pack1 size = %d, want 2", len(pack1))
	}
	pack2 := gm.GetGroup("pack2")
	if len(pack2) != 1 {
		t.Errorf("pack2 size = %d, want 1", len(pack2))
	}
}

func TestGroupManager_GetLeader(t *testing.T) {
	compReg := component.DefaultRegistry()
	gm := social.NewGroupManager()

	leader := makeTestNPC(t, "leader1", "pack1", "leader", compReg)
	follower := makeTestNPC(t, "f1", "pack1", "follower", compReg)

	gm.Register(leader)
	gm.Register(follower)

	l := gm.GetLeader("pack1")
	if l == nil {
		t.Fatal("leader should not be nil")
	}
	if l.ID != "leader1" {
		t.Errorf("leader ID = %q, want %q", l.ID, "leader1")
	}
}

func TestGroupManager_Unregister(t *testing.T) {
	compReg := component.DefaultRegistry()
	gm := social.NewGroupManager()

	a := makeTestNPC(t, "a", "pack1", "follower", compReg)
	gm.Register(a)

	if len(gm.GetGroup("pack1")) != 1 {
		t.Fatal("should have 1 member")
	}

	gm.Unregister(a)
	if len(gm.GetGroup("pack1")) != 0 {
		t.Error("should have 0 members after unregister")
	}
}

func TestGroupManager_LeaderLost(t *testing.T) {
	compReg := component.DefaultRegistry()
	gm := social.NewGroupManager()

	leader := makeTestNPC(t, "leader1", "pack1", "leader", compReg)
	f1 := makeTestNPC(t, "f1", "pack1", "follower", compReg)
	f2 := makeTestNPC(t, "f2", "pack1", "follower", compReg)

	gm.Register(leader)
	gm.Register(f1)
	gm.Register(f2)

	// 移除 leader
	gm.Unregister(leader)

	// follower 应收到 leader_lost
	lost1, _ := blackboard.Get(f1.BB, blackboard.KeyLeaderLost)
	lost2, _ := blackboard.Get(f2.BB, blackboard.KeyLeaderLost)

	if !lost1 {
		t.Error("f1 should have leader_lost=true")
	}
	if !lost2 {
		t.Error("f2 should have leader_lost=true")
	}
}

func TestGroupManager_NoSocialComponent(t *testing.T) {
	compReg := component.DefaultRegistry()
	gm := social.NewGroupManager()

	// NPC 无 social 组件
	comps := map[string]json.RawMessage{
		"identity": json.RawMessage(`{"name":"test","model_id":"test"}`),
		"position": json.RawMessage(`{"x":0,"z":0}`),
	}
	tmpl := &npc.TemplateConfig{Name: "test", Components: comps}
	inst, _ := npc.NewInstanceFromTemplate("nosocial", npc.ZeroVec3(), tmpl, compReg, nil, nil)

	gm.Register(inst)
	// 不应 panic，不应加入任何组
	if len(gm.AllGroups()) != 0 {
		t.Error("NPC without social should not join any group")
	}
}
