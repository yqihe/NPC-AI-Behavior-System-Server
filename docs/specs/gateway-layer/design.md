# Gateway 层设计方案

## 方案描述

### 整体架构

采用 **Hub + Goroutine-per-Connection** 模式：

```
Unity/e2e Client
    │ WebSocket (JSON)
    ▼
┌─────────────────────────────────────────┐
│  internal/gateway/                      │
│                                         │
│  Server (HTTP→WS升级, 生命周期管理)       │
│    │                                    │
│    ▼                                    │
│  Hub (连接注册表 + 广播扇出)              │
│    │         ▲                          │
│    ▼         │                          │
│  Conn       Conn       Conn  ...        │
│  (读goroutine → Router → Runtime调用)    │
│  (写goroutine ← send chan ← Hub广播)     │
└──────────────────┬──────────────────────┘
                   │ 调用
                   ▼
           internal/runtime/
           (Scheduler, Registry, EventBus)
```

### 组件职责

| 组件 | 文件 | 职责 |
|------|------|------|
| `Server` | `server.go` | HTTP 服务启动、WS 升级、优雅关闭 |
| `Hub` | `hub.go` | 连接注册/注销、广播扇出 |
| `Conn` | `conn.go` | 单连接的读写 goroutine、消息分发 |
| `Router` | `router.go` | 消息类型 → handler 映射（注册表模式，非 switch-case） |
| 协议定义 | `pkg/protocol/message.go` | 请求/响应/广播消息结构体 |
| 程序入口 | `cmd/server/main.go` | 初始化 Runtime + Gateway，启动服务 |

### 数据流

**客户端请求流（入站）**：
```
Client → WS → Conn.readPump → JSON 解码 → Router.Dispatch → Handler → Runtime API → 响应写入 Conn.send
```

**状态广播流（出站）**：
```
Scheduler.Tick 完成 → Hub.Broadcast(snapshot) → 每个 Conn.send channel → Conn.writePump → WS → Client
```

---

### 接口定义

#### pkg/protocol/message.go

```go
package protocol

// --- 信封 ---

// Message WS 消息信封，所有消息都用此结构包装
type Message struct {
    Type string          `json:"type"`            // 消息类型
    ID   string          `json:"id,omitempty"`    // 请求 ID（客户端生成，响应原样返回）
    Data json.RawMessage `json:"data,omitempty"`  // 载荷（延迟解析）
}

// --- 请求载荷 ---

type SpawnNPCRequest struct {
    NpcID    string  `json:"npc_id"`
    TypeName string  `json:"type_name"`
    X        float64 `json:"x"`
    Z        float64 `json:"z"`
}

type RemoveNPCRequest struct {
    NpcID string `json:"npc_id"`
}

type PublishEventRequest struct {
    EventType string  `json:"event_type"`
    X         float64 `json:"x"`
    Z         float64 `json:"z"`
    Severity  float64 `json:"severity,omitempty"` // 可选，0 则用默认值
    SourceID  string  `json:"source_id,omitempty"`
}

type QueryNPCRequest struct {
    NpcID string `json:"npc_id"`
}

// --- 响应载荷 ---

type SpawnNPCResponse struct {
    NpcID    string `json:"npc_id"`
    TypeName string `json:"type_name"`
}

type RemoveNPCResponse struct {
    NpcID string `json:"npc_id"`
}

type PublishEventResponse struct {
    EventID string `json:"event_id"`
}

type QueryNPCResponse struct {
    NpcID         string  `json:"npc_id"`
    TypeName      string  `json:"type_name"`
    X             float64 `json:"x"`
    Z             float64 `json:"z"`
    FSMState      string  `json:"fsm_state"`
    CurrentAction string  `json:"current_action"`
    ThreatLevel   float64 `json:"threat_level"`
}

type ErrorResponse struct {
    Code    string `json:"code"`    // 错误码（如 "npc_not_found"）
    Message string `json:"message"` // 人类可读描述
}

// --- 广播载荷 ---

type NPCState struct {
    NpcID         string  `json:"npc_id"`
    TypeName      string  `json:"type_name"`
    X             float64 `json:"x"`
    Z             float64 `json:"z"`
    FSMState      string  `json:"fsm_state"`
    CurrentAction string  `json:"current_action"`
    ThreatLevel   float64 `json:"threat_level"`
}

type WorldSnapshot struct {
    Tick uint64     `json:"tick"`  // 当前 Tick 序号
    NPCs []NPCState `json:"npcs"` // 所有 NPC 状态
}
```

**消息类型常量**（注册表 key）：

| Type 字段 | 方向 | 说明 |
|-----------|------|------|
| `"spawn_npc"` | C→S | 创建 NPC |
| `"remove_npc"` | C→S | 移除 NPC |
| `"publish_event"` | C→S | 发布事件 |
| `"query_npc"` | C→S | 查询 NPC 状态 |
| `"response"` | S→C | 请求响应（成功） |
| `"error"` | S→C | 请求响应（失败） |
| `"world_snapshot"` | S→C | 每 Tick 广播 |

#### internal/gateway/hub.go

```go
// Hub 管理所有活跃 WebSocket 连接
type Hub struct {
    conns      map[*Conn]struct{}  // 活跃连接集合
    register   chan *Conn          // 注册通道
    unregister chan *Conn          // 注销通道
    broadcast  chan []byte         // 广播通道（已序列化的 JSON）
    mu         sync.RWMutex       // 保护 conns（仅用于 Count 等读操作）
}

func NewHub() *Hub
func (h *Hub) Run(ctx context.Context)          // 主循环：处理注册/注销/广播
func (h *Hub) Broadcast(data []byte)            // 向广播通道发送数据
func (h *Hub) Count() int                       // 当前连接数
```

Hub.Run 是单 goroutine 事件循环，通过 channel 串行化对 conns map 的操作，避免锁竞争。Broadcast 和 Count 是少数需要从外部调用的方法，Count 用 RWMutex 保护。

#### internal/gateway/conn.go

```go
// Conn 封装单个 WebSocket 连接
type Conn struct {
    hub    *Hub
    ws     *websocket.Conn
    send   chan []byte       // 出站消息缓冲（带容量上限）
    router *Router
}

func NewConn(hub *Hub, ws *websocket.Conn, router *Router) *Conn
func (c *Conn) ReadPump()    // 读循环：解码 → 路由 → 响应（阻塞）
func (c *Conn) WritePump()   // 写循环：从 send channel 取数据发送（阻塞）
```

每个连接启动 2 个 goroutine（readPump + writePump），通过 send channel 解耦读写。

#### internal/gateway/router.go

```go
// HandlerFunc 消息处理函数签名
type HandlerFunc func(conn *Conn, msg *protocol.Message) error

// Router 消息路由器（注册表模式）
type Router struct {
    handlers map[string]HandlerFunc  // type → handler
}

func NewRouter() *Router
func (r *Router) Register(msgType string, handler HandlerFunc)
func (r *Router) Dispatch(conn *Conn, msg *protocol.Message) error
```

Router 在初始化时注册所有 handler，运行时通过 map 查找分发——**不用 switch-case**。

#### internal/gateway/server.go

```go
// Server WebSocket 网关服务
type Server struct {
    hub       *Hub
    router    *Router
    scheduler *runtime.Scheduler
    addr      string
    httpSrv   *http.Server
}

func NewServer(addr string, scheduler *runtime.Scheduler, hub *Hub, router *Router) *Server
func (s *Server) Start(ctx context.Context) error   // 启动 HTTP/WS 服务（阻塞）
func (s *Server) Shutdown(ctx context.Context) error // 优雅关闭
```

#### cmd/server/main.go

```go
func main() {
    // 1. 加载配置
    src := config.NewJSONSource("configs")
    evtTypes := loadAllEventTypes(src)
    
    // 2. 初始化 Runtime
    bus := event.NewBus()
    reg := npc.NewRegistry()
    dec := decision.NewCenter(decayRate)
    sched := runtime.NewScheduler(bus, reg, dec, evtTypes, tickRate)
    
    // 3. 初始化 Gateway
    hub := gateway.NewHub()
    router := gateway.NewRouter()
    gateway.RegisterHandlers(router, reg, bus, src, btReg, evtTypes)  // 注册所有 handler
    srv := gateway.NewServer(":9820", hub, router)
    
    // 4. 启动
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
    defer cancel()
    
    go sched.Run(ctx)        // Tick 循环
    go hub.Run(ctx)          // 连接管理循环
    go broadcastLoop(ctx, hub, reg, sched)  // 广播循环
    
    srv.Start(ctx)           // HTTP 监听（阻塞）
}
```

### 广播机制

每 Tick 结束后，构建 `WorldSnapshot` 并通过 Hub 广播：

```go
// broadcastLoop 独立于 Tick 循环，按 TickRate 频率采样并广播
func broadcastLoop(ctx context.Context, hub *Hub, reg *npc.Registry, sched *runtime.Scheduler) {
    ticker := time.NewTicker(sched.TickRate)
    defer ticker.Stop()
    var tickCount uint64
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            tickCount++
            if hub.Count() == 0 {
                continue  // 无客户端时跳过序列化
            }
            snapshot := buildSnapshot(tickCount, reg)
            data, _ := json.Marshal(protocol.Message{
                Type: "world_snapshot",
                Data: json.RawMessage(snapshotJSON),
            })
            hub.Broadcast(data)
        }
    }
}
```

广播循环独立于 Scheduler.Run，两者通过 Registry（线程安全）解耦。广播只是读取当前状态快照，不干预 Tick 逻辑。

---

## 方案对比

### 方案 A（选定）：Hub + Goroutine-per-Connection

如上所述。每个连接独立读写 goroutine，Hub 集中管理广播扇出。

**优点**：
- Go 惯用模式，gorilla/websocket 官方推荐
- 代码直观，顺序逻辑无回调
- Hub 单 goroutine 串行化避免锁竞争
- goroutine 轻量，万级连接无压力

**缺点**：
- 每连接 2 个 goroutine + 1 个 send channel，资源占用略高于事件驱动
- 毕设场景下可忽略

### 方案 B（备选）：单 goroutine Reactor 事件循环

所有连接共享一个事件循环，用 epoll 驱动读写。

**不选原因**：
- Go runtime 已在 netpoller 层做了 epoll，再包一层无收益
- 需要手动管理读写状态机，代码复杂度高
- 与 gorilla/websocket 等标准库不兼容，需要用 gobwas/ws 等底层库
- 毕设场景连接数极少，性能差异无意义
- 代码可维护性差，不利于答辩展示和后续扩展

### 方案 C（备选）：直接在 handler 中广播（无 Hub）

每个请求处理完后直接遍历所有连接发送。

**不选原因**：
- 需要全局连接列表 + 锁，handler 与连接管理耦合
- 广播和请求处理在同一 goroutine，广播慢会阻塞请求
- 连接注销时可能向已关闭连接写入

---

## 红线检查

逐条对照 `docs/architecture/red-lines.md`：

| 红线 | 状态 | 说明 |
|------|------|------|
| 禁止硬编码 FSM 状态/转换规则 | ✅ 不涉及 | Gateway 不操作 FSM |
| 禁止硬编码事件→感知映射 | ✅ 不涉及 | Gateway 不做感知 |
| 禁止硬编码 NPC 参数 | ✅ 不涉及 | Gateway 透传 type_name 给工厂 |
| 禁止 switch-case 做类型分发 | ✅ 符合 | Router 用 map 注册表，不用 switch |
| 禁止 BT 反向驱动 FSM | ✅ 不涉及 | Gateway 不操作 BT/FSM |
| 禁止 core/ import runtime/gateway/ | ✅ 符合 | 依赖方向：gateway → runtime → core |
| 禁止 gateway/ 承担非网络职责 | ✅ **核心约束** | Gateway 只做 WS 连接/路由/广播，AOI/事件发布/Tick 调度全在 Runtime |
| 禁止 Blackboard 裸 map | ✅ 不涉及 | Gateway 通过 `blackboard.Get` 泛型访问读取状态 |
| 禁止实验污染核心 | ✅ 不涉及 | Gateway 不 import experiment/ |
| 禁止过度设计 | ✅ 符合 | 不引入 Redis、认证、AOI、消息压缩 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面/中性** | `publish_event` handler 通过 event_type 字符串查找配置，新事件类型零代码改动 |
| 加 NPC 类型 | **正面/中性** | `spawn_npc` handler 通过 type_name 字符串查找配置，新 NPC 类型零代码改动 |
| NPC 间交互 | **中性** | 交互由决策中心处理，Gateway 只负责广播结果 |

**关键设计保证**：Gateway 中不出现任何 NPC 类型名或事件类型名的字面量——全部由配置和 Runtime 注册表驱动。

---

## 依赖方向

```
cmd/server/main.go
    ├─→ internal/gateway/     (Server, Hub, Conn, Router)
    ├─→ internal/runtime/     (Scheduler)
    ├─→ internal/config/      (JSONSource)
    └─→ pkg/protocol/        (消息定义)

internal/gateway/
    ├─→ internal/runtime/     (Registry, EventBus, NPC 工厂)
    ├─→ internal/core/blackboard/  (读取 NPC 状态用的 Key)
    ├─→ internal/config/      (Source 接口加载 NPC 配置)
    └─→ pkg/protocol/        (消息结构体)

pkg/protocol/
    └─→ (无内部依赖，纯数据定义)
```

依赖方向：**cmd → gateway → runtime → core**，单向向下，符合约束。

`pkg/protocol/` 是独立的协议定义包，可被 Gateway、e2e 测试、以及 Unity 侧参考。

---

## 并发安全

| 共享资源 | 访问者 | 保护方式 |
|---------|--------|---------|
| `Hub.conns` map | Hub.Run goroutine（读写）、Hub.Count（读） | Hub.Run 内通过 channel 串行化读写；Count 用 RWMutex |
| `Conn.send` channel | readPump（写入响应）、Hub（写入广播）、writePump（读取） | 带缓冲 channel，满则丢弃并关闭连接 |
| `npc.Registry` | Scheduler.Tick goroutine（读写）、Gateway handler goroutine（读写） | Registry 内部 RWMutex（已实现） |
| `event.Bus` | Scheduler.Tick goroutine（Tick/Active）、Gateway handler goroutine（Publish） | Bus 内部 RWMutex（已实现） |
| `blackboard.Blackboard` | Scheduler.Tick（写）、Gateway handler（读，query_npc） | Blackboard 内部 RWMutex（已实现） |

**风险点与对策**：

1. **Conn.send channel 满**：如果客户端接收慢，send channel 堆满。对策：send channel 带容量上限（如 256），写入时用 select + default 检测，满则关闭该连接
2. **广播序列化与 Tick 并发**：broadcastLoop 读取 Registry 快照时，Scheduler 可能正在 Tick。对策：Registry.ForEach 已有 RWMutex 保护，读到的是一致性快照
3. **handler 中 NewInstance 可能耗时**（涉及配置加载和 BT 构建）：对策：在 readPump goroutine 中同步执行即可，不阻塞其他连接

---

## 配置变更

### 新增配置文件

**`configs/server.json`**（服务端启动配置）：

```json
{
    "addr": ":9820",
    "tick_rate_ms": 100,
    "decision_decay_rate": 5.0,
    "log_level": "debug",
    "log_format": "text",
    "mongo_uri": ""
}
```

schema:

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `addr` | string | `":9820"` | WebSocket 监听地址 |
| `tick_rate_ms` | int | `100` | Tick 间隔（毫秒） |
| `decision_decay_rate` | float64 | `5.0` | 决策中心威胁衰减速率 |
| `log_level` | string | `"debug"` | 日志级别：debug / info / warn / error |
| `log_format` | string | `"text"` | 日志格式：text（可读）/ json（可解析） |
| `mongo_uri` | string | `""` | MongoDB 连接串，空则使用 JSON 文件配置源 |

环境变量可覆盖配置文件中的值（优先级：环境变量 > server.json > 默认值）。环境变量命名：`NPC_ADDR`、`NPC_LOG_LEVEL`、`NPC_MONGO_URI` 等。

### 不修改现有配置

NPC 类型配置、事件配置、FSM 配置、BT 树配置均不变。

---

## 日志方案

使用 Go 1.21 标准库 `log/slog`，结构化日志输出到 stdout，由 Docker 捕获。

### 初始化

```go
func initLogger(level, format string) {
    var lvl slog.Level
    lvl.UnmarshalText([]byte(level))

    var handler slog.Handler
    opts := &slog.HandlerOptions{Level: lvl}
    if format == "json" {
        handler = slog.NewJSONHandler(os.Stdout, opts)
    } else {
        handler = slog.NewTextHandler(os.Stdout, opts)
    }
    slog.SetDefault(slog.New(handler))
}
```

### 日志点

| 位置 | 级别 | 示例 |
|------|------|------|
| 服务启动/关闭 | INFO | `slog.Info("server.start", "addr", addr)` |
| WS 连接/断开 | INFO | `slog.Info("gateway.connect", "remote", addr, "clients", hub.Count())` |
| 消息路由 | DEBUG | `slog.Debug("router.dispatch", "type", msg.Type, "id", msg.ID)` |
| Handler 执行 | DEBUG | `slog.Debug("handler.spawn_npc", "npc_id", req.NpcID, "type", req.TypeName)` |
| Handler 错误 | WARN | `slog.Warn("handler.error", "type", msg.Type, "err", err)` |
| 广播 | DEBUG | `slog.Debug("broadcast.snapshot", "tick", tick, "npc_count", count)` |

### 备选方案：第三方日志库（zerolog/zap）

**不选原因**：slog 是标准库，零依赖，性能足够，Go 1.21+ 原生支持。毕设不需要 zerolog 的极致性能或 zap 的生态。

---

## Docker 容器化

### Dockerfile（多阶段构建）

```dockerfile
# ---- builder ----
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server ./cmd/server/

# ---- runtime ----
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/configs ./configs
EXPOSE 9820
CMD ["./server"]
```

要点：
- builder 阶段先复制 go.mod/go.sum 再复制源码，利用层缓存
- runtime 阶段只含编译后的二进制 + configs 目录
- 不使用 `go run`，确保编译期错误在构建阶段暴露

### docker-compose.yml

```yaml
services:
  server:
    build: .
    ports:
      - "${NPC_PORT:-9820}:9820"
    volumes:
      - ./configs:/app/configs:ro
    environment:
      - NPC_ADDR=:9820
      - NPC_LOG_LEVEL=${NPC_LOG_LEVEL:-debug}
      - NPC_LOG_FORMAT=${NPC_LOG_FORMAT:-text}
      - NPC_MONGO_URI=${NPC_MONGO_URI:-}
    depends_on:
      mongo:
        condition: service_started
    restart: unless-stopped

  mongo:
    image: mongo:7
    ports:
      - "${MONGO_PORT:-27017}:27017"
    volumes:
      - mongo_data:/data/db

volumes:
  mongo_data:
```

要点：
- 不包含 Redis（无使用场景，遵守红线）
- configs 目录挂载为只读卷，容器内可读取最新配置
- 环境变量通过 `.env` 注入，compose 文件中只有变量引用
- MongoDB 用命名卷持久化数据

### 环境区分（.env 文件）

**`.env`**（开发环境，git 跟踪）：

```env
NPC_PORT=9820
NPC_LOG_LEVEL=debug
NPC_LOG_FORMAT=text
NPC_MONGO_URI=
MONGO_PORT=27017
```

**`.env.prod.example`**（生产环境模板，git 跟踪；实际使用时 `cp .env.prod.example .env.prod`）：

```env
NPC_PORT=9820
NPC_LOG_LEVEL=info
NPC_LOG_FORMAT=json
NPC_MONGO_URI=mongodb://mongo:27017/npc_ai
MONGO_PORT=27017
```

切换方式：
```bash
# 开发（默认读 .env）
docker compose up --build

# 生产
docker compose --env-file .env.prod up --build -d
```

| 配置项 | 开发 | 生产 |
|--------|------|------|
| 配置源 | `configs/` JSON 文件 | MongoDB（mongo_uri 非空时） |
| 日志级别 | DEBUG | INFO |
| 日志格式 | Text | JSON |
| MongoDB | 本地容器，无认证 | 持久化卷 |

### 备选方案：dev/prod 分两个 compose 文件

**不选原因**：只有日志和配置源的差异，用 `.env` 覆盖足够。两个 compose 文件会引入维护负担（改一处要改两处）。

---

## 测试策略

### 单元测试（internal/gateway/）

| 测试文件 | 覆盖内容 |
|---------|---------|
| `router_test.go` | Router 注册、分发、未知类型返回错误 |
| `hub_test.go` | 连接注册/注销、广播扇出、无连接时广播不 panic |

### e2e 测试（test/e2e/）

使用 Go 测试代码作为无头 WS 客户端，走完整协议路径：

| 测试用例 | 关联需求 | 验证内容 |
|---------|---------|---------|
| `TestConnect` | R1 | 连接成功、断开无异常 |
| `TestSpawnAndQuery` | R2, R6 | spawn → query → 验证字段完整 |
| `TestRemoveNPC` | R3 | spawn → remove → query 返回 not_found |
| `TestPublishEvent` | R4 | spawn → publish explosion → 等待 Tick → query 验证 FSM 状态变化 |
| `TestWorldSnapshot` | R5 | spawn → 等待广播 → 验证 snapshot 包含 NPC |
| `TestMultiClient` | R7, R8 | 两个客户端连接 → spawn → 两个都收到广播 → 断开一个 → 另一个继续收 |
| `TestUnknownMessage` | - | 发送未知类型 → 收到 error 响应 |
| `TestSpawnDuplicateID` | - | 同 ID spawn 两次 → 第二次返回错误 |

### 不需要的测试

- `server.go` 不需要单独单元测试——它只是 HTTP 启动胶水代码，由 e2e 覆盖
- `conn.go` 不需要单独单元测试——readPump/writePump 是 goroutine 循环，由 e2e 覆盖

---

## 外部依赖

需要引入一个 WebSocket 库：

| 库 | 选择 | 理由 |
|---|------|------|
| `github.com/gorilla/websocket` | **选定** | Go 生态最成熟的 WS 库，API 简洁，v1 已使用过，与 Hub 模式完美适配 |
| `golang.org/x/net/websocket` | 不选 | 标准库子包但功能较弱，不支持 ping/pong 控制 |
| `github.com/gobwas/ws` | 不选 | 偏底层，适合极致性能场景，API 复杂度高 |
| `github.com/coder/websocket` (nhooyr) | 不选 | 更现代但社区规模不及 gorilla |
