package npc

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// NPCTypeConfig NPC 类型配置（从 JSON 加载）
type NPCTypeConfig struct {
	TypeName   string                      `json:"type_name"`
	FSMRef     string                      `json:"fsm_ref"`
	BTRefs     map[string]string           `json:"bt_refs"`    // 状态名 → BT 树名
	Perception perception.PerceptionConfig `json:"perception"`
}

// Instance 一个 NPC 运行时实例
type Instance struct {
	ID         string
	TypeName   string
	Position   event.Vec3
	BB         *blackboard.Blackboard
	FSM        *fsm.FSM
	BTrees     map[string]bt.Node // 状态名 → 已构建的 BT 根节点
	Perception *perception.PerceptionConfig
}

// NewInstance 从配置创建 NPC 实例（工厂函数）
func NewInstance(id string, pos event.Vec3, typeCfg *NPCTypeConfig, src config.Source, btReg *bt.Registry) (*Instance, error) {
	// 1. 创建 Blackboard
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCType, typeCfg.TypeName)
	blackboard.Set(bb, blackboard.KeyNPCPosX, pos.X)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, pos.Z)
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))

	// 2. 加载 FSM 配置并创建 FSM
	fsmCfg, err := src.LoadFSMConfig(typeCfg.FSMRef)
	if err != nil {
		return nil, fmt.Errorf("npc %s: load FSM: %w", id, err)
	}
	f, err := fsm.NewFSM(fsmCfg, bb)
	if err != nil {
		return nil, fmt.Errorf("npc %s: create FSM: %w", id, err)
	}

	// 3. 加载并构建所有状态的 BT
	btrees := make(map[string]bt.Node, len(typeCfg.BTRefs))
	for state, treeName := range typeCfg.BTRefs {
		treeData, err := src.LoadBTTree(treeName)
		if err != nil {
			return nil, fmt.Errorf("npc %s: load BT %q for state %q: %w", id, treeName, state, err)
		}
		tree, err := bt.BuildFromJSON(treeData, btReg)
		if err != nil {
			return nil, fmt.Errorf("npc %s: build BT %q for state %q: %w", id, treeName, state, err)
		}
		btrees[state] = tree
	}

	percCfg := typeCfg.Perception
	return &Instance{
		ID:         id,
		TypeName:   typeCfg.TypeName,
		Position:   pos,
		BB:         bb,
		FSM:        f,
		BTrees:     btrees,
		Perception: &percCfg,
	}, nil
}

// Tick 执行单个 NPC 的一次 Tick（FSM + BT）
func (inst *Instance) Tick() {
	// 1. FSM Tick（评估转换条件）
	inst.FSM.Tick(inst.BB)

	// 2. 获取当前状态对应的 BT
	currentState := inst.FSM.Current()
	tree, ok := inst.BTrees[currentState]
	if !ok {
		slog.Debug("npc.tick.bt_not_found", "npc_id", inst.ID, "state", currentState)
		return
	}

	// 3. BT Tick
	ctx := &bt.Context{BB: inst.BB}
	tree.Tick(ctx)
}

// ParseNPCTypeConfig 从 JSON 字节解析 NPC 类型配置
func ParseNPCTypeConfig(data []byte) (*NPCTypeConfig, error) {
	var cfg NPCTypeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("npc: parse type config: %w", err)
	}
	return &cfg, nil
}
