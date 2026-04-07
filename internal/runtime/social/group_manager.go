package social

import (
	"log/slog"
	"sync"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
)

// GroupManager 管理 NPC 群组关系
type GroupManager struct {
	mu     sync.RWMutex
	groups map[string][]*npc.Instance // group_id → members
}

// NewGroupManager 创建空的群组管理器
func NewGroupManager() *GroupManager {
	return &GroupManager{
		groups: make(map[string][]*npc.Instance),
	}
}

// Register 将 NPC 注册到所属群组（有 social 组件且 group_id 非空）
func (gm *GroupManager) Register(inst *npc.Instance) {
	social, ok := npc.GetComponent[*component.SocialComponent](inst, "social")
	if !ok || social.GroupID == "" {
		return
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()
	gm.groups[social.GroupID] = append(gm.groups[social.GroupID], inst)
	slog.Debug("social.registered", "npc_id", inst.ID, "group", social.GroupID, "role", social.Role)
}

// Unregister 将 NPC 从群组移除。如果移除的是 leader，向同组 follower 写 BB leader_lost=true
func (gm *GroupManager) Unregister(inst *npc.Instance) {
	social, ok := npc.GetComponent[*component.SocialComponent](inst, "social")
	if !ok || social.GroupID == "" {
		return
	}
	gm.mu.Lock()
	defer gm.mu.Unlock()

	groupID := social.GroupID
	members := gm.groups[groupID]
	for i, m := range members {
		if m.ID == inst.ID {
			gm.groups[groupID] = append(members[:i], members[i+1:]...)
			break
		}
	}

	// leader 丢失检测
	if social.Role == "leader" {
		for _, m := range gm.groups[groupID] {
			blackboard.Set(m.BB, blackboard.KeyLeaderLost, true)
			slog.Debug("social.leader_lost", "group", groupID, "follower", m.ID)
		}
	}

	// 清理空群组
	if len(gm.groups[groupID]) == 0 {
		delete(gm.groups, groupID)
	}

	slog.Debug("social.unregistered", "npc_id", inst.ID, "group", groupID)
}

// GetGroup 获取同组所有成员
func (gm *GroupManager) GetGroup(groupID string) []*npc.Instance {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	members := gm.groups[groupID]
	result := make([]*npc.Instance, len(members))
	copy(result, members)
	return result
}

// GetLeader 获取指定群组的 leader（无则返回 nil）
func (gm *GroupManager) GetLeader(groupID string) *npc.Instance {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	for _, m := range gm.groups[groupID] {
		social, ok := npc.GetComponent[*component.SocialComponent](m, "social")
		if ok && social.Role == "leader" {
			return m
		}
	}
	return nil
}

// AllGroups 返回所有群组 ID 列表（遍历用）
func (gm *GroupManager) AllGroups() map[string][]*npc.Instance {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	result := make(map[string][]*npc.Instance, len(gm.groups))
	for k, v := range gm.groups {
		members := make([]*npc.Instance, len(v))
		copy(members, v)
		result[k] = members
	}
	return result
}
