# 扩展轴验证任务

验证核心价值："加新 NPC 类型或新事件源 = 加配置 + 加测试，不改核心代码"。

**硬性约束**：如果任何步骤需要修改 `internal/` 下的 Go 代码，说明架构有缺陷，必须先修底层再继续。

---

## [x] T1: 加第 4 种事件源 — fire (扩展轴 1)

**改动**：只加配置文件
- `configs/events/fire.json`

**验证**：加测试
- `test/e2e/extension_test.go` — spawn civilian → publish fire → 验证 NPC 响应

**证明**：新事件类型零代码改动，civilian 自动响应 fire

---

## [x] T2: 加 police NPC 类型 (扩展轴 2)

**改动**：只加配置文件
- `configs/npc_types/police.json`
- `configs/fsm/police.json`
- `configs/bt_trees/police_idle.json`
- `configs/bt_trees/police_alarmed.json`
- `configs/bt_trees/police_engage.json`

**验证**：加测试
- `test/e2e/extension_test.go` — spawn police → publish explosion → 验证 police 响应（与 civilian 不同行为）

**police 行为设计**（与 civilian 的区别）：
- civilian: Idle → Alarmed → Flee（逃跑）
- police: Idle → Alarmed → Engage（迎敌）
- police 感知范围更大，threat 阈值不同

---

## [x] T3: 多类型 × 多事件自动交叉 (扩展轴 1+2)

**改动**：无
**验证**：加测试
- `test/e2e/extension_test.go` — spawn civilian + police → publish fire → civilian 逃跑 + police 迎敌

**证明**：新 NPC 类型自动响应所有已有事件，无需任何改动

---

## 依赖顺序

```
T1 (加事件) → T2 (加 NPC 类型) → T3 (交叉验证)
```
