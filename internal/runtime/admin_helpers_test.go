package runtime_test

import (
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
)

// wolfBehavior 返回 wolf_common 原 v2 fixture 的 FSM + BT refs。
// 迁移后测试不再加载 configs/npc_templates/wolf_common.json（将由 T7 删除），
// 本函数保留该 fixture 的行为轮廓供 npctest 注入测试使用。
// civilian FSM/BT 已按 R10 保留在 configs/（实验层+e2e 依赖）。
func wolfBehavior() npc.ADMINBehavior {
	return npc.ADMINBehavior{
		FSMRef: "civilian",
		BTRefs: map[string]string{
			"Idle":    "civilian/idle",
			"Alarmed": "civilian/alarmed",
			"Flee":    "civilian/flee",
			"Cower":   "civilian/cower",
		},
	}
}

// wolfFields 返回 wolf_common 视觉/听觉/攻击性等感知字段的基线，
// 对应 v2 fixture 的 perception.visual_range=150 / auditory_range=300 /
// attention_capacity=3；personality 的 aggression=aggressive 通过 fields
// 表达（T1 翻译层读此推断 personality_type）。
//
// 测试按需 override 部分 key（如 TestPerceptionIntegration_AttentionCapacity
// 需要 attention_capacity=3，通过 extras.perception 注入）。
func wolfFields() map[string]any {
	return map[string]any{
		"visual_range":   150.0,
		"auditory_range": 300.0,
		"aggression":     "aggressive",
		"move_speed":     4.0,
	}
}

// wolfADMINTemplate 构造一个等价 wolf_common 的 ADMIN 形状模板，
// 可选 extraFields 覆盖/扩展默认 fields。
func wolfADMINTemplate(extraFields map[string]any) *npc.ADMINTemplate {
	fields := wolfFields()
	for k, v := range extraFields {
		fields[k] = v
	}
	return &npc.ADMINTemplate{
		Name:        "wolf_common",
		TemplateRef: "reactive_carnivore",
		Fields:      fields,
		Behavior:    wolfBehavior(),
	}
}

// butterflyFields 返回 butterfly_01 simple 级 NPC 的字段基线，
// 对应 design.md §4.2 目标：aggression=passive / max_hp=1 / move_speed=1.5 /
// perception_range=5（T1 fallback 链给 visual_range=5 / auditory_range=5）。
func butterflyFields() map[string]any {
	return map[string]any{
		"aggression":       "passive",
		"max_hp":           1.0,
		"move_speed":       1.5,
		"perception_range": 5.0,
	}
}

// butterflyADMINTemplate 构造 butterfly_01 的 ADMIN shape 模板。
// fsm_ref 使用 civilian（本仓保留 FSM/BT 配置链）——butterfly 5m 感知范围意味着
// 大多数测试事件超出范围，FSM 在 Idle 状态不迁移，与 v2 butterfly 无 behavior
// 组件的行为语义等价。可选 extraFields 覆盖默认字段。
func butterflyADMINTemplate(extraFields map[string]any) *npc.ADMINTemplate {
	fields := butterflyFields()
	for k, v := range extraFields {
		fields[k] = v
	}
	return &npc.ADMINTemplate{
		Name:        "butterfly_01",
		TemplateRef: "passive_npc",
		Fields:      fields,
		Behavior: npc.ADMINBehavior{
			FSMRef: "civilian",
			BTRefs: map[string]string{
				"Idle":    "civilian/idle",
				"Alarmed": "civilian/alarmed",
				"Flee":    "civilian/flee",
				"Cower":   "civilian/cower",
			},
		},
	}
}
