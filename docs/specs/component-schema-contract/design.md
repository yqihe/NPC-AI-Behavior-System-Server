# component-schema-contract 设计方案

## 方案描述

### 文件结构

```
configs/schemas/
├── components/          # 10 个组件 Schema
│   ├── identity.json
│   ├── position.json
│   ├── behavior.json
│   ├── perception.json
│   ├── movement.json
│   ├── personality.json
│   ├── needs.json
│   ├── emotion.json
│   ├── memory.json
│   └── social.json
├── presets/              # 4 个预设定义
│   ├── simple.json
│   ├── reactive.json
│   ├── autonomous.json
│   └── social.json
├── node_types/           # 8 个 BT 节点类型 Schema
│   ├── sequence.json
│   ├── selector.json
│   ├── parallel.json
│   ├── inverter.json
│   ├── check_bb_float.json
│   ├── check_bb_string.json
│   ├── set_bb_value.json
│   └── stub_action.json
├── condition_types/      # 2 个 FSM 条件类型 Schema
│   ├── leaf.json
│   └── composite.json
└── region.json           # 区域数据结构
```

### 组件 Schema 格式

每个组件文件统一信封：

```json
{
  "component": "<英文标识>",
  "display_name": "<中文显示名>",
  "blackboard_keys": ["<该组件读写的 BB Key 列表>"],
  "schema": { "<JSON Schema Draft 7>" }
}
```

- `component`：ADMIN 用于匹配组件的唯一标识
- `display_name`：ADMIN 表单中展示的中文名
- `blackboard_keys`：ADMIN BT 编辑器的 Key 下拉框数据源。已有 Key 名称与 `keys.go` 精确一致，新增 Key 按 `组件缩写_语义` 命名避免冲突
- `schema`：标准 JSON Schema Draft 7，ADMIN 直接传入表单渲染库

### 10 个组件的 Schema 设计

#### identity — 身份（所有 NPC 必有）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 游戏内显示名称 |
| model_id | string | 是 | 客户端模型资源 ID |
| tags | string[] | 否 | 分类标签，默认 `[]` |

BB Keys：`npc_type`

#### position — 位置（所有 NPC 必有）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| x | number | 是 | 世界坐标 X |
| y | number | 否 | 高度，默认 0 |
| z | number | 是 | 世界坐标 Z |
| orientation | number | 否 | 朝向角度 0-360，默认 0 |
| zone_id | string | 否 | 所属区域 ID |

BB Keys：`npc_pos_x`, `npc_pos_z`

#### behavior — AI 行为（FSM + BT）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| fsm_ref | string | 是 | FSM 配置名（对应 `configs/fsm/<name>.json`） |
| bt_refs | object | 是 | 状态名 → BT 树名映射，至少 1 项 |

BB Keys：`fsm_state`, `current_action`, `threat_level`, `threat_source`, `threat_expire_at`, `last_event_type`, `current_time`

#### perception — 感知

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| visual_range | number | 是 | 视觉距离（米），≥0 |
| auditory_range | number | 是 | 听觉距离（米），≥0 |
| attention_capacity | integer | 否 | 最大同时关注事件数，默认 5，≥1 |

BB Keys：无（感知结果通过决策中心写入 BB）

#### movement — 移动

| 字段 | 类型 | 必填 | 条件 | 说明 |
|------|------|------|------|------|
| move_type | enum | 是 | — | wander / patrol / follow |
| move_speed | number | 是 | — | 米/秒，≥0 |
| wander_radius | number | 条件必填 | move_type=wander | 游荡半径（米） |
| patrol_waypoints | array[{x,z}] | 条件必填 | move_type=patrol | 巡逻路点列表 |

BB Keys：`move_state`, `npc_pos_x`, `npc_pos_z`

#### personality — 性格

| 字段 | 类型 | 必填 | 条件 | 说明 |
|------|------|------|------|------|
| personality_type | enum | 是 | — | timid / aggressive / docile / curious |
| decision_weights | object | 是 | — | `{threat, needs, emotion}` 三项权重 |
| aggro_range | number | 条件必填 | type=aggressive | 主动攻击距离（米） |
| flee_threshold | number | 条件必填 | type=timid | 逃跑威胁阈值 0-100 |

BB Keys：无（性格通过决策中心权重间接影响 BB）

#### needs — 需求

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| need_types | array | 是 | 需求列表，至少 1 项 |

need_types 每项：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 需求名（hunger / fatigue / curiosity 等） |
| current | number | 否 | 当前值，默认等于 max |
| max | number | 是 | 最大值 |
| decay_rate | number | 是 | 每秒衰减量 |

BB Keys：`need_lowest`(string), `need_lowest_val`(float64)

#### emotion — 情绪

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| emotion_states | array | 是 | 情绪列表，至少 1 项 |

emotion_states 每项：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 情绪名（fear / anger / calm 等） |
| value | number | 否 | 当前值，默认 0 |
| accumulate_rate | number | 是 | 受刺激时每秒累积量 |
| decay_rate | number | 是 | 无刺激时每秒衰减量 |

BB Keys：`emotion_dominant`(string), `emotion_dominant_val`(float64)

#### memory — 记忆

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| capacity | integer | 是 | 最大记忆条目数，≥1 |
| memory_types | string[] | 是 | 支持的记忆类型（threat / location / social） |
| decay_time | number | 是 | 默认记忆 TTL（秒） |

BB Keys：`memory_count`(int64)

#### social — 社交

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| group_id | string | 否 | 群组 ID |
| faction | string | 否 | 阵营名 |
| role | enum | 否 | leader / follower |
| follow_target | string | 否 | 跟随目标 NPC ID（仅 follower） |

BB Keys：`group_id`(string), `social_role`(string)

### 4 个预设定义

| 预设 | 定位 | required | default | optional |
|------|------|----------|---------|----------|
| simple | 环境装饰 | identity, position | movement | behavior, perception, personality, needs, emotion, memory, social |
| reactive | 感知反应 | identity, position | behavior, perception, movement, personality | needs, emotion, memory, social |
| autonomous | 自主需求 | identity, position | behavior, perception, movement, personality, needs, emotion, memory | social |
| social | 完整社交 AI | identity, position | behavior, perception, movement, personality, needs, emotion, memory, social | — |

### 8 个 BT 节点类型 Schema

精确对照 `bt.DefaultRegistry()` 和各 factory 函数：

| node_type | category | params |
|-----------|----------|--------|
| sequence | composite | 无参数（children 由树结构定义） |
| selector | composite | 无参数 |
| parallel | composite | `policy`: enum(require_all / require_one)，默认 require_all |
| inverter | decorator | 无参数（child 由树结构定义） |
| check_bb_float | leaf | `key`: string(必填), `op`: enum(6 种，必填), `value`: number(必填) |
| check_bb_string | leaf | `key`: string(必填), `op`: enum(== / !=，必填), `value`: string(必填) |
| set_bb_value | leaf | `key`: string(必填), `value`: any(必填) |
| stub_action | leaf | `name`: string(必填), `result`: enum(success / failure / running)，默认 success |

注意：`check_bb_float` 的 op 是 6 种（== != > >= < <=），不含 `in`。`check_bb_string` 的 op 只有 2 种（== !=）。这与 FSM 条件的 7 种 op 不同，来自 Go 代码 `compareFloat64` 和 `compareString` 函数的实际实现。

### 2 个 FSM 条件类型 Schema

**leaf 条件**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | string | 是 | Blackboard Key 名 |
| op | enum | 是 | == / != / > / >= / < / <= / in（7 种） |
| value | any | 二选一 | 字面量比较值 |
| ref_key | string | 二选一 | 引用另一个 BB Key |

**composite 条件**：

| 字段 | 类型 | 说明 |
|------|------|------|
| and | array[condition] | 所有子条件为 true 时为 true |
| or | array[condition] | 任一子条件为 true 时为 true |

与 `rule.Condition` 结构精确一致。leaf 和 composite 互斥（代码中有校验）。

### 区域数据结构

```json
{
  "region_id": "string (必填)",
  "name": "string (必填)",
  "description": "string",
  "region_type": "enum: wilderness | town | dungeon",
  "boundary": {
    "type": "polygon",
    "points": [{"x": 0, "z": 0}, ...]
  },
  "weather": {
    "default": "string",
    "cycle": [{"type": "string", "weight": "number(0-100)"}]
  },
  "spawn_table": [
    {
      "template_ref": "string (NPC 模板名)",
      "count": "integer ≥1",
      "spawn_points": [{"x": 0, "z": 0}],
      "wander_radius": "number ≥0",
      "respawn_seconds": "number ≥0"
    }
  ],
  "properties": {
    "level_range": {"min": "integer", "max": "integer"},
    "bgm": "string"
  }
}
```

### 新增 Blackboard Key 汇总

| Key 名 | 类型 | 所属组件 | 说明 |
|--------|------|---------|------|
| need_lowest | string | needs | 当前最低需求名 |
| need_lowest_val | float64 | needs | 当前最低需求值 |
| emotion_dominant | string | emotion | 主导情绪名 |
| emotion_dominant_val | float64 | emotion | 主导情绪值 |
| memory_count | int64 | memory | 当前记忆条目数 |
| group_id | string | social | 群组 ID |
| social_role | string | social | 社交角色 |
| move_state | string | movement | 移动状态（idle/moving/arrived） |

与已有 12 个 Key 无命名冲突。

---

## 方案对比

### 备选方案：自定义 Schema 格式（不选）

自定义一个更简洁的格式代替 JSON Schema Draft 7：

```json
{
  "fields": [
    {"name": "move_type", "type": "enum", "options": ["wander", "patrol"], "required": true},
    {"name": "move_speed", "type": "number", "min": 0, "required": true}
  ]
}
```

**不选的理由**：
1. JSON Schema Draft 7 是行业标准，Vue 生态有 `@lljj/vue3-form-element` 等库直接渲染，自定义格式需要 ADMIN 自己写渲染逻辑
2. JSON Schema 支持 `if/then`、`$ref`、`oneOf` 等高级特性，自定义格式要重新发明这些
3. ADMIN CC 已确认接受 JSON Schema Draft 7，改用自定义格式违反已达成的约定

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码业务逻辑 | **不违反** | Schema 是数据文件，不是 Go 代码 |
| 禁止 BT 反向驱动 FSM | **不涉及** | Schema 只定义数据格式 |
| 禁止 core/ import runtime/ | **不涉及** | 无 Go 代码改动 |
| 禁止 Blackboard 裸 map | **不涉及** | 新 Key 的 Go 注册在需求 1 |
| 禁止 Key 散落各文件 | **不违反** | Schema 文件中的 `blackboard_keys` 是文档声明，Go 注册仍在 `keys.go`（需求 1 做） |
| 禁止过度设计 | **不违反** | Schema 文件服务于 ADMIN 表单渲染和服务端结构体对照，有明确使用场景 |
| 禁止静默降级 | **不涉及** | 无运行时行为 |
| 禁止协作失序 | **不违反** | Schema 契约正是为了避免协作失序 |
| 禁止口头承诺不落文档 | **不违反** | Schema 文件就是书面契约 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | 中性 | Schema 不涉及事件配置 |
| 加 NPC 类型 | **正面** | 组件 Schema + 预设 = NPC 类型的配置规范 |
| NPC 间交互 | **正面** | social 组件 Schema 为交互奠基 |

---

## 依赖方向

本 spec 无 Go 代码，不涉及包依赖方向。

Schema 文件的数据流向：
```
configs/schemas/ → ADMIN 导入 MongoDB → ADMIN 前端渲染表单
configs/schemas/ → 服务端开发者参照 → Go 结构体（需求 1）
```

---

## 并发安全

本 spec 无 Go 代码，不涉及并发问题。

---

## 配置变更

新增 `configs/schemas/` 目录及 25 个 JSON 文件。这些文件是元数据（描述配置的配置），服务端运行时不加载，不影响现有系统。

---

## 测试策略

### JSON 语法校验

写一个测试文件 `configs/schemas/schemas_test.go`，遍历 `configs/schemas/` 下所有 JSON 文件，验证：
1. `json.Valid()` 通过
2. 组件 Schema 能 Unmarshal 为统一信封结构（`component` + `display_name` + `blackboard_keys` + `schema` 字段存在）
3. 预设文件能 Unmarshal 为预设结构（`preset` + `required_components` 等字段存在）
4. 节点类型文件能 Unmarshal 为节点结构（`node_type` + `category` + `params_schema` 字段存在）

### 一致性校验

同一个测试文件中：
1. 检查组件 `blackboard_keys` 中的已有 Key（`keys.go` 注册表中存在的）名称精确匹配
2. 检查 4 个预设的 `required_components` 都包含 `identity` 和 `position`
3. 检查 8 个节点类型的 `node_type` 值与 `bt.DefaultRegistry()` 注册的名称一致
