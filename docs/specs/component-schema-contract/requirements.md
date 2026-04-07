# component-schema-contract 需求分析

## 动机

v3 将 NPC 从固定类型改为组件自由组合。服务端和 ADMIN 平台需要一份共享的数据格式契约：

1. **ADMIN 被阻塞**：ADMIN 要做 Schema 驱动的动态表单，需要知道每个组件有哪些字段、什么类型、什么校验规则。没有 Schema 文件，ADMIN 无法开工。
2. **后续 8 个需求无格式依据**：组件化架构、感知、决策、记忆、移动、社交、区域、可观测性——每个需求都要定义配置字段。如果不先统一契约，各自定义会导致格式不一致。
3. **BT/FSM 编辑器硬编码**：ADMIN 当前 BT 编辑器的节点类型和 FSM 条件操作符是硬编码的。Schema 化后，服务端加新节点类型只需加 Schema 文件，ADMIN 自动适配。

不做这个，ADMIN 和服务端并行开发时字段对不上，联调返工。

## 优先级

**最高**。v3 的第 0 步，阻塞 ADMIN 平台和全部后续需求。

## 预期效果

### 场景 1：ADMIN 导入组件 Schema 渲染动态表单

ADMIN 读取 `configs/schemas/components/movement.json`，自动渲染：
- `move_type` 下拉框（wander / patrol / follow）
- `move_speed` 数字输入框（默认 3.0）
- 选 `wander` → 出现 `wander_radius` 必填输入
- 选 `patrol` → 出现 `patrol_waypoints` 路点列表编辑器
- 选 `follow` → 两个条件字段都不出现

### 场景 2：ADMIN 使用预设快速创建 NPC

ADMIN 读取 `configs/schemas/presets/reactive.json`，得知：
- 必选：identity、position
- 默认勾选：behavior、perception、movement、personality
- 可选添加：needs、emotion、memory、social

运营选"反应型"预设 → 6 个组件自动勾选 → 表单只展示这 6 个组件的字段。

### 场景 3：ADMIN BT 编辑器节点类型自动发现

ADMIN 读取 `configs/schemas/node_types/*.json`，"添加节点"下拉框自动列出 8 个节点类型，按 category 分组（composite / decorator / leaf）。选 `check_bb_float` → 渲染 `key`、`op`、`value` 三个参数字段。

### 场景 4：服务端新增 BT 节点类型

服务端 Go 代码注册新节点 `move_to` → 在 `configs/schemas/node_types/` 新增 `move_to.json` → ADMIN BT 编辑器自动出现 `move_to` 选项及其参数表单。双方零沟通零改代码。

### 场景 5：服务端后续需求引用契约

需求 2（组件化架构）实现 Go 结构体时，字段名、类型、校验规则严格对照 Schema 文件，确保与 ADMIN 数据一致。

## 依赖分析

- **依赖**：无。纯数据文件输出，不依赖任何代码改动
- **被依赖**：
  - ADMIN 平台：动态表单、预设模板、BT/FSM 编辑器
  - 需求 1 组件化架构：Go 结构体对照 Schema
  - 需求 2-6 各子系统：组件字段定义
  - 需求 7 区域系统：区域配置格式
  - 需求 8 可观测性：NPC 状态字段范围

## 改动范围

| 路径 | 类型 | 文件数 | 说明 |
|------|------|--------|------|
| `configs/schemas/components/` | 新增 | 10 | 10 个 AI 组件 Schema |
| `configs/schemas/presets/` | 新增 | 4 | 4 个复杂度预设 |
| `configs/schemas/node_types/` | 新增 | 8 | 8 个 BT 节点类型（与 `bt.DefaultRegistry()` 一致） |
| `configs/schemas/condition_types/` | 新增 | 2 | FSM 条件叶子节点 + 组合节点 |
| `configs/schemas/region.json` | 新增 | 1 | 区域数据结构 |

共 25 个 JSON 文件，零 Go 代码改动。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | 不直接服务 | 事件配置格式不变，本 spec 不涉及 |
| 加 NPC 类型 | **直接服务** | 组件 Schema + 预设 = "加 NPC 类型"的配置规范 |
| NPC 间交互 | **间接服务** | social 组件 Schema 定义群组/阵营数据结构 |

BT 节点和 FSM 条件 Schema 不直接服务于三个扩展轴，属于 ADMIN 工具链基础设施——让"加新节点类型 = 加 Schema 文件"成为可能。

## 验收标准

### 组件 Schema（10 个文件）

- **R1**：每个文件格式为 `{"component": string, "display_name": string, "blackboard_keys": string[], "schema": {Draft 7 JSON Schema}}`
- **R2**：`schema.required` 正确标注必填字段
- **R3**：枚举字段使用 `enum` 约束（`move_type`: wander/patrol/follow，`personality_type`: timid/aggressive/docile/curious，`role`: leader/follower）
- **R4**：条件依赖字段使用 `if/then` 表达（如 `move_type=wander` → `wander_radius` 必填）
- **R5**：`blackboard_keys` 列出该组件读写的 Blackboard Key 名称；已有 Key（`keys.go` 中 12 个）名称精确匹配，新增 Key 名称不冲突

### 预设定义（4 个文件）

- **R6**：每个文件格式为 `{"preset": string, "display_name": string, "description": string, "required_components": [], "default_components": [], "optional_components": []}`
- **R7**：所有预设的 `required_components` 包含 `identity` 和 `position`
- **R8**：4 个预设按 AI 复杂度递增：simple（环境装饰）→ reactive（感知反应）→ autonomous（自主需求）→ social（社交群体），每级在前一级基础上增加组件

### BT 节点类型 Schema（8 个文件）

- **R9**：每个文件格式为 `{"node_type": string, "display_name": string, "category": "composite"|"decorator"|"leaf", "params_schema": {Draft 7 JSON Schema}}`
- **R10**：8 个节点类型与 `bt.DefaultRegistry()` 注册的完全一致：sequence、selector、parallel、inverter、check_bb_float、check_bb_string、set_bb_value、stub_action
- **R11**：`params_schema` 与 Go factory 函数的参数结构精确匹配（如 `parallel` 的 `policy` 字段枚举为 require_all/require_one）

### FSM 条件类型 Schema（2 个文件）

- **R12**：leaf 条件包含 `key`(string)、`op`(枚举 7 种)、`value`(any)、`ref_key`(string) 字段，`value` 和 `ref_key` 二选一
- **R13**：composite 条件包含 `and`(array) 和 `or`(array) 字段，子项递归引用条件结构
- **R14**：`op` 枚举与 `rule.validOps` 精确一致：==、!=、>、>=、<、<=、in

### 区域数据结构（1 个文件）

- **R15**：包含 `region_id`、`name`、`region_type`(枚举)、`boundary`(多边形坐标数组)、`weather`(天气配置)、`spawn_table`(刷怪表)、`properties`(扩展属性)
- **R16**：`spawn_table` 条目包含 `template_ref`(NPC 模板名)、`count`(数量)、`spawn_points`(坐标数组)、`respawn_seconds`(刷新间隔)

### 整体质量

- **R17**：25 个文件全部为合法 JSON，`json.Valid()` 通过
- **R18**：所有 `blackboard_keys` 中的已有 Key 名称与 `keys.go` 一致，新增 Key 名称在 25 个文件间不冲突

## 不做什么

- **不修改 Go 代码**：结构体实现在需求 1
- **不删除旧配置**：`configs/npc_types/` 清理在需求 1
- **不创建 NPC 模板示例**：`configs/npc_templates/` 在需求 1
- **不实现 ADMIN API**：`component_schemas` 集合由 ADMIN 团队实现
- **不定义 NPC 模板配置格式**：`name + preset + components` 结构在需求 1
