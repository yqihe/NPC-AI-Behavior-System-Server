# movement-system 设计方案

## 方案描述

### 1. MovementComponent 运行时状态

```go
type MovementComponent struct {
    MoveType        string     `json:"move_type"`
    MoveSpeed       float64    `json:"move_speed"`
    WanderRadius    float64    `json:"wander_radius,omitempty"`
    PatrolWaypoints []Waypoint `json:"patrol_waypoints,omitempty"`

    // 运行时状态（不序列化）
    spawnX, spawnZ   float64   // wander 的原点（创建时记录）
    targetX, targetZ float64   // 当前移动目标
    hasTarget        bool      // 是否有目标
    waitTimer        float64   // 到达后等待计时器
    patrolIndex      int       // 当前巡逻路点索引
}
```

### 2. MovementComponent.Tick 重构

```go
func (c *MovementComponent) Tick(bb *blackboard.Blackboard, dt float64) {
    // 读取当前位置
    posX, _ := blackboard.Get(bb, blackboard.KeyNPCPosX)
    posZ, _ := blackboard.Get(bb, blackboard.KeyNPCPosZ)

    switch c.MoveType {
    case "wander":
        c.tickWander(bb, posX, posZ, dt)
    case "patrol":
        c.tickPatrol(bb, posX, posZ, dt)
    case "follow":
        // follow 在需求 6 实现，当前写 idle
        blackboard.Set(bb, blackboard.KeyMoveState, "idle")
    }
}
```

#### tickWander

```
if !hasTarget → 在 spawn + wander_radius 内随机选目标 → hasTarget=true
if hasTarget:
    dist = distance(pos, target)
    if dist < 1.0:
        state = "arrived"
        hasTarget = false
        waitTimer = rand(1.0, 3.0)
    else:
        移动 min(speed*dt, dist) 距离
        state = "moving"
if waitTimer > 0:
    waitTimer -= dt
    state = "arrived"
```

#### tickPatrol

```
target = waypoints[patrolIndex]
dist = distance(pos, target)
if dist < 1.0:
    patrolIndex = (patrolIndex + 1) % len(waypoints)
    state = "arrived"
else:
    移动 min(speed*dt, dist) 距离
    state = "moving"
```

#### 移动核心函数

```go
func moveToward(posX, posZ, targetX, targetZ, maxDist float64) (newX, newZ float64) {
    dx := targetX - posX
    dz := targetZ - posZ
    dist := math.Sqrt(dx*dx + dz*dz)
    if dist <= maxDist || dist == 0 {
        return targetX, targetZ
    }
    ratio := maxDist / dist
    return posX + dx*ratio, posZ + dz*ratio
}
```

每次移动后写 BB：
```go
blackboard.Set(bb, blackboard.KeyNPCPosX, newX)
blackboard.Set(bb, blackboard.KeyNPCPosZ, newZ)
blackboard.Set(bb, blackboard.KeyMoveTargetX, targetX)
blackboard.Set(bb, blackboard.KeyMoveTargetZ, targetZ)
blackboard.Set(bb, blackboard.KeyMoveState, state)
```

### 3. spawn 点记录

在 `NewInstanceFromTemplate` 中，创建 MovementComponent 后记录 spawn 点：

```go
if mov, ok := components["movement"].(*component.MovementComponent); ok {
    pos := components["position"].(*component.PositionComponent)
    mov.SetSpawn(pos.X, pos.Z)
}
```

`SetSpawn(x, z)` 方法设置 wander 原点。

### 4. 位置同步

MovementComponent.Tick 写 BB（npc_pos_x/z）后，Instance.Position 需要同步。

**方案**：在 `Instance.TickComponents(dt)` 之后，Scheduler 调用 `inst.SyncPosition()` 从 BB 读取最新坐标更新 Position 字段。

```go
func (inst *Instance) SyncPosition() {
    x, _ := blackboard.Get(inst.BB, blackboard.KeyNPCPosX)
    z, _ := blackboard.Get(inst.BB, blackboard.KeyNPCPosZ)
    inst.Position.X = x
    inst.Position.Z = z
    // 同步 PositionComponent
    if pos, ok := inst.RawComponent("position"); ok {
        p := pos.(*component.PositionComponent)
        p.X = x
        p.Z = z
    }
}
```

Scheduler.Tick 末尾：
```go
inst.TickComponents(dt)
inst.SyncPosition()
```

### 5. BT 移动节点

#### move_to（追逐/移动到目标）

```go
type moveTo struct {
    targetKeyX string  // BB key for target X
    targetKeyZ string  // BB key for target Z
    speed      float64
}

func (n *moveTo) Tick(ctx *bt.Context) bt.Status {
    posX, _ := ctx.BB.GetRaw(n.targetKeyX)
    posZ, _ := ctx.BB.GetRaw(n.targetKeyZ)
    // 类型转换 + 存在性检查...
    npcX, _ := blackboard.Get(ctx.BB, blackboard.KeyNPCPosX)
    npcZ, _ := blackboard.Get(ctx.BB, blackboard.KeyNPCPosZ)
    
    dist := distance(npcX, npcZ, targetX, targetZ)
    if dist < 1.0 {
        return bt.Success
    }
    
    // 移动（使用固定 dt=0.1，BT 节点无 dt 参数）
    newX, newZ := moveToward(npcX, npcZ, targetX, targetZ, n.speed * 0.1)
    blackboard.Set(ctx.BB, blackboard.KeyNPCPosX, newX)
    blackboard.Set(ctx.BB, blackboard.KeyNPCPosZ, newZ)
    blackboard.Set(ctx.BB, blackboard.KeyMoveState, "moving")
    return bt.Running
}
```

**dt 问题**：BT Context 没有 dt 字段。两个方案：
- A：给 Context 加 DeltaTime 字段（改 core/bt）
- B：BT 节点用固定 dt（不精确但简单）

**选 A**：Context 加 `DeltaTime float64` 字段，Scheduler 创建 Context 时传入。这是 core 层改动，但 DeltaTime 不是业务逻辑，是通用的帧时间。

```go
// core/bt/context.go
type Context struct {
    BB        *blackboard.Blackboard
    DeltaTime float64 // 本帧时间间隔（秒）
}
```

#### flee_from（逃跑）

```go
type fleeFrom struct {
    sourceKeyX string
    sourceKeyZ string
    distance   float64 // 安全距离
    speed      float64
}

func (n *fleeFrom) Tick(ctx *bt.Context) bt.Status {
    // 读取威胁源坐标
    // 计算反方向
    // 距离 >= safeDistance → Success
    // 否则沿反方向移动 → Running
    // 威胁源不存在 → Failure
}
```

#### 注册

```go
func DefaultRegistry() *Registry {
    // ...现有节点...
    r.Register("move_to", moveToFactory)
    r.Register("flee_from", fleeFromFactory)
    return r
}
```

### 6. BB Key 新增

```go
var KeyMoveTargetX = NewKey[float64]("move_target_x")
var KeyMoveTargetZ = NewKey[float64]("move_target_z")
```

`move_state` 已在需求 1 注册。

---

## 方案对比

### 备选方案：移动逻辑全在 BT 节点中（不选）

不改 MovementComponent.Tick，游荡/巡逻全用 BT 节点实现（patrol 节点、wander 节点）。

**不选的理由**：
1. 游荡和巡逻是组件配置驱动的行为，不应该硬编码到 BT 树中——运营配置 `move_type: "wander"` 就应该自动游荡，不应该要求 BT 树里写 wander 节点
2. BT 节点适合"有明确触发条件的动作"（逃跑、追逐），而游荡/巡逻是"无事时的默认行为"
3. 如果全放 BT → simple 级 NPC（无 behavior）不能移动，但需求说 simple NPC 应该能游荡

**选定方案**：游荡/巡逻在 MovementComponent.Tick（组件配置驱动），追逐/逃跑在 BT 节点（行为树动态驱动）。

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码 NPC 参数 | **不违反** | move_speed/wander_radius/waypoints 从配置读取 |
| 禁止 core/ import runtime/ | **需评估** | Context 加 DeltaTime 是通用字段，不引入业务依赖。move_to/flee_from 节点读写 BB，不 import runtime/ |
| 禁止 BT 反向驱动 FSM | **不违反** | move_to/flee_from 只改位置和 move_state，不改 FSM 状态 |
| 禁止 Blackboard 裸 map | **不违反** | 新 Key 通过 BBKey[T] |
| 禁止过度设计 | **不违反** | moveToward 是纯数学函数，无抽象 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | 中性 | 移动不影响事件 |
| 加 NPC 类型 | **正面** | 不同 move_speed/radius 产生不同移动行为 |
| NPC 间交互 | **正面** | move_to/flee_from 是交互的基础动作 |

---

## 依赖方向

```
internal/runtime/component/movement.go
  → internal/core/blackboard/      (BB Key)

internal/core/bt/move_to.go
  → internal/core/blackboard/      (BB Key)
  → internal/core/bt/              (Node, Context, Status)

internal/runtime/ (Scheduler)
  → internal/runtime/npc/          (SyncPosition)
```

move_to/flee_from 在 core/bt/ 包中，只依赖 core/blackboard。不 import runtime/。

---

## 并发安全

无新增共享状态。MovementComponent 的运行时字段（spawnX/target/etc）是 NPC 私有数据。SyncPosition 在 Scheduler ForEach 回调内顺序执行。

---

## 配置变更

无新增配置文件。MovementComponent 的字段已在需求 0 Schema 定义。

需新增 BT 节点 Schema（`configs/schemas/node_types/move_to.json` 和 `flee_from.json`）。

---

## 测试策略

### 单元测试

| 模块 | 覆盖 |
|------|------|
| `movement.go` | tickWander：选目标/移动/到达/等待/边界 |
| `movement.go` | tickPatrol：顺序路点/循环/到达切换 |
| `movement.go` | moveToward：直线移动/到达/零距离 |
| `bt/move_to` | 正常移动 Running/到达 Success/目标不存在 Failure |
| `bt/flee_from` | 反方向移动 Running/安全距离 Success/源不存在 Failure |

### 集成测试

| 场景 | 验证 |
|------|------|
| wander NPC 多 Tick 后位置变化 | npc_pos_x/z 不再等于 spawn 坐标 |
| patrol NPC 经过全部路点后循环 | 回到 waypoint[0] |
| 广播坐标更新 | world_snapshot 中 NPC 坐标随移动变化 |
