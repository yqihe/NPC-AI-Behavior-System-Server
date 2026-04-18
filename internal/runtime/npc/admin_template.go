package npc

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/blackboard"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/fsm"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/perception"
)

// 默认感知距离（fields 中缺少 visual_range/auditory_range 时使用）
const (
	defaultVisualRange   = 50.0
	defaultAuditoryRange = 100.0
)

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
	return &tmpl, nil
}

// NewInstanceFromADMIN 从 ADMIN 形状模板创建 NPC 实例。
// fields 通过 SetDynamic 写入 BB（自动注册 Key）；visual_range/auditory_range 缺失时取默认值。
func NewInstanceFromADMIN(id string, pos event.Vec3, tmpl *ADMINTemplate, src config.Source, btReg *bt.Registry) (*Instance, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("npc: nil ADMIN template")
	}

	// 1. 创建 Blackboard，写入运行时 Key
	bb := blackboard.New()
	blackboard.Set(bb, blackboard.KeyNPCType, tmpl.Name)
	blackboard.Set(bb, blackboard.KeyNPCPosX, pos.X)
	blackboard.Set(bb, blackboard.KeyNPCPosZ, pos.Z)
	blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))

	// 2. 将 fields 写入 BB（动态注册 Key）
	for k, v := range tmpl.Fields {
		blackboard.SetDynamic(bb, k, v)
	}

	// 3. 从 fields 提取感知距离。
	// ADMIN 当前只有合并的 perception_range 字段（visual_range/auditory_range 未拆分），
	// 为兼容两种命名，fallback 链：专用字段 > 合并字段 > 内置默认值。
	percCfg := perception.PerceptionConfig{
		VisualRange:   readFloatChain(tmpl.Fields, []string{"visual_range", "perception_range"}, defaultVisualRange),
		AuditoryRange: readFloatChain(tmpl.Fields, []string{"auditory_range", "perception_range"}, defaultAuditoryRange),
	}

	// 4. 加载 FSM
	fsmCfg, err := src.LoadFSMConfig(tmpl.Behavior.FSMRef)
	if err != nil {
		return nil, fmt.Errorf("npc %s: load FSM %q: %w", id, tmpl.Behavior.FSMRef, err)
	}
	f, err := fsm.NewFSM(fsmCfg, bb)
	if err != nil {
		return nil, fmt.Errorf("npc %s: create FSM: %w", id, err)
	}

	// 5. 加载并构建 BT
	btrees := make(map[string]bt.Node, len(tmpl.Behavior.BTRefs))
	for state, treeName := range tmpl.Behavior.BTRefs {
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

	return &Instance{
		ID:         id,
		TypeName:   tmpl.Name,
		Position:   pos,
		BB:         bb,
		FSM:        f,
		BTrees:     btrees,
		Perception: &percCfg,
	}, nil
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
