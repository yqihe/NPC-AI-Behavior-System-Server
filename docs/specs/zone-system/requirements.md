# zone-system 需求分析

## 动机

当前系统没有"区域"概念——所有 NPC 在同一个全局空间中，Scheduler 每 Tick 遍历全部 NPC。这导致：

1. **无法休眠**：即使某片区域没有玩家，该区域的 NPC 仍然每 Tick 执行感知/决策/移动——纯粹浪费 CPU。《洛克王国：世界》等游戏在无玩家区域停止 NPC 逻辑。
2. **广播浪费**：`world_snapshot` 把所有 NPC 状态广播给所有客户端。如果有 1000 个 NPC 分布在 10 个区域，每个玩家只关心自己所在区域的 NPC，但收到全部 1000 个。
3. **事件隔离不完整**：需求 2 的 `ShouldFilterByZone` 已做了感知层的区域隔离，但事件本身没有区域归属管理——所有事件仍在全局事件总线中。
4. **NPC 生成无规划**：NPC 通过 `spawn_npc` 消息逐个创建，没有"区域刷怪表"按模板批量生成。

区域系统是 AI 角色系统在千级 NPC 规模下的性能基础设施。

## 优先级

**中**。依赖需求 1-6 全部完成。被需求 8（可观测性）弱依赖——区域维度的指标需要区域系统。

## 预期效果

### 场景 1：区域定义和 NPC 批量生成

从配置加载区域定义：
```json
{
  "region_id": "meadow",
  "name": "风语草原",
  "spawn_table": [
    {"template_ref": "butterfly_01", "count": 5, "spawn_points": [{"x":100,"z":200}], "wander_radius": 30}
  ]
}
```
系统启动时（或玩家首次进入区域时）从 spawn_table 批量创建 NPC，每个 NPC 的 position.zone_id 自动设为区域 ID。

### 场景 2：休眠/唤醒

1. meadow 区域有 5 只蝴蝶正在游荡
2. 最后一个玩家离开 meadow → 区域进入"休眠倒计时"（30 秒）
3. 30 秒内无玩家进入 → 区域休眠：所有 NPC 停止 Tick（从 Scheduler 的活跃列表中移除）
4. 玩家进入 meadow → 区域唤醒：NPC 恢复 Tick
5. NPC 状态不丢失——休眠只是"暂停"，不是销毁

### 场景 3：区域级广播

1. 玩家 A 在 meadow，玩家 B 在 mountain
2. `world_snapshot` 广播时：玩家 A 只收到 meadow 区域的 NPC 状态，玩家 B 只收到 mountain 的
3. 减少带宽和客户端处理量

### 场景 4：区域级事件隔离

事件发布时指定 zone_id（需求 2 已支持）。ZoneManager 维护区域→事件的映射，Scheduler 在感知过滤时只取本区域的事件（而非全局事件总线的全量）。

## 依赖分析

- **依赖**：
  - 需求 0 Schema（已完成）：区域 Schema（region.json）已定义
  - 需求 1 组件化（已完成）：PositionComponent.ZoneID
  - 需求 2 感知（已完成）：ShouldFilterByZone + Event.ZoneID

- **被依赖**：
  - 需求 8 可观测性：区域维度的 NPC 数量指标

## 改动范围

| 包 | 变更类型 | 文件数 | 说明 |
|----|---------|--------|------|
| `internal/runtime/zone/` | **新增** | 3-4 | ZoneManager + Zone struct + 休眠/唤醒 |
| `internal/config/` | **扩展** | 2 | Source 接口新增 LoadRegionConfig / JSONSource 实现 |
| `internal/runtime/scheduler.go` | **修改** | 1 | 按区域分组 Tick，跳过休眠区域 |
| `internal/gateway/` | **修改** | 2 | 区域级广播 + 玩家区域注册 |
| `cmd/server/main.go` | **修改** | 1 | 初始化 ZoneManager |
| `configs/regions/` | **新增** | 1-2 | 示例区域配置 |
| 测试文件 | **新增** | 3-4 | ZoneManager 测试、休眠/唤醒测试、集成测试 |

预估 14-18 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **正面** | 事件自动归属区域，区域隔离生效 |
| 加 NPC 类型 | **正面** | 区域 spawn_table 引用模板名，新模板自动可用 |
| NPC 间交互 | **中性** | 群组系统不受影响（同组 NPC 在同区域） |

## 验收标准

### ZoneManager

- **R1**：定义 `Zone{ID, Name, SpawnTable, Active bool}` 结构体
- **R2**：定义 `ZoneManager` 管理所有区域，支持按 ID 查询
- **R3**：从配置加载区域定义（JSON 文件或 Config Source）

### NPC 批量生成

- **R4**：`Zone.Spawn(compReg, src, btReg, npcReg)` 从 spawn_table 批量创建 NPC，设置 zone_id
- **R5**：spawn_table 每项的 count 个 NPC 分配到 spawn_points（循环分配）

### 休眠/唤醒

- **R6**：`Zone.Sleep()` 将区域标记为休眠，区域内 NPC 从 Scheduler 的活跃遍历中跳过
- **R7**：`Zone.Wake()` 将区域标记为活跃，NPC 恢复 Tick
- **R8**：休眠时 NPC 实例不销毁，状态保留
- **R9**：Scheduler.Tick 中跳过休眠区域的 NPC（通过检查 NPC 的 zone_id 对应区域是否 Active）

### 区域级广播

- **R10**：Gateway Hub 支持按区域广播：每个连接关联一个 zone_id
- **R11**：`world_snapshot` 只发送连接关联区域的 NPC 状态
- **R12**：新增 WS 消息 `enter_zone` / `leave_zone` 让客户端切换区域

### Config Source

- **R13**：Source 接口新增 `LoadRegionConfig(regionID string) ([]byte, error)`
- **R14**：JSONSource 从 `configs/regions/<regionID>.json` 加载

### 向后兼容

- **R15**：无区域配置时（configs/regions/ 为空），行为与 v2 一致——全局 Tick 全局广播
- **R16**：现有集成测试和 e2e 测试全部通过

### 性能

- **R17**：1000 NPC 分 10 区域，5 区域休眠，活跃区域 500 NPC Tick < 50ms
- **R18**：休眠区域的 NPC Tick 耗时 = 0（完全跳过）

## 不做什么

- **不做区域边界碰撞**：NPC 移出区域边界不阻挡，只是 zone_id 不变。真实边界碰撞需要客户端地图数据
- **不做动态区域创建**：区域从配置加载，运行时不增删区域
- **不做跨区域 NPC 迁移**：NPC 的 zone_id 在创建时固定
- **不做玩家管理**：玩家连接的 zone_id 由客户端发送 `enter_zone` 消息设置，服务端不验证玩家实际位置
- **不做天气系统**：区域 Schema 有 weather 字段但本 spec 不实现天气逻辑
