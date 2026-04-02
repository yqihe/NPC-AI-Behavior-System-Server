# core-engine 任务拆解

## 依赖关系

```
T1 Blackboard → T2 Rule(依赖BB) → T3 FSM(依赖Rule+BB) → T5 Config(依赖FSM+BT)
                                    T4 BT(依赖BB)       ↗
```

---

## 任务列表

### [x] T1: Blackboard 强类型实现 (R1, R2, R3)

**产出**：Blackboard 核心 + 所有 Key 定义

**文件**：
- `internal/core/blackboard/blackboard.go` — BBKey[T] 泛型定义、Blackboard struct、Get/Set/Has 函数、注册表
- `internal/core/blackboard/keys.go` — 所有 Key 常量集中定义
- `internal/core/blackboard/blackboard_test.go` — 测试

**做完是什么样**：
- `Get(bb, KeyThreatLevel)` 返回 `(float64, bool)`，编译期确定类型
- `Set(bb, KeyThreatLevel, 75.0)` 编译通过，`Set(bb, KeyThreatLevel, "abc")` 编译报错
- 对未注册的 Key 写入 panic
- 并发读写安全（RWMutex）

---

### [x] T2: 条件规则匹配器 (R4, R5)

**产出**：规则匹配器，能对 Blackboard 求值

**文件**：
- `internal/core/rule/condition.go` — Condition 结构体、Evaluate 方法、Validate 方法
- `internal/core/rule/condition_test.go` — 测试

**做完是什么样**：
- `{"key": "threat_level", "op": ">=", "value": 50}` 对 BB 求值返回 true/false
- AND/OR 组合嵌套正确求值
- `ref_key` 引用另一个 BB Key 作为比较值
- `Validate(registry)` 检查引用的 Key 是否注册，未注册返回 error
- 支持 float64/int64/string/bool 四种类型比较

---

### [x] T3: FSM 配置驱动引擎 (R6, R7, R8, R15)

**产出**：FSM 引擎，从配置创建，Tick 驱动转换

**文件**：
- `internal/core/fsm/fsm.go` — FSMConfig、FSM struct、NewFSM、Tick、回调注册
- `internal/core/fsm/fsm_test.go` — 测试

**做完是什么样**：
- `NewFSM(config, bb)` 从 FSMConfig 创建 FSM 实例
- `Tick()` 按优先级评估转换条件，匹配则切换状态
- 转换时触发 OnEnter/OnExit/OnTransition 回调
- 配置中 from/to 引用不存在的状态名时，创建阶段返回 error
- 配置中的 Condition 引用不存在的 BB Key 时，创建阶段返回 error

---

### [ ] T4: BT 引擎 + 基础节点 (R9, R10, R11, R12)

**产出**：BT 引擎框架 + 注册表 + 7 种基础节点

**文件**：
- `internal/core/bt/node.go` — Node 接口、Status 枚举
- `internal/core/bt/composite.go` — Sequence、Selector、Parallel
- `internal/core/bt/decorator.go` — Inverter
- `internal/core/bt/leaves.go` — check_bb_float、check_bb_string、set_bb_value
- `internal/core/bt/registry.go` — Registry + NodeFactory + 默认节点注册
- `internal/core/bt/builder.go` — 从 TreeConfig JSON 递归构建节点树
- `internal/core/bt/bt_test.go` — 测试

**注意**：文件数为 7 个，超过了 3 个文件的限制。拆分为 T4a 和 T4b。

---

### [x] T4a: BT 引擎框架 (R9, R10)

**产出**：Node 接口 + 组合节点 + 装饰节点

**文件**：
- `internal/core/bt/node.go` — Node 接口、Status 枚举
- `internal/core/bt/composite.go` — Sequence、Selector、Parallel
- `internal/core/bt/decorator.go` — Inverter

**做完是什么样**：
- Sequence 依次 Tick 子节点，遇 Failure 返回 Failure，全 Success 返回 Success
- Selector 依次 Tick 子节点，遇 Success 返回 Success，全 Failure 返回 Failure
- Parallel 执行所有子节点，根据策略判定结果
- Inverter 反转子节点结果

---

### [x] T4b: BT 注册表 + Builder + 叶子节点 (R11, R12)

**产出**：节点工厂注册表 + JSON 构建 + 基础叶子节点

**文件**：
- `internal/core/bt/registry.go` — Registry、NodeFactory、Register、默认节点注册
- `internal/core/bt/builder.go` — 从 TreeConfig 递归构建节点树
- `internal/core/bt/leaves.go` — check_bb_float、check_bb_string、set_bb_value

**做完是什么样**：
- `Registry.Build(treeJSON)` 递归构建完整节点树
- 节点 type 名在 Registry 中不存在时返回 error
- check_bb_float 能读 BB 中的 float64 Key 并比较
- set_bb_value 能向 BB 写入值

---

### [x] T5: Config 加载层 + JSON 配置文件 (R13, R14)

**产出**：配置接口 + JSON 实现 + 示例配置文件

**文件**：
- `internal/config/source.go` — Source 接口定义
- `internal/config/json_source.go` — JSONSource 实现
- `configs/fsm/civilian.json` — 示例 FSM 配置

**做完是什么样**：
- `JSONSource.LoadFSMConfig("civilian")` 读取 `configs/fsm/civilian.json` 返回 FSMConfig
- 文件不存在时返回明确的 error
- JSON 格式错误时返回明确的 error

---

### [x] T6: 集成测试 + go.mod 初始化 (R14)

**产出**：模块初始化 + 跨模块集成测试

**文件**：
- `go.mod` — 模块定义
- `internal/core/integration_test.go` — 集成测试：JSON 配置 → Config 加载 → FSM 创建 → BB 写入 → Tick → 状态转换

**做完是什么样**：
- `go test ./...` 全部通过
- `go build ./...` 编译通过
- 集成测试验证完整链路：加载配置 → 创建 FSM → 设置 BB 值 → Tick → 验证状态变化
