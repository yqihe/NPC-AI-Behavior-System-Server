# external-contract-server-adaptation 设计方案

**对应需求**：[requirements.md](requirements.md) R1–R24（2026-04-19 落盘）
**ADMIN 契约锚**：[docs/architecture/api-contract.md](../../architecture/api-contract.md) v1.1（源 commit `0aa77b2`）

---

## 1. 方案描述

### 1.1 翻译层总览

`NewInstanceFromADMIN` 升级为"ADMIN shape → 组件实例 Instance"的完整翻译层，是 NPC 实例创建的**唯一**对外入口（R1）：

```
ADMIN {template_ref, fields, behavior}
         │
         ├─ 默认组件层（5 个）
         │   ├─ identity    ← 合成自 tmpl.Name + 默认 tags=[]
         │   ├─ position    ← spawn pos 参数覆盖
         │   ├─ behavior    ← fields.fsm_ref + bt_refs → FSM/BT 加载 + 绑定
         │   ├─ perception  ← visual_range / auditory_range / perception_range（R8 fallback 链）
         │   └─ movement    ← move_type（按 fsm_ref 推断：fsm_passive→wander, 其他→默认 wander）+ move_speed（fields）+ wander_radius（默认 20，zone 级可覆盖）
         │
         ├─ opt-in 组件层（5 个，R17 驱动）
         │   ├─ fields.enable_memory=true     → MemoryComponent（默认容量/types 由 factory 决定）
         │   ├─ fields.enable_emotion=true    → EmotionComponent（默认 fear 状态）
         │   ├─ fields.enable_needs=true      → NeedsComponent（默认 need types）
         │   ├─ fields.enable_personality=true→ PersonalityComponent（weights 从 fields.aggression 推断）
         │   └─ fields.enable_social=true     → SocialComponent（GroupID/Role 从 fields.group_id / social_role 读，缺失为空）
         │
         └─ 动态扁平字段写 BB（R7）
             └─ 其他未声明 fields（如 hp, attack_power, is_boss, loot_table）→ SetDynamic
```

**5 默认 + 5 opt-in 对应 10 个 component factory**，全部保留（R5）。

### 1.2 级联校验（R18，落点详见 §2）

main.go 在 Registry 填充完成后、Scheduler.Tick 启动之前，**逐 NPC 遍历**检查：

```go
for _, inst := range registry.All() {
    if getBool(inst.Fields, "enable_emotion") && !getBool(inst.Fields, "enable_memory") {
        violations = append(violations, inst.ID)
    }
}
if len(violations) > 0 { log.Fatalf(...) }
```

### 1.3 main.go zone + ADMIN 双 spawn 路径收敛（R15 硬依赖）

**当前 [cmd/server/main.go:114-117](cmd/server/main.go#L114-L117)**：
```go
if zm.Count() == 0 {
    spawnFromADMINTemplates(src, btReg, reg)  // zones 非零就 skip
}
```

**问题**：meadow.json 存在时 `zm.Count() == 1` → ADMIN 6 NPC 不 spawn → **R15 端到端验收失败**。

**新逻辑**：
```go
// 4a. Zone spawn（按 spawn_table，引用 ADMIN templates 或本地 butterfly_01）
for _, data := range regionConfigs { zm.AddZone(...); z.Spawn(...) }

// 4b. ADMIN-orphan spawn：ADMIN templates 未被任何 region spawn_table 引用的 → 默认网格位置 spawn
spawnOrphanADMINTemplates(src, btReg, reg, zm)
```

`spawnOrphanADMINTemplates` 实现：
1. `src.LoadAllNPCTemplates()` 取 ADMIN 全量
2. 对每个 template.Name，扫描所有 region.spawn_table entry 是否引用它
3. 未被引用的 → 走 `NewInstanceFromADMIN` 默认网格位置 spawn

这个改动保留"zone 为 primary spawn 入口"的语义，同时保证 ADMIN NPC 即使没 region 引用也会被 spawn，R15 天然满足。

### 1.4 JSONSource fallback 路径（R16）

`configs/` 目录承担"ADMIN 镜像 + 本地兜底 fixture"**双重角色**，但两种数据都以 ADMIN shape 存在：
- ADMIN sync 产物（5 NPC + 6 BT + 3 FSM + 3 event_types）
- 本地 `butterfly_01.json`（ADMIN shape，但 ADMIN DB 没有，仅 JSONSource 可读）

HTTPSource 离线时自动降级到 JSONSource（[internal/config/http_source.go](internal/config/http_source.go) 现有逻辑）。降级后 butterfly_01 可用；ADMIN 的 6 NPC 依赖 `configs/npc_templates/` 里 sync 的副本。

### 1.5 备选方案对比

见 §7 — 备选 A（双路径）+ 备选 B（内部翻译器）均已否决。

---

## 2. Registry 级联校验落点（R18）

### 设计

**落点：`cmd/server/main.go` 的 Registry 填充阶段，`sched.Tick()` 启动之前。**

```
main()
├─ 1. 加载 Source（ADMIN live / JSON fallback）
├─ 2. LoadAllNPCTemplates() → map[name]rawJSON
├─ 3. 逐个 ParseADMINTemplate(name, raw)
├─ 4. 为每个 template 调用 NewInstanceFromADMIN → *Instance
├─ 5. 【新增】 cascadeValidate(registry) ← R18 fatal 点
│     ├─ 遍历 registry.ForEach
│     ├─ 对每个 inst 读 tmpl.Fields["enable_emotion"] / ["enable_memory"]
│     └─ 若 emotion=true ∧ memory=false，收集违规 NPC name
├─ 6. 违规非空 → log.Fatalf 打印完整列表 + ADMIN UI 修正路径
└─ 7. （无违规）启动 Scheduler + Gateway
```

### 为什么是 main.go 不是 NewInstanceFromADMIN

| 候选位置 | 否决理由 |
|---|---|
| `NewInstanceFromADMIN` 内部单 NPC 校验 | 违规时 fatal 单个 NPC，跳过剩余——违反 R18 "不跳过、不部分启动"明确要求 |
| `zone.Spawn()` | zone 可能多轮 spawn（respawn_seconds），每次重复校验——时机不对，应在进程启动一次性完成 |
| `Scheduler.Tick` 首次迭代 | 已启动才 fatal = 延迟暴露错误 |

**main.go 填充后 Tick 前是唯一符合 R18 语义的时机。**

### 错误信息格式

```
FATAL: cascade validation failed (enable_emotion=true requires enable_memory=true)
Offending NPCs:
  - villager_merchant (enable_emotion=true, enable_memory=false)
  - wolf_alpha (enable_emotion=true, enable_memory=false)
Fix via ADMIN UI: /templates/detail/{npc_name} → 字段 → 启用记忆 (enable_memory)
Or locally: configs/npc_templates/{npc_name}.json "fields.enable_memory" = true
Process aborting — 2 violations
```

### 实现锚

- 新增函数：`internal/runtime/npc/cascade.go::ValidateCascade(reg *Registry) []CascadeViolation`
- main.go 调用：`if violations := npc.ValidateCascade(reg); len(violations) > 0 { log.Fatalf(...) }`
- zone.Spawn 内部 respawn 时**不再校验**（spawn 走的 template 已在启动期校验过）

### 并发安全

校验发生在 Scheduler 启动之前，Registry 尚无并发写入。无锁读安全。

---

## 3. 默认 + opt-in 组件清单

10 个 component factory 全部保留（R5）。按实例化策略分层：

| Factory | 层 | 创建条件 | 数据源 | 备注 |
|---|---|---|---|---|
| identity | **默认** | 始终 | 合成：`{name: tmpl.Name, model_id: tmpl.Name, tags: []}` | name 作为 model_id 兜底；future 若 ADMIN 加 model_id 字段再升级 |
| position | **默认** | 始终 | `spawn pos 参数`（zone/orphan path 均提供） | zone 维度 `zone_id` 从 region 注入 |
| behavior | **默认** | 始终 | `fields.fsm_ref` + `fields.bt_refs` | 加载 FSM + BT，绑定到 BehaviorComponent |
| perception | **默认** | 始终 | `fields.visual_range` / `auditory_range` / `perception_range` | R8 fallback 链保留 |
| movement | **默认** | 始终 | 混合：`move_type` 由 `fsm_ref` 推断（passive → wander, combat → wander 默认）；`move_speed` 来自 fields；`wander_radius` 取默认 20.0（zone `spawn_table.wander_radius` 可覆盖） | ADMIN shape 无 `move_type` / `wander_radius` 字段，此处做约定 |
| memory | **opt-in** | `fields.enable_memory=true` | factory 默认构造（capacity=10, memory_types=["threat"], decay_time=30） | 常量定义在 `internal/runtime/component/defaults.go` 新增 `DefaultMemoryConfig` |
| emotion | **opt-in** | `fields.enable_emotion=true` | factory 默认构造（单 fear 状态，accumulate_rate=20, decay_rate=5） | 同上 `DefaultEmotionConfig` |
| needs | **opt-in** | `fields.enable_needs=true` | factory 默认构造（hunger/rest 两个 needs） | 同上 |
| personality | **opt-in** | `fields.enable_personality=true` | 从 `fields.aggression` 映射 weights：aggressive→(Threat:1.5, Needs:0.8, Emotion:1.0) / neutral→默认 / passive→(Threat:0.7, Needs:1.2, Emotion:1.3) | aggression 已是 ADMIN 一等字段 |
| social | **opt-in** | `fields.enable_social=true` | `fields.group_id` / `fields.social_role`（若 ADMIN 无这两字段则空值，GroupManager 不会注册该 NPC，等价功能下线）| 已知短板：当前 ADMIN seed 不含 group_id/social_role；social 能力 opt-in 后若无这两字段等价空开。本 spec 接受此短板，未来 ADMIN 加字段后自动激活 |

### opt-in 默认值策略

默认配置从 factory 内部常量读取（写入 `internal/runtime/component/defaults.go`），**不走 ADMIN fields**。理由：
- ADMIN 加 memory capacity 等字段会爆发字段池（10 NPC × 5 组件 × 4 子参数 = 200+ 字段）
- 毕设场景不需要 per-NPC 调优；真需要时，运营通过 npctest 后门注入定制 JSON（R21）
- 保持 ADMIN field schema 收敛于"NPC 属性 + 能力开关"两类

**已知约束 / 过期条件**：opt-in 组件的内部参数（memory.capacity / emotion.accumulate_rate / needs.decay_rate 等）当前来自 `defaults.go` 常量。运营在 ADMIN 侧只能**开/关**能力，无法**调参**。升级触发条件：
- 连续 2 次 demo 反馈"想调 memory 容量 / 情绪累积速率"等 tune 需求
- 正式上线前的 performance tuning 需求出现
- 或 opt-in 组件间出现"同一组件、不同 NPC 需不同参数"的语义场景

触发后方案：把常量升级为 `config.fields.{memory_capacity,emotion_fear_rate,...}` 这类 per-NPC field，ADMIN 补 seed，翻译层按 field 读取兜底到 `defaults.go`。此路径不影响 R17/R18/R19 契约。

### Instance 创建流程（伪代码）

```go
func NewInstanceFromADMIN(id, pos, tmpl, src, btReg, compReg) (*Instance, error) {
    components := map[string]Component{}

    // 1. 默认 5 组件
    components["identity"]  = compReg.Create("identity",  synthIdentity(tmpl))
    components["position"]  = compReg.Create("position",  synthPosition(pos))
    components["behavior"]  = compReg.Create("behavior",  loadBehavior(tmpl, src, btReg))
    components["perception"]= compReg.Create("perception",synthPerception(tmpl.Fields))
    components["movement"]  = compReg.Create("movement",  synthMovement(tmpl))

    // 2. Opt-in 5 组件
    for name, flag := range map[string]string{
        "memory": "enable_memory", "emotion": "enable_emotion", "needs": "enable_needs",
        "personality": "enable_personality", "social": "enable_social",
    } {
        if readBool(tmpl.Fields, flag) {
            components[name] = compReg.Create(name, defaultComponentConfig(name, tmpl))
        }
    }

    // 3. BB 初始化
    bb := blackboard.New()
    for k, v := range tmpl.Fields { blackboard.SetDynamic(bb, k, v) }
    blackboard.Set(bb, KeyNPCType, tmpl.Name) /* + KeyNPCPosX/Z/CurrentTime 等 */

    // 4. tickables 排序 + Instance 组装（复用老 template.go 逻辑）
    // ...
    return &Instance{...}, nil
}
```

### 对老 template.go 逻辑的保留/改造

| 老 template.go 逻辑 | 新去向 |
|---|---|
| identity 硬类型断言（L67）| 保留——默认创建后可安全断言 |
| position 硬类型断言（L71）| 保留——默认创建 |
| social BB 写入 `KeyGroupID`/`KeySocialRole`（L82-88） | 迁到 NewInstanceFromADMIN 的 social opt-in 分支 |
| movement.SetSpawn（L92） | 迁到 NewInstanceFromADMIN 的 movement 默认分支 |
| FSM 状态变迁日志 OnTransition（L110） | 迁到 behavior 默认分支 |
| tickablePriority 排序（L59）| 函数整体迁移到 admin_template.go |

---

## 4. configs/ 迁移方案

### 4.1 文件级处置清单

> ⚠️ **Phase 2 修正记录**（2026-04-19 late，两轮）：
> **第一轮**：误判 civilian 为孤儿。grep 暴露 civilian 是实验层 + 广泛测试核心 fixture（`internal/experiment/modes/*` 5 模式、`scenario.go` 4 场景、`internal/core/integration_test.go` + 多处 Source 测试）。civilian 改保留。
> **第二轮**：误判 police 为孤儿——同一模式错误，grep 遗漏 `test/e2e/extension_test.go`。police 是毕设**扩展轴 2 演示核心**（police spawn → explosion → Engage 响应，对比 civilian Flee，证明"加 NPC 类型零代码"）。police 改保留。
> 修正：civilian + police 全套（npc_types/fsm/bt_trees）保留；仅 guard(V2) 的 npc_types/guard.json + bt_trees/guard/{alert,defend}.json 可删（compat_test.go 随 R3 同步删后无消费者）。
> 本修正记录同时暴露 **R4/R10 requirements-level 偏差**，见 §13。
>
> **Recurrent gap 防范**：声明任何文件/配置"零消费者"前，grep 覆盖必须包含 `internal/` + `test/e2e/` + `internal/experiment/` + `docs/specs/` 全域。本 spec 自身已犯该错两次，记入 memory 作为稳定 feedback。

| 路径 | 处置 | 理由 |
|---|---|---|
| `configs/npc_templates/butterfly_01.json` | **Rewrite 到 ADMIN shape** | 新内容见 §4.2，保留 name（meadow.json 不动）|
| `configs/npc_templates/wolf_common.json` | **删除** | 选项 (c)，15 处测试迁 npctest；civilian FSM/BT 不存在 ADMIN，rewrite 不可行 |
| `configs/npc_templates/{goblin_archer,guard_basic,villager_guard,villager_merchant,wolf_alpha,wolf_common}.json` | **cmd/sync 产出新增** | 6 个 ADMIN NPC，sync 后直接 git add |
| `configs/npc_types/civilian.json` | ✅ **保留** | 实验层（hybrid.go 等）+ 广泛测试依赖，删除击穿 measurement framework |
| `configs/npc_types/police.json` | ✅ **保留** | `test/e2e/extension_test.go` 扩展轴 2 演示（spawn police → publish explosion → assert police Engage ≠ civilian Flee），是毕设"加 NPC 类型零代码"narrative 的核心证据。删除击穿扩展轴 2 演示 |
| `configs/npc_types/guard.json`（V2） | **删除** | 唯一 Go 消费者 `compat_test.go` 随 R3 同步删除；admin_template_test.go 里的 "guard" 是 fakeSource 注入名不读此文件 |
| `configs/fsm/civilian.json` | ✅ **保留** | 实验层 `pure_fsm.go` / `fsm_dc.go` / `hybrid.go` 直接 LoadFSMConfig("civilian") |
| `configs/fsm/police.json` | ✅ **保留** | extension_test.go 扩展轴 2 演示：police FSM 含 Engage 状态，是"相同 explosion 事件、不同 NPC 响应"的演示核心 |
| `configs/fsm/guard.json` | **保留**（ADMIN 已镜像）| ADMIN 有 `guard` FSM，sync 覆盖后本地即镜像 |
| `configs/fsm/{fsm_combat_basic,fsm_passive}.json` | **cmd/sync 产出新增** | ADMIN 侧 FSM |
| `configs/bt_trees/civilian/{idle,alarmed,flee,cower,pure_bt}.json` | ✅ **保留整目录** | 实验层 `bt_dc.go` / `pure_bt.go` LoadBTTree("civilian/pure_bt")；e2e/extension_test.go 依赖 |
| `configs/bt_trees/police/{idle,alarmed,engage}.json` | ✅ **保留整目录** | extension_test.go 扩展轴 2：police spawn → Engage 分支依赖此 BT 树集合 |
| `configs/bt_trees/guard/patrol.json` | **保留**（ADMIN 镜像）| ADMIN 有此 BT |
| `configs/bt_trees/guard/{alert,defend}.json` | **删除** | V2 guard NPC 专用，ADMIN 无对应；无 Go 消费者 |
| `configs/bt_trees/bt/{combat,passive}/*` | **cmd/sync 产出新增** | ADMIN 侧 BT（5 个 combat/passive BT）|
| `configs/events/*.json` | **cmd/sync 产出** | ADMIN 3 个 event_types |
| `configs/regions/meadow.json` | **保留不动** | 引用 `butterfly_01`（name 不变）|
| `configs/server.json` | **保留** | 服务端配置，与 ADMIN 无关 |

### 4.2 butterfly_01.json rewrite 目标内容

```json
{
  "template_ref": "passive_npc",
  "fields": {
    "aggression": "passive",
    "max_hp": 1,
    "move_speed": 1.5,
    "perception_range": 5
  },
  "behavior": {
    "fsm_ref": "fsm_passive",
    "bt_refs": {"wander": "bt/passive/wander"}
  }
}
```

**设计决策记录**：
- `template_ref: passive_npc` — 对齐 ADMIN 已有 passive_npc 模板，不引入新模板
- `max_hp: 1` — 蝴蝶的"一击即死"脆弱感
- `move_speed: 1.5` — V2 版本同值
- `perception_range: 5` — V2 版本同值
- `aggression: "passive"` — 与 `fsm_passive` 一致
- **丢失字段 / 不影响行为**：
  - `model_id: "butterfly_blue"` — 视觉层面，服务端无消费者（若未来有渲染指令协议再加回）
  - `tags: []` — V2 也是空
  - `movement.wander_radius: 20` — `meadow.json` 的 `wander_radius: 30` 在 zone spawn 层覆盖 NPC 内嵌值
  - `movement.move_type: "wander"` — 由 `fsm_passive` 推断（见 §3）
  - `position.zone_id` — 由 region spawn 注入

### 4.3 wolf_common civilian 删除的 15 处测试迁移

涉及文件：`decision_integration_test.go`（4）/ `memory_integration_test.go`（3）/ `perception_integration_test.go`（3）/ `social_integration_test.go`（1）/ `movement_integration_test.go`（1）/ `component_integration_test.go`（2）/ `benchmark_test.go`（1）

现模式（将失效）：
```go
raw, _ := src.LoadNPCTemplate("wolf_common")
tmpl, _ := npc.ParseNPCTemplate(raw)
tmpl.Components["memory"] = []byte(`{...}`)
inst, _ := npc.NewInstanceFromTemplate(...)
```

新模式（npctest 内联构造）：
```go
adminTmpl := &npc.ADMINTemplate{
    Name: "wolf_test", TemplateRef: "warrior_base",
    Fields: map[string]any{
        "max_hp": 120, "move_speed": 5.5, "perception_range": 20, "attack_power": 18,
    },
    Behavior: npc.ADMINBehavior{
        FSMRef: "fsm_combat_basic",
        BTRefs: map[string]string{
            "idle": "bt/combat/idle", "patrol": "bt/combat/patrol",
            "chase": "bt/combat/chase", "attack": "bt/combat/attack",
        },
    },
}
inst, _ := npctest.NewInstanceWithExtras("wolf_test", pos, adminTmpl,
    map[string]json.RawMessage{
        "memory":  []byte(`{...}`),
        "emotion": []byte(`{...}`),
    },
    src, btReg, compReg,
)
```

**关键语义转换**：原测试使用 civilian FSM（Idle/Alarmed/Flee/Cower 4 状态）——**测试本质不是验证 civilian 特定 FSM 行为**，而是验证 memory/emotion/perception/decision 等 component 的工作情况。切换到 `fsm_combat_basic`（6 状态含 flee）后断言含义**保持不变**。

社交测试特殊处理：`social_integration_test.go` 的 wolf pack 需要 `enable_social + group_id + social_role` 字段——通过 adminTmpl.Fields 直接写入（不走 enable_* 也行，因为测试通过 npctest 绕过 opt-in）。

### 4.4 验证 Gate

- `go test ./...` 全绿（R12）
- `go run ./cmd/sync -api http://localhost:9821 && go test ./...` 仍全绿（R11 + R12）
- `docker compose up --build` 启动看 6 + 3 蝴蝶 = 9 NPC 全部 spawn（R15，butterfly 不抢 ADMIN 的 6 个）
- `HTTPSource 不可达 + JSONSource 降级` 场景 9 NPC 同样 spawn（R16）

---

## 5. npctest 子包 API 设计（R21）

### 包路径与结构

```
internal/runtime/npc/npctest/
├── helpers.go       # 导出的测试 helper API
├── doc.go           # 包文档 + 使用示例
└── helpers_test.go  # helper 自身的单测
```

### 核心 API 签名

```go
// Package npctest 为集成测试提供绕过 ADMIN opt-in 契约的 NPC 构造 helper。
// 禁止从 internal/runtime/ 或 internal/gateway/ 生产代码 import（见 docs/standards/red-lines.md）。
package npctest

import (
    "encoding/json"
    "github.com/yqihe/NPC-AI-Behavior-System-Server/internal/config"
    "github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt"
    "github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/component"
    "github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/event"
    "github.com/yqihe/NPC-AI-Behavior-System-Server/internal/runtime/npc"
)

// NewInstanceWithExtras 构造 NPC 实例，允许测试绕过 R17 opt-in 直接注入附加组件。
//
// 参数：
//   id       - NPC 唯一标识
//   pos      - spawn 位置
//   admin    - ADMIN shape 模板（R6 翻译层解析后的产物）
//   extras   - 附加组件 JSON；key 为组件 name（如 "memory"/"emotion"），value 为 factory 期望的 JSON payload
//              注入的组件直接写入 Instance.components，不受 enable_* 标志影响
//   src      - 配置源（加载 FSM/BT）
//   btReg    - BT 节点 registry
//   compReg  - component registry（用于 factory 调用）
//
// 行为：
//   1. 按 R6 翻译 admin → Instance（默认组件 + opt-in 组件）
//   2. 按 extras 逐个调用 compReg.Create(name, raw)，追加到 components
//   3. 若 extras 中组件名与 opt-in 结果重复，extras 覆盖（测试显式意图优先）
//   4. 追加后重新排序 tickables（保证 memory(0) → needs(1) → emotion(2) → movement(3) 顺序）
//
// 测试使用示例：
//
//   tmpl := &npc.ADMINTemplate{
//       Name:        "wolf_test",
//       TemplateRef: "warrior_base",
//       Fields:      map[string]any{"max_hp": 100, "perception_range": 20},
//       Behavior:    npc.ADMINBehavior{FSMRef: "fsm_combat_basic", BTRefs: map[string]string{"idle": "bt/combat/idle"}},
//   }
//   inst, err := npctest.NewInstanceWithExtras("test_wolf", pos, tmpl,
//       map[string]json.RawMessage{
//           "memory":  []byte(`{"capacity":10,"memory_types":["threat"],"decay_time":30}`),
//           "emotion": []byte(`{"emotion_states":[{"name":"fear","value":0,"accumulate_rate":20,"decay_rate":5}]}`),
//       },
//       src, btReg, compReg,
//   )
func NewInstanceWithExtras(
    id string,
    pos event.Vec3,
    admin *npc.ADMINTemplate,
    extras map[string]json.RawMessage,
    src config.Source,
    btReg *bt.Registry,
    compReg *component.Registry,
) (*npc.Instance, error)
```

### 为什么不直接暴露 `extras` 给生产 API

- 生产路径唯一合法入口必须是 ADMIN 契约 → R17 opt-in 驱动（见 R1）
- extras 是测试专用"显式 > 契约"后门；生产代码误用会悄悄绕过级联校验 R18
- 独立包 + 生产代码禁止 import 的红线（R21）是硬保障

### 封闭机制

- **物理层**：`internal/runtime/npc/npctest/` 在 Go internal 规则下只对 `internal/runtime/npc` 及其子树可见，`internal/runtime/` 和 `internal/gateway/` 的生产文件本就无法 import（违反 internal 规则报编译错）。验证：空 server 编译 + `go vet`
- **约定层**：`docs/standards/red-lines.md` "禁止过度设计" 章节追加："禁止生产代码 import 名字以 `test` 结尾的 Go 包"（已由 R21 指定）
- **审计层**：未来 CI 可加 lint（`grep -r "npctest" internal/runtime/ internal/gateway/ --include="*.go" --exclude="*_test.go"` 非空则失败）——**不在本 spec 范围**，只留 TODO

### 并发安全

helper 构造阶段无并发（单测 / 集成测试主线程构造）。构造完 Instance 返回后，并发读写由 Scheduler 负责 — 与生产路径等价。

---

## 6. 红线逐条 review（R23 + R24）

### `docs/architecture/red-lines.md`（项目专属）

| 红线条目 | 设计是否合规 | 依据 |
|---|---|---|
| 禁止硬编码 FSM 状态 | ✅ | FSM 仍从 ADMIN `behavior.fsm_ref` 动态加载，未触 |
| 禁止硬编码事件-感知映射 | ✅ | 本 spec 不改事件层 |
| 禁止硬编码 NPC 类型参数 | ✅ | 参数从 ADMIN `config.fields` 读（R6），未触 |
| 禁止 switch-case 做 NPC 类型分发 | ✅ | 翻译层按 `enable_*` 通用条件分支，非按 NPC 类型枚举 |
| 禁止 BT 反向驱动 FSM | ✅ | 本 spec 不改 BT↔FSM 交互 |
| 禁止 core/ import runtime/ 或 gateway/ | ✅ | npctest 在 `runtime/npc/npctest/` 下，依赖方向向下不反向 |
| 禁止 core/ 或 runtime/ import experiment/ | ✅ | 未动 |
| 禁止 gateway/ 承担非网络职责 | ✅ | 未动 handler.go 业务逻辑，只随翻译层 API 升级调用点 |
| 禁止 Blackboard 裸 map[string]any | ✅ | R7 `SetDynamic` 已封装动态 Key 注册 |
| 禁止 BB Key 散落 | ✅ | 新增 Key（如无）统一在 `internal/core/blackboard/keys.go` |
| 禁止 Key 拼错静默失败 | ✅ | 本 spec 不引入新 Key；`enable_*` 是 ADMIN 字段不是 BB Key |
| 禁止 FSM 状态魔法字符串 | ✅ | 未动 |
| 禁止实验污染核心 | ✅ | 未涉及 |
| 禁止实验作弊 | ✅ | 未涉及 |
| 禁止联调配置脱节（不走 REST 写 MongoDB）| ✅ | 本 spec 消费 ADMIN HTTP API，不走 MongoDB 直连 |
| 禁止服务端跳过 cmd/sync 验证 | ✅ | R11 明示 `cmd/sync` 产物可直接 git add |

### `docs/standards/red-lines.md`（通用）

| 红线条目 | 设计是否合规 | 依据 |
|---|---|---|
| 禁止静默降级 | ✅ | R18 fatal 不降级；R20 缺席降级每项有文档化 BB key 语义 |
| 禁止运行时吞错 | ✅ | R24 条约束翻译层字段缺失/类型错打 WARN |
| 禁止 lookup 失败 silent return | ✅ | 翻译层 component factory 失败 → 返回 error 向上传播 |
| 禁止外部输入拼接文件路径 | ✅ | ADMIN name 来自契约可信源；`cmd/sync` 已有 sanitize（TODO 验证） |
| 禁止配置错误延迟暴露 | ✅ | R18 级联校验在 Registry 填充后立即 fatal |
| 禁止 DB 查询接受用户输入操作符 | N/A | 本 spec 不涉 DB |
| 禁止无超时外部 IO | ✅ | HTTPSource 已有 30s timeout（PR #17 实现） |
| 禁止信任前端校验 | ✅ | R18 服务端 fatal 校验是对 ADMIN UI 校验缺席的兜底 |
| 禁止暴露内部错误详情 | ✅ | Fatal 日志内部可见；API 响应不受本 spec 改动 |
| 禁止收到请求不回复 | ✅ | 跨仓协作流程已稳定（本 spec 即证明） |
| 禁止口头承诺不落文档 | ✅ | 契约全在 api-contract.md + spec 文档 |
| 禁止只改本地参考文件 | ✅ | ADMIN 侧已通过 API 写 MySQL（PR #39），本仓只镜像文档 |
| 禁止引入无使用场景依赖 | ✅ | 无新依赖；`npctest` 有明确测试使用场景 |
| 禁止死配置 | ✅ | R3/R4 删 `NPCTypeConfig` + `configs/npc_types/` 消除死配置 |
| 禁止为单调用点创建抽象 | ⚠️ | `npctest.NewInstanceWithExtras` 有多测试调用点（8 + 15 处），不违反 |
| 禁止无敏感信息加密 | N/A |
| 禁止测试硬编码外部数据计数 | ⚠️ | R14 锚 `guard_basic.hp=100` 是数据噪声锚点——**刻意为之**，设计决策 |
| 禁止测试忽略反序列化错 | ✅ | 测试改造时配合检查 JSON 解析错 |
| 禁止测试未使用变量 / 空断言 | ✅ | 测试改造时清理 |
| 禁止测试死代码 | ✅ | R3/R4 删除 compat 路径相关死测试 |

**红线 review 结论**：无违反。一个 ⚠️ 标注（R14 硬编码 hp=100）属于刻意锚点，设计决策明示非无意违反。

---

## 7. 方案对比（备选 + 为何不选）

### 备选 A：保留双路径，ADMIN 仅作 "新 NPC 来源"

**设计**：
- `admin_template.go` 与 `template.go` 并存，`ParseNPCTemplate` 自动分流
- 组件化 schema 仍是主路径；ADMIN shape 仅覆盖通过 `cmd/sync` 新增的 NPC
- configs/npc_templates/ 允许两种 shape 混合

**不选理由**：
- 直接违反 path A 决策（用户已拒绝）
- 双 schema 技术债持续累积；加新 NPC 类型必须考虑两条路径的一致性
- R17 opt-in 契约在双 schema 下语义不清：组件化 NPC 的 enable_* 字段去哪读？
- 测试基线问题（sync 打碎 13 测试）只是被推迟到下一次；根本矛盾未解

### 备选 B：保留组件化 shape 作为内部真相，翻译 ADMIN → 组件化

**设计**：
- 内部保留 `TemplateConfig` + `NewInstanceFromTemplate`
- `admin_template.go` 降级为 "ADMIN shape → TemplateConfig" 转换器
- 组件化 shape 仍是 Instance 创建的唯一入口

**不选理由**：
- 本质是旧决策（path C "双向翻译器"）的复刻，已在 Phase 1 讨论阶段拒绝
- 多一个翻译层 = 多一套维护成本；每次 ADMIN 加字段都要写 "ADMIN field → component JSON" 映射
- 与 R1 "admin_template.go 是唯一入口"直接冲突

### 选定方案：ADMIN shape 一路到底 + opt-in 驱动 + npctest 后门

**设计**（当前 design.md 主线）：
- `admin_template.go` 唯一 NPC 实例入口（R1）
- `fields.enable_*` 5 bool 驱动 component factory 调用（R17）
- 测试通过 `npctest.NewInstanceWithExtras` 注入额外组件（R21）
- 组件化 `TemplateConfig` / v2 `NPCTypeConfig` / `compat.go` 全删（R2/R3/R4）

**为什么选**：
- 单路径消除双 schema 技术债（扩展轴 2 根治）
- opt-in 契约显式化了 NPC "有哪些能力"（扩展轴 3 友好）
- npctest 后门承认测试是特殊场景，不把生产路径复杂化
- 与 ADMIN 单向依赖方向一致（权威源 ADMIN → 镜像源服务端）

---

## 8. 扩展轴影响

| 扩展轴 | 本 spec 影响 | 证据 |
|---|---|---|
| **加事件源** | 中性 | 事件 schema 不变；感知层不动；cmd/sync 端点已修（PR #20）|
| **加 NPC 类型** | **正面强** | 双 schema 消除→单一 ADMIN shape；加新 NPC 真正做到"ADMIN UI 加 → cmd/sync → git add → 自动生效"零代码 |
| **NPC 间交互** | **正面弱** | Instance 创建路径单一化后，social 组件 opt-in 语义清晰；编排层（scheduler/group_manager）对 5 组件的访问保留现状（非技术债，见 R19）|

### 扩展轴 2 的具体增量证明

之前扩展轴 2 的扩展成本：
1. 改 ADMIN UI（配新 NPC）
2. 本地改一份 JSON（组件化 shape）
3. 确保两份 schema 一致

本 spec 完成后：
1. 改 ADMIN UI（配新 NPC）
2. cmd/sync 拉下来 = 成
3. 测试跟着加一个 npctest 调用（可选）

### 扩展轴 3 的局限

R19 明示：scheduler / group_manager / gateway 对 5 组件的 12 处直接访问是**编排层协调职责**，不是违反。但加第 11 个组件（如未来的 FactionComponent）仍需要改 scheduler.go 增加 7 行左右代码——这不是本 spec 的目标，属于架构演进题。

## 9. 依赖方向

```
cmd/server/main.go
  │
  ├── internal/config/           (Source 接口)
  │     ├── http_source.go
  │     ├── json_source.go
  │     └── mongo_source.go
  │
  ├── internal/runtime/
  │     ├── npc/
  │     │   ├── admin_template.go    ← 本 spec 核心改造对象
  │     │   ├── instance.go
  │     │   ├── cascade.go           ← R18 新增
  │     │   └── npctest/             ← R21 新增（测试专用）
  │     │       └── helpers.go
  │     ├── zone/zone.go
  │     ├── component/               (13 factory，保留全部 10 个 component type)
  │     ├── scheduler.go
  │     ├── social/group_manager.go
  │     ├── decision/
  │     └── event/
  │
  ├── internal/gateway/handler.go
  │
  └── internal/core/                 (blackboard/fsm/bt/rule，不被本 spec 改动)
```

**关键依赖**：
- `admin_template.go` → `internal/runtime/component/` + `internal/config/` + `internal/core/{fsm,bt,blackboard}`（单向向下，合规）
- `cascade.go` → `internal/runtime/npc/registry` + `internal/core/blackboard`（读 BB）
- `npctest/helpers.go` → `internal/runtime/npc/` + `internal/runtime/component/`（子包依赖父包，Go 允许）
- **生产代码 MUST NOT** import `internal/runtime/npc/npctest/`（R21 红线）

**红线合规**：core/ 不 import runtime/（未触及）；runtime/ 不 import gateway/（未触及）；npctest 子包不被生产代码 import（R21）。

## 10. 并发安全

| 场景 | 并发状态 | 保护机制 |
|---|---|---|
| `NewInstanceFromADMIN` 创建 Instance | main.go 启动期串行 | 无需锁 |
| `cascade.ValidateCascade` 遍历 Registry | 填充完成后、Tick 启动前 | Registry 尚无并发写入 |
| `spawnOrphanADMINTemplates` | main.go 启动期 | 无 |
| Scheduler.Tick 消费 Instance | 并发 tick | Instance 内组件各自管理并发（现有 social/GroupManager 有 RWMutex；其他组件目前单 tick 读写，无跨 tick 并发）|
| `npctest.NewInstanceWithExtras` | 测试 goroutine | 单测 t.Parallel() 需各自构造独立 Instance |

**新增并发考量**：无。所有新代码（cascade / npctest / translation 升级）都在 main.go 启动串行期执行，不引入新的共享状态。

## 11. 配置变更

### 11.1 JSON Schema 变更

- 无新 schema 引入
- 所有 NPC 配置采用 `{template_ref, fields, behavior}` ADMIN shape（已在 api-contract.md v1.1 固化）

### 11.2 新增字段（由 ADMIN seed 提供，本 spec 仅消费）

- `fields.enable_memory` / `enable_emotion` / `enable_needs` / `enable_personality` / `enable_social` (5 × bool, default false)
- ADMIN 0aa77b2 已 seed 完毕，本 spec 不做 seed 工作

### 11.3 删除字段 / 配置

见 §4.1 清单：配置层删除 6 个 fsm/bt 文件 + 1 个 npc_templates 文件 + `configs/npc_types/` 目录。

### 11.4 `cmd/sync` 行为影响

- 端点已对齐（PR #20）
- sync 一次后 `git status` 下的文件列表：新增 5 个 ADMIN NPC + 新增 3 个 event_types + 新增 2 个 FSM + 新增 5 个 BT（butterfly_01 是本地 rewrite, guard.json / guard/patrol 是 ADMIN 覆盖本地）
- 预期 `git add configs/` 后无冲突提交

## 12. 测试策略

### 12.1 测试迁移 规模（23 处）

| 测试文件 | 引用数 | 迁移策略 |
|---|---|---|
| `decision_integration_test.go` | 4（wolf_common）| npctest inline 构造 `fsm_combat_basic` wolf |
| `memory_integration_test.go` | 3（wolf_common）| 同上 + extras 注入 memory/emotion |
| `perception_integration_test.go` | 3（wolf_common）| 同上（无 extras）|
| `social_integration_test.go` | 1（wolf_common）| npctest + extras 注入 social |
| `movement_integration_test.go` | 1（wolf_common）+ 1（butterfly_01 file load）| wolf 走 npctest；butterfly 走新 ADMIN shape file |
| `component_integration_test.go` | 2（wolf_common）+ 2（butterfly_01）| 同上 |
| `zone_integration_test.go` | 3（butterfly_01） + 1（meadow 加载） | butterfly 走新 file；meadow 加载逻辑改读 ADMIN shape |
| `benchmark_test.go` | 1 + 1 | 分别按 wolf/butterfly 路径改 |

### 12.2 新增测试

- `internal/runtime/npc/cascade_test.go` — R18 级联校验单测（violations 为空/非空两路径）
- `internal/runtime/npc/npctest/helpers_test.go` — npctest 自身行为单测（extras 覆盖、opt-in 绕过）
- `internal/runtime/npc/admin_template_test.go` 扩展 — 5 × opt-in 组件创建验证 + 默认组件创建验证

### 12.3 E2E

- R15：`docker compose up --build` 跑 ≥30s 无 ERROR，9 NPC 全部出现在 `/debug/npcs` 查询
- R16：停 ADMIN 进程（保留 server 进程），重启 server，JSONSource 降级路径跑通，9 NPC 仍全部 spawn

### 12.4 红线相关测试验证

- 生产代码 import `npctest` 子包：`grep -r "npctest" internal/runtime/ internal/gateway/ --include="*.go" --exclude="*_test.go"` 必须为空（Makefile 可增这条 lint，不强制本 spec 实现）
- cmd/sync 后 `go test ./...` 一次通过（不需 revert configs/）

### 12.5 组件缺席场景覆盖

对应 R20 文档化契约，每个组件至少一个缺席集成测试：

| 缺席场景 | 覆盖测试 | 锚定行为 |
|---|---|---|
| memory 缺席 | 现有 `memory_integration_test` fixture 改造版本 | fear 衰减到 0 |
| emotion 缺席（memory 开）| 新增单测 | decision.EmotionValue = 0 |
| needs 缺席 | 现有 `decision_integration` 改造 | calcNeedUrgency=0 |
| personality 缺席 | 现有 `decision_integration` 改造 | 用 DefaultWeights |
| social 缺席 | 现有 `social_integration_test.go::TestGroupManager_NoSocialComponent` | GroupManager 不可见 |

---

## 13. Phase 2 发现的 requirements.md 偏差与修正建议

Phase 2 实施阶段 grep 验证发现 Phase 1 requirements.md 有两条过激：

### 13.1 R4 过激——NPCTypeConfig API 不可删

**原 R4**：
> 删除 `NPCTypeConfig`（v2）及所有加载器（`Source.LoadNPCTypeConfig` 接口方法、JSONSource / MongoSource / HTTPSource 的对应实现）

**发现**：grep 显示多处消费者：
- `internal/experiment/modes/hybrid.go:25` — 毕设核心 Hybrid 模式构造器
- `internal/runtime/{benchmark,integration}_test.go` — 基准 + 集成测试 fixture
- `internal/core/{integration,verify_attack}_test.go` — core 层集成测试
- `internal/config/{json,http,mongo}_source_test.go` — 3 个 Source 实现的契约测试

**修正 R4**：
> 删除 `NPCTypeConfig`（v2）在**生产 spawn 路径**（`zone/zone.go` / `gateway/handler.go` / `main.go`）的所有调用点。**保留** `NPCTypeConfig` struct、`ParseNPCTypeConfig` 函数、`Source.LoadNPCTypeConfig` 接口方法、3 个 Source 实现——供实验层（`internal/experiment/`）与测试层（integration/benchmark/source contract test）消费。
>
> **边界**：NPCTypeConfig 是"本地 V2 fixture API"，不作为 production NPC 创建路径；R1 "NewInstanceFromADMIN 是唯一生产入口"语义不变。

### 13.2 R10 过激——configs/npc_types/ 不整删

**原 R10**：
> 删除 `configs/npc_types/` 目录

**发现**（两轮 grep）：
- `civilian.json` 被实验层 + 多处 Source 测试 + integration 测试 + core 测试消费
- `police.json` 被 `test/e2e/extension_test.go` 扩展轴 2 演示消费（第一轮遗漏，第二轮补查发现）
- `guard.json`（V2）唯一 Go 消费者是 `compat_test.go`，随 R3 同步删除后无消费者

**修正 R10**：
> 删除 `configs/npc_types/guard.json`（compat_test.go 同步删除，无其他 Go 消费者）+ `configs/bt_trees/guard/{alert,defend}.json` + `configs/fsm/police.json` 无消费者则删（确认 grep）。**保留** `configs/npc_types/{civilian,police}.json` + 各自 FSM + 各自 BT 目录——civilian 是实验层依赖，police 是 extension_test.go 扩展轴 2 演示依赖。Phase 2 修正记录：原草案误判 civilian/police 为孤儿；civilian 错过一轮 review 才纠正，police 同模式补救。
>
> **目录保留**：civilian + police 存在故 `configs/npc_types/` 目录保留。未来若实验层 + e2e 演示彻底迁移到 ADMIN shape（独立 spec），再整删。

**注意**：`configs/fsm/police.json` 实际上是 extension_test.go 的依赖（police FSM 含 Engage 状态，是扩展轴 2 的核心），所以也保留。design.md §4.1 表格已修正。

### 13.3 其他 R 无需改动

R1/R2/R3/R5-R9/R11-R24 经 grep 验证合规，无偏差。特别是：
- **R2**（删 TemplateConfig + NewInstanceFromTemplate）：组件化 shape 无实验层消费者，可删
- **R3**（删 compat.go）：compat.go 是组件化 path parser 分发，hybrid.go 直接用 V2 `ParseNPCTypeConfig` 不经 compat.go，可删

### 13.4 修正程序

Phase 2 approve 的同时，requirements.md 修正 R4 + R10 作为**同一提交**合入。流程不回退 Phase 1，**原因**：
- 偏差来自 Phase 1 时对实验层消费面的 grep 漏扫，非需求本质错误
- R4/R10 修正边界清晰（只限定"生产 path"），不影响其他 R
- 单独跑 Phase 1 → Phase 2 循环无额外信息收益
