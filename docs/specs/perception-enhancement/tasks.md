# perception-enhancement 任务拆解

## T1: PerceiveResult + CalcStrength + ShouldFilterByZone (R1, R2, R3, R4, R10, R11, R12, R13) [x]

**文件**：
- `internal/runtime/perception/perception.go`
- `internal/runtime/perception/perception_test.go`

**做完了是什么样**：
- 新增 `PerceiveResult{Event, Strength}` 结构体
- 新增 `CalcStrength(npcPos, cfg, evt, evtTypeCfg) float64`：visual/auditory 返回 `severity × max(0, 1 - dist/min(npcRange, evtRange))`，global 返回 severity，超出范围返回 0
- 新增 `ShouldFilterByZone(npcZoneID, evtZoneID, perceptionMode) bool`：global 返回 false，任一为空返回 false，不同区域返回 true
- `CanPerceive` 保留，改为 `return CalcStrength(...) > 0`
- 测试覆盖：各 mode 强度计算（0/中间/边界/超出）、global 强度=severity、区域过滤各种组合、CanPerceive 兼容

---

## T2: Event 新增 ZoneID (R8, R9) [x]

**文件**：
- `internal/runtime/event/event.go`
- `pkg/protocol/message.go`
- `internal/gateway/handler.go`

**做完了是什么样**：
- `Event` struct 新增 `ZoneID string` 字段
- `NewEvent` 函数签名新增 `zoneID string` 参数
- `PublishEventRequest` 新增 `ZoneID string json:"zone_id,omitempty"`
- `makePublishEventHandler` 传入 `req.ZoneID`
- 所有现有 `NewEvent` 调用点补空字符串（向后兼容）

---

## T3: Decision.Evaluate 接收 PerceiveResult (R15) [x]

**文件**：
- `internal/runtime/decision/decision.go`
- `internal/runtime/decision/decision_test.go`

**做完了是什么样**：
- `Evaluate` 签名从 `events []*event.Event` 改为 `perceived []perception.PerceiveResult`
- 遍历 perceived，直接取 `pr.Strength` 作为威胁值（不调用 CalcThreat）
- `CalcThreat` 函数保留不删
- 测试适配新签名，验证 Strength 直接使用

---

## T4: Scheduler filterPerception 重构 (R5, R6, R7, R14) [x]

**文件**：
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- `filterPerception` 返回 `[]perception.PerceiveResult`（替代 `[]*event.Event`）
- 三步流程：区域过滤（ShouldFilterByZone）→ 强度计算（CalcStrength）→ 注意力裁剪（sort + truncate by AttentionCapacity）
- AttentionCapacity ≤ 0 时不裁剪
- v2 兼容路径（inst.Perception）保持返回旧类型，Decision.Evaluate 调用处包装为 PerceiveResult

---

## T5: v2 兼容路径适配 (R16) [x]

**文件**：
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- v2 兼容路径（`inst.Perception != nil` 分支）中，CanPerceive 过滤后的事件包装为 `PerceiveResult{Event: evt, Strength: CalcThreat(...)}`
- Decision.Evaluate 调用统一使用 `[]perception.PerceiveResult`
- 不存在两种签名的 Evaluate

---

## T6: 集成测试 — 注意力裁剪 + 区域隔离 + 强度传递 (R5, R10, R17) [x]

**文件**：
- `internal/runtime/perception_integration_test.go`

**做完了是什么样**：
- 测试 1 注意力裁剪：5 个事件 + capacity=3，验证 Decision 只收到 3 个最强事件
- 测试 2 区域隔离：NPC zone=meadow，事件 zone=mountain（auditory），验证不感知；global 事件验证仍可感知
- 测试 3 强度传递：近距离和远距离两个事件，验证决策中心选择强度更高的近距离事件
- 现有 6 个集成测试场景通过

---

## T7: 现有测试修复 + e2e 验证 (R17, R18) [x]

**文件**：
- `internal/runtime/integration_test.go`（适配 NewEvent 签名）
- `internal/runtime/component_integration_test.go`（适配 NewEvent 签名）
- `test/e2e/helpers_test.go`（适配 NewEvent 签名如需要）

**做完了是什么样**：
- 所有 `NewEvent(...)` 调用补 zoneID 空字符串参数
- `go test ./...` 全部通过

---

## T8: Benchmark (R19) [x]

**文件**：
- `internal/runtime/benchmark_test.go`

**做完了是什么样**：
- 新增 `BenchmarkPerceptionFilter_100NPC_10Events`：100 NPC × 10 事件含强度计算+排序+裁剪，< 1ms
- 现有 benchmark 适配新签名

---

## 执行顺序

```
T1  CalcStrength + ShouldFilterByZone + PerceiveResult
 └→ T2  Event.ZoneID + protocol + handler
     └→ T3  Decision.Evaluate 新签名
         └→ T4  Scheduler filterPerception 重构
             └→ T5  v2 兼容路径适配
                 └→ T6  集成测试
                     └→ T7  现有测试修复 + e2e
                         └→ T8  Benchmark
```

严格顺序，每步依赖上一步的类型/签名变更。
