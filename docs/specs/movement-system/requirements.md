# movement-system 需求分析

## 动机

v2 的 NPC 不移动——所有移动行为是 `stub_action` 返回固定状态，NPC 坐标从创建起就不变。这导致：

1. **NPC 是静态的**：客户端看到 NPC 坐标永远不变，无论 FSM 状态是 Idle、Flee 还是 Patrol，位置都一样。
2. **位置相关逻辑失效**：感知系统的距离计算（CalcStrength）基于 NPC 位置，但 NPC 不移动 → 距离不变 → 无法"跑出爆炸范围"。
3. **移动组件只写 move_state**：需求 1 的 MovementComponent.Tick 写 `move_state = "idle"`，但不更新坐标。move_speed/wander_radius/patrol_waypoints 配置字段全部闲置。

不做这个，NPC 的"AI 行为"只是状态机的状态名在变，没有任何可观察的物理表现。

## 优先级

**高**。依赖需求 1（MovementComponent）。被需求 6（社交系统的 follower 跟随）和需求 7（区域边界约束）依赖。

## 预期效果

### 场景 1：游荡（wander）

NPC 配置 `move_type: "wander"`, `wander_radius: 50`, `move_speed: 3`：
1. NPC 在 spawn 点 (100, 0, 200) 生成
2. 随机选一个 wander_radius 内的目标点 (130, 0, 220)
3. 每 Tick 朝目标点移动 `move_speed × dt` 距离
4. `move_state = "moving"`，BB 的 `npc_pos_x/npc_pos_z` 每 Tick 更新
5. 到达目标点 → `move_state = "arrived"` → 等待随机时间 → 选新目标

### 场景 2：巡逻（patrol）

NPC 配置 `move_type: "patrol"`, waypoints: [(100,200), (150,250), (200,200)]：
1. 从当前位置向 waypoint[0] 移动
2. 到达 → 切换到 waypoint[1]
3. 到达最后一个 → 回到 waypoint[0]（循环）
4. `move_state` 在 moving/arrived 间切换

### 场景 3：追逐（chase）— BT 节点驱动

BT 树中使用 `move_to` 节点：
```json
{"type": "move_to", "params": {"target_key": "threat_source_pos_x", "target_key_z": "threat_source_pos_z", "speed": 5.0}}
```
1. 从 BB 读取目标坐标
2. 每 Tick 朝目标移动
3. 到达（距离 < 1m）→ 返回 Success
4. 目标不存在 → 返回 Failure

### 场景 4：逃跑（flee）— BT 节点驱动

BT 树中使用 `flee_from` 节点：
```json
{"type": "flee_from", "params": {"source_key": "threat_source_pos_x", "source_key_z": "threat_source_pos_z", "distance": 100, "speed": 6.0}}
```
1. 计算威胁源反方向
2. 每 Tick 沿反方向移动
3. 距离威胁源 >= `distance` → 返回 Success
4. 威胁源不存在 → 返回 Failure

### 场景 5：位置更新同步

每次移动后：
- `PositionComponent.X/Z` 更新
- BB `npc_pos_x/npc_pos_z` 更新
- `Instance.Position` 更新（Scheduler 和广播使用）
- 客户端通过 `world_snapshot` 广播看到新坐标

## 依赖分析

- **依赖**：
  - 需求 1 组件化架构（已完成）：MovementComponent + PositionComponent
  - 需求 4 记忆系统（已完成）：位置记忆存取（巡逻恢复）

- **被依赖**：
  - 需求 6 社交系统：follower 跟随 leader 移动
  - 需求 7 区域系统：区域边界约束

## 改动范围

| 包 | 变更类型 | 文件数 | 说明 |
|----|---------|--------|------|
| `internal/runtime/component/movement.go` | **重构** | 1 | Tick 实现真实移动（wander/patrol），位置更新 |
| `internal/core/bt/` | **新增** | 2 | move_to + flee_from 节点工厂 |
| `internal/core/bt/registry.go` | **修改** | 1 | DefaultRegistry 注册新节点 |
| `internal/runtime/npc/instance.go` | **修改** | 1 | Position 同步方法 |
| `internal/core/blackboard/keys.go` | **修改** | 1 | 新增移动相关 BB Key |
| 测试文件 | **新增+修改** | 3-4 | 移动单元测试、BT 节点测试、集成测试 |

预估 10-12 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **间接** | NPC 能移动后，距离衰减有实际意义（跑远了感知弱） |
| 加 NPC 类型 | **正面** | 不同 NPC 配置不同 move_speed/wander_radius，行为差异更大 |
| NPC 间交互 | **正面** | 追逐/逃跑是 NPC 间交互的基础动作 |

## 验收标准

### 游荡

- **R1**：wander 模式下，NPC 在 spawn 点 + wander_radius 范围内随机选目标点
- **R2**：每 Tick 朝目标点移动 `move_speed × dt` 距离，更新 PositionComponent 和 BB
- **R3**：到达目标点（距离 < 1m）后 move_state="arrived"，等待 1-3 秒后选新目标
- **R4**：移动中 move_state="moving"

### 巡逻

- **R5**：patrol 模式下按 waypoints 顺序移动，到达最后一个后回到第一个（循环）
- **R6**：每 Tick 移动逻辑与 wander 一致（匀速朝目标）

### BT 移动节点

- **R7**：`move_to` 节点：从 BB 读取目标坐标，每 Tick 朝目标移动，到达返回 Success，目标不存在返回 Failure，移动中返回 Running
- **R8**：`flee_from` 节点：从 BB 读取威胁源坐标，沿反方向移动，到达安全距离返回 Success，移动中返回 Running
- **R9**：两个节点注册到 DefaultRegistry，Schema 文件已在需求 0 预留位置（需新增）

### 位置同步

- **R10**：移动后 PositionComponent.X/Z 更新
- **R11**：移动后 BB npc_pos_x/npc_pos_z 更新
- **R12**：Instance.Position 与 PositionComponent 同步（广播用）

### BB Key 新增

- **R13**：新增 `move_target_x`(float64)、`move_target_z`(float64)：当前移动目标坐标

### 向后兼容

- **R14**：无 movement 组件的 NPC 位置不变
- **R15**：现有集成测试和 e2e 测试全部通过

### 性能

- **R16**：500 个移动中 NPC 的 MovementComponent.Tick 总耗时 < 1ms

## 不做什么

- **不做寻路**：NPC 直线移动到目标，不绕障碍物。寻路需要地图数据（客户端/世界系统）
- **不做碰撞检测**：NPC 之间和 NPC 与地形不做碰撞
- **不做区域边界约束**：NPC 可以移出区域。边界约束在需求 7
- **不做跟随（follow）模式的实现**：follow 需要目标 NPC 位置，在需求 6（社交系统）中实现
- **不做 Y 轴移动**：只在 XZ 平面移动，Y 值不变
