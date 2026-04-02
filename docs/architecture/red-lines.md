# 禁止事项（红线）

违反任何一条都意味着架构退化回 v1。

## 禁止硬编码业务逻辑

- **禁止**在 Go 代码中硬编码 FSM 状态定义或转换规则。FSM 状态枚举和转换条件必须从配置加载
- **禁止**在 Go 代码中硬编码事件类型与感知方式的映射（v1 的 `canPerceive()` switch-case）。感知规则必须随事件配置声明
- **禁止**在 Go 代码中硬编码 NPC 类型特定的参数（速度、感知距离等）。必须从配置加载
- **禁止**用 switch-case 做 NPC 类型分发（v1 的 `wireStates()`、`npcSpeed()`）。必须用注册表/工厂模式

## 禁止破坏层次

- **禁止** BT 反向驱动 FSM（v1 的 `KeyRequestedTransition`）。FSM 管"什么时候切状态"，BT 管"切了之后做什么"，方向单向
- **禁止** core/ 包 import runtime/ 或 gateway/。依赖方向必须单向向下
- **禁止** experiment/ 包 import internal/core/ 或 internal/runtime/ 的具体实现。实验框架通过接口注入，build tag 隔离
- **禁止** gateway/ 承担非网络职责（v1 的 Gateway 里做了 AOI 计算、事件发布、Tick 调整）

## 禁止弱类型通信

- **禁止** Blackboard 使用裸 `map[string]any`。必须通过泛型 `BBKey[T]` 访问，编译期确定类型
- **禁止** Blackboard Key 散落在各文件中。所有 Key 必须集中定义在一个 const 文件（`keys.go`）中，声明名称、类型、读者、写者
- **禁止** Key 拼错静默失败。配置加载时必须校验所有引用的 Key 是否在注册表中存在，不存在直接 panic
- **禁止** FSM 状态名使用魔法字符串。状态名必须有枚举或 const 约束

## 禁止实验污染核心

- **禁止**核心代码中出现任何实验相关的字段、方法或 import（v1 的 Instance 中有 `pureBTEng`、`SetMode()`，Manager 中有 `SpawnDummies()`）
- **禁止**为了实验需求修改核心数据结构。实验通过外部组合核心组件实现

## 禁止过度设计

- **禁止**引入脚本引擎（govaluate、expr-lang 等）做条件求值。用简单规则匹配器（`bb_key + op + value/ref_key`，AND/OR 组合）
- **禁止**在没有使用场景时提前接入 Redis。v1 的 redis.go 是死代码
- **禁止**为只有一个调用点的逻辑创建抽象层
