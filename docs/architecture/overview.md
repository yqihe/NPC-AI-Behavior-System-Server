# 服务端架构总览

## 系统定位

基于 **FSM + BT + 决策中心** 三位一体架构的游戏 NPC AI 行为系统服务端。通过 WebSocket 与 Unity 客户端通信，驱动 NPC 在开放世界中自主感知、决策、行动。

**核心价值**：加新 NPC 类型或新事件源 = 加配置 + 加测试，不改核心代码。

## 架构分层

```
┌─────────────────────────────────────────────────────┐
│  cmd/server/main.go                    程序入口      │
│  (配置加载 → slog → Runtime → Gateway → 启动)       │
├─────────────────────────────────────────────────────┤
│  internal/gateway/          网络层（Gateway）         │
│  ┌───────┐ ┌──────┐ ┌────────┐ ┌─────────┐         │
│  │Server │→│ Hub  │→│  Conn  │→│ Router  │         │
│  └───────┘ └──────┘ └────────┘ └─────────┘         │
│  pkg/protocol/              消息协议定义             │
├─────────────────────────────────────────────────────┤
│  internal/runtime/          运行时层（Runtime）       │
│  ┌───────────┐ ┌────────────┐ ┌──────────────────┐  │
│  │ Scheduler │→│  EventBus  │→│ Decision Center  │  │
│  └───────────┘ └────────────┘ └──────────────────┘  │
│  ┌───────────┐ ┌────────────┐                       │
│  │ Registry  │ │ Perception │                       │
│  └───────────┘ └────────────┘                       │
├─────────────────────────────────────────────────────┤
│  internal/core/             引擎层（Core）           │
│  ┌────────────┐ ┌─────┐ ┌────┐ ┌──────┐            │
│  │ Blackboard │ │ FSM │ │ BT │ │ Rule │            │
│  └────────────┘ └─────┘ └────┘ └──────┘            │
├─────────────────────────────────────────────────────┤
│  internal/config/           配置层                    │
│  ┌────────────┐ ┌─────────────┐                     │
│  │ JSONSource │ │ MongoSource │                     │
│  └────────────┘ └─────────────┘                     │
├ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─┤
│  internal/experiment/       实验层（build tag 隔离）  │
│  5 种模式对照 × 定性/定量验证                        │
└─────────────────────────────────────────────────────┘
```

**依赖方向**：cmd → gateway → runtime → core ← config，严格单向向下。experiment 通过 build tag 隔离，正向依赖 core + runtime。

---

## 各层详解

### Core 层 — 纯引擎，零业务逻辑

| 组件 | 文件 | 职责 |
|------|------|------|
| **Blackboard** | `blackboard.go`, `keys.go` | 强类型键值存储（`BBKey[T]` 泛型），NPC 各子系统间的共享数据总线。所有 Key 集中定义在 `keys.go`，编译期校验类型 |
| **FSM** | `fsm.go` | 配置驱动的有限状态机。状态、转换规则、条件全部从 JSON 加载。支持 OnEnter/OnExit/OnTransition 回调。RWMutex 保护并发安全 |
| **BT** | `node.go`, `composite.go`, `leaves.go`, `decorator.go`, `builder.go`, `registry.go` | 行为树引擎。Sequence/Selector/Parallel/Inverter 复合节点 + check_bb_float/check_bb_string/set_bb_value/stub_action 叶子节点。从 JSON 配置构建，注册表模式扩展节点类型 |
| **Rule** | `condition.go` | 条件规则匹配器。支持 `key op value`、`key op ref_key`、`AND/OR` 组合。FSM 转换条件和 BT 检查节点共用 |

**已实现的功能**：
- 泛型 Blackboard 编译期类型安全
- FSM 配置化状态转换（任意状态数、任意转换规则）
- BT 从 JSON 构建任意深度树结构
- 规则引擎 6 种比较操作符 + AND/OR 嵌套组合
- 构建期校验（Key 注册、状态合法性、条件合法性）

### Runtime 层 — 运行时业务组装

| 组件 | 文件 | 职责 |
|------|------|------|
| **Scheduler** | `scheduler.go` | Tick 循环驱动器。每 Tick：事件 TTL 衰减 → 感知过滤 → 决策评估 → NPC Tick（FSM + BT） |
| **EventBus** | `event/event.go` | 事件总线。GTA5 风格 TTL 衰减模型，事件发布后自动过期移除。RWMutex 并发安全 |
| **Decision Center** | `decision/decision.go` | 威胁评估 + 优先级仲裁。`threat = severity × (1 - distance/range)`，多事件取最高威胁写入 BB，无事件时按速率衰减 |
| **Perception** | `perception/perception.go` | 感知过滤。三种模式（global/visual/auditory），基于 NPC 感知范围和事件传播范围计算是否可感知 |
| **NPC Registry** | `npc/registry.go` | NPC 注册表。增删查遍历，RWMutex 并发安全 |
| **NPC Instance** | `npc/instance.go` | NPC 运行时实例。工厂模式从配置创建，持有 BB + FSM + BT 树集合 |

**已实现的功能**：
- 完整的 Tick 管线（感知 → 决策 → FSM → BT）
- TTL 事件衰减（事件自动过期，NPC 自动恢复）
- 距离衰减威胁计算（远处高 severity 事件不会误判）
- 多事件优先级仲裁（最高威胁胜出）
- 高威胁事件打断低威胁行为（事件抢占）

### Gateway 层 — WebSocket 网络接口

| 组件 | 文件 | 职责 |
|------|------|------|
| **Server** | `server.go` | HTTP 服务 + WS 升级 + 优雅关闭 |
| **Hub** | `hub.go` | 连接管理 + 广播扇出。单 goroutine 事件循环，channel 串行化 |
| **Conn** | `conn.go` | 单连接读写循环。ReadPump（解码→路由→响应）+ WritePump（send channel→WS）+ ping/pong 心跳 |
| **Router** | `router.go` | 消息路由。注册表模式 `map[string]HandlerFunc`，无 switch-case |
| **Handler** | `handler.go` | 4 个业务处理器：spawn_npc / remove_npc / publish_event / query_npc |
| **Protocol** | `pkg/protocol/message.go` | 消息信封 + 7 种消息类型 + 请求/响应/广播结构体 |

**已实现的功能**：
- 完整的 WS 协议（连接/断开/请求/响应/广播）
- 每 Tick 广播 WorldSnapshot（所有 NPC 状态快照）
- 多客户端并发连接，互不影响
- 慢客户端自动踢出（send channel 满）
- 路径穿越防护（客户端输入校验）

### Config 层 — 配置加载抽象

| 组件 | 文件 | 职责 |
|------|------|------|
| **Source 接口** | `source.go` | 5 个方法：LoadFSMConfig / LoadBTTree / LoadEventConfig / LoadAllEventConfigs / LoadNPCTypeConfig |
| **JSONSource** | `json_source.go` | 从 `configs/` 目录读取 JSON 文件。开发环境使用 |
| **MongoSource** | `mongo_source.go` | 从 MongoDB 全量加载到内存。生产环境使用 |

**已实现的功能**：
- Source 接口抽象，上层代码不感知配置来源
- JSONSource：文件读取 + JSON 校验 + 路径穿越防护
- MongoSource：启动时全量加载 → 内存 map → 断开连接，运行时零 MongoDB 依赖
- dev/prod 环境切换：`NPC_MONGO_URI` 空 → JSONSource，非空 → MongoSource

### Experiment 层 — 对照实验框架（build tag 隔离）

| 组件 | 文件 | 职责 |
|------|------|------|
| **Scenario** | `scenario.go` | 3 个定性场景（距离陷阱 / 多步骤行为 / 状态生命周期） |
| **Generator** | `generator.go` | 程序化生成 N 行为规模的 FSM/BT/Hybrid 配置 |
| **Runner** | `runner.go` | ExperimentNPC 接口 + 驱动模式跑场景 |
| **Metrics** | `metrics.go` | 指标收集 + 对比报告 |
| **5 种模式** | `modes/` | Hybrid / FSM+DC / BT+DC / PureFSM / PureBT |

**已实现的功能**：
- 5 种模式的公平实现（控制变量法）
- 3 个定性场景验证三层不可替代性
- 规模配置生成器（10/50/100/150/200 行为）
- 7 张图的数据采集测试
- build tag 完全隔离，正常构建不含实验代码

---

## 数据流

```
客户端 publish_event("explosion", pos)
  ↓
Gateway Handler → EventBus.Publish(event)
  ↓
Scheduler.Tick()
  ├→ EventBus.Tick(dt)          // TTL 衰减
  ├→ EventBus.Active()          // 获取活跃事件快照
  └→ ForEach NPC:
       ├→ Perception.CanPerceive()  // 感知过滤
       ├→ Decision.Evaluate()       // 威胁计算 → 写 BB
       └→ Instance.Tick()
            ├→ FSM.Tick(BB)         // 读 BB → 状态转换
            └→ BT.Tick(BB)         // 执行当前状态的行为树
  ↓
broadcastLoop → WorldSnapshot → Hub.Broadcast → 所有客户端
```

---

## 代码规模

| 类别 | 文件数 | 说明 |
|------|--------|------|
| 源码 | 37 | internal/ + cmd/ + pkg/ + scripts/ |
| 测试 | 26 | 单元测试 + 集成测试 + 攻击测试 + e2e |
| 配置 | 16 | server.json + NPC/FSM/BT/事件配置 |

| 层 | 源码文件 | 测试文件 |
|----|---------|---------|
| Core | 10 | 8 |
| Runtime | 5 | 7 |
| Gateway | 5 | 3 |
| Config | 3 | 2 |
| Experiment | 8 | 3 |
| Protocol | 1 | 0（e2e 覆盖） |
| 入口 | 1 | 0（e2e 覆盖） |
| 脚本 | 1 | 0（手动验证） |
| e2e | 0 | 3 |

---

## 已验证的扩展轴

| 步骤 | 内容 | 改了什么 | Go 代码改动 |
|------|------|---------|------------|
| 1 | 平民 × 3 事件 | 基线 | — |
| 2 | 平民 × 4 事件（+fire） | 加 1 个 JSON | 零 |
| 3 | (平民+警察) × 4 事件 | 加 5 个 JSON | 零 |
| 4 | 交叉验证 | 无 | 零 |

---

## 技术选型

| 技术 | 选择 | 理由 |
|------|------|------|
| 语言 | Go 1.21 | 并发原生支持，单二进制部署 |
| WebSocket | gorilla/websocket | Go 生态最成熟，Hub 模式官方推荐 |
| 数据库 | MongoDB | 文档型适合配置存储，BSON ↔ JSON 天然适配 |
| 日志 | log/slog | Go 1.21 标准库，零依赖，结构化输出 |
| 容器 | Docker Compose | server + mongo 编排，dev/prod 一套 compose |
| 测试 | Go testing | 标准框架，e2e 进程内启动服务 |
