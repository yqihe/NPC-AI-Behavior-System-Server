# observability 设计方案

## 方案描述

### 1. 决策日志

在 Decision.Evaluate 末尾加 slog.Debug：

```go
func (c *Center) Evaluate(...) {
    // ... 现有评分+仲裁逻辑 ...

    slog.Debug("decision.evaluated",
        "npc_id", bb.GetRaw("npc_type"), // 用 npc_type 做标识
        "threat_score", threatScore,
        "need_score", needScore,
        "emotion_score", emotionScore,
        "winner", winner,
        "threat_source", maxEventID,
    )
}
```

**NPC ID 问题**：Decision 包不知道 NPC ID（只有 BB）。解决：从 BB 读 `npc_type` 作为日志标识。或者在 DecisionInput 中加 NPCID 字段——更直接。

选择后者：`DecisionInput` 新增 `NPCID string` 字段，Scheduler 填入 inst.ID。

### 2. FSM 状态变迁日志

FSM 已有 `OnTransition` 回调机制。在 NPC 创建时注册：

```go
// template.go NewInstanceFromTemplate 中
if beh.FSM != nil {
    npcID := id // capture
    beh.FSM.SetOnTransition(func(from, to string) {
        slog.Debug("fsm.transition", "npc_id", npcID, "from", from, "to", to)
    })
}
```

需确认 FSM 是否已有 SetOnTransition 方法。

<检查>FSM 有 onTransition TransitionCallback 字段，在 NewFSM 中可设置，但目前是在 Tick 中调用。需要暴露一个 SetOnTransition 方法（如果没有）。

### 3. Metrics 结构体

```go
// internal/runtime/metrics/metrics.go
type Metrics struct {
    mu                sync.RWMutex
    TickCount         uint64
    TickDurationLast  float64            // 上一次 Tick 耗时（秒）
    ActiveNPCCount    int
    ZoneActiveCounts  map[string]int     // zone_id → 活跃 NPC 数
    ZoneSleepingCount int                // 休眠区域的 NPC 总数
}

func New() *Metrics
func (m *Metrics) RecordTick(duration float64, activeCount int, zoneCounts map[string]int, sleepingCount int)
func (m *Metrics) PrometheusText() string
```

PrometheusText 输出格式：
```
# HELP npc_tick_total Total number of ticks
# TYPE npc_tick_total counter
npc_tick_total 158432
# HELP npc_tick_duration_seconds Last tick duration
# TYPE npc_tick_duration_seconds gauge
npc_tick_duration_seconds 0.032
# HELP npc_active_count Active NPC count by zone
# TYPE npc_active_count gauge
npc_active_count{zone="meadow"} 15
npc_active_count{zone="forest"} 0
# HELP npc_sleeping_count Total sleeping NPCs
# TYPE npc_sleeping_count gauge
npc_sleeping_count 25
```

不引入 prometheus client 库——手动拼文本，零依赖。

### 4. Scheduler 指标采集

```go
func (s *Scheduler) Tick(dt float64) {
    start := time.Now()
    // ... 现有逻辑 ...
    duration := time.Since(start).Seconds()

    if s.Metrics != nil {
        zoneCounts := map[string]int{}
        sleepingCount := 0
        // 从 ZoneManager 统计
        s.Metrics.RecordTick(duration, len(states), zoneCounts, sleepingCount)
    }
}
```

### 5. /metrics HTTP 端点

在 Gateway Server 上注册 HTTP handler（不走 WS）：

```go
// server.go 中 HTTP mux
mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/plain")
    w.Write([]byte(metrics.PrometheusText()))
})
```

与 WS upgrade handler 共存在同一 HTTP server 上——WS 只在 `/ws` 路径，`/metrics` 是普通 HTTP。

### 6. Blackboard.Dump

```go
// blackboard.go
func (bb *Blackboard) Dump() map[string]any {
    bb.mu.RLock()
    defer bb.mu.RUnlock()
    result := make(map[string]any, len(bb.data))
    for k, v := range bb.data {
        result[k] = v
    }
    return result
}
```

### 7. debug_npc 消息

```go
// protocol
const TypeDebugNPC = "debug_npc"

type DebugNPCRequest struct {
    NpcID string `json:"npc_id"`
}

type DebugNPCResponse struct {
    NpcID      string            `json:"npc_id"`
    Template   string            `json:"template"`
    Position   DebugPosition     `json:"position"`
    FSMState   string            `json:"fsm_state"`
    Components []string          `json:"components"`
    Blackboard map[string]any    `json:"blackboard"`
    Memories   []DebugMemory     `json:"memories,omitempty"`
}
```

Handler 从 Instance 提取全部信息：组件名列表、BB.Dump()、MemoryComponent.GetMemories("threat") 等。

---

## 方案对比

### 备选方案：引入 OpenTelemetry SDK（不选）

接入 OTel SDK，用 Span 追踪每次 Tick 的完整链路。

**不选的理由**：
1. 引入重量级依赖（OTel SDK + exporter），违反"禁止引入没有使用场景的依赖"
2. 单进程 slog + 手动指标已满足当前需求
3. OTel 适合分布式微服务，单进程 AI 系统用牛刀

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止 core/ import runtime/ | **需评估** | FSM 的 OnTransition 回调携带 npc_id 是通过闭包捕获，FSM 包本身不 import runtime |
| 禁止 Gateway 承担非网络职责 | **不违反** | /metrics 是 HTTP 响应，debug_npc 是 WS 查询，都是网络职责 |
| 禁止过度设计 | **不违反** | 手动 Prometheus text 而非引入 client 库 |
| 禁止静默降级 | **不违反** | 日志是 Debug 级别，不是静默忽略 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | 中性 | 新事件的决策过程自动被日志记录 |
| 加 NPC 类型 | 中性 | 新 NPC 自动出现在指标和 debug 查询中 |
| NPC 间交互 | 中性 | 群组信息可通过 debug 查询观察 |

---

## 依赖方向

```
internal/runtime/metrics/
  → (无依赖，纯数据)

internal/runtime/ (Scheduler)
  → internal/runtime/metrics/    (RecordTick)

internal/runtime/decision/
  → (slog, 无新依赖)

internal/gateway/
  → internal/runtime/metrics/    (/metrics handler)
  → internal/runtime/npc/        (debug_npc handler)
  → internal/runtime/component/  (读取组件信息)
```

无循环。

---

## 并发安全

Metrics struct 被 Scheduler goroutine 写、HTTP handler goroutine 读。RWMutex 保护。Blackboard.Dump 使用 BB 自带的 RWMutex。

---

## 配置变更

无新增配置文件。

---

## 测试策略

### 单元测试

| 模块 | 覆盖 |
|------|------|
| `metrics/` | RecordTick + PrometheusText 格式正确 |
| `blackboard/` | Dump 返回全部 key-value |

### 集成测试

| 场景 | 验证 |
|------|------|
| debug_npc 查询 | 返回 NPC 完整状态（BB dump + 组件列表 + 记忆） |
| /metrics 端点 | 返回 Prometheus text 格式，含 tick_total |
