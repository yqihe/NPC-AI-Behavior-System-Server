# movement-system 任务拆解

## T1: moveToward 核心函数 + BB Key (R2, R13)

**文件**：
- `internal/runtime/component/movement.go`
- `internal/core/blackboard/keys.go`

**做完了是什么样**：
- keys.go 新增 `move_target_x`(float64)、`move_target_z`(float64)
- movement.go 新增 `moveToward(posX, posZ, targetX, targetZ, maxDist) (newX, newZ)` 纯函数
- 新增运行时字段：spawnX/spawnZ/targetX/targetZ/hasTarget/waitTimer/patrolIndex
- 新增 `SetSpawn(x, z)` 方法
- 编译通过，Tick 逻辑下一步改

---

## T2: tickWander 实现 (R1, R2, R3, R4)

**文件**：
- `internal/runtime/component/movement.go`
- `internal/runtime/component/movement_test.go`

**做完了是什么样**：
- Tick 中 `move_type == "wander"` 调用 tickWander
- tickWander：spawn + radius 内随机目标 → 朝目标匀速移动 → 到达（<1m）→ 等待 1-3s → 选新目标
- 移动后写 BB npc_pos_x/z + move_target_x/z + move_state
- 测试：多 Tick 后位置变化、到达后状态切换、不超出 wander_radius

---

## T3: tickPatrol 实现 (R5, R6)

**文件**：
- `internal/runtime/component/movement.go`
- `internal/runtime/component/movement_test.go`

**做完了是什么样**：
- Tick 中 `move_type == "patrol"` 调用 tickPatrol
- tickPatrol：朝 waypoints[patrolIndex] 移动 → 到达 → patrolIndex++ → 循环
- 测试：按路点顺序移动、到达最后一个后回到第一个

---

## T4: SyncPosition + spawn 点记录 (R10, R11, R12)

**文件**：
- `internal/runtime/npc/instance.go`
- `internal/runtime/npc/template.go`
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- Instance 新增 `SyncPosition()` 方法：从 BB 读 npc_pos_x/z → 更新 Position + PositionComponent
- NewInstanceFromTemplate 中对 MovementComponent 调用 SetSpawn
- Scheduler.Tick 末尾 TickComponents 后调用 SyncPosition

---

## T5: BT Context 加 DeltaTime (R7, R8)

**文件**：
- `internal/core/bt/context.go`
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- Context struct 新增 `DeltaTime float64` 字段
- Scheduler 创建 Context 时传入 dt
- 现有 BT 节点不使用 DeltaTime，不受影响

---

## T6: move_to BT 节点 (R7)

**文件**：
- `internal/core/bt/movement_nodes.go`
- `internal/core/bt/movement_nodes_test.go`

**做完了是什么样**：
- `move_to` 节点：params `target_key_x`/`target_key_z`/`speed`
- Tick：从 BB 读目标坐标 → moveToward → 写 BB npc_pos_x/z + move_state
- 到达（<1m）→ Success，目标不存在 → Failure，移动中 → Running
- 测试：正常移动/到达/目标缺失

---

## T7: flee_from BT 节点 (R8)

**文件**：
- `internal/core/bt/movement_nodes.go`（追加）
- `internal/core/bt/movement_nodes_test.go`（追加）

**做完了是什么样**：
- `flee_from` 节点：params `source_key_x`/`source_key_z`/`distance`/`speed`
- Tick：计算反方向 → moveToward → 距离 >= distance → Success
- 威胁源不存在 → Failure，移动中 → Running
- 测试：反方向移动/到达安全距离/源缺失

---

## T8: DefaultRegistry 注册 + Schema (R9)

**文件**：
- `internal/core/bt/registry.go`
- `configs/schemas/node_types/move_to.json`
- `configs/schemas/node_types/flee_from.json`

**做完了是什么样**：
- DefaultRegistry 注册 `move_to` 和 `flee_from`
- 两个 Schema 文件格式与现有节点一致
- Schema 测试 `TestNodeTypeSchemas` 通过（节点数从 8 → 10）

---

## T9: 集成测试 + 现有测试验证 (R14, R15, R16)

**文件**：
- `internal/runtime/movement_integration_test.go`
- `configs/schemas/schemas_test.go`（适配节点数）

**做完了是什么样**：
- 集成测试 1：wander NPC 多 Tick 后 npc_pos_x/z 变化
- 集成测试 2：patrol NPC 经过全部路点后循环回 waypoint[0]
- 集成测试 3：move_to BT 节点驱动追逐到目标
- 现有集成测试 + e2e 全部通过
- Benchmark：500 个移动 NPC Tick < 1ms

---

## 执行顺序

```
T1  moveToward + BB Key + 运行时字段
 ├→ T2  tickWander
 ├→ T3  tickPatrol
 └→ T4  SyncPosition + spawn
     └→ T5  Context.DeltaTime
         ├→ T6  move_to 节点
         └→ T7  flee_from 节点
             └→ T8  注册 + Schema
                 └→ T9  集成测试
```

T2/T3 可并行。T6/T7 可并行。
