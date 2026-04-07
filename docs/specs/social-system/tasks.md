# social-system 任务拆解

## T1: BB Key 新增 (R12, R15)

**文件**：
- `internal/core/blackboard/keys.go`

**做完了是什么样**：
- 新增 4 个 Key：`leader_lost`(bool)、`group_alert`(bool)、`follow_target_x`(float64)、`follow_target_z`(float64)
- 编译通过

---

## T2: GroupManager 核心（Register/Unregister/GetGroup/GetLeader）(R1, R2, R3, R4)

**文件**：
- `internal/runtime/social/group_manager.go`
- `internal/runtime/social/group_manager_test.go`

**做完了是什么样**：
- GroupManager struct + RWMutex
- Register：有 social 组件且 group_id 非空时加入
- Unregister：移除，leader 移除时写同组 follower BB `leader_lost=true`
- GetGroup/GetLeader/AllGroups 查询
- 测试覆盖：注册/注销/查询/leader 丢失写 BB

---

## T3: leader 丢失检测 (R11)

**文件**：
- `internal/runtime/social/group_manager.go`（Unregister 中实现）
- `internal/runtime/social/group_manager_test.go`（追加）

**做完了是什么样**：
- Unregister 中检测移除的 NPC 是否 role=leader
- 如是，遍历同组成员写 BB `leader_lost=true`
- 测试：移除 leader → follower BB 验证

---

## T4: MovementComponent tickFollow 实现 (R8, R9, R10)

**文件**：
- `internal/runtime/component/movement.go`
- `internal/runtime/component/movement_test.go`

**做完了是什么样**：
- tickFollow 从 BB 读 `follow_target_x/z` → MoveToward
- 距离 < 2m 时 move_state="arrived"（避免重叠）
- follow_target 不存在时 move_state="idle"
- 测试：跟随移动/最小距离/目标缺失

---

## T5: Scheduler 两遍遍历重构 + 群组感知共享 (R5, R6, R7)

**文件**：
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- Scheduler 新增 `GroupManager *social.GroupManager` 字段
- Tick 重构为两遍：第一遍感知暂存 → shareGroupPerception → 第二遍决策+行为
- shareGroupPerception：同组 perceived 合并去重（Event.ID），Strength 取 max
- 无 GroupManager 时保持单遍（v2 兼容）

---

## T6: follower 目标更新 + 群体状态传播 (R13, R14)

**文件**：
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- updateFollowerTargets：从 GroupManager 查 leader 位置 → 写 follower BB follow_target_x/z
- propagateGroupState：检查各组成员 FSM Flee → 写 group_alert
- 在 Tick 的第二遍前调用 updateFollowerTargets，第二遍后调用 propagateGroupState

---

## T7: Gateway handler 集成 (R2, R3)

**文件**：
- `internal/gateway/handler.go`
- `cmd/server/main.go`

**做完了是什么样**：
- RegisterHandlers 签名新增 `gm *social.GroupManager` 参数
- spawn_npc handler 创建后调用 `gm.Register(inst)`
- remove_npc handler 移除前调用 `gm.Unregister(inst)`
- main.go 创建 GroupManager 传入

---

## T8: 集成测试 (R16, R17, R18)

**文件**：
- `internal/runtime/social_integration_test.go`
- `test/e2e/helpers_test.go`（适配新签名）

**做完了是什么样**：
- 集成测试 1 群组感知共享：一只感知→同组其他成员 perceived 有数据
- 集成测试 2 follower 跟随：leader 移动 → follower 位置跟随变化
- 集成测试 3 leader 丢失：移除 leader → follower BB leader_lost=true
- 集成测试 4 群体逃跑：一只 Flee → 同组 group_alert=true
- 现有集成测试 + e2e 全部通过

---

## 执行顺序

```
T1  BB Key
 └→ T2  GroupManager 核心
     └→ T3  leader 丢失
         └→ T4  tickFollow
             └→ T5  Scheduler 两遍 + 感知共享
                 └→ T6  follower 更新 + 状态传播
                     └→ T7  Gateway 集成
                         └→ T8  集成测试
```

严格顺序。
