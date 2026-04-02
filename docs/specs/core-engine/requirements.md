# core-engine 需求分析

## 动机

Core 层是整个系统的地基——FSM 引擎、BT 引擎、Blackboard、条件规则匹配器。所有上层（Runtime、Gateway、Experiment）都依赖它。不做这个，什么都做不了。

v1 的核心问题就出在这一层：FSM 硬编码、Blackboard 无类型约束。v2 必须先把这层做对，否则上层全部重蹈覆辙。

## 优先级

**最高**。这是第一个需求，零依赖，所有后续需求都依赖它。

## 预期效果

做完后，以下场景可以工作：

1. 从 JSON 配置文件加载一个 FSM（比如平民的 5 个状态 + 6 条转换规则），FSM 能根据 Blackboard 中的值自动评估转换条件并切换状态
2. 从 JSON 配置文件加载一棵 BT，BT 能 Tick 执行，节点能读写 Blackboard
3. Blackboard 通过 `BBKey[T]` 泛型读写，Key 拼错编译报错，配置中引用的 Key 启动时校验
4. FSM 转换条件用结构化 JSON 描述（`bb_key + op + value`），由规则匹配器在运行时求值

**不能工作的**（不在本 spec 范围）：事件总线、决策中心、感知过滤、网络通信、NPC 实例管理。

## 依赖分析

- **依赖**：无，这是第一个 spec
- **被依赖**：后续所有 spec（Runtime 层、Gateway 层、Experiment 层）都依赖本 spec 的产出

## 改动范围

| 包 | 新增文件数 | 说明 |
|----|-----------|------|
| `internal/core/blackboard/` | 2-3 | Blackboard 实现 + keys.go + 测试 |
| `internal/core/rule/` | 2 | 规则匹配器 + 测试 |
| `internal/core/fsm/` | 2-3 | FSM 引擎 + 测试 |
| `internal/core/bt/` | 3-4 | BT 引擎 + 节点库 + 测试 |
| `internal/config/` | 2 | 配置加载接口 + JSON 实现 |
| `configs/` | 3-4 | 示例 JSON 配置文件 |

预估 12-16 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **是** | 事件通过 Blackboard Key 影响 FSM 转换，Key 注册表保证新事件的 Key 能被校验 |
| 加 NPC 类型 | **是** | FSM 配置驱动，新 NPC 类型只需新增一份 FSM JSON 配置 |
| NPC 间交互 | **间接** | Blackboard 是交互数据的载体，强类型保证交互数据的正确性 |

## 验收标准

- **R1**：Blackboard 支持泛型 `BBKey[T]` 读写，Get/Set 编译期类型安全
- **R2**：所有 Blackboard Key 集中定义在 `keys.go` 一个文件中，每个 Key 声明名称和类型
- **R3**：Blackboard 对未注册的 Key 写入时 panic
- **R4**：条件规则匹配器支持 `bb_key + op + value/ref_key`，op 至少支持 `==`、`!=`、`>`、`>=`、`<`、`<=`、`in`
- **R5**：条件规则匹配器支持 AND/OR 组合嵌套
- **R6**：FSM 引擎从 JSON 配置加载状态枚举和转换规则，不含任何硬编码的状态名或条件
- **R7**：FSM 转换条件使用规则匹配器求值，转换按优先级排序，最高优先级的匹配条件触发转换
- **R8**：FSM 状态转换时触发回调（OnEnter/OnExit/OnTransition），供上层监听
- **R9**：BT 引擎支持 DFS Tick，节点返回 Success/Failure/Running 三态
- **R10**：BT 支持 Sequence、Selector、Parallel 三种组合节点
- **R11**：BT 从 JSON 配置加载树结构，通过节点类型名从注册表查找节点工厂
- **R12**：BT 节点能读写 Blackboard
- **R13**：配置加载层提供接口抽象，当前实现 JSON 文件数据源
- **R14**：所有组件有单元测试，覆盖核心逻辑路径
- **R15**：FSM 状态名使用 const 约束，不允许魔法字符串

## 不做什么

- 不做 MongoDB 数据源实现（只做接口 + JSON 实现）
- 不做 Action 节点的具体业务实现（如 move_to、play_animation），只做节点注册表框架和基础组合节点
- 不做 NPC 实例管理、Tick 调度
- 不做事件总线、决策中心
- 不做网络通信
- 不做 FSM 和 BT 的组装（谁持有谁、怎么联动），那是 Runtime 层的事
