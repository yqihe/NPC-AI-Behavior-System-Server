# 游戏服务端 ↔ ADMIN API 契约

**ADMIN 为权威源**。本文件定义服务端启动时通过 HTTP 拉取 ADMIN 配置的导出接口形态；服务端侧 `internal/runtime/npc/admin_template.go` 等反序列化逻辑反向依赖此 schema。

**同步方式：人工同步**（毕设体量不引入 git submodule / CI mirror）：
- ADMIN 仓改本文件 → commit + push
- 契约变更必须在 commit message 显式标注"影响服务端"
- 服务端仓发 PR 时 description 引用 ADMIN 对应 commit hash 作为契约版本锚
- 若 ADMIN 改契约未通知服务端，由 `docs/development/standards/red-lines/general.md` "禁止协作失序"红线兜底

**当前版本**：v1.1（2026-04-19，新增组件 opt-in 依赖矩阵；对齐服务端仓 spec `external-contract-server-adaptation` R17-R21）

**当前仅覆盖**：`GET /api/configs/npc_templates`。其他导出接口（event_types / fsm_configs / bt_trees / regions）按实际契约演进再补。

---

## GET /api/configs/npc_templates

**用途**：服务端启动或 `cmd/sync` 拉取 NPC 模板配置到 configs/。

**调用方**：服务端 `internal/runtime/npc/admin_template.go`——**唯一的反序列化落点**，本 schema 的反向依赖源。

**ADMIN 实现位置**：`backend/internal/service/npc_service.go` 的 `assembleExportItem`（从 `npcs.fields` JSON 组装 `map[name]value`），`backend/internal/model/npc.go` 的 `NPCExportItem` / `NPCExportConfig` / `NPCExportBehavior`。

**返回状态**：始终 `200 OK`（失败时进通用错误响应，超出本契约范围）。

### Schema

```json
{
  "items": [
    {
      "name": "wolf_common",
      "config": {
        "template_ref": "warrior_base",
        "fields": {
          "aggression": "aggressive",
          "attack_power": 18.5,
          "defense": 8.0,
          "is_boss": false,
          "loot_table": "loot_wolf_common",
          "max_hp": 120,
          "move_speed": 5.5,
          "perception_range": 20.0
        },
        "behavior": {
          "fsm_ref": "fsm_combat_basic",
          "bt_refs": {
            "attack": "bt/combat/attack",
            "chase": "bt/combat/chase",
            "idle": "bt/combat/idle",
            "patrol": "bt/combat/patrol"
          }
        }
      }
    }
  ]
}
```

### 字段说明

| 路径 | 类型 | 语义 | 空值 / 可选 |
|---|---|---|---|
| `items` | array | NPC 列表，顺序无保证（服务端不得依赖序） | 空列表合法（无 NPC）|
| `items[].name` | string | NPC 唯一标识，小写 + 下划线，`^[a-z][a-z0-9_]*$` | 必填非空 |
| `items[].config` | object | 配置体 | 必填 |
| `items[].config.template_ref` | string | 模板标识（仅做名字引用，**服务端视为不透明字符串**，不要求预先声明）| 必填非空 |
| `items[].config.fields` | object<string, any> | 字段名 → 值映射；value 保留 JSON 原类型（number/string/bool/null）| 必填；可为 `{}`（如纯占位模板）|
| `items[].config.behavior` | object | 行为配置容器 | 必填（对象本身存在），内部键可能被 `omitempty` 省略 |
| `items[].config.behavior.fsm_ref` | string | FSM 配置 name；服务端按此名到 fsm_configs 集合查 | **可选**：空串时 JSON 中**整键省略** |
| `items[].config.behavior.bt_refs` | object<string, string> | FSM 状态 name → 行为树 name；value 是已启用的行为树标识 | **可选**：空 map 时 JSON 中**整键省略** |

**key 顺序**：`items[].config.fields` 和 `behavior.bt_refs` 的 key 由 Go `encoding/json` 按字典序输出（Go 1.12+ 稳定行为）；服务端解析时不要依赖业务顺序，应按 key 读。

**value 类型约定**：
- number 保留浮点形态（`8.0` 不归一化为 `8`）。ADMIN 用 `json.RawMessage` 存 `npcs.fields[].value` 字节，MySQL JSON 列不改写数值形态。服务端若做精确 diff 对比 snapshot 需注意此点。
- 枚举类字段（如 `aggression`）value 为 string；ADMIN 侧 constraint_schema 约束合法枚举值，但**服务端解析时不做再次校验**（ADMIN 为权威）。

### 双边契约锚定

**服务端 admin_template.go 反向依赖此 schema**。任何以下改动都属于 breaking change，必须先改本文件再改代码：
- `items[].name` / `items[].config` 层级结构变动
- `template_ref` 语义变化（比如从"字符串"变成"对象"）
- `fields` 从 `object<string, any>` 变为 `array<object>`
- `behavior.fsm_ref` / `behavior.bt_refs` 的 omitempty 语义切换（必填化或删除）

**非 breaking change**（免通知）：新增 `items[].config.*` 下的可选字段（带 omitempty）、字段值类型在兼容子集内调整（如 int↔float 表示的同数值）。

### 组件 opt-in 依赖矩阵（v1.1 新增）

#### 5 个 opt-in bool 字段

ADMIN `items[].config.fields` 中**约定的 5 个 bool 字段**控制服务端侧能力组件实例化：

| 字段名 | 语义 | default_value | absent 时 |
|---|---|---|---|
| `enable_memory` | 记忆组件：写入威胁记忆 → 驱动 emotion | `false`（必填）| 等价 false |
| `enable_emotion` | 情绪组件：读记忆累积 fear → 驱动 decision | `false`（必填）| 等价 false |
| `enable_needs` | 需求组件：计算 lowest need → 驱动 decision | `false`（必填）| 等价 false |
| `enable_personality` | 性格组件：提供 decision weights 覆盖默认值 | `false`（必填）| 等价 false |
| `enable_social` | 社交组件：group/follower/leader 机制 | `false`（必填）| 等价 false |

**absent ≡ false 语义锁定**：字段缺失等价显式 `false`，避免"未声明 vs 显式关闭"歧义。

**ADMIN seed 强制要求**：5 个 bool field 的 `properties.default_value` 必须显式设为 `false`，确保新建 NPC 携带 false 而非 null。若存 null，导出时字段会变为 `null` 而非 `false`，服务端解析歧义。

#### 级联依赖（硬约束）

| 如启用 | 必须同时启用 | 校验位置 | 违规后果 |
|---|---|---|---|
| `enable_emotion` | `enable_memory` | 服务端启动 Registry 填充阶段，**逐 NPC** 校验 | **Fatal**：打印违规 NPC name 列表 + ADMIN UI 修正路径，**不跳过违规 NPC、不部分启动** |

**根因**：`emotion.Tick()` 读 `KeyMemoryThreatValue`（由 `memory.Tick()` 写入）；无 memory 则 emotion.fear 永不累积 → emotion 独立开启无意义。

**ADMIN 侧已知缺陷**：ADMIN 字段系统当前不支持跨字段联动校验（见 `deferred-features.md`），运营侧可以保存 `enable_emotion=true, enable_memory=false` 的非法组合。服务端兜底 fatal 校验承担此约束。

#### 其他组件的独立性

| 组合 | 级别 | 说明 |
|---|---|---|
| `enable_needs` 单开 | ⚠️ 弱耦合警告 | `personality.weights.Needs` 失去乘数意义，但不违法 |
| `enable_personality` 单开 | ✅ 合法 | 无 BB 链路，decision 用自定义 weights 但 NeedUrgency=0 |
| `enable_social` 单开 | ✅ 合法 | 完全独立，只影响 group 可见性 |

#### 组件缺席时的系统行为

服务端必须保证**任意组合缺席下 Tick 不崩溃**。当前实现已合规，契约要求不退化：

| 缺席组件 | 直接效果 | 可观测二阶效果 |
|---|---|---|
| memory | `KeyMemoryThreatValue` unset | emotion.fear 衰减到 0（若 emotion 开） |
| emotion | `KeyEmotionDominant/Val` unset | `scheduler.buildDecisionInput.EmotionValue=0` |
| needs | `KeyNeedLowest/Val` unset | `scheduler.calcNeedUrgency=0` → `decision.NeedUrgency=0` |
| personality | 无 BB 影响 | `decision.Weights` 用 `decision.DefaultWeights`（Threat/Needs/Emotion 各为 1）|
| social | `KeyGroupID/SocialRole` unset | `GroupManager` 对该 NPC 不可见（逐 NPC 跳过，不影响其他 NPC） |

**服务端实现准入模式**：所有组件访问点通过 `npc.GetComponent[T](inst, name)` 的 `(T, ok)` 返回值决策。**禁止**裸 nil 访问、类型断言 panic、或整体禁用系统。当前生产代码面 12 处访问全部合规（`scheduler.go` 7 处 + `group_manager.go` 4 处 + `gateway/handler.go` 1 处）。

#### 软/硬依赖契约

- **软依赖（允许）**：组件 X 读取组件 Y 写入的 BB key（如 emotion 读 `KeyMemoryThreatValue`）。Y 缺席时 X 必须降级到默认值，不得阻塞 tick 或 panic

- **硬依赖（禁止）**：**组件代码内部**出现 `GetComponent[Y]` 访问其他组件类型 Y。例如 `emotion.Tick()` 里调用 `GetComponent[*MemoryComponent](inst, "memory")` 属违规

- **编排层例外**：scheduler / gateway / group_manager 等**编排器**对组件的直接访问是组件化架构的合法协调机制——编排层的核心职责就是把组件粘合起来。此类访问**不计入硬依赖**，也不构成技术债

- **当前全部合规**：scheduler.go（7）+ group_manager.go（4）+ gateway/handler.go（1）共 **12 处组件访问全部位于编排层**，无组件间硬依赖违规。服务端代码已遵循"编排层协调组件 / 组件间只通过 BB 交互"的双层架构原则

### 已知数据噪声

#### `guard_basic.fields.hp`

- **现象**：`items[].name="guard_basic"` 的 `config.fields` 返回 `{"hp": 100}` 而非更规范的 `{"max_hp": 100}`
- **原因**：T9 建字段时没看到已存在的 `max_hp` 造成重复（属 ADMIN 侧数据治理遗留，非服务端 bug）
- **当前策略**：ADMIN 把 `hp` seed 为孤儿字段（`enabled=0` + 不进任何模板 `fields` 数组），仅被 `guard_basic` NPC 的字段快照引用；UI 层默认不暴露此字段给策划选择
- **服务端影响**：`SetDynamic` 把 `hp` 写入 BB，但无任何 BT 节点消费，**实际对行为无影响**
- **清除时机**：ADMIN 41008 硬约束（模板被 NPC 引用时字段不可编辑）解封后，一次性把 `guard_basic` 的字段改为 `max_hp`，同时删除 fields 表 hp 行
- **参考**：memory `project_guard_basic_hp_deferred.md`；本 spec design.md §2 OQ3 方案 A

---

## 待补充

以下导出接口待后续契约补齐（当前服务端已能工作，schema 以服务端实际消费为准）：

- `GET /api/configs/event_types`
- `GET /api/configs/fsm_configs`
- `GET /api/configs/bt_trees`
- `GET /api/configs/regions`

新增段落时按本文件"npc_templates"段落格式：Schema + 字段说明表 + 双边契约锚定 + 已知数据噪声（如有）。
