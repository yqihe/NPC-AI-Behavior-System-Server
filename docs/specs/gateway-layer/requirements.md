# Gateway 层需求分析

## 动机

当前系统 Core 层（FSM/BT/Blackboard/Rule 引擎）和 Runtime 层（事件总线/NPC 调度/决策中心/感知过滤）已完整实现并通过测试，但没有网络入口——无法与 Unity 客户端通信。

**不做会怎样**：
- 整个服务端只是一个可测试的库，无法作为独立服务运行
- Unity 客户端无法接入（v1 已有联调经验和 WS 协议格式可复用）
- 无法进行真实场景的系统演示和答辩展示
- e2e 测试无法走 WS 协议验证端到端行为

Gateway 是将"库"变成"服务"的关键层。

## 优先级

**高**——这是最后一个核心功能层。Core → Runtime → Experiment 均已完成，Gateway 是上线和答辩演示的前置条件。

## 预期效果

### 场景 1：Unity 客户端连接并观察 NPC 行为

1. Unity 客户端通过 WebSocket 连接到服务端 `ws://host:port/ws`
2. 客户端发送 `spawn_npc` 消息，指定 NPC 类型和位置
3. 服务端创建 NPC 实例，返回确认
4. 服务端每 Tick 广播所有 NPC 的状态快照（位置、FSM 状态、当前行为）
5. 客户端根据状态快照驱动 NPC 表现

### 场景 2：客户端触发事件

1. 客户端发送 `publish_event` 消息（如爆炸事件，指定位置和严重度）
2. 服务端将事件投入 EventBus
3. NPC 在下一个 Tick 感知并响应事件
4. 客户端通过状态广播观察 NPC 行为变化（如平民逃跑）

### 场景 3：GM 面板调试

1. 客户端发送查询命令（如 `query_npc`）获取指定 NPC 的详细状态
2. 客户端发送 `remove_npc` 移除指定 NPC
3. 支持 GM 面板实时调试整个系统

### 场景 4：e2e 测试

1. Go 测试代码作为"无头客户端"通过 WS 连接服务端
2. 发送 spawn、event、query 消息
3. 验证 NPC 行为响应符合预期
4. 与 Unity 客户端走同一入口，协议完全一致

## 依赖分析

### 依赖的已完成工作

| 依赖项 | 状态 | Gateway 使用方式 |
|--------|------|-----------------|
| `runtime.Scheduler` | 已完成 | 创建并启动 Tick 循环 |
| `npc.Registry` | 已完成 | 增删查 NPC 实例 |
| `npc.NewInstance()` | 已完成 | 工厂创建 NPC |
| `event.Bus` | 已完成 | 发布事件 |
| `event.NewEvent()` | 已完成 | 创建事件实例 |
| `config.Source` | 已完成 | 加载 NPC/Event 配置 |

### 谁依赖 Gateway

| 依赖方 | 依赖内容 |
|--------|----------|
| `cmd/server/main.go` | 程序入口，初始化并启动 Gateway |
| `test/e2e/` | 通过 WS 协议验证端到端行为 |
| Unity 客户端 | 通过 WS 协议与服务端交互 |

## 改动范围

| 包 | 预估文件数 | 说明 |
|----|-----------|------|
| `pkg/protocol/` | 1-2 | WS 消息协议定义（请求/响应/广播类型） |
| `internal/gateway/` | 3-4 | WS Server、连接管理、消息路由、状态广播 |
| `cmd/server/` | 1 | 程序入口 main.go |
| `test/e2e/` | 1-2 | WS 协议 e2e 测试 |
| 项目根目录 | 3 | Dockerfile、docker-compose.yml、.env |
| `configs/` | 1 | server.json 服务端配置 |
| 总计 | 10-13 个文件 | |

**不改动**：`internal/core/`、`internal/runtime/`、`internal/config/`、`internal/experiment/`——Gateway 只是 Runtime 的消费者。

## 扩展轴检查

Gateway 不直接服务于三个扩展轴（加事件源、加 NPC 类型、NPC 间交互），但它是扩展轴可验证的**必要前提**：

- **加事件源**：客户端通过 `publish_event` 触发任意已配置的事件类型，Gateway 不关心具体类型——由 Runtime 处理
- **加 NPC 类型**：客户端通过 `spawn_npc` 创建任意已配置的 NPC 类型，Gateway 不关心具体类型——由 NPC 工厂处理
- Gateway 的设计必须确保：**新增事件类型或 NPC 类型时，Gateway 代码零改动**

## 验收标准

| 编号 | 验收标准 | 验证方式 |
|------|---------|---------|
| R1 | 服务端启动后在指定端口监听 WebSocket 连接，客户端可连接和断开 | e2e 测试：连接 → 收到确认 → 断开 |
| R2 | 客户端发送 `spawn_npc` 后，NPC 被创建并加入 Registry，返回成功响应 | e2e 测试：spawn → query 验证存在 |
| R3 | 客户端发送 `remove_npc` 后，NPC 被移除，返回成功响应 | e2e 测试：spawn → remove → query 验证不存在 |
| R4 | 客户端发送 `publish_event` 后，事件被投入 EventBus | e2e 测试：spawn NPC → publish event → 等待 Tick → 查询 NPC 状态变化 |
| R5 | 服务端每 Tick 向所有连接的客户端广播 NPC 状态快照（ID、类型、位置、FSM 状态、当前行为） | e2e 测试：spawn → 等待广播 → 验证内容 |
| R6 | 客户端发送 `query_npc` 可获取指定 NPC 的详细状态 | e2e 测试：spawn → query → 验证字段 |
| R7 | 多个客户端可同时连接，每个都能收到广播 | e2e 测试：两个客户端连接 → spawn → 两个都收到广播 |
| R8 | 客户端断开不影响服务端和其他客户端 | e2e 测试：两个客户端 → 断开一个 → 另一个继续收广播 |
| R9 | 新增 NPC 类型或事件类型时，Gateway 层代码零改动 | 代码审查：Gateway 中无类型 switch-case |
| R10 | `pkg/protocol/` 消息格式可被 Unity C# 和 Go e2e 测试共同使用 | 协议定义为 JSON，结构清晰可跨语言 |
| R11 | `docker compose up --build` 可一键启动完整服务（Go 服务端 + MongoDB） | 手动测试：容器启动成功，WS 端口可连接 |
| R12 | 服务端使用 `log/slog` 结构化日志，开发环境 Text 格式 DEBUG 级别，生产环境 JSON 格式 INFO 级别 | 代码审查 + 日志输出验证 |
| R13 | 通过 `.env` 文件区分开发/生产环境，同一份 `docker-compose.yml` 适配两种环境 | 代码审查：无硬编码环境值 |

## 不做什么

- **不做 AOI 优化**：v1 教训，Gateway 不做空间计算。当前阶段全量广播即可，AOI 是 `internal/runtime/world/` 的未来职责
- **不做认证/鉴权**：毕设阶段不需要
- **不做 Admin 管理接口**：`internal/admin/` 独立于 Gateway，后续按需开发
- **不做消息压缩/二进制协议**：JSON 明文足够，便于调试
- **不做负载均衡/多节点**：单节点足够满足毕设需求
- **不做持久化连接状态**：客户端断线重连视为新连接
- **不做 Redis 集成**：当前阶段不需要，避免 v1 的死代码问题
- **不做独立日志服务**：slog 输出到 stdout，Docker 捕获即可，不引入 ELK/Loki 等
- **不做监控服务**：不引入 Prometheus/Grafana，实验数据走 benchmark
