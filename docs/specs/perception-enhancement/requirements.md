# perception-enhancement 需求分析

## 动机

v2 的感知系统是 0/1 二值判断——事件在范围内就能感知，所有感知到的事件平等传给决策中心。这导致三个问题：

1. **无注意力上限**：一个 NPC 同时收到 20 个事件，决策中心要全部评估。现实中生物有注意力容量，不可能同时处理 20 个刺激。`PerceptionComponent.AttentionCapacity` 字段在需求 1 已定义但未使用。
2. **无感知强度**：距离 10 米和距离 490 米的爆炸（范围 500 米）对 NPC 的感知效果完全相同——都是"感知到了"。缺少强度信息，决策中心无法区分"就在身边"和"远处隐约听到"。
3. **无区域隔离**：所有 NPC 对所有事件做感知判断，跨区域的事件也会被评估。当有多个区域时，草原上的爆炸不应该被雪山上的 NPC 感知。

不做这个，需求 3（决策深化）缺少感知强度输入，需求 7（区域系统）缺少区域隔离基础。

## 优先级

**高**。需求 1 完成后的第一批可并行需求之一。需求 3（决策深化）依赖感知强度输出，需求 7（区域系统）依赖区域隔离。

## 预期效果

### 场景 1：注意力容量裁剪

NPC 的 `attention_capacity = 3`，同时有 5 个事件在感知范围内。

- 计算 5 个事件的感知强度（距离衰减后的值）
- 按强度降序排列
- 只保留前 3 个传给决策中心
- 最弱的 2 个被忽略

### 场景 2：感知强度连续衰减

爆炸事件 severity=80，range=500m：

| NPC 距离 | v2 结果 | v3 结果 |
|---------|--------|--------|
| 10m | 感知 = true | 强度 = 80 × (1 - 10/500) = 78.4 |
| 250m | 感知 = true | 强度 = 80 × (1 - 250/500) = 40.0 |
| 490m | 感知 = true | 强度 = 80 × (1 - 490/500) = 1.6 |
| 501m | 感知 = false | 强度 = 0（超出范围） |

强度值传给决策中心，替代当前的二值 bool。

### 场景 3：区域隔离

NPC 在区域 `meadow`，事件发生在区域 `mountain`：

- 事件 `perception_mode` 为 `visual` 或 `auditory` → 跳过（不同区域不感知）
- 事件 `perception_mode` 为 `global` → 仍然感知（全局事件无视区域）

### 场景 4：PerceiveResult 替代 bool

`filterPerception` 返回 `[]PerceiveResult` 而非 `[]*Event`：

```go
type PerceiveResult struct {
    Event    *event.Event
    Strength float64  // 0.0~severity，距离衰减后的感知强度
}
```

决策中心接收 `[]PerceiveResult`，直接使用 `Strength` 作为威胁值（不再自己重新计算距离衰减——避免重复计算）。

## 依赖分析

- **依赖**：
  - 需求 1 组件化架构（已完成）：PerceptionComponent.AttentionCapacity 字段、PositionComponent.ZoneID 字段
  - event 包：Event 结构体（需新增 ZoneID 字段）

- **被依赖**：
  - 需求 3 决策深化：接收感知强度输入
  - 需求 6 社交系统：群组感知共享需要感知结果
  - 需求 7 区域系统：区域隔离逻辑

## 改动范围

| 包 | 变更类型 | 文件数 | 说明 |
|----|---------|--------|------|
| `internal/runtime/perception/` | **重构** | 2 | CanPerceive → CalcStrength 返回 float64；新增 PerceiveResult |
| `internal/runtime/event/` | **修改** | 1 | Event 新增 ZoneID 字段 |
| `internal/runtime/scheduler.go` | **修改** | 1 | filterPerception 返回 []PerceiveResult + 注意力裁剪 + 区域过滤 |
| `internal/runtime/decision/` | **修改** | 1 | Evaluate 接收 []PerceiveResult 替代 []*Event |
| `internal/gateway/handler.go` | **修改** | 1 | publish_event 支持 zone_id 字段 |
| `pkg/protocol/` | **修改** | 1 | PublishEventRequest 新增 ZoneID |
| 测试文件 | **修改+新增** | 4-5 | perception 测试、decision 测试适配、集成测试、Scheduler 测试 |

预估 12-15 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **正面** | 新事件自动获得强度衰减和区域隔离，无需特殊处理 |
| 加 NPC 类型 | **正面** | 不同 NPC 可配置不同 attention_capacity，影响感知行为 |
| NPC 间交互 | **间接** | 感知结果（PerceiveResult）是社交感知共享的基础数据 |

## 验收标准

### 感知强度

- **R1**：`CanPerceive` 重命名为 `CalcStrength`，返回 `float64`（0 表示不可感知，>0 表示感知强度）
- **R2**：强度计算公式：`strength = severity × max(0, 1 - distance / min(npc_range, event_range))`，与现有 `CalcThreat` 公式一致
- **R3**：`global` 模式事件强度 = severity（无距离衰减）
- **R4**：超出范围的事件强度 = 0

### 注意力容量

- **R5**：`filterPerception` 在计算所有事件强度后，按强度降序排列，只保留前 `AttentionCapacity` 个
- **R6**：`AttentionCapacity = 0` 或未设置时不裁剪（保持 v2 行为）
- **R7**：事件数 ≤ AttentionCapacity 时不裁剪

### 区域隔离

- **R8**：Event 结构体新增 `ZoneID string` 字段
- **R9**：`publish_event` 消息支持 `zone_id` 可选字段
- **R10**：感知过滤时，如果 NPC 的 `position.zone_id` 不为空且事件的 `ZoneID` 不为空，两者不同则跳过（强度=0）
- **R11**：`global` 模式事件无视区域隔离
- **R12**：NPC 或事件的 zone_id 为空时不做区域过滤（向后兼容）

### 感知结果传递

- **R13**：定义 `PerceiveResult{Event *event.Event, Strength float64}` 结构体
- **R14**：Scheduler 的 `filterPerception` 返回 `[]PerceiveResult`
- **R15**：Decision.Evaluate 接收 `[]PerceiveResult`，直接使用 `Strength` 作为威胁值（不再重复计算距离衰减）

### 向后兼容

- **R16**：v2 兼容路径（inst.Perception != nil）继续使用旧 CanPerceive bool 行为
- **R17**：现有 6 个集成测试场景全部通过
- **R18**：e2e 测试全部通过

### 性能

- **R19**：100 NPC × 10 事件的感知过滤（含强度计算+排序+裁剪）单 Tick < 1ms

## 不做什么

- **不做遮挡机制**：需要地图数据（障碍物信息），属于客户端/世界系统的事
- **不做感知记忆**：NPC 不记住"之前感知过的事件"，每 Tick 重新计算。记忆在需求 4
- **不做感知类型扩展**：不加新的感知模式（如嗅觉），当前三种（visual/auditory/global）足够
- **不做决策逻辑改动**：Decision.Evaluate 的仲裁逻辑（取最高威胁）不变，只改输入类型。决策深化在需求 3
