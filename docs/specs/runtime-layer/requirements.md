# runtime-layer 需求分析

## 动机

core-engine 提供了 FSM、BT、Blackboard、Rule 四个独立引擎，但它们还是散件——没有东西把它们组装成一个能跑的 NPC。Runtime 层就是那个组装层：

1. **NPC 实例**：把 BB + FSM + BT 装配到一起，提供 Tick 驱动
2. **事件总线**：外部事件（爆炸、枪声、呼救）进入系统的入口
3. **决策中心**：事件→威胁评估→优先级仲裁→写 BB，这是毕设核心创新点的关键一环
4. **感知过滤**：不是所有事件对所有 NPC 可见，按距离和感知能力过滤
5. **Tick 调度**：驱动所有 NPC 的 FSM 和 BT 周期性执行

不做这个，core-engine 就是一堆零件，无法验证"平民 × 3 事件源"的基线场景，毕设的三位一体架构无法成立。

### 为什么不拆成多个 spec

事件总线、决策中心、感知过滤、NPC 实例是一条完整的数据流：

```
外部事件 → 事件总线 → 感知过滤 → 决策中心(威胁评估+仲裁) → 写 BB → FSM Tick → BT Tick
```

拆开后任何一个单独都无法验证。它们是同一个功能的不同环节，不是独立功能。

### 不含 world/AOI

世界状态和空间索引（AOI）是优化层，不影响核心逻辑正确性。本 spec 用简单的距离计算替代 AOI，后续在 gateway-layer spec 中实现空间索引优化。

## 优先级

**最高**。这是 core-engine 之后的第一个 spec，是从"引擎零件"到"能跑的 NPC"的关键一步。验证路径的第一个里程碑（平民 × 3 事件源 → 基线）依赖它。

## 预期效果

做完后，以下场景可以工作：

### 场景 1：平民遇到爆炸逃跑

1. 创建一个 civilian NPC 实例（从 JSON 配置加载 FSM + BT）
2. 发布一个 Explosion 事件（位置 [100, 0, 100]，severity=80）
3. 感知过滤：NPC 在爆炸 500m 内 → 能感知到
4. 决策中心：评估威胁等级 = severity × 距离衰减 → 写入 BB（threat_level=72, threat_source="explosion_001"）
5. FSM Tick：threat_level >= 50 → Idle→Alarmed→Flee
6. BT Tick：在 Flee 状态下执行逃跑行为树

### 场景 2：事件过期后恢复

1. 事件总线的 TTL 衰减：Explosion 事件 TTL 到期后自动清除
2. 决策中心：无活跃威胁 → threat_level 衰减到 0
3. FSM Tick：threat_level < 20 → Flee→Idle

### 场景 3：多事件同时到达——优先级仲裁

1. NPC 同时收到 Gunshot（severity=90）和 Shout（severity=30）
2. 决策中心按威胁等级降序排列，取最高威胁写入 BB
3. FSM 响应的是 Gunshot 而非 Shout

### 场景 4：高威胁事件打断低威胁行为（事件抢占）

1. t0：NPC 收到 Shout（severity=30）→ 决策中心写 BB: threat_level=30 → FSM: Idle→Alarmed
2. t1-t5：NPC 在 Alarmed 状态执行警戒行为树（BT 返回 Running）
3. t6：Gunshot 到达（severity=90）→ 决策中心重新仲裁: 90 > 30 → 覆写 BB: threat_level=90
4. t7：FSM Tick: threat_level >= 50 → Alarmed→Flee，触发 OnExit(Alarmed) + OnEnter(Flee)
5. BT 切换到 Flee 状态对应的行为树，之前 Running 的警戒行为被中断

关键机制：决策中心每次 Tick 都重新仲裁所有活跃事件，不是只在新事件到达时评估。FSM 每次 Tick 都评估转换条件。两者结合实现自然的事件抢占。

### 场景 5：加新事件源不改代码（扩展轴验证）

1. 在 `configs/events/` 中新增一个事件类型定义（JSON）
2. 发布该类型事件
3. 现有 NPC 根据配置的感知规则自动响应，无需修改 Go 代码

**不能工作的**（不在本 spec 范围）：
- 网络通信（WebSocket 广播）
- AOI 空间索引优化
- NPC 间交互（警察追捕平民）
- 对照实验框架
- 具体业务 BT Action 节点（move_to、play_animation 等，只提供 stub）

## 依赖分析

- **依赖**：core-engine spec（Blackboard、FSM、BT、Rule、Config）— 已完成
- **被依赖**：
  - gateway-layer（WebSocket 接入、状态广播、世界状态/AOI）
  - experiment-layer（对照实验需要能创建和驱动 NPC）
  - e2e 测试（需要无头客户端通过 Runtime API 操作 NPC）

## 改动范围

| 包 | 新增文件数 | 说明 |
|----|-----------|------|
| `internal/runtime/event/` | 2-3 | 事件总线 + 事件定义 + 测试 |
| `internal/runtime/npc/` | 3-4 | NPC 实例 + 注册表 + Tick 调度 + 测试 |
| `internal/runtime/decision/` | 2-3 | 决策中心 + 威胁评估 + 测试 |
| `internal/runtime/perception/` | 2 | 感知过滤器 + 测试 |
| `internal/config/` | 1-2 | Source 接口扩展（LoadEventConfig、LoadNPCTypeConfig） |
| `internal/core/blackboard/keys.go` | 0（修改） | 新增 Runtime 层需要的 Key |
| `configs/` | 3-4 | 事件类型配置 + NPC 类型配置 + BT 树配置 |

预估 16-22 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **是** | 事件类型从 JSON 配置加载，新事件类型只需加配置文件，感知规则随事件声明 |
| 加 NPC 类型 | **是** | NPC 类型从 JSON 配置加载（FSM + BT + 感知参数），工厂注册表创建实例 |
| NPC 间交互 | **间接** | 决策中心架构支持多 NPC 协调，但本 spec 不实现交互逻辑 |

## 验收标准

### 事件总线

- **R1**：事件总线支持发布/订阅模式，事件包含 type、position、severity、TTL 字段
- **R2**：事件 TTL 衰减，每 Tick 递减，TTL <= 0 时自动从活跃列表移除
- **R3**：事件类型从 JSON 配置加载（`configs/events/*.json`），包含名称、默认 severity、默认 TTL、感知方式（视觉/听觉/全局）

### 感知过滤

- **R4**：感知过滤器根据 NPC 类型配置中的感知参数（视觉距离、听觉距离）和事件的感知方式，判断 NPC 能否感知到事件
- **R5**：感知参数从 NPC 类型配置加载，不硬编码

### 决策中心

- **R6**：决策中心接收过滤后的事件，计算威胁等级（severity × 距离衰减因子），写入 NPC 的 Blackboard
- **R7**：多个事件同时存在时，决策中心每次 Tick 都重新仲裁所有活跃事件，按威胁等级降序排列，最高威胁写入 BB 的 threat_level/threat_source
- **R8**：无活跃威胁时，threat_level 衰减到 0（由决策中心在 Tick 中处理，而非 FSM）
- **R9**：高威胁事件到达时，决策中心覆写 BB，FSM 在下一次 Tick 自然转换到更高优先级状态，实现事件抢占（不需要特殊的"打断"机制，由 Tick 循环自然驱动）

### NPC 实例与注册表

- **R10**：NPC 实例持有 Blackboard + FSM + BT，通过工厂从配置创建，不使用 switch-case 做类型分发
- **R11**：NPC 类型配置（`configs/npc_types/*.json`）包含 FSM 配置引用、BT 树引用列表（按状态名映射）、感知参数
- **R12**：NPC 注册表管理所有活跃 NPC 实例，支持按 ID 查找、遍历

### Tick 调度

- **R13**：Tick 调度器以固定间隔驱动所有 NPC：事件 TTL 衰减 → 感知过滤 → 决策中心 → FSM Tick → BT Tick
- **R14**：单次 Tick 处理 100 个 NPC 时延迟 < 10ms（无网络 IO 的纯逻辑 benchmark）

### 配置驱动

- **R15**：新增事件类型只需添加 JSON 配置文件 + 测试，不改 Go 代码
- **R16**：新增 NPC 类型只需添加 JSON 配置文件（FSM + BT + 类型参数）+ 测试，不改 Go 代码
- **R17**：Config Source 接口扩展 `LoadEventConfig`、`LoadNPCTypeConfig`，JSONSource 实现

### 测试

- **R18**：所有组件有单元测试，覆盖核心逻辑路径
- **R19**：集成测试覆盖场景 1-5（平民遇爆炸逃跑、事件过期恢复、多事件同时仲裁、高威胁打断低威胁、加新事件源不改代码）

## 不做什么

- 不做 WebSocket 通信（Gateway 层的事）
- 不做 AOI 空间索引（用简单距离计算，后续 world/ 包优化）
- 不做 NPC 间交互逻辑（决策中心架构预留，但本 spec 只处理单 NPC 对事件的响应）
- 不做具体业务 BT Action（move_to、play_animation），只做 stub 节点返回 Success
- 不做 Tick 三级调度优化（Sleep/Mid/High），统一频率
- 不做 MongoDB 数据源实现
- 不做性格参数对行为的影响（后续 spec）
