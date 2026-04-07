package npc

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// TemplateConfig 组件化 NPC 模板配置
type TemplateConfig struct {
	Name       string                        `json:"name"`
	Preset     string                        `json:"preset"`
	Components map[string]json.RawMessage    `json:"components"`
}

// NewInstanceFromTemplate 从组件化模板创建 NPC 实例
func NewInstanceFromTemplate(
	id string,
	pos event.Vec3,
	tmpl *TemplateConfig,
	compReg *component.Registry,
	src config.Source,
	btReg *bt.Registry,
) (*Instance, error) {
	// 1. 校验必选组件
	if _, ok := tmpl.Components["identity"]; !ok {
		return nil, fmt.Errorf("npc %s: identity component is required", id)
	}
	if _, ok := tmpl.Components["position"]; !ok {
		return nil, fmt.Errorf("npc %s: position component is required", id)
	}

	// 2. 创建所有组件
	components := make(map[string]component.Component, len(tmpl.Components))
	var tickables []component.Tickable

	for name, raw := range tmpl.Components {
		comp, err := compReg.Create(name, raw)
		if err != nil {
			return nil, fmt.Errorf("npc %s: component %q: %w", id, name, err)
		}
		components[name] = comp

		if t, ok := comp.(component.Tickable); ok {
			tickables = append(tickables, t)
		}
	}

	// 3. 创建 Blackboard 并设置初始值
	bb := blackboard.New()

	// 从 identity 组件读取 NPC 类型名
	identity := components["identity"].(*component.IdentityComponent)
	blackboard.Set(bb, blackboard.KeyNPCType, tmpl.Name)

	// 从 position 组件读取位置（spawn 时传入的 pos 优先覆盖）
	posComp := components["position"].(*component.PositionComponent)
	if pos.X != 0 || pos.Z != 0 {
		posComp.X = pos.X
		posComp.Z = pos.Z
		posComp.Y = pos.Y
	}
	blackboard.Set(bb, blackboard.KeyNPCPosX, posComp.X)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, posComp.Z)
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))

	// 从 social 组件初始化 BB
	if social, ok := components["social"].(*component.SocialComponent); ok {
		if social.GroupID != "" {
			blackboard.Set(bb, blackboard.KeyGroupID, social.GroupID)
		}
		if social.Role != "" {
			blackboard.Set(bb, blackboard.KeySocialRole, social.Role)
		}
	}

	// 4. 如果有 behavior 组件，加载 FSM + BT
	if beh, ok := components["behavior"].(*component.BehaviorComponent); ok {
		fsmCfg, err := src.LoadFSMConfig(beh.FSMRef)
		if err != nil {
			return nil, fmt.Errorf("npc %s: load FSM %q: %w", id, beh.FSMRef, err)
		}
		f, err := fsm.NewFSM(fsmCfg, bb)
		if err != nil {
			return nil, fmt.Errorf("npc %s: create FSM: %w", id, err)
		}
		beh.FSM = f

		btrees := make(map[string]bt.Node, len(beh.BTRefs))
		for state, treeName := range beh.BTRefs {
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
		beh.BTrees = btrees
	}

	slog.Debug("npc.created_from_template",
		"npc_id", id,
		"template", tmpl.Name,
		"components", len(components),
		"tickables", len(tickables),
	)

	return &Instance{
		ID:         id,
		TypeName:   tmpl.Name,
		Position:   posComp.ToVec3(),
		BB:         bb,
		components: components,
		tickables:  tickables,
		// 旧字段兼容：如果有 behavior，填入 FSM/BTrees/Perception
		FSM:        behaviorFSM(components),
		BTrees:     behaviorBTrees(components),
		Perception: perceptionConfig(components),
		displayName: identity.DisplayName,
	}, nil
}

// behaviorFSM 从组件提取 FSM（兼容旧代码读 inst.FSM）
func behaviorFSM(components map[string]component.Component) *fsm.FSM {
	if beh, ok := components["behavior"].(*component.BehaviorComponent); ok {
		return beh.FSM
	}
	return nil
}

// behaviorBTrees 从组件提取 BTrees（兼容旧代码读 inst.BTrees）
func behaviorBTrees(components map[string]component.Component) map[string]bt.Node {
	if beh, ok := components["behavior"].(*component.BehaviorComponent); ok {
		return beh.BTrees
	}
	return nil
}

// perceptionConfig 从组件提取 PerceptionConfig（兼容旧代码读 inst.Perception）
func perceptionConfig(components map[string]component.Component) *perception.PerceptionConfig {
	if perc, ok := components["perception"].(*component.PerceptionComponent); ok {
		return &perception.PerceptionConfig{
			VisualRange:   perc.VisualRange,
			AuditoryRange: perc.AuditoryRange,
		}
	}
	return nil
}
