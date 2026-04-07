# memory-system 设计方案

## 方案描述

### 1. MemoryEntry 结构体

```go
// 定义在 component/memory.go
type MemoryEntry struct {
    Type      string  // "threat" / "location" / "social"
    TargetID  string  // 记忆目标标识（事件 SourceID / 路点名 / NPC ID）
    Value     float64 // 记忆强度（威胁值 / 路点索引 / 好感度）
    Timestamp int64   // 创建/最后强化的时间戳（毫秒）
    TTL       float64 // 剩余生存时间（秒）
}
```

### 2. MemoryComponent 扩展

```go
type MemoryComponent struct {
    Capacity    int      `json:"capacity"`
    MemoryTypes []string `json:"memory_types"`
    DecayTime   float64  `json:"decay_time"`

    // 运行时状态（不序列化）
    entries []MemoryEntry
}
```

#### 写入（AddMemory）

```go
func (c *MemoryComponent) AddMemory(entry MemoryEntry) {
    // 1. 查找同 Type+TargetID 的已有记忆
    for i := range c.entries {
        if c.entries[i].Type == entry.Type && c.entries[i].TargetID == entry.TargetID {
            // 强化：Value 取 max，TTL 重置
            if entry.Value > c.entries[i].Value {
                c.entries[i].Value = entry.Value
            }
            c.entries[i].TTL = c.DecayTime
            c.entries[i].Timestamp = entry.Timestamp
            return
        }
    }
    // 2. 容量检查 → 淘汰最旧
    if len(c.entries) >= c.Capacity {
        oldestIdx := 0
        for i := 1; i < len(c.entries); i++ {
            if c.entries[i].Timestamp < c.entries[oldestIdx].Timestamp {
                oldestIdx = i
            }
        }
        c.entries[oldestIdx] = entry
        return
    }
    // 3. 新增
    c.entries = append(c.entries, entry)
}
```

#### 读取

```go
func (c *MemoryComponent) GetMemories(memType string) []MemoryEntry   // 按类型过滤
func (c *MemoryComponent) HasMemory(memType, targetID string) bool    // 精确检查
func (c *MemoryComponent) GetMemory(memType, targetID string) (MemoryEntry, bool)  // 精确获取
func (c *MemoryComponent) Count() int                                  // 当前条目数
```

#### Tick（清理过期）

```go
func (c *MemoryComponent) Tick(bb *blackboard.Blackboard, dt float64) {
    alive := c.entries[:0]
    for i := range c.entries {
        c.entries[i].TTL -= dt
        if c.entries[i].TTL > 0 {
            alive = append(alive, c.entries[i])
        }
    }
    c.entries = alive

    // 写 BB
    blackboard.Set(bb, blackboard.KeyMemoryCount, int64(len(c.entries)))

    // 写最高威胁记忆 value
    maxThreatVal := 0.0
    for _, e := range c.entries {
        if e.Type == "threat" && e.Value > maxThreatVal {
            maxThreatVal = e.Value
        }
    }
    blackboard.Set(bb, blackboard.KeyMemoryThreatValue, maxThreatVal)
}
```

### 3. Scheduler 写入威胁记忆

在 Decision.Evaluate 之后，如果 perceived 非空且 NPC 有 memory 组件：

```go
// 在 Scheduler Tick 的 AI 管线中，Decision.Evaluate 之后
if mem, ok := npc.GetComponent[*component.MemoryComponent](inst, "memory"); ok {
    if len(perceived) > 0 {
        // 取最高强度事件写入威胁记忆
        best := perceived[0]
        for _, pr := range perceived[1:] {
            if pr.Strength > best.Strength {
                best = pr
            }
        }
        if best.Event.SourceID != "" && mem.SupportsType("threat") {
            mem.AddMemory(component.MemoryEntry{
                Type:      "threat",
                TargetID:  best.Event.SourceID,
                Value:     best.Strength,
                Timestamp: now,
                TTL:       mem.DecayTime,
            })
        }
    }
}
```

`SupportsType(t string) bool` 检查 MemoryTypes 是否包含指定类型。

### 4. 情绪累积联动

EmotionComponent.Tick 需要知道是否有威胁记忆。但 emotion 组件不应直接依赖 memory 组件（组件间不互相引用）。

**方案**：通过 BB 间接通信。memory.Tick 写 `memory_threat_value` → emotion.Tick 读 `memory_threat_value`。

```go
// EmotionComponent.Tick 中
func (c *EmotionComponent) Tick(bb *blackboard.Blackboard, dt float64) {
    threatMemVal, _ := blackboard.Get(bb, blackboard.KeyMemoryThreatValue)

    for i := range c.EmotionStates {
        e := &c.EmotionStates[i]
        if e.Name == "fear" && threatMemVal > 0 {
            // 有威胁记忆 → 恐惧累积
            e.Value += e.AccumulateRate * dt
        } else {
            // 无威胁记忆 → 正常衰减
            e.Value = math.Max(0, e.Value - e.DecayRate * dt)
        }
    }
    // ... 写 BB emotion_dominant/emotion_dominant_val
}
```

**关键**：TickComponents 的执行顺序。tickables 列表的顺序取决于组件添加顺序。需要保证 memory.Tick（写 BB）在 emotion.Tick（读 BB）之前执行。

**解决方案**：在 NewInstanceFromTemplate 中，组件 Tick 顺序按固定优先级排序：memory → needs → emotion → movement。这不是"设计顺序"而是"逻辑依赖顺序"——数据生产者在消费者之前。

### 5. BB Key 新增

```go
var KeyMemoryThreatValue = NewKey[float64]("memory_threat_value") // 最高威胁记忆的 value
```

`memory_count` 已在需求 1 注册。

---

## 方案对比

### 备选方案：组件间直接引用（不选）

EmotionComponent 持有 `*MemoryComponent` 引用，直接调用 `mem.GetMemories("threat")`。

**不选的理由**：
1. 组件间耦合——emotion 依赖 memory 的具体实现，加新记忆类型可能需要改 emotion
2. 组件创建时需要注入依赖——Instance 工厂变复杂，需要管理组件间依赖图
3. 违反"组件是独立单元"的设计原则——通过 BB 间接通信是已有的组件通信模式

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码 NPC 参数 | **不违反** | capacity/decay_time/memory_types 从配置读取 |
| 禁止 core/ import runtime/ | **不违反** | 改动在 runtime/component 和 runtime/scheduler |
| 禁止 Blackboard 裸 map | **不违反** | 新 Key 通过 BBKey[T] 注册 |
| 禁止 Key 散落各文件 | **不违反** | 新 Key 在 keys.go |
| 禁止静默降级 | **不违反** | 无 memory 组件跳过写入，无 "threat" 类型不写入 |
| 禁止过度设计 | **不违反** | MemoryEntry 是 plain struct，无 interface |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面** | 新事件自动触发威胁记忆 |
| 加 NPC 类型 | **正面** | 不同配置的 capacity/decay_time 产生不同记忆行为 |
| NPC 间交互 | **正面** | 社交记忆为未来交互奠基 |

---

## 依赖方向

```
internal/runtime/ (Scheduler)
  → internal/runtime/component/    (MemoryComponent.AddMemory)
  → internal/runtime/decision/     (不变)
  → internal/runtime/perception/   (不变)

internal/runtime/component/memory.go
  → internal/core/blackboard/      (BB Key)

internal/runtime/component/emotion.go
  → internal/core/blackboard/      (读 KeyMemoryThreatValue)
```

组件间通过 BB 通信，无直接依赖。方向不变。

---

## 并发安全

MemoryComponent.entries 是 NPC 私有数据，Tick 内顺序操作。AddMemory 在 Scheduler ForEach 回调内调用（同一 NPC 顺序执行）。无并发风险。

---

## 配置变更

无新增配置文件。MemoryComponent 的 capacity/memory_types/decay_time 已在需求 0 Schema 和需求 1 struct 定义。

---

## 测试策略

### 单元测试

| 模块 | 覆盖 |
|------|------|
| `memory.go` | AddMemory 新增/强化/淘汰 |
| `memory.go` | GetMemories/HasMemory/GetMemory 读取 |
| `memory.go` | Tick TTL 衰减和清理 |
| `memory.go` | SupportsType 检查 |
| `memory.go` | BB 写入 memory_count 和 memory_threat_value |
| `emotion.go` | 有威胁记忆时恐惧累积 |
| `emotion.go` | 无威胁记忆时恐惧只衰减 |

### 集成测试

| 场景 | 验证 |
|------|------|
| 事件→记忆→情绪→决策链路 | 威胁事件写入记忆 → 恐惧累积 → emotion_score 升高 |
| 记忆过期后情绪恢复 | 记忆 TTL 到期 → memory_threat_value=0 → 恐惧开始衰减 |
| 重复刺激强化 | 同源事件多次 → 记忆 value 取 max，TTL 重置 |
