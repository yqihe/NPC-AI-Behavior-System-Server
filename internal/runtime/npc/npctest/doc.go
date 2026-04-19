// Package npctest 为集成测试提供绕过 R17 opt-in 契约的 NPC 构造 helper。
//
// 生产路径唯一合法入口是 npc.NewInstanceFromADMIN（见 R1）：ADMIN
// fields.enable_* bool 驱动 5 个能力组件的实例化。集成测试常常需要
// 在不修改 ADMIN seed 的前提下，显式启用某些组件，本包提供测试专用
// 后门 NewInstanceWithExtras。
//
// # 封闭红线（R21 + docs/standards/red-lines.md）
//
// 本包**仅**允许被 `*_test.go` 文件从 internal/runtime/ 子树中引用。
// 禁止 internal/runtime/ 或 internal/gateway/ 的生产代码 import；
// 违反者直接绕过 R17 opt-in 契约与 R18 级联校验。
//
// # 典型用法
//
//	tmpl := &npc.ADMINTemplate{
//	    Name:        "wolf_test",
//	    TemplateRef: "warrior_base",
//	    Fields:      map[string]any{"max_hp": 100, "perception_range": 20},
//	    Behavior: npc.ADMINBehavior{
//	        FSMRef: "fsm_combat_basic",
//	        BTRefs: map[string]string{"idle": "bt/combat/idle"},
//	    },
//	}
//	inst, err := npctest.NewInstanceWithExtras("wolf_test", pos, tmpl,
//	    map[string]json.RawMessage{
//	        "memory":  []byte(`{"capacity":10,"memory_types":["threat"],"decay_time":30}`),
//	        "emotion": []byte(`{"emotion_states":[{"name":"fear","value":0,"accumulate_rate":20,"decay_rate":5}]}`),
//	    },
//	    src, btReg, compReg,
//	)
package npctest
