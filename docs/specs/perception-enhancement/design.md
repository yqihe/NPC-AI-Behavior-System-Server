# perception-enhancement 设计方案

## 方案描述

### 1. PerceiveResult 结构体

```go
// 定义在 perception 包
type PerceiveResult struct {
    Event    *event.Event
    Strength float64 // 0.0~severity，距离衰减后的感知强度
}
```

### 2. CalcStrength 替代 CanPerceive

```go
// CalcStrength 计算 NPC 对事件的感知强度
// 返回 0 表示不可感知，>0 表示感知强度
// 公式：strength = severity × max(0, 1 - distance / min(npc_range, event_range))
func CalcStrength(npcPos event.Vec3, cfg *PerceptionConfig, evt *event.Event, evtTypeCfg *event.EventTypeConfig) float64 {
    switch evtTypeCfg.PerceptionMode {
    case "global":
        return evt.Severity
    case "visual":
        return calcRangeStrength(npcPos, evt, cfg.VisualRange, evtTypeCfg.Range)
    case "auditory":
        return calcRangeStrength(npcPos, evt, cfg.AuditoryRange, evtTypeCfg.Range)
    default:
        return 0
    }
}

func calcRangeStrength(npcPos event.Vec3, evt *event.Event, npcRange, evtRange float64) float64 {
    maxRange := min(npcRange, evtRange)
    if maxRange <= 0 {
        return 0
    }
    dist := event.Distance(npcPos, evt.Position)
    factor := 1 - dist/maxRange
    if factor <= 0 {
        return 0
    }
    return evt.Severity * factor
}
```

**CanPerceive 保留**：改为调用 `CalcStrength > 0`，v2 兼容路径继续使用。

### 3. 区域隔离

Event 新增 ZoneID：

```go
type Event struct {
    // ...现有字段
    ZoneID string // 事件发生的区域，空字符串表示不限区域
}
```

区域过滤逻辑在 `CalcStrength` 之前执行（Scheduler 层）：

```go
func shouldFilterByZone(npcZoneID, evtZoneID, perceptionMode string) bool {
    if perceptionMode == "global" {
        return false // global 无视区域
    }
    if npcZoneID == "" || evtZoneID == "" {
        return false // 任一为空不过滤（向后兼容）
    }
    return npcZoneID != evtZoneID
}
```

区域过滤是纯逻辑判断，放在 `perception` 包中导出，Scheduler 调用。不放在 Scheduler 里避免 Scheduler 承担感知职责（红线：gateway 不承担非网络职责，同理 scheduler 的感知逻辑应委托给 perception 包）。

### 4. Scheduler filterPerception 重构

```go
func (s *Scheduler) filterPerception(inst *npc.Instance, perc *component.PerceptionComponent, events []*event.Event) []perception.PerceiveResult {
    npcZoneID := ""
    if pos, ok := npc.GetComponent[*component.PositionComponent](inst, "position"); ok {
        npcZoneID = pos.ZoneID
    }

    cfg := &perception.PerceptionConfig{
        VisualRange:   perc.VisualRange,
        AuditoryRange: perc.AuditoryRange,
    }

    var results []perception.PerceiveResult
    for _, evt := range events {
        typeCfg, ok := s.EvtTypes[evt.Type]
        if !ok {
            continue
        }
        // 区域过滤
        if perception.ShouldFilterByZone(npcZoneID, evt.ZoneID, typeCfg.PerceptionMode) {
            continue
        }
        // 强度计算
        strength := perception.CalcStrength(inst.Position, cfg, evt, typeCfg)
        if strength > 0 {
            results = append(results, perception.PerceiveResult{Event: evt, Strength: strength})
        }
    }

    // 注意力裁剪
    if perc.AttentionCapacity > 0 && len(results) > perc.AttentionCapacity {
        sort.Slice(results, func(i, j int) bool {
            return results[i].Strength > results[j].Strength
        })
        results = results[:perc.AttentionCapacity]
    }

    return results
}
```

### 5. Decision.Evaluate 适配

```go
// 新签名
func (c *Center) Evaluate(bb *blackboard.Blackboard, npcPos event.Vec3,
    perceived []perception.PerceiveResult, evtTypes map[string]*event.EventTypeConfig, dt float64)
```

变化：
- 输入从 `[]*event.Event` 变为 `[]perception.PerceiveResult`
- 威胁值直接使用 `result.Strength`，不再调用 `CalcThreat`（避免重复计算距离衰减）
- `CalcThreat` 函数保留（可能被其他地方或测试使用），但 Evaluate 不再调用它

```go
for _, pr := range perceived {
    if pr.Strength > maxThreat {
        maxThreat = pr.Strength
        maxEvent = pr.Event
    }
}
```

### 6. publish_event 支持 zone_id

```go
type PublishEventRequest struct {
    EventType string  `json:"event_type"`
    X         float64 `json:"x"`
    Z         float64 `json:"z"`
    Severity  float64 `json:"severity,omitempty"`
    SourceID  string  `json:"source_id,omitempty"`
    ZoneID    string  `json:"zone_id,omitempty"` // 新增
}
```

`event.NewEvent` 增加 `zoneID` 参数，handler 传入 `req.ZoneID`。

---

## 方案对比

### 备选方案：感知强度在决策中心计算（不选）

保持 `CanPerceive` 返回 bool，在 Decision.Evaluate 里计算强度（即现有的 CalcThreat）。

**不选的理由**：
1. 感知和决策职责混淆——"能不能感知到"和"感知有多强"都是感知层的事
2. 距离被计算两次：一次在 CanPerceive（判断范围内），一次在 CalcThreat（计算衰减）
3. 注意力裁剪需要强度排序，如果强度在决策中心才计算，裁剪只能用距离排序——不准确（不同 severity 的事件距离近不代表强度高）

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码事件→感知映射 | **不违反** | 感知模式仍从事件配置读取 |
| 禁止硬编码 NPC 参数 | **不违反** | attention_capacity 从组件配置读取 |
| 禁止 core/ import runtime/ | **不违反** | perception 在 runtime/ 下 |
| 禁止 Gateway 承担非网络职责 | **不违反** | 区域过滤和强度计算在 perception 包，Scheduler 调用 |
| 禁止静默降级 | **不违反** | 未知 perception_mode 返回强度 0 + warn 日志（保持现有行为） |
| 禁止过度设计 | **不违反** | PerceiveResult 只有 2 个字段，ShouldFilterByZone 是纯函数 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面** | 新事件自动获得强度衰减和区域隔离 |
| 加 NPC 类型 | **正面** | attention_capacity 可配不同值 |
| NPC 间交互 | **正面** | PerceiveResult 是未来群组感知共享的数据单元 |

---

## 依赖方向

```
internal/runtime/ (Scheduler)
  → internal/runtime/perception/   (CalcStrength, ShouldFilterByZone, PerceiveResult)
  → internal/runtime/component/    (PerceptionComponent, PositionComponent)
  → internal/runtime/decision/     (Evaluate 新签名)
  → internal/runtime/event/        (Event.ZoneID)

internal/runtime/perception/
  → internal/runtime/event/        (不变)

internal/runtime/decision/
  → internal/runtime/perception/   (新增：PerceiveResult 类型引用)
  → internal/core/blackboard/      (不变)
  → internal/runtime/event/        (不变)
```

新增依赖：`decision/ → perception/`（PerceiveResult 类型）。方向合理——decision 接收 perception 的输出。无循环。

---

## 并发安全

无新增共享状态。PerceiveResult 是每 NPC 每 Tick 临时创建的局部变量，不跨 goroutine。Event.ZoneID 在创建时写入一次，之后只读。

---

## 配置变更

无新增配置文件。现有配置不修改：
- `PerceptionComponent.AttentionCapacity` 已在 Schema 和 Go struct 中定义，本次启用
- `PositionComponent.ZoneID` 已在 Schema 和 Go struct 中定义，本次启用
- `publish_event` 协议新增可选 `zone_id` 字段，向后兼容

---

## 测试策略

### 单元测试

| 模块 | 覆盖 |
|------|------|
| `perception/` | CalcStrength：各 mode 的强度计算、边界距离（0/中间/刚好/超出）、global 模式 |
| `perception/` | ShouldFilterByZone：同区域/不同区域/空 zone_id/global 模式 |
| `perception/` | CanPerceive 兼容：调用 CalcStrength > 0，结果与 v2 一致 |
| `decision/` | Evaluate 接收 PerceiveResult：直接使用 Strength，最高优先 |

### 集成测试

| 场景 | 验证 |
|------|------|
| 注意力裁剪 | 5 个事件 + capacity=3，决策中心只收到前 3 个 |
| 区域隔离 | NPC 在 meadow，事件在 mountain，非 global 事件不感知 |
| 强度传递 | 近距离事件 strength 高于远距离，决策中心选近距离 |
| v2 兼容 | 现有 6 个集成场景通过 |

### Benchmark

| 测试 | 目标 |
|------|------|
| 100 NPC × 10 事件感知过滤 | < 1ms |
