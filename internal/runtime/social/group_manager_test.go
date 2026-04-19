package social_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc/npctest"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/social"
)

// fakeSource 本地最小 Source 实现，避免 group_manager 测试依赖文件系统
type fakeSource struct{}

func (fakeSource) LoadFSMConfig(name string) (*fsm.FSMConfig, error) {
	return &fsm.FSMConfig{
		InitialState: "Idle",
		States:       []fsm.StateConfig{{Name: "Idle"}},
	}, nil
}
func (fakeSource) LoadBTTree(string) ([]byte, error) {
	return []byte(`{"type":"stub_action","params":{"name":"idle","result":"success"}}`), nil
}
func (fakeSource) LoadEventConfig(string) ([]byte, error)           { return nil, nil }
func (fakeSource) LoadAllEventConfigs() (map[string][]byte, error)  { return nil, nil }
func (fakeSource) LoadNPCTypeConfig(string) ([]byte, error)         { return nil, fmt.Errorf("not used") }
func (fakeSource) LoadNPCTemplate(string) ([]byte, error)           { return nil, nil }
func (fakeSource) LoadAllNPCTemplates() (map[string][]byte, error)  { return nil, nil }
func (fakeSource) LoadRegionConfig(string) ([]byte, error)          { return nil, nil }
func (fakeSource) LoadAllRegionConfigs() (map[string][]byte, error) { return nil, nil }

func minimalADMINTemplate() *npc.ADMINTemplate {
	return &npc.ADMINTemplate{
		Name:        "test",
		TemplateRef: "test",
		Fields:      map[string]any{},
		Behavior: npc.ADMINBehavior{
			FSMRef: "stub",
			BTRefs: map[string]string{"Idle": "stub/idle"},
		},
	}
}

func makeTestNPC(t *testing.T, id, groupID, role string, compReg *component.Registry) *npc.Instance {
	t.Helper()
	extras := map[string]json.RawMessage{}
	if groupID != "" {
		extras["social"] = json.RawMessage(`{"group_id":"` + groupID + `","role":"` + role + `"}`)
	}
	inst, err := npctest.NewInstanceWithExtras(id, npc.ZeroVec3(), minimalADMINTemplate(),
		extras, fakeSource{}, bt.DefaultRegistry(), compReg)
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

	// NPC 无 social 组件（groupID 空）
	inst := makeTestNPC(t, "nosocial", "", "", compReg)

	gm.Register(inst)
	// 不应 panic，不应加入任何组
	if len(gm.AllGroups()) != 0 {
		t.Error("NPC without social should not join any group")
	}
}
