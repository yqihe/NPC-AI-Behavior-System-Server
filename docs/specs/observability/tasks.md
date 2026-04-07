# observability 任务拆解

## T1: Blackboard.Dump 方法 (R11)

**文件**：
- `internal/core/blackboard/blackboard.go`
- `internal/core/blackboard/blackboard_test.go`（追加）

**做完了是什么样**：
- `Dump() map[string]any` 方法，RLock 保护，返回全部 key-value 副本
- 测试：设置几个 key → Dump → 验证全部存在

---

## T2: 决策日志 (R1, R2)

**文件**：
- `internal/runtime/decision/decision.go`

**做完了是什么样**：
- DecisionInput 新增 `NPCID string` 字段
- Evaluate 末尾输出 slog.Debug("decision.evaluated", npc_id, threat_score, need_score, emotion_score, winner, threat_source)
- Scheduler 中 buildDecisionInput 填入 inst.ID

---

## T3: FSM 状态变迁日志 (R3, R4)

**文件**：
- `internal/core/fsm/fsm.go`（确认 SetOnTransition 方法存在）
- `internal/runtime/npc/template.go`

**做完了是什么样**：
- FSM 暴露 `SetOnTransition(fn)` 方法（如已有则跳过）
- NewInstanceFromTemplate 中注册回调：输出 slog.Debug("fsm.transition", npc_id, from, to)

---

## T4: Metrics 结构体 + PrometheusText (R5, R7, R8)

**文件**：
- `internal/runtime/metrics/metrics.go`
- `internal/runtime/metrics/metrics_test.go`

**做完了是什么样**：
- Metrics struct + RWMutex + RecordTick + PrometheusText
- PrometheusText 输出 npc_tick_total/npc_tick_duration_seconds/npc_active_count/npc_sleeping_count
- 测试：RecordTick → PrometheusText 格式正确

---

## T5: Scheduler 指标采集 (R6)

**文件**：
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- Scheduler 新增 `Metrics *metrics.Metrics` 字段
- Tick 首尾计时，末尾调用 Metrics.RecordTick
- 统计 active NPC 数、zone 分布、sleeping 数

---

## T6: /metrics HTTP 端点 (R7)

**文件**：
- `internal/gateway/server.go`
- `cmd/server/main.go`

**做完了是什么样**：
- Gateway Server 的 HTTP mux 注册 `/metrics` handler
- 读取 Scheduler.Metrics.PrometheusText() 返回 text/plain
- main.go 创建 Metrics 实例传入 Scheduler 和 Server

---

## T7: debug_npc 消息 + handler (R9, R10)

**文件**：
- `pkg/protocol/message.go`
- `internal/gateway/handler.go`

**做完了是什么样**：
- protocol 新增 TypeDebugNPC + DebugNPCRequest + DebugNPCResponse
- handler 从 Instance 提取：npc_id、template、position、fsm_state、components 列表、BB.Dump()、memories
- RegisterHandlers 注册 debug_npc

---

## T8: Scheduler DecisionInput.NPCID 适配 (R1)

**文件**：
- `internal/runtime/scheduler.go`
- `internal/runtime/decision/decision_test.go`（适配新字段）

**做完了是什么样**：
- buildDecisionInput 填入 `NPCID: inst.ID`
- v2 兼容路径也填入 NPCID
- 现有 decision 测试适配（DecisionInput 新增 NPCID 字段不影响现有测试，但 defaultInput helper 需更新）

---

## T9: 集成测试 + 全量验证 (R12, R13, R14)

**文件**：
- `internal/runtime/observability_integration_test.go`

**做完了是什么样**：
- 测试 1：Tick 后 Metrics.TickCount > 0
- 测试 2：debug_npc 返回完整 NPC 状态（BB dump 含 decision_winner）
- `go test ./...` 全绿

---

## 执行顺序

```
T1  Blackboard.Dump
 └→ T2  决策日志 + DecisionInput.NPCID
     └→ T3  FSM 变迁日志
         └→ T4  Metrics 结构体
             └→ T5  Scheduler 指标采集
                 └→ T6  /metrics HTTP
                     └→ T7  debug_npc handler
                         └→ T8  NPCID 适配
                             └→ T9  集成测试
```

严格顺序。
