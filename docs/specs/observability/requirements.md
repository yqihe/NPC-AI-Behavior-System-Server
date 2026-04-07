# observability 需求分析

## 动机

v3 的 AI 角色系统已具备感知、决策、记忆、移动、社交、区域六大子系统，NPC 行为复杂度大幅提升。但当 NPC 行为不符合预期时，开发者和运营无法快速定位原因：

1. **无法回答"为什么"**：NPC 突然从 Idle 跳到 Flee——是因为感知到了什么事件？决策中心的三维评分是多少？哪个维度胜出？当前没有任何日志记录决策过程。
2. **无性能基线**：1000 NPC 的 Tick 耗时多少？每个区域有多少活跃 NPC？休眠了多少？没有指标就无法设告警、无法做容量规划。
3. **调试靠猜**：排查问题时只能加临时 log 重启 → 复现 → 看日志 → 删 log。企业级系统需要运行时查询任意 NPC 的完整内部状态。

可观测性是将 AI 系统从"能跑"推进到"能运营"的最后一步。

## 优先级

**中**。v3 的最后一个需求，依赖需求 1-7 全部完成。不阻塞其他需求。

## 预期效果

### 场景 1：决策日志

NPC wolf_1 从 Idle 转到 Flee，slog 输出：
```
level=DEBUG msg=decision.evaluated npc_id=wolf_1 threat_score=72.5 need_score=15.0 emotion_score=45.0 winner=threat threat_source=bomb_1
```
运营看到这条日志就知道：是 bomb_1 的威胁分最高（72.5 × threat_weight），所以决策结果是 threat。

### 场景 2：FSM 状态变迁日志

```
level=DEBUG msg=fsm.transition npc_id=wolf_1 from=Idle to=Alarmed trigger=last_event_type!=""
level=DEBUG msg=fsm.transition npc_id=wolf_1 from=Alarmed to=Flee trigger=threat_level>=50
```

### 场景 3：性能指标（Prometheus 格式）

HTTP 端点 `GET /metrics` 返回：
```
npc_tick_duration_seconds{quantile="0.99"} 0.032
npc_active_count{zone="meadow"} 15
npc_active_count{zone="forest"} 0
npc_sleeping_count 25
npc_tick_total 158432
```

### 场景 4：NPC 调试查询

WS 消息 `debug_npc` 或 HTTP 端点 `GET /debug/npc/{id}` 返回：
```json
{
  "npc_id": "wolf_1",
  "template": "wolf_common",
  "position": {"x": 312.5, "z": 418.2},
  "fsm_state": "Flee",
  "components": ["identity","position","behavior","perception","movement","personality"],
  "blackboard": {
    "threat_level": 72.5,
    "decision_winner": "threat",
    "move_state": "moving",
    "emotion_dominant": "fear",
    "emotion_dominant_val": 45.0
  },
  "memories": [
    {"type":"threat","target_id":"bomb_1","value":72.5,"ttl":48.3}
  ]
}
```

## 依赖分析

- **依赖**：需求 1-7 全部完成
- **被依赖**：无（v3 最后一个需求）

## 改动范围

| 包 | 变更类型 | 文件数 | 说明 |
|----|---------|--------|------|
| `internal/runtime/decision/` | **修改** | 1 | Evaluate 末尾加决策日志 |
| `internal/core/fsm/` | **修改** | 1 | 状态转换回调加日志 |
| `internal/runtime/scheduler.go` | **修改** | 1 | Tick 计时 + 指标采集 |
| `internal/runtime/metrics/` | **新增** | 2 | 指标收集器 + Prometheus 导出 |
| `internal/gateway/` | **修改** | 2 | debug_npc handler + /metrics HTTP 端点 |
| `pkg/protocol/` | **修改** | 1 | debug_npc 消息类型 |
| `internal/core/blackboard/` | **修改** | 1 | Dump 方法（导出全部 key-value） |
| 测试文件 | **新增** | 2-3 | metrics 测试、debug 查询测试 |

预估 12-15 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **间接** | 新事件的感知/决策过程自动被日志记录 |
| 加 NPC 类型 | **间接** | 新 NPC 类型自动出现在指标和调试查询中 |
| NPC 间交互 | **间接** | 群组状态可通过调试查询观察 |

可观测性不直接服务于扩展轴，而是服务于系统的**可运营性**——企业级系统的基础要求。

## 验收标准

### 决策日志

- **R1**：Decision.Evaluate 每次执行后输出 slog.Debug，包含 npc_id、threat_score、need_score、emotion_score、winner、threat_source
- **R2**：日志格式遵循 `component.action` 命名规范

### FSM 状态变迁日志

- **R3**：FSM 状态转换时输出 slog.Debug，包含 npc_id、from、to
- **R4**：NPC ID 通过 FSM 的 OnTransition 回调传入（需在 NPC 创建时注册回调）

### 性能指标

- **R5**：定义 `Metrics` 结构体，记录：tick_count(uint64)、tick_duration_last(float64 秒)、active_npc_count(int)、zone_active_counts(map[string]int)、zone_sleeping_count(int)
- **R6**：Scheduler.Tick 末尾更新 Metrics
- **R7**：HTTP 端点 `GET /metrics` 输出 Prometheus text 格式
- **R8**：指标采集本身不增加超过 1% 的 Tick 耗时

### NPC 调试查询

- **R9**：WS 消息 `debug_npc`（请求 `{npc_id}`）返回 NPC 完整内部状态
- **R10**：返回内容包含：npc_id、template、position、fsm_state、components 列表、blackboard 全量 dump、memories 列表
- **R11**：Blackboard 新增 `Dump() map[string]any` 方法导出全部已注册 key 的值

### 向后兼容

- **R12**：所有新增日志为 Debug 级别，生产环境 Info 级别不受影响
- **R13**：现有测试全部通过
- **R14**：/metrics 和 debug_npc 不影响现有 WS 消息处理

## 不做什么

- **不做分布式追踪**：不接 OpenTelemetry/Jaeger，单进程 slog 足够
- **不做日志持久化**：日志输出到 stdout，持久化由部署层（Docker log driver）处理
- **不做告警**：指标导出后由外部 Prometheus + AlertManager 处理
- **不做 ADMIN 可视化**：NPC 状态看板由 ADMIN 平台消费 debug_npc 接口实现
- **不做 BT 逐节点追踪**：只记录 BT Tick 的最终 Status，不记录每个中间节点。逐节点追踪需要改 core/bt，侵入性太强
