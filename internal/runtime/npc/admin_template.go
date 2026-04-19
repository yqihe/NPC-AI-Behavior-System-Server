package npc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// 默认感知距离（fields 中缺少 visual_range/auditory_range 时使用）
const (
	defaultVisualRange   = 50.0
	defaultAuditoryRange = 100.0
	defaultMoveSpeed     = 3.0
	defaultWanderRadius  = 20.0
)

// optInBoolFields 5 个 R17 opt-in bool 字段名（absent ≡ false）
var optInBoolFields = [...]string{
	"enable_memory",
	"enable_emotion",
	"enable_needs",
	"enable_personality",
	"enable_social",
}

// ADMINTemplate ADMIN v3 导出的 NPC 模板形状 `{template_ref, fields, behavior}`
// 不同于组件化 TemplateConfig（`{name, preset, components}`）
type ADMINTemplate struct {
	Name        string         `json:"-"`            // 由外部 items[].name 填入
	TemplateRef string         `json:"template_ref"` // ADMIN 内部标识，服务端忽略
	Fields      map[string]any `json:"fields"`       // 扁平属性，写入 BB（含 hp/attack/visual_range 等）
	Behavior    ADMINBehavior  `json:"behavior"`
}

// ADMINBehavior 指向 FSM/BT 配置的引用
type ADMINBehavior struct {
	FSMRef string            `json:"fsm_ref"`
	BTRefs map[string]string `json:"bt_refs"` // 状态名 → BT 树名
}

// ParseADMINTemplate 从 ADMIN 导出的 config JSON 解析 ADMINTemplate。
// name 由调用方从外层 items[].name 传入。
func ParseADMINTemplate(name string, data []byte) (*ADMINTemplate, error) {
	var tmpl ADMINTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("npc: parse ADMIN template %q: %w", name, err)
	}
	tmpl.Name = name
	if tmpl.Behavior.FSMRef == "" {
		return nil, fmt.Errorf("npc: ADMIN template %q missing behavior.fsm_ref", name)
	}
	if tmpl.Fields == nil {
		tmpl.Fields = map[string]any{}
	}
	return &tmpl, nil
}

// NewInstanceFromADMIN 从 ADMIN 形状模板创建 NPC 实例（R1 唯一生产入口）。
//
// 翻译流程：
//  1. 5 默认组件层：identity / position / behavior / perception / movement 均实例化
//  2. 5 opt-in 组件层：按 fields.enable_{memory,emotion,needs,personality,social} 决定是否实例化（R17）
//  3. fields 扁平字段通过 SetDynamic 写入 BB，保持 R7 透明透传（含 hp 孤儿等未消费字段）
//  4. behavior 组件持有 FSM + BTrees；Instance.FSM / BTrees / Perception 兼容字段从组件派生
//
// R8 perception_range fallback 链：visual_range > perception_range > 默认；auditory_range > perception_range > 默认
func NewInstanceFromADMIN(
	id string,
	pos event.Vec3,
	tmpl *ADMINTemplate,
	src config.Source,
	btReg *bt.Registry,
	compReg *component.Registry,
) (*Instance, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("npc: nil ADMIN template")
	}
	if compReg == nil {
		return nil, fmt.Errorf("npc: nil component registry")
	}

	// 1. 创建 Blackboard 并写入基础运行时 Key
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCType, tmpl.Name)
	blackboard.Set(bb, blackboard.KeyNPCPosX, pos.X)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, pos.Z)
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))

	// 2. fields 扁平字段写入 BB（R7 透明透传）
	for k, v := range tmpl.Fields {
		blackboard.SetDynamic(bb, k, v)
	}

	components := make(map[string]component.Component, 10)
	var tickables []component.Tickable

	// 3. 5 默认组件实例化（全部走 factory 保证 schema 一致性）
	if err := buildDefaultComponents(id, pos, tmpl, compReg, components, &tickables); err != nil {
		return nil, err
	}

	// 4. 5 opt-in 组件实例化（R17 absent ≡ false）
	if err := buildOptInComponents(id, tmpl, compReg, components, &tickables); err != nil {
		return nil, err
	}

	// 5. tickables 排序：memory(0) → needs(1) → emotion(2) → movement(3) → 其他(99)
	sort.SliceStable(tickables, func(i, j int) bool {
		return tickablePriority(tickables[i].Name()) < tickablePriority(tickables[j].Name())
	})

	// 6. behavior 组件加载 FSM + BT
	if beh, ok := components["behavior"].(*component.BehaviorComponent); ok {
		if err := wireBehavior(id, beh, bb, src, btReg); err != nil {
			return nil, err
		}
	}

	// 7. social 组件 BB 初始化（若启用）
	if social, ok := components["social"].(*component.SocialComponent); ok {
		if social.GroupID != "" {
			blackboard.Set(bb, blackboard.KeyGroupID, social.GroupID)
		}
		if social.Role != "" {
			blackboard.Set(bb, blackboard.KeySocialRole, social.Role)
		}
	}

	// 8. movement 记录 spawn 原点
	if mov, ok := components["movement"].(*component.MovementComponent); ok {
		mov.SetSpawn(pos.X, pos.Z)
	}

	identity, _ := components["identity"].(*component.IdentityComponent)
	displayName := ""
	if identity != nil {
		displayName = identity.DisplayName
	}

	slog.Debug("npc.created_from_admin",
		"npc_id", id,
		"template", tmpl.Name,
		"components", len(components),
		"tickables", len(tickables),
	)

	return &Instance{
		ID:          id,
		TypeName:    tmpl.Name,
		Position:    pos,
		BB:          bb,
		components:  components,
		tickables:   tickables,
		FSM:         behaviorFSMFromComponents(components),
		BTrees:      behaviorBTreesFromComponents(components),
		Perception:  perceptionConfigFromComponents(components),
		displayName: displayName,
	}, nil
}

// buildDefaultComponents 实例化 5 个默认组件（identity/position/behavior/perception/movement）。
func buildDefaultComponents(
	id string,
	pos event.Vec3,
	tmpl *ADMINTemplate,
	compReg *component.Registry,
	components map[string]component.Component,
	tickables *[]component.Tickable,
) error {
	identityJSON, err := json.Marshal(map[string]any{
		"name":     tmpl.Name,
		"model_id": tmpl.Name, // ADMIN 未声明 model_id；用 NPC 名作标识，有需要再扩展字段
		"tags":     []string{},
	})
	if err != nil {
		return fmt.Errorf("npc %s: marshal identity: %w", id, err)
	}
	if err := createAndRegister(compReg, "identity", identityJSON, components, tickables); err != nil {
		return fmt.Errorf("npc %s: %w", id, err)
	}

	positionJSON, err := json.Marshal(map[string]any{
		"x": pos.X,
		"y": pos.Y,
		"z": pos.Z,
	})
	if err != nil {
		return fmt.Errorf("npc %s: marshal position: %w", id, err)
	}
	if err := createAndRegister(compReg, "position", positionJSON, components, tickables); err != nil {
		return fmt.Errorf("npc %s: %w", id, err)
	}

	behaviorJSON, err := json.Marshal(map[string]any{
		"fsm_ref": tmpl.Behavior.FSMRef,
		"bt_refs": tmpl.Behavior.BTRefs,
	})
	if err != nil {
		return fmt.Errorf("npc %s: marshal behavior: %w", id, err)
	}
	if err := createAndRegister(compReg, "behavior", behaviorJSON, components, tickables); err != nil {
		return fmt.Errorf("npc %s: %w", id, err)
	}

	// R8 perception_range fallback 链
	visualRange := readFloatChain(tmpl.Fields, []string{"visual_range", "perception_range"}, defaultVisualRange)
	auditoryRange := readFloatChain(tmpl.Fields, []string{"auditory_range", "perception_range"}, defaultAuditoryRange)
	perceptionJSON, err := json.Marshal(map[string]any{
		"visual_range":   visualRange,
		"auditory_range": auditoryRange,
	})
	if err != nil {
		return fmt.Errorf("npc %s: marshal perception: %w", id, err)
	}
	if err := createAndRegister(compReg, "perception", perceptionJSON, components, tickables); err != nil {
		return fmt.Errorf("npc %s: %w", id, err)
	}

	movementJSON, err := json.Marshal(map[string]any{
		"move_type":     inferMoveType(tmpl.Behavior.FSMRef),
		"move_speed":    readFloat(tmpl.Fields, "move_speed", defaultMoveSpeed),
		"wander_radius": defaultWanderRadius,
	})
	if err != nil {
		return fmt.Errorf("npc %s: marshal movement: %w", id, err)
	}
	if err := createAndRegister(compReg, "movement", movementJSON, components, tickables); err != nil {
		return fmt.Errorf("npc %s: %w", id, err)
	}

	return nil
}

// buildOptInComponents 按 R17 opt-in 契约条件实例化 5 个能力组件。
// absent ≡ false 语义：fields 中 bool 字段缺失或为 false 时跳过对应组件。
func buildOptInComponents(
	id string,
	tmpl *ADMINTemplate,
	compReg *component.Registry,
	components map[string]component.Component,
	tickables *[]component.Tickable,
) error {
	if readBool(tmpl.Fields, "enable_memory") {
		raw, err := json.Marshal(map[string]any{
			"capacity":     10,
			"memory_types": []string{"threat"},
			"decay_time":   30.0,
		})
		if err != nil {
			return fmt.Errorf("npc %s: marshal memory: %w", id, err)
		}
		if err := createAndRegister(compReg, "memory", raw, components, tickables); err != nil {
			return fmt.Errorf("npc %s: %w", id, err)
		}
	}

	if readBool(tmpl.Fields, "enable_emotion") {
		raw, err := json.Marshal(map[string]any{
			"emotion_states": []map[string]any{
				{"name": "fear", "value": 0, "accumulate_rate": 20, "decay_rate": 5},
			},
		})
		if err != nil {
			return fmt.Errorf("npc %s: marshal emotion: %w", id, err)
		}
		if err := createAndRegister(compReg, "emotion", raw, components, tickables); err != nil {
			return fmt.Errorf("npc %s: %w", id, err)
		}
	}

	if readBool(tmpl.Fields, "enable_needs") {
		raw, err := json.Marshal(map[string]any{
			"need_types": []map[string]any{
				{"name": "energy", "current": 100, "max": 100, "decay_rate": 1},
			},
		})
		if err != nil {
			return fmt.Errorf("npc %s: marshal needs: %w", id, err)
		}
		if err := createAndRegister(compReg, "needs", raw, components, tickables); err != nil {
			return fmt.Errorf("npc %s: %w", id, err)
		}
	}

	if readBool(tmpl.Fields, "enable_personality") {
		raw, err := json.Marshal(personalityJSONFromFields(tmpl.Fields))
		if err != nil {
			return fmt.Errorf("npc %s: marshal personality: %w", id, err)
		}
		if err := createAndRegister(compReg, "personality", raw, components, tickables); err != nil {
			return fmt.Errorf("npc %s: %w", id, err)
		}
	}

	if readBool(tmpl.Fields, "enable_social") {
		raw, err := json.Marshal(map[string]any{
			"group_id": readString(tmpl.Fields, "group_id", ""),
			"role":     readString(tmpl.Fields, "social_role", ""),
		})
		if err != nil {
			return fmt.Errorf("npc %s: marshal social: %w", id, err)
		}
		if err := createAndRegister(compReg, "social", raw, components, tickables); err != nil {
			return fmt.Errorf("npc %s: %w", id, err)
		}
	}

	return nil
}

// createAndRegister 通过 registry 工厂创建组件，注册到 map 和 tickables 列表。
func createAndRegister(
	compReg *component.Registry,
	name string,
	raw json.RawMessage,
	components map[string]component.Component,
	tickables *[]component.Tickable,
) error {
	comp, err := compReg.Create(name, raw)
	if err != nil {
		return fmt.Errorf("component %q: %w", name, err)
	}
	components[name] = comp
	if t, ok := comp.(component.Tickable); ok {
		*tickables = append(*tickables, t)
	}
	return nil
}

// wireBehavior 加载 behavior 组件引用的 FSM + BT 资源并绑定到组件运行时字段。
func wireBehavior(id string, beh *component.BehaviorComponent, bb *blackboard.Blackboard, src config.Source, btReg *bt.Registry) error {
	fsmCfg, err := src.LoadFSMConfig(beh.FSMRef)
	if err != nil {
		return fmt.Errorf("npc %s: load FSM %q: %w", id, beh.FSMRef, err)
	}
	f, err := fsm.NewFSM(fsmCfg, bb)
	if err != nil {
		return fmt.Errorf("npc %s: create FSM: %w", id, err)
	}
	npcID := id
	f.OnTransition(func(from, to string) {
		slog.Debug("fsm.transition", "npc_id", npcID, "from", from, "to", to)
	})
	beh.FSM = f

	btrees := make(map[string]bt.Node, len(beh.BTRefs))
	for state, treeName := range beh.BTRefs {
		treeData, err := src.LoadBTTree(treeName)
		if err != nil {
			return fmt.Errorf("npc %s: load BT %q for state %q: %w", id, treeName, state, err)
		}
		tree, err := bt.BuildFromJSON(treeData, btReg)
		if err != nil {
			return fmt.Errorf("npc %s: build BT %q for state %q: %w", id, treeName, state, err)
		}
		btrees[state] = tree
	}
	beh.BTrees = btrees
	return nil
}

// inferMoveType 按 FSM 引用推断默认 move_type。fsm_passive→wander，其余一律 wander（BT 层处理 chase/patrol）。
func inferMoveType(fsmRef string) string {
	return "wander"
}

// personalityJSONFromFields 从 aggression 字段推断 personality_type + decision_weights。
func personalityJSONFromFields(fields map[string]any) map[string]any {
	aggression := readString(fields, "aggression", "")
	personalityType := "docile"
	weights := map[string]any{"threat": 1.0, "needs": 1.0, "emotion": 1.0}
	if aggression == "aggressive" {
		personalityType = "aggressive"
		weights = map[string]any{"threat": 1.5, "needs": 0.7, "emotion": 0.8}
	}
	return map[string]any{
		"personality_type": personalityType,
		"decision_weights": weights,
	}
}

// behaviorFSMFromComponents 兼容旧代码读 inst.FSM
func behaviorFSMFromComponents(components map[string]component.Component) *fsm.FSM {
	if beh, ok := components["behavior"].(*component.BehaviorComponent); ok {
		return beh.FSM
	}
	return nil
}

// behaviorBTreesFromComponents 兼容旧代码读 inst.BTrees
func behaviorBTreesFromComponents(components map[string]component.Component) map[string]bt.Node {
	if beh, ok := components["behavior"].(*component.BehaviorComponent); ok {
		return beh.BTrees
	}
	return nil
}

// perceptionConfigFromComponents 兼容旧代码读 inst.Perception
func perceptionConfigFromComponents(components map[string]component.Component) *perception.PerceptionConfig {
	if perc, ok := components["perception"].(*component.PerceptionComponent); ok {
		return &perception.PerceptionConfig{
			VisualRange:   perc.VisualRange,
			AuditoryRange: perc.AuditoryRange,
		}
	}
	return nil
}

// readFloat 从 fields map 读取 float64，缺失或类型不匹配时返回 fallback
func readFloat(fields map[string]any, key string, fallback float64) float64 {
	v, ok := fields[key]
	if !ok {
		return fallback
	}
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case int:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return fallback
		}
		return f
	}
	return fallback
}

// readFloatChain 按顺序尝试 keys，返回第一个解析成功的 float64，全部失败时返回 fallback
func readFloatChain(fields map[string]any, keys []string, fallback float64) float64 {
	sentinel := math.NaN()
	for _, k := range keys {
		v := readFloat(fields, k, sentinel)
		if !math.IsNaN(v) {
			return v
		}
	}
	return fallback
}

// readString 从 fields 读 string，缺失或类型不匹配返回 fallback
func readString(fields map[string]any, key string, fallback string) string {
	v, ok := fields[key]
	if !ok {
		return fallback
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fallback
}

// readBool 从 fields 读 bool。absent ≡ false（R17 语义锁定）
func readBool(fields map[string]any, key string) bool {
	v, ok := fields[key]
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
