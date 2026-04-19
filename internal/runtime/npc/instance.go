package npc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// ZeroVec3 返回零值 Vec3（便捷函数）
func ZeroVec3() event.Vec3 { return event.Vec3{} }

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
	FSM        *fsm.FSM                     // 兼容旧代码，组件化后从 behavior 组件提取
	BTrees     map[string]bt.Node            // 兼容旧代码，组件化后从 behavior 组件提取
	Perception *perception.PerceptionConfig  // 兼容旧代码，组件化后从 perception 组件提取

	// 组件化字段（v3 新增）
	components  map[string]component.Component
	tickables   []component.Tickable
	displayName string
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

// HasComponent 检查 NPC 是否拥有指定组件
func (inst *Instance) HasComponent(name string) bool {
	if inst.components == nil {
		return false
	}
	_, ok := inst.components[name]
	return ok
}

// RawComponent 获取原始组件接口
func (inst *Instance) RawComponent(name string) (component.Component, bool) {
	if inst.components == nil {
		return nil, false
	}
	c, ok := inst.components[name]
	return c, ok
}

// TickComponents 执行所有 Tickable 组件的 Tick
func (inst *Instance) TickComponents(dt float64) {
	for _, t := range inst.tickables {
		t.Tick(inst.BB, dt)
	}
}

// SyncPosition 从 BB 同步位置到 Instance.Position 和 PositionComponent
func (inst *Instance) SyncPosition() {
	x, okX := blackboard.Get(inst.BB, blackboard.KeyNPCPosX)
	z, okZ := blackboard.Get(inst.BB, blackboard.KeyNPCPosZ)
	if !okX || !okZ {
		return
	}
	inst.Position.X = x
	inst.Position.Z = z
	if pos, ok := inst.RawComponent("position"); ok {
		if p, ok := pos.(*component.PositionComponent); ok {
			p.X = x
			p.Z = z
		}
	}
}

// InjectComponentForTest 追加或替换组件并重建 tickables 排序。
//
// **仅供 npctest 子包使用**（R21 封闭红线）。生产代码调用此方法会绕过
// R17 opt-in 契约与 R18 级联校验。通过 `internal/runtime/npc/npctest`
// 子包包装，方便 grep 审计；手动调用从生产路径调用违反 red-lines.md。
func (inst *Instance) InjectComponentForTest(name string, comp component.Component) {
	if inst.components == nil {
		inst.components = make(map[string]component.Component)
	}
	inst.components[name] = comp

	// 移除旧同名 tickable（如果有）并重建列表
	filtered := inst.tickables[:0]
	for _, t := range inst.tickables {
		if t.Name() != name {
			filtered = append(filtered, t)
		}
	}
	inst.tickables = filtered
	if t, ok := comp.(component.Tickable); ok {
		inst.tickables = append(inst.tickables, t)
	}
	sort.SliceStable(inst.tickables, func(i, j int) bool {
		return tickablePriority(inst.tickables[i].Name()) < tickablePriority(inst.tickables[j].Name())
	})
}

// GetComponent 类型安全获取组件
func GetComponent[T component.Component](inst *Instance, name string) (T, bool) {
	var zero T
	if inst.components == nil {
		return zero, false
	}
	c, ok := inst.components[name]
	if !ok {
		return zero, false
	}
	typed, ok := c.(T)
	if !ok {
		return zero, false
	}
	return typed, ok
}

// ParseNPCTypeConfig 从 JSON 字节解析 NPC 类型配置
func ParseNPCTypeConfig(data []byte) (*NPCTypeConfig, error) {
	var cfg NPCTypeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("npc: parse type config: %w", err)
	}
	return &cfg, nil
}
