package npctest

import (
	"encoding/json"
	"fmt"

	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
	"github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
)

// NewInstanceWithExtras 构造 NPC 实例，先按 R17 opt-in 契约走翻译层，
// 再按 extras 显式注入附加组件（覆盖同名 opt-in 结果）。
//
// 参数：
//   - id       NPC 唯一标识
//   - pos      spawn 位置
//   - admin    ADMIN shape 模板（通常由测试 fixture 构造）
//   - extras   附加组件 JSON；key 为组件 name（"memory"/"emotion" 等），
//     value 为 factory 期望的 JSON payload。nil 或空 map 等价于未注入。
//   - src      配置源（加载 FSM/BT）
//   - btReg    BT 节点 registry
//   - compReg  component registry（用于 factory 调用）
//
// 行为：
//  1. 调用 npc.NewInstanceFromADMIN 按默认 + opt-in 契约翻译 admin
//     形状为 Instance（5 默认组件 + 按 fields.enable_* 条件的 0–5 opt-in 组件）
//  2. 遍历 extras，对每个 (name, raw) 调用 compReg.Create(name, raw)
//  3. 调用 Instance.InjectComponentForTest 替换或追加：若组件名与 opt-in
//     结果重名，extras 版本覆盖（测试显式意图优先）
//  4. tickables 在每次 Inject 时重排，保证 memory(0)→needs(1)→emotion(2)→movement(3) 顺序
func NewInstanceWithExtras(
	id string,
	pos event.Vec3,
	admin *npc.ADMINTemplate,
	extras map[string]json.RawMessage,
	src config.Source,
	btReg *bt.Registry,
	compReg *component.Registry,
) (*npc.Instance, error) {
	inst, err := npc.NewInstanceFromADMIN(id, pos, admin, src, btReg, compReg)
	if err != nil {
		return nil, fmt.Errorf("npctest: base instance: %w", err)
	}
	if compReg == nil {
		// NewInstanceFromADMIN 已先校验 nil compReg 返回错误；此处 defensive
		return nil, fmt.Errorf("npctest: nil component registry")
	}

	for name, raw := range extras {
		comp, err := compReg.Create(name, raw)
		if err != nil {
			return nil, fmt.Errorf("npctest: create extra component %q: %w", name, err)
		}
		inst.InjectComponentForTest(name, comp)
	}
	return inst, nil
}
