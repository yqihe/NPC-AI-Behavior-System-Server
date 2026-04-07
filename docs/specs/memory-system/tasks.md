# memory-system 任务拆解

## T1: MemoryEntry + 记忆 CRUD 方法 (R1, R2, R3, R4, R5, R6, R7, R8)

**文件**：
- `internal/runtime/component/memory.go`
- `internal/runtime/component/memory_test.go`

**做完了是什么样**：
- 定义 `MemoryEntry{Type, TargetID, Value, Timestamp, TTL}` struct
- MemoryComponent 新增 `entries []MemoryEntry` 运行时字段
- `AddMemory(entry)`：同 Type+TargetID 强化（Value 取 max，TTL 重置）；满时淘汰 Timestamp 最小；写 BB memory_count
- `GetMemories(memType) []MemoryEntry`
- `HasMemory(memType, targetID) bool`
- `GetMemory(memType, targetID) (MemoryEntry, bool)`
- `SupportsType(t) bool`：检查 MemoryTypes 包含指定类型
- `Count() int`
- 测试覆盖：新增/强化/淘汰/读取/SupportsType

---

## T2: Tick 清理过期 + BB 写入 (R9, R10, R15)

**文件**：
- `internal/runtime/component/memory.go`
- `internal/runtime/component/memory_test.go`
- `internal/core/blackboard/keys.go`

**做完了是什么样**：
- keys.go 新增 `memory_threat_value`(float64)
- Tick 中 TTL -= dt，移除 TTL <= 0 的条目
- Tick 写 BB `memory_count` 和 `memory_threat_value`（最高威胁记忆 value，无则 0）
- 测试覆盖：TTL 衰减/过期移除/BB 写入/memory_threat_value 正确

---

## T3: Tickable 顺序保证 (R13)

**文件**：
- `internal/runtime/npc/template.go`

**做完了是什么样**：
- NewInstanceFromTemplate 中收集 tickables 后，按固定优先级排序：memory(0) → needs(1) → emotion(2) → movement(3) → 其他(99)
- 保证 memory.Tick 先于 emotion.Tick 执行

---

## T4: EmotionComponent 读取威胁记忆触发累积 (R13, R14)

**文件**：
- `internal/runtime/component/emotion.go`
- `internal/runtime/component/emotion_test.go`

**做完了是什么样**：
- Tick 中读 BB `memory_threat_value`
- 如果 > 0 且有名为 "fear" 的情绪 → fear.Value += AccumulateRate × dt
- 其余情绪保持只衰减
- 无威胁记忆（memory_threat_value=0）时恐惧只衰减（现有行为不变）
- 测试覆盖：有威胁记忆时恐惧累积、无时只衰减、无 fear 情绪不报错

---

## T5: Scheduler 写入威胁记忆 (R11, R12)

**文件**：
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- AI 管线 Decision.Evaluate 之后，如果 perceived 非空且 NPC 有 memory 组件且 SupportsType("threat")
- 取最高 Strength 的事件，用 SourceID 作为 TargetID 写入 AddMemory
- SourceID 为空时不写入
- 无 memory 组件时跳过

---

## T6: 现有测试适配 + 集成测试 (R16, R17, R18)

**文件**：
- `internal/runtime/memory_integration_test.go`
- `internal/runtime/component/memory_test.go`（benchmark 追加）

**做完了是什么样**：
- 集成测试 1：事件→记忆→情绪链路（威胁事件 → 记忆写入 → 恐惧累积 → emotion_score 升高）
- 集成测试 2：记忆过期后恢复（记忆 TTL 到期 → memory_threat_value=0 → 恐惧衰减）
- 集成测试 3：重复刺激强化（同源事件多次 → value 取 max）
- 现有集成测试和 e2e 全部通过
- Benchmark：1000 条记忆 Tick 清理 < 10μs

---

## 执行顺序

```
T1  记忆 CRUD
 └→ T2  Tick 清理 + BB Key
     └→ T3  Tickable 顺序
         └→ T4  情绪累积联动
             └→ T5  Scheduler 写入
                 └→ T6  集成测试
```

严格顺序。
