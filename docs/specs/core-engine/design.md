# core-engine 设计方案

## 1. Blackboard（强类型黑板）

### 方案描述

```go
// BBKey[T] 是泛型 Key，编译期绑定类型
type BBKey[T any] struct {
    name string
}

// Blackboard 底层仍然是 map[string]any，但只能通过 BBKey[T] 访问
type Blackboard struct {
    mu   sync.RWMutex
    data map[string]any
}

func Get[T any](bb *Blackboard, key BBKey[T]) (T, bool)
func Set[T any](bb *Blackboard, key BBKey[T], val T)
```

所有 Key 集中在 `keys.go` 一个文件中定义：

```go
// keys.go — 所有 Blackboard Key 的唯一定义处
package blackboard

var (
    KeyThreatLevel    = NewKey[float64]("threat_level")
    KeyThreatSource   = NewKey[string]("threat_source")
    KeyThreatExpireAt = NewKey[int64]("threat_expire_at")
    KeyLastEventType  = NewKey[string]("last_event_type")
    KeyFSMState       = NewKey[string]("fsm_state")
    KeyTargetPos      = NewKey[Vec3]("target_pos")
    // ... 后续按需追加
)
```

注册表机制：`NewKey` 内部自动注册到全局表，记录 Key 名称和类型。配置加载时，扫描配置中引用的所有 Key 名称，和注册表比对，不存在则 panic。

### 备选方案：纯 map + 运行时校验（不选）

只做运行时注册表校验，不用泛型。调用方需要自己做类型断言：`bb.Get("threat_level").(float64)`。

**不选的理由**：类型断言遗漏时运行时 panic，且编译器无法检查。v1 的 `map[string]any` 就是这个方案，已经证明会产生隐形 bug。

### 并发安全

Blackboard 被 FSM 转换条件读、BT 节点读写、决策中心写，必须加 `sync.RWMutex`。Get 用 RLock，Set 用 Lock。

---

## 2. Rule（条件规则匹配器）

### 方案描述

条件是一个树形结构，叶子节点是单个比较，非叶子节点是 AND/OR：

```go
// Condition 是条件树的节点
type Condition struct {
    // 叶子节点字段
    Key    string `json:"key,omitempty"`     // Blackboard Key 名称
    Op     string `json:"op,omitempty"`      // ==, !=, >, >=, <, <=, in
    Value  any    `json:"value,omitempty"`   // 字面量
    RefKey string `json:"ref_key,omitempty"` // 引用另一个 BB Key 作为比较值

    // 组合节点字段
    And []Condition `json:"and,omitempty"`
    Or  []Condition `json:"or,omitempty"`
}

// Evaluate 对 Blackboard 求值，返回 true/false
func (c *Condition) Evaluate(bb *Blackboard) bool
```

JSON 配置示例：

```json
{
    "and": [
        {"key": "threat_level", "op": ">=", "value": 50},
        {"key": "threat_expire_at", "op": ">", "ref_key": "current_time"}
    ]
}
```

Evaluate 内部：
- 叶子节点：从 BB 取 Key 的值，与 Value 或 RefKey 的值比较
- AND：所有子条件都为 true
- OR：任一子条件为 true
- 空条件：返回 true（无条件转换）

**类型处理**：Evaluate 时从 BB 取出的值是 `any`，需要根据 Op 做类型判断。支持 `float64`、`int64`、`string`、`bool` 四种基础类型的比较。`in` 操作符支持值在数组中的包含检测。

### 备选方案：每个 Op 一个 struct 类型（不选）

用 Go 接口多态，`GreaterThan`、`Equals` 各一个 struct 实现 `Evaluator` 接口。

**不选的理由**：Op 数量少（7 个），用 switch 在一个函数里处理更简洁。为 7 个操作符创建 7 个 struct 是过度设计。

### Key 校验

`Condition.Validate(registry)` 方法在配置加载时调用，检查所有引用的 Key 名称是否在 Blackboard 注册表中。不存在则返回错误，上层 panic。

---

## 3. FSM（配置驱动状态机）

### 方案描述

```go
// StateConfig 一个状态的配置
type StateConfig struct {
    Name string `json:"name"`
}

// TransitionConfig 一条转换规则的配置
type TransitionConfig struct {
    From      string          `json:"from"`
    To        string          `json:"to"`
    Priority  int             `json:"priority"`
    Condition rule.Condition  `json:"condition"`
}

// FSMConfig 完整的 FSM 配置
type FSMConfig struct {
    InitialState string             `json:"initial_state"`
    States       []StateConfig      `json:"states"`
    Transitions  []TransitionConfig `json:"transitions"`
}

// FSM 运行时实例
type FSM struct {
    config      *FSMConfig
    current     string
    transitions map[string][]TransitionConfig  // from_state → 按 priority 降序排列的转换列表
    onTransition func(from, to string)          // 转换回调
}

// Tick 每帧调用，评估当前状态的所有转换条件
func (f *FSM) Tick(bb *Blackboard)
```

Tick 逻辑：
1. 取当前状态对应的转换列表（已按 priority 降序排列）
2. 遍历转换，用规则匹配器对 Blackboard 求值
3. 第一个匹配的转换触发状态切换
4. 触发 OnExit(old) → OnEnter(new) → OnTransition(old, new) 回调

JSON 配置示例（`configs/fsm/civilian.json`）：

```json
{
    "initial_state": "Idle",
    "states": [
        {"name": "Idle"},
        {"name": "Alarmed"},
        {"name": "Flee"},
        {"name": "Dead"}
    ],
    "transitions": [
        {
            "from": "Idle",
            "to": "Alarmed",
            "priority": 10,
            "condition": {"key": "last_event_type", "op": "!=", "value": ""}
        },
        {
            "from": "Alarmed",
            "to": "Flee",
            "priority": 10,
            "condition": {
                "and": [
                    {"key": "threat_level", "op": ">=", "value": 50},
                    {"key": "threat_expire_at", "op": ">", "ref_key": "current_time"}
                ]
            }
        }
    ]
}
```

### 备选方案：状态对象模式，每个状态是一个 interface 实现（不选）

```go
type State interface {
    OnEnter(bb *Blackboard)
    OnTick(bb *Blackboard)
    OnExit(bb *Blackboard)
}
```

**不选的理由**：这就是 v1 的做法——每种 NPC 类型需要写一套 Go 实现。配置驱动的核心就是消灭 per-type Go 代码。State 的行为由 BT 负责，FSM 只管转换。

### 状态名约束

FSM 配置加载时，将所有状态名收集为一个集合。转换规则的 from/to 必须在集合中，否则 panic。运行时 `Current()` 返回 string，但该 string 一定在配置定义的状态集合内。

---

## 4. BT（行为树引擎）

### 方案描述

```go
// Status 节点执行结果
type Status int
const (
    Success Status = iota
    Failure
    Running
)

// Node 行为树节点接口
type Node interface {
    Tick(bb *Blackboard) Status
}

// 组合节点：Sequence、Selector、Parallel
// 装饰节点：Inverter、RepeatUntil、AlwaysSucceed 等
// 叶子节点：通过注册表按 type 名称查找

// Registry 节点工厂注册表
type Registry struct {
    factories map[string]NodeFactory
}
type NodeFactory func(cfg json.RawMessage) (Node, error)

// Register 注册节点工厂
func (r *Registry) Register(typeName string, factory NodeFactory)
```

BT 配置 JSON 示例（`configs/bt_trees/civilian_idle.json`）：

```json
{
    "type": "sequence",
    "children": [
        {
            "type": "check_bb_float",
            "params": {"key": "threat_level", "op": "<", "value": 10}
        },
        {
            "type": "set_bb_value",
            "params": {"key": "target_pos", "value": {"x": 0, "y": 0, "z": 0}}
        }
    ]
}
```

Builder 从 JSON 递归构建节点树：读 `type` 字段 → 从 Registry 查工厂 → 调工厂函数传 params → 递归处理 children。

### 备选方案：BT 用代码构建，不做 JSON 配置（不选）

**不选的理由**：和 FSM 一样，加 NPC 类型就要写 Go 代码。v1 已经证明这条路行不通。

### 本 spec 实现的节点

只实现框架和基础组合节点，不实现具体业务 Action：

| 节点类型 | 分类 | 说明 |
|----------|------|------|
| Sequence | 组合 | 依次执行子节点，遇到 Failure 停止 |
| Selector | 组合 | 依次执行子节点，遇到 Success 停止 |
| Parallel | 组合 | 并行执行所有子节点，可配置成功/失败策略 |
| Inverter | 装饰 | 反转子节点结果 |
| check_bb_float | 条件 | 检查 BB 中 float64 Key 满足条件 |
| check_bb_string | 条件 | 检查 BB 中 string Key 满足条件 |
| set_bb_value | 动作 | 向 BB 写入值 |

后续业务节点（move_to、play_animation 等）在 Runtime 层的 spec 中追加。

---

## 5. Config（配置加载层）

### 方案描述

```go
// Source 配置数据源接口
type Source interface {
    LoadFSMConfig(npcType string) (*fsm.FSMConfig, error)
    LoadBTTree(treeName string) (*bt.TreeConfig, error)
    // 后续按需追加：LoadNPCTypeConfig, LoadEventConfig 等
}

// JSONSource 从 configs/ 目录加载 JSON 文件
type JSONSource struct {
    basePath string
}
```

当前只实现 `JSONSource`。未来加 `MongoSource` 时实现同一接口。

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码 FSM 状态/转换规则 | **不违反** | FSM 从 JSON 配置加载 |
| 禁止硬编码事件→感知映射 | **不涉及** | 本 spec 不含事件/感知 |
| 禁止 switch-case 做 NPC 类型分发 | **不违反** | BT 用注册表，FSM 用配置 |
| 禁止 BT 反向驱动 FSM | **不违反** | Core 层不含 FSM-BT 组装逻辑 |
| 禁止 core/ import runtime/gateway | **不违反** | core/ 无外部依赖 |
| 禁止 Blackboard 裸 map[string]any | **不违反** | 泛型 BBKey[T] 访问 |
| 禁止 Key 散落各文件 | **不违反** | 集中在 keys.go |
| 禁止 Key 拼错静默失败 | **不违反** | 注册表 + 配置加载校验 |
| 禁止 FSM 状态名魔法字符串 | **不违反** | 配置加载时收集状态集合，转换规则校验 |
| 禁止引入脚本引擎 | **不违反** | 简单规则匹配器，结构化 JSON |
| 禁止过度设计 | **不违反** | 不做 MongoDB Source、不做业务 Action 节点 |

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面** | 新事件只需在 keys.go 加 Key + 在 FSM 配置加转换条件 |
| 加 NPC 类型 | **正面** | 新类型只需新增 FSM JSON + BT JSON，不改 Go 代码 |
| NPC 间交互 | **正面** | 强类型 Blackboard 保证交互数据类型安全 |

## 依赖方向

```
config/ ──→ core/fsm/    ──→ core/rule/ ──→ core/blackboard/
         ──→ core/bt/     ──→              ──→ core/blackboard/
```

单向向下，无循环依赖。config 知道 core，core 不知道 config。core 各子包之间：fsm 和 bt 依赖 rule 和 blackboard，rule 依赖 blackboard，blackboard 不依赖任何人。

## 测试策略

| 模块 | 单元测试覆盖 |
|------|-------------|
| blackboard | Get/Set 类型安全、未注册 Key panic、并发读写安全 |
| rule | 各 Op 求值正确性、AND/OR 组合、空条件、RefKey、非法 Key 校验 |
| fsm | 配置加载、Tick 转换、优先级排序、回调触发、非法状态名校验 |
| bt | Sequence/Selector/Parallel 执行语义、JSON 构建、Registry 查找失败 |
| config | JSON 文件加载、文件不存在错误处理 |
