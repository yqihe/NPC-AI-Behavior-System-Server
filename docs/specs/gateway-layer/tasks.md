# Gateway 层任务拆解

## [x] T1: 定义 WS 消息协议 (R10)

**文件**：
- `pkg/protocol/message.go`

**产出**：
- `Message` 信封结构体（Type/ID/Data）
- 所有请求载荷结构体：`SpawnNPCRequest`、`RemoveNPCRequest`、`PublishEventRequest`、`QueryNPCRequest`
- 所有响应载荷结构体：`SpawnNPCResponse`、`RemoveNPCResponse`、`PublishEventResponse`、`QueryNPCResponse`、`ErrorResponse`
- 广播载荷结构体：`NPCState`、`WorldSnapshot`
- 消息类型常量：`TypeSpawnNPC`、`TypeRemoveNPC`、`TypePublishEvent`、`TypeQueryNPC`、`TypeResponse`、`TypeError`、`TypeWorldSnapshot`
- 辅助函数：`NewResponse(id, data)` → 序列化为 `Message{Type: "response"}`、`NewError(id, code, msg)` → 序列化为 `Message{Type: "error"}`

**做完了是什么样**：
- `go build ./pkg/protocol/` 编译通过
- 结构体 JSON tag 正确，可序列化/反序列化

---

## [x] T2: 实现 Hub 连接管理 (R7, R8)

**文件**：
- `internal/gateway/hub.go`
- `internal/gateway/hub_test.go`

**产出**：
- `Hub` 结构体：conns map、register/unregister/broadcast channel
- `NewHub()` 构造函数
- `Hub.Run(ctx)` 主循环：select 处理注册/注销/广播/ctx 取消
- `Hub.Broadcast(data []byte)` 向广播通道发送
- `Hub.Count() int` 当前连接数

**做完了是什么样**：
- 单元测试验证：注册 → Count=1 → 广播 → 注销 → Count=0
- 无连接时 Broadcast 不阻塞不 panic

---

## [x] T3: 实现 Conn 读写循环 (R1)

**文件**：
- `internal/gateway/conn.go`

**产出**：
- `Conn` 结构体：hub、ws、send channel、router
- `NewConn(hub, ws, router)` 构造函数
- `ReadPump()`：循环读取 WS 消息 → JSON 解码为 `protocol.Message` → `router.Dispatch` → 响应写入 send channel。连接关闭时通知 Hub 注销
- `WritePump()`：循环从 send channel 取数据 → 写入 WS。设置写超时和 ping/pong 心跳
- send channel 满时关闭连接（防慢客户端阻塞）

**做完了是什么样**：
- 编译通过，ReadPump/WritePump 逻辑完整
- 由 e2e 测试（T7）覆盖验证

---

## [x] T4: 实现 Router 消息路由 (R9)

**文件**：
- `internal/gateway/router.go`
- `internal/gateway/router_test.go`

**产出**：
- `HandlerFunc` 类型定义：`func(conn *Conn, msg *protocol.Message) error`
- `Router` 结构体：`handlers map[string]HandlerFunc`
- `NewRouter()` 构造函数
- `Register(msgType, handler)` 注册处理器
- `Dispatch(conn, msg) error` 按 type 查找并调用 handler，未知类型返回错误

**做完了是什么样**：
- 单元测试验证：注册 handler → dispatch 正确类型 → handler 被调用；dispatch 未知类型 → 返回错误
- 无 switch-case

---

## [x] T5: 实现业务 Handler + Server 启动 (R2, R3, R4, R6)

**文件**：
- `internal/gateway/handler.go`
- `internal/gateway/server.go`

**产出**：

`handler.go`：
- `RegisterHandlers(router, registry, bus, src, evtTypes)` 一次性注册所有 handler
- `handleSpawnNPC`：解析请求 → `npc.NewInstance` → `registry.Add` → 返回成功响应
- `handleRemoveNPC`：`registry.Get` 检查存在 → `registry.Remove` → 返回成功响应
- `handlePublishEvent`：查找事件类型配置 → `event.NewEvent` → `bus.Publish` → 返回成功响应（含 event_id）
- `handleQueryNPC`：`registry.Get` → 读取 Blackboard 状态 → 返回 `QueryNPCResponse`

`server.go`：
- `Server` 结构体：hub、router、scheduler、addr、httpSrv
- `NewServer(addr, scheduler, hub, router)` 构造函数
- `Start(ctx)` 方法：注册 `/ws` HTTP handler（升级 WS → 创建 Conn → 启动读写 goroutine）、启动 HTTP 监听
- `Shutdown(ctx)` 方法：优雅关闭 HTTP 服务

**做完了是什么样**：
- 编译通过，handler 正确调用 Runtime API
- handler 中无 NPC 类型名/事件类型名字面量
- 由 e2e 测试（T7）覆盖验证

---

## [x] T6: 实现 main.go 入口 + 广播循环 + 服务端配置 + slog 日志 (R5, R11, R12)

**文件**：
- `cmd/server/main.go`
- `configs/server.json`

**产出**：

`main.go`：
- 加载 `configs/server.json` 获取 addr、tick_rate_ms、decision_decay_rate、log_level、log_format
- 环境变量覆盖：`NPC_ADDR`、`NPC_LOG_LEVEL`、`NPC_LOG_FORMAT`、`NPC_MONGO_URI`
- 初始化 slog（根据 log_level + log_format 配置 handler）
- 初始化 config.Source → EventBus → Registry → Decision Center → Scheduler
- 初始化 Hub → Router → RegisterHandlers → Server
- 启动 goroutine：`scheduler.Run(ctx)`、`hub.Run(ctx)`、`broadcastLoop(ctx, hub, reg, tickRate)`
- 监听 SIGINT 信号优雅关闭
- `broadcastLoop` 函数：按 tickRate 频率用 `registry.ForEach` 构建 `WorldSnapshot` → JSON 序列化 → `hub.Broadcast`

`configs/server.json`：
- `{"addr": ":9820", "tick_rate_ms": 100, "decision_decay_rate": 5.0, "log_level": "debug", "log_format": "text", "mongo_uri": ""}`

**做完了是什么样**：
- `go run cmd/server/main.go` 启动成功，slog 输出结构化日志
- 设置 `NPC_LOG_LEVEL=info` 后 DEBUG 日志不输出

---

## [x] T7: Docker 容器化 + 环境配置 (R11, R13)

**文件**：
- `Dockerfile`
- `docker-compose.yml`
- `.env`

**产出**：

`Dockerfile`：
- 多阶段构建：golang:1.21-alpine builder → alpine:3.19 runtime
- 先复制 go.mod/go.sum 利用层缓存，再复制源码 `go build`
- runtime 阶段仅含二进制 + configs 目录

`docker-compose.yml`：
- `server` 服务：本地构建，挂载 configs 只读卷，环境变量从 `.env` 注入
- `mongo` 服务：mongo:7 镜像，命名卷持久化
- 不含 Redis（无使用场景）

`.env`（开发环境默认值，git 跟踪）：
- `NPC_PORT=9820`、`NPC_LOG_LEVEL=debug`、`NPC_LOG_FORMAT=text`、`NPC_MONGO_URI=`
- `.env.prod` 由运维手动创建，git 忽略

**做完了是什么样**：
- `docker compose up --build` 一键启动 server + mongo
- `docker compose logs -f server` 可看到 slog 结构化日志
- `wscat -c ws://localhost:9820/ws` 可连接（手动验证）

---

## [x] T8: e2e 测试 (R1-R8)

**文件**：
- `test/e2e/gateway_test.go`
- `test/e2e/helpers_test.go`

**产出**：

`helpers_test.go`：
- `startTestServer(t) (url string, cleanup func)`：在随机端口启动完整服务，返回 WS URL
- `dial(t, url) *websocket.Conn`：连接 WS 并注册 cleanup
- `sendAndRecv(t, conn, msg) protocol.Message`：发送请求并等待响应（带超时）
- `waitForSnapshot(t, conn) protocol.WorldSnapshot`：等待下一个 world_snapshot 广播

`gateway_test.go`：
- `TestConnect`：连接 → 断开无异常 (R1)
- `TestSpawnAndQuery`：spawn → query → 验证字段 (R2, R6)
- `TestRemoveNPC`：spawn → remove → query 返回 not_found (R3)
- `TestPublishEvent`：spawn civilian → publish explosion → 等待数个 Tick → query 验证 FSM 状态非 Idle (R4)
- `TestWorldSnapshot`：spawn → 等待广播 → 验证 snapshot 包含该 NPC (R5)
- `TestMultiClient`：两客户端连接 → spawn → 两个都收到广播 → 断开一个 → 另一个继续收 (R7, R8)
- `TestUnknownMessage`：发送未知类型 → 收到 error 响应
- `TestSpawnDuplicateID`：同 ID spawn 两次 → 第二次返回错误

**做完了是什么样**：
- `go test ./test/e2e/... -v` 全部通过
- `-race` 无竞态报告

---

## 依赖顺序

```
T1 (protocol)
 ↓
T2 (hub) ←── T4 (router)
 ↓              ↓
T3 (conn) ←────┘
 ↓
T5 (handler + server)
 ↓
T6 (main.go + broadcast + config + slog)
 ↓
T7 (Docker + 环境配置)
 ↓
T8 (e2e tests)
```

T1 必须最先，是所有后续任务的基础。
T2 和 T4 可并行。
T3 依赖 T2（Hub）和 T4（Router）。
T5 依赖 T3（Conn）。
T6 依赖 T5（Server）。
T7 依赖 T6（需要可编译的服务端）。
T8 依赖 T7（e2e 测试在容器或本地均可运行）。
