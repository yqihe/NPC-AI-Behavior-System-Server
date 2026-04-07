# memory-system 需求分析

## 动机

v2 的 NPC 没有记忆——每 Tick 只看当前活跃事件，上一秒发生的事全部遗忘。这导致：

1. **NPC 行为不连贯**：爆炸事件 TTL 过期后，NPC 立刻忘记"刚才这里爆炸过"，回到 Idle 无防备。真实的 AI 角色应该记住"那个方向有过威胁"，短时间内保持警觉。
2. **重复刺激无强化**：同一个威胁源反复出现（比如同一个敌人连续攻击），NPC 每次反应完全相同。应该有"被同一个敌人攻击三次→更加恐惧/更加愤怒"的强化效应。
3. **情绪累积无触发源**：需求 3 的情绪组件有 AccumulateRate 但从未被触发——情绪只衰减不累积。记忆系统是情绪累积的触发源：记住威胁 → 恐惧累积。

MemoryComponent 在需求 1 已定义 struct（Capacity/MemoryTypes/DecayTime）和空 Tick，但没有记忆条目存储和逻辑。

## 优先级

**高**。依赖需求 3（多维决策）。被需求 6（社交系统）依赖——社交记忆是群组关系的基础。

## 预期效果

### 场景 1：威胁记忆——事件过期后仍记得

1. 爆炸事件到达，NPC 感知 → 决策中心评估 → FSM 转 Flee
2. 同时，Scheduler 将此事件写入 NPC 的威胁记忆：`{type: "threat", target_id: "bomb_1", value: 75, timestamp: now, ttl: 60}`
3. 爆炸事件 TTL 过期，从事件总线消失
4. 但 NPC 仍有这条威胁记忆（TTL 60 秒未到期）
5. 决策中心的情绪维度因威胁记忆存在而保持较高（记忆触发恐惧累积）
6. 60 秒后，记忆 TTL 到期，自动遗忘，NPC 恢复正常

### 场景 2：重复刺激强化

1. NPC 记住 "bomb_1" 的威胁记忆，value=75，ttl=60
2. 又一个来自 "bomb_1" 的爆炸事件到达
3. 记忆强化：value 取新旧最大值，TTL 重置为 decay_time
4. 第三次：value 再次更新
5. 效果：反复来自同一源的威胁 → 记忆 value 越来越高 → 情绪（恐惧）累积越快

### 场景 3：记忆影响情绪累积

1. NPC 有威胁记忆（value=75）
2. EmotionComponent.Tick 中，检查记忆：有威胁记忆 → 恐惧按 AccumulateRate 累积
3. 无威胁记忆时 → 恐惧只衰减不累积
4. 记忆是情绪累积和衰减之间的桥梁

### 场景 4：容量上限 + LRU 淘汰

NPC 记忆容量 = 5，已有 5 条记忆。新记忆写入时：
1. 找最旧的一条（timestamp 最小）
2. 淘汰它
3. 写入新记忆

### 场景 5：位置记忆——记住巡逻进度

1. NPC 正在巡逻（waypoint 3/5），收到威胁中断巡逻
2. 位置记忆写入：`{type: "location", target_id: "patrol_progress", value: 3, ttl: 300}`
3. 威胁消除后，NPC 从位置记忆读取巡逻进度，从 waypoint 3 恢复
4. （本 spec 实现记忆存取，巡逻恢复逻辑在需求 5 移动系统）

## 依赖分析

- **依赖**：
  - 需求 1 组件化架构（已完成）：MemoryComponent struct + Tickable 接口
  - 需求 3 决策深化（已完成）：多维决策框架（情绪维度受记忆影响）

- **被依赖**：
  - 需求 5 移动系统：位置记忆存取（巡逻恢复）
  - 需求 6 社交系统：社交记忆（记住见过的 NPC）

## 改动范围

| 包 | 变更类型 | 文件数 | 说明 |
|----|---------|--------|------|
| `internal/runtime/component/memory.go` | **重构** | 1 | 新增 MemoryEntry struct、记忆 CRUD、Tick 清理过期 |
| `internal/runtime/component/emotion.go` | **修改** | 1 | Tick 中读取威胁记忆触发恐惧累积 |
| `internal/runtime/scheduler.go` | **修改** | 1 | 感知结果写入威胁记忆 |
| `internal/core/blackboard/keys.go` | **修改** | 1 | 新增 BB Key（记忆相关） |
| 测试文件 | **新增+修改** | 3-4 | 记忆 CRUD 测试、情绪联动测试、集成测试 |

预估 8-10 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **正面** | 新事件自动触发威胁记忆写入 |
| 加 NPC 类型 | **正面** | 不同 NPC 可配不同 capacity/decay_time/memory_types |
| NPC 间交互 | **正面** | 社交记忆（记住见过的 NPC）是 NPC 间交互的基础 |

## 验收标准

### 记忆条目

- **R1**：定义 `MemoryEntry{Type string, TargetID string, Value float64, Timestamp int64, TTL float64}` 结构体
- **R2**：MemoryComponent 持有 `[]MemoryEntry` 切片，初始为空

### 记忆写入

- **R3**：`AddMemory(entry MemoryEntry)` 方法：如果同 Type+TargetID 的记忆已存在 → 强化（Value 取 max，TTL 重置）；不存在 → 新增
- **R4**：新增时如果 len(entries) >= Capacity → 淘汰 Timestamp 最小的条目
- **R5**：写入后更新 BB `memory_count`

### 记忆读取

- **R6**：`GetMemories(memType string) []MemoryEntry` 按类型查询
- **R7**：`HasMemory(memType, targetID string) bool` 精确查询
- **R8**：`GetMemory(memType, targetID string) (MemoryEntry, bool)` 精确获取

### 记忆清理

- **R9**：Tick 中遍历 entries，TTL -= dt，TTL <= 0 的移除
- **R10**：清理后更新 BB `memory_count`

### 威胁记忆写入

- **R11**：Scheduler 在 Decision.Evaluate 后，如果 perceived 中有事件 → 将最高威胁事件写入 NPC 的威胁记忆（Type="threat", TargetID=event.SourceID, Value=strength）
- **R12**：只有 NPC 有 memory 组件且 memory_types 包含 "threat" 时才写入

### 记忆触发情绪累积

- **R13**：EmotionComponent.Tick 中，如果 NPC 有 memory 组件且存在 "threat" 类型记忆 → 恐惧（名为 "fear" 的情绪）按 AccumulateRate × dt 累积
- **R14**：无威胁记忆时恐惧只衰减不累积（当前行为不变）

### BB Key 新增

- **R15**：新增 `memory_threat_value`(float64)：最高威胁记忆的 value（供 FSM 条件使用）

### 向后兼容

- **R16**：无 memory 组件的 NPC 行为不变
- **R17**：现有集成测试和 e2e 测试全部通过

### 性能

- **R18**：1000 条记忆的 Tick 清理 < 10μs

## 不做什么

- **不做位置记忆的消费逻辑**：位置记忆的写入和存储在本 spec 实现，但读取并恢复巡逻进度在需求 5
- **不做社交记忆的消费逻辑**：社交记忆的写入和存储在本 spec 实现，但群组关系判断在需求 6
- **不做记忆持久化**：记忆存在内存中，NPC 销毁或服务重启后丢失
- **不做记忆共享**：NPC 间不共享记忆（群组感知共享在需求 6）
- **不做记忆影响感知**：原计划"有威胁记忆时感知阈值降低"推迟——需要改 perception 包签名，复杂度高且当前无具体 FSM 配置消费此行为。改为通过记忆影响情绪 → 情绪影响决策权重来间接实现
