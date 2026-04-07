# social-system 设计方案

## 方案描述

### 1. GroupManager

```go
// 定义在 internal/runtime/social/group_manager.go
type GroupManager struct {
    groups map[string][]*npc.Instance // group_id → members
}

func NewGroupManager() *GroupManager
func (gm *GroupManager) Register(inst *npc.Instance)    // 有 social 组件且 group_id 非空时加入
func (gm *GroupManager) Unregister(inst *npc.Instance)  // 移除，检测 leader 丢失
func (gm *GroupManager) GetGroup(groupID string) []*npc.Instance
func (gm *GroupManager) GetLeader(groupID string) *npc.Instance
```

Register 逻辑：
```go
func (gm *GroupManager) Register(inst *npc.Instance) {
    social, ok := npc.GetComponent[*component.SocialComponent](inst, "social")
    if !ok || social.GroupID == "" {
        return
    }
    gm.groups[social.GroupID] = append(gm.groups[social.GroupID], inst)
}
```

Unregister 逻辑：
```go
func (gm *GroupManager) Unregister(inst *npc.Instance) {
    social, ok := npc.GetComponent[*component.SocialComponent](inst, "social")
    if !ok || social.GroupID == "" {
        return
    }
    // 从列表移除 inst
    // 如果 inst 是 leader → 向同组 follower 写 BB leader_lost=true
}
```

### 2. 群组感知共享

在 Scheduler.Tick 中，感知过滤后、决策前：

```go
// 收集阶段：每个 NPC 的 perceived 结果暂存
type npcPerceived struct {
    inst      *npc.Instance
    perceived []perception.PerceiveResult
}

// 共享阶段：同组成员的 perceived 合并
func (s *Scheduler) shareGroupPerception(gm *social.GroupManager, allPerceived []npcPerceived) {
    // 按 group_id 分组
    // 同组内：每个成员的 perceived 合并到组池
    // 组池去重（同 Event.ID）
    // 组池分发回每个成员
}
```

**去重**：同一个 Event 可能被多个成员感知到。用 Event.ID 去重，保留 Strength 最高的。

**执行位置**：在所有 NPC 的感知过滤完成后、决策评估前执行共享。这意味着 Scheduler.Tick 需要两遍遍历：
1. 第一遍：所有 NPC 做感知过滤，暂存 perceived
2. 群组共享：合并同组 perceived
3. 第二遍：所有 NPC 做决策+FSM+BT+组件Tick

### 3. Scheduler.Tick 重构为两遍

```go
func (s *Scheduler) Tick(dt float64) {
    s.EventBus.Tick(dt)
    activeEvents := s.EventBus.Active()
    now := time.Now().UnixMilli()

    // --- 第一遍：感知 ---
    type npcState struct {
        inst      *npc.Instance
        perceived []perception.PerceiveResult
    }
    var states []npcState

    s.Registry.ForEach(func(inst *npc.Instance) {
        blackboard.Set(inst.BB, blackboard.KeyCurrentTime, now)
        var perceived []perception.PerceiveResult
        // ... 感知过滤（现有逻辑）
        states = append(states, npcState{inst, perceived})
    })

    // --- 群组感知共享 ---
    if s.GroupManager != nil {
        s.shareGroupPerception(states)
    }

    // --- 第二遍：决策+行为 ---
    for i := range states {
        st := &states[i]
        // ... 决策+FSM+BT+组件Tick（现有逻辑，用 st.perceived）
    }

    // --- 群体状态传播 ---
    if s.GroupManager != nil {
        s.propagateGroupState()
    }
}
```

### 4. follower 跟随

MovementComponent.Tick 中 follow 模式：

```go
case "follow":
    c.tickFollow(bb, posX, posZ, dt)
```

tickFollow 需要 leader 的位置。但 MovementComponent 不持有 GroupManager 引用——通过 BB 间接通信：

- Scheduler 在 Tick 前，对每个 follower NPC，从 GroupManager 查找 leader 位置，写入 follower 的 BB：`follow_target_x`、`follow_target_z`
- MovementComponent.tickFollow 从 BB 读 `follow_target_x/z`，用 MoveToward 跟随

```go
// Scheduler 中
func (s *Scheduler) updateFollowerTargets() {
    s.Registry.ForEach(func(inst *npc.Instance) {
        social, ok := npc.GetComponent[*component.SocialComponent](inst, "social")
        if !ok || social.Role != "follower" || social.GroupID == "" {
            return
        }
        leader := s.GroupManager.GetLeader(social.GroupID)
        if leader == nil {
            return
        }
        blackboard.Set(inst.BB, blackboard.KeyFollowTargetX, leader.Position.X)
        blackboard.Set(inst.BB, blackboard.KeyFollowTargetZ, leader.Position.Z)
    })
}
```

MovementComponent.tickFollow：
```go
func (c *MovementComponent) tickFollow(bb *blackboard.Blackboard, posX, posZ, dt float64) {
    targetX, okX := blackboard.Get(bb, blackboard.KeyFollowTargetX)
    targetZ, okZ := blackboard.Get(bb, blackboard.KeyFollowTargetZ)
    if !okX || !okZ {
        blackboard.Set(bb, blackboard.KeyMoveState, "idle")
        return
    }
    dist := Distance2D(posX, posZ, targetX, targetZ)
    if dist < 2.0 { // 最小距离，避免重叠
        blackboard.Set(bb, blackboard.KeyMoveState, "arrived")
        return
    }
    maxStep := c.MoveSpeed * dt
    newX, newZ := MoveToward(posX, posZ, targetX, targetZ, maxStep)
    c.writePosition(bb, newX, newZ, "moving")
}
```

### 5. 群体状态传播

```go
func (s *Scheduler) propagateGroupState() {
    for groupID, members := range s.GroupManager.AllGroups() {
        hasFlee := false
        for _, inst := range members {
            if beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior"); ok {
                if beh.FSM != nil && beh.FSM.Current() == "Flee" {
                    hasFlee = true
                    break
                }
            }
        }
        for _, inst := range members {
            blackboard.Set(inst.BB, blackboard.KeyGroupAlert, hasFlee)
        }
    }
}
```

### 6. BB Key 新增

```go
var KeyLeaderLost     = NewKey[bool]("leader_lost")       // leader 被移除
var KeyGroupAlert     = NewKey[bool]("group_alert")        // 同组有成员 Flee
var KeyFollowTargetX  = NewKey[float64]("follow_target_x") // leader X 坐标
var KeyFollowTargetZ  = NewKey[float64]("follow_target_z") // leader Z 坐标
```

### 7. Gateway handler 集成

spawn_npc handler 创建后调用 `groupManager.Register(inst)`。
remove_npc handler 移除前调用 `groupManager.Unregister(inst)`。

---

## 方案对比

### 备选方案：NPC 间直接引用（不选）

SocialComponent 持有 `leader *Instance` 指针，follower 直接读 leader.Position。

**不选的理由**：
1. 循环引用风险——A 引用 B，B 引用 A
2. 生命周期管理复杂——leader 被移除后 follower 的指针悬空
3. 无法序列化/调试——指针无法在 BB/日志中展示
4. 违反组件独立原则——组件间通过 BB 通信是已建立的模式

选定方案通过 GroupManager 中心化管理 + BB 间接通信，职责清晰。

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码 NPC 参数 | **不违反** | group_id/faction/role 从配置读取 |
| 禁止 core/ import runtime/ | **不违反** | GroupManager 在 runtime/social/ |
| 禁止 BT 反向驱动 FSM | **不违反** | group_alert 写 BB，FSM 读 BB 转换 |
| 禁止 Blackboard 裸 map | **不违反** | 新 Key 通过 BBKey[T] |
| 禁止 Gateway 承担非网络职责 | **不违反** | Gateway 只调 Register/Unregister，逻辑在 GroupManager |
| 禁止过度设计 | **不违反** | GroupManager 是简单 map，无框架 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面** | 群组共享自动传播新事件 |
| 加 NPC 类型 | **正面** | 不同 group_id/role 配置 |
| NPC 间交互 | **直接实现** | 扩展轴 3 |

---

## 依赖方向

```
internal/runtime/ (Scheduler)
  → internal/runtime/social/       (GroupManager)
  → internal/runtime/component/    (SocialComponent)
  → internal/runtime/npc/          (Instance)

internal/runtime/social/
  → internal/runtime/npc/          (Instance 引用)
  → internal/runtime/component/    (GetComponent)
  → internal/core/blackboard/      (BB Key)

internal/gateway/
  → internal/runtime/social/       (Register/Unregister)
```

单向向下，无循环。

---

## 并发安全

GroupManager.groups 由 Scheduler（ForEach 内）读，Gateway（Register/Unregister）写。需要 RWMutex 保护。

```go
type GroupManager struct {
    mu     sync.RWMutex
    groups map[string][]*npc.Instance
}
```

---

## 配置变更

无新增配置文件。SocialComponent 的 group_id/faction/role 已在需求 0 Schema 和需求 1 struct 定义。

---

## 测试策略

### 单元测试

| 模块 | 覆盖 |
|------|------|
| `social/` | Register/Unregister/GetGroup/GetLeader |
| `social/` | leader 移除写 leader_lost |
| `movement.go` | tickFollow：跟随/最小距离/leader 不存在 |

### 集成测试

| 场景 | 验证 |
|------|------|
| 群组感知共享 | 一只感知→全组收到 perceived |
| follower 跟随 | follower 位置跟随 leader 变化 |
| leader 丢失 | 移除 leader → follower BB leader_lost=true |
| 群体逃跑 | 一只 Flee → 全组 group_alert=true |
