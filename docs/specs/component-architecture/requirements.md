# component-architecture 需求分析

## 动机

v2 的 `Instance` 是固定结构体——`ID` + `TypeName` + `Position` + `BB` + `FSM` + `BTrees` + `Perception`。所有 NPC 无论复杂度，都携带相同的字段集。这带来三个问题：

1. **加新能力必须改结构体**：要给 NPC 加"需求系统"（需求 3）或"记忆"（需求 4），必须修改 `Instance` struct、`NewInstance` 工厂、`Scheduler.Tick` 逻辑——每加一个能力改 3+ 个文件，且影响所有 NPC。
2. **无法表达复杂度差异**：一个只需位置和移动的装饰型蝴蝶，和一个有感知、决策、情绪、记忆的完整 AI 狼，被迫使用相同数据结构。缺失的字段填零值或 nil，Scheduler 无法区分该不该跑感知/决策。
3. **Scheduler 强耦合**：`Scheduler.Tick` 硬编码了"感知→决策→FSM→BT"管线（第 52-69 行），默认所有 NPC 都走全流程。没有 behavior 的装饰型 NPC 不应该走 AI 管线，但当前无法跳过。

不做这个，后续需求 2-8 每一个都会遇到"改 Instance + 改 Scheduler + 改 Handler"的重复痛苦，且互相冲突合并。

## 优先级

**最高**。需求 1，是需求 2-8 全部的前置依赖。需求 0（Schema 契约）已完成，本需求可以开始。

## 预期效果

### 场景 1：组件化配置创建不同复杂度 NPC

```json
// simple 级蝴蝶：只有身份+位置+移动，无 AI
{
  "name": "butterfly_01",
  "preset": "simple",
  "components": {
    "identity": {"name": "蝴蝶", "model_id": "butterfly_blue", "tags": ["ambient"]},
    "position": {"x": 100, "y": 5, "z": 200, "orientation": 0, "zone_id": "meadow"},
    "movement": {"move_type": "wander", "move_speed": 1.5, "wander_radius": 20}
  }
}

// reactive 级灰狼：有完整 AI 管线
{
  "name": "wolf_01",
  "preset": "reactive",
  "components": {
    "identity": {"name": "灰狼", "model_id": "wolf_gray", "tags": ["predator"]},
    "position": {"x": 300, "y": 0, "z": 400, "orientation": 90, "zone_id": "forest"},
    "behavior": {"fsm_ref": "wolf", "bt_refs": {"Idle": "wolf/idle", "Chase": "wolf/chase"}},
    "perception": {"visual_range": 150, "auditory_range": 300, "attention_capacity": 3},
    "movement": {"move_type": "patrol", "move_speed": 4.0, "patrol_waypoints": [{"x": 300, "z": 400}, {"x": 350, "z": 450}]},
    "personality": {"personality_type": "aggressive", "decision_weights": {"threat": 0.7, "needs": 0.2, "emotion": 0.1}}
  }
}
```

系统根据 `components` 中包含的组件决定 NPC 拥有哪些能力。

### 场景 2：Scheduler 按组件有无分支执行

蝴蝶（无 behavior/perception）：
- Tick 不走感知过滤（无 perception）
- Tick 不走决策中心（无 behavior）
- Tick 不走 FSM/BT（无 behavior）
- 只 Tick movement 组件（更新 BB 中的 `move_state`）

灰狼（有 behavior+perception）：
- 完整 AI 管线：perception → decision → FSM → BT
- 额外 Tick movement 组件

### 场景 3：v2 旧配置无缝兼容

`configs/npc_types/civilian.json`（v2 格式）在新架构下自动检测并转换为组件化结构。现有 6 个集成测试场景和 e2e 测试全部通过，无需修改配置文件。

### 场景 4：加新组件不改现有代码

后续需求 4 实现 memory 组件的深入逻辑时：
1. 在 `internal/runtime/component/` 中实现 `MemoryComponent` 的 `Tick` 方法
2. 不修改 `Instance`、`Scheduler`、其他组件的代码

### 场景 5：Gateway 和广播适配

- `spawn_npc` 消息兼容新旧两种配置格式
- `query_npc` 响应中，无 behavior 组件的 NPC 的 `fsm_state` 返回空字符串
- `world_snapshot` 广播中，NPC 的 `x`/`z` 从 position 组件读取

## 依赖分析

- **依赖**：
  - 需求 0 Schema 契约（已完成）：组件字段定义
  - core 层（FSM/BT/BB/Rule）：已完成，不修改
  - runtime 层（Event/Decision/Perception）：已完成，小幅适配

- **被依赖**：
  - 需求 2 感知深化
  - 需求 3 决策深化
  - 需求 4 记忆系统
  - 需求 5 移动系统
  - 需求 6 社交系统
  - 需求 7 区域系统
  - 需求 8 可观测性

## 改动范围

| 包 | 变更类型 | 文件数 | 说明 |
|----|---------|--------|------|
| `internal/runtime/component/` | **新增** | 5-6 | Component/Tickable 接口、10 个组件 struct、组件注册表 |
| `internal/runtime/npc/` | **重构** | 2-3 | Instance 改为组件容器、NewInstance 改为 NewInstanceFromTemplate、旧格式兼容 |
| `internal/runtime/scheduler.go` | **修改** | 1 | Tick 逻辑按组件有无分支 |
| `internal/config/` | **扩展** | 2-3 | Source 接口新增 LoadNPCTemplate、JSONSource/HTTPSource 实现 |
| `internal/gateway/handler.go` | **修改** | 1 | spawn_npc 适配新配置格式 |
| `cmd/server/main.go` | **修改** | 1 | broadcastLoop 和 buildSnapshot 适配 |
| `internal/core/blackboard/keys.go` | **修改** | 1 | 新增 8 个 BB Key |
| `configs/npc_templates/` | **新增** | 2-3 | 示例模板（蝴蝶/灰狼） |
| 测试文件 | **新增+修改** | 5-7 | 组件测试、兼容性测试、集成测试、benchmark |

预估 20-28 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **中性** | 事件系统不变 |
| 加 NPC 类型 | **强化** | 从"加 JSON 文件"升级为"选组件组合"，更灵活且无需的组件不占资源 |
| NPC 间交互 | **奠基** | social 组件数据结构就位，为需求 6 铺路 |

新增第四扩展轴：**加 NPC 能力** → 实现 Component 接口 + 注册工厂，不改现有代码。

## 验收标准

### 组件框架

- **R1**：定义 `Component` 接口（含 `Name() string`）和 `Tickable` 接口（含 `Tick(bb, dt)`），所有 10 个组件实现 `Component`
- **R2**：定义组件注册表 `Registry`，支持 `Register(name, factory)` 和 `Create(name, rawJSON) (Component, error)`
- **R3**：NPC `Instance` 持有 `map[string]Component`，通过 `GetComponent[T](inst, name) (T, bool)` 类型安全获取

### 10 个组件数据结构

- **R4**：10 个组件的 Go struct 字段名和类型与需求 0 的 Schema 精确一致
- **R5**：`BehaviorComponent` 持有运行时 `*fsm.FSM` 和 `map[string]bt.Node`（创建时从配置构建）
- **R6**：`MovementComponent`、`NeedsComponent`、`EmotionComponent`、`MemoryComponent` 实现 `Tickable`；其余 6 个不实现
- **R7**：Tickable 组件的 `Tick` 做最小实现：needs 按 decay_rate 衰减 current 值并写 BB（need_lowest/need_lowest_val），emotion 按 decay_rate 衰减 value 并写 BB（emotion_dominant/emotion_dominant_val），movement 写 BB（move_state），memory 清理过期条目并写 BB（memory_count）

### 配置格式

- **R8**：NPC 模板配置格式为 `{"name", "preset", "components": {"component_name": {...}}}`
- **R9**：配置加载时校验 identity 和 position 必须存在，缺失报错
- **R10**：v2 旧格式（含 `type_name` 字段）自动检测并转换为组件化结构

### Scheduler 适配

- **R11**：无 perception 组件的 NPC 跳过感知过滤
- **R12**：无 behavior 组件的 NPC 跳过决策中心和 FSM/BT
- **R13**：所有 Tickable 组件在每次 Tick 中执行

### Config Source

- **R14**：Source 接口新增 `LoadNPCTemplate(name string) ([]byte, error)`
- **R15**：JSONSource 从 `configs/npc_templates/` 加载
- **R16**：HTTPSource 从 `/api/v1/npc-templates/export` 加载

### Gateway 适配

- **R17**：spawn_npc handler 兼容新旧两种配置格式
- **R18**：query_npc 和 world_snapshot 从组件读取数据（position 组件取坐标，behavior 组件取 FSM 状态）

### BB Key 注册

- **R19**：在 keys.go 新增 8 个 Key：need_lowest, need_lowest_val, emotion_dominant, emotion_dominant_val, memory_count, group_id, social_role, move_state

### 测试

- **R20**：组件注册表单元测试：注册/创建/未知组件报错
- **R21**：组件化 NPC 创建单元测试：从 JSON → Instance，覆盖 simple/reactive 两种预设
- **R22**：v2 兼容性测试：civilian/guard/police 旧配置加载后行为与 v2 一致
- **R23**：Scheduler 集成测试：simple NPC 不走 AI 管线，reactive NPC 走完整管线
- **R24**：现有 6 个集成测试场景和 e2e 测试全部通过
- **R25**：Benchmark：只有 identity+position+movement 的 NPC 单 Tick < 1μs；全组件 NPC 单 Tick < 100μs

## 不做什么

- **不做组件深入逻辑**：needs/emotion/memory/movement/social/personality 只做数据结构 + 最小 Tick，深入实现在需求 2-6
- **不做区域系统**：position 有 zone_id 字段，但区域管理在需求 7
- **不做 NPC 间交互**：social 定义群组关系，但群体行为在需求 6
- **不做真实移动**：movement Tick 只写 BB move_state，实际位置更新在需求 5
- **不删除旧代码路径**：保留 `LoadNPCTypeConfig` 和 `configs/npc_types/`，兼容期后清理
- **不做配置热更新**：配置仍启动时加载
