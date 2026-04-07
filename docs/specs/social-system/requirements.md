# social-system 需求分析

## 动机

v2 的 NPC 之间完全隔离——每个 NPC 独立感知、独立决策，互不知道对方存在。这导致：

1. **无群体行为**：一群狼中一只发现猎物，其他狼毫无反应。真实 AI 系统中同群组 NPC 应该共享感知信息。
2. **无阵营关系**：NPC 无法区分"同伴"和"敌人"。目前所有 NPC 对事件反应一致，没有阵营差异。
3. **leader/follower 无法实现**：SocialComponent 已有 Role/FollowTarget 字段（需求 1 定义），但无逻辑消费——follower 不跟随 leader，leader 被消灭后 follower 无反应。
4. **扩展轴 3 未验证**：v2 的三个扩展轴中"NPC 间交互"始终是预留未实现。

SocialComponent 在需求 1 已定义数据结构，但没有 GroupManager、没有群组感知共享、没有跟随逻辑。

## 优先级

**中高**。依赖需求 2（感知）和需求 5（移动）。被需求 7（区域系统）弱依赖——区域管理时需要知道 NPC 群组归属。

## 预期效果

### 场景 1：群组感知共享

5 只狼（group_id="wolf_pack_1"），分布在不同位置：
1. 狼 A 在感知范围内检测到爆炸事件，获得 PerceiveResult
2. GroupManager 将此感知结果广播给同组的狼 B/C/D/E（即使它们不在感知范围内）
3. 狼 B/C/D/E 的决策中心也收到此威胁信息，做出逃跑决策
4. 效果：一只发现威胁 → 全组反应

### 场景 2：follower 跟随 leader 移动

狼群 leader="wolf_1"，follower="wolf_2"/"wolf_3"：
1. wolf_1 巡逻移动
2. wolf_2/wolf_3 的 movement 组件自动跟随 wolf_1 的位置
3. wolf_1 和 wolf_2/wolf_3 保持一定距离（不完全重叠）

### 场景 3：leader 被消灭后 follower 行为变化

1. wolf_1（leader）被移除（remove_npc）
2. GroupManager 检测到 leader 丢失
3. wolf_2/wolf_3 的 BB 写入 `leader_lost = true`
4. FSM 可读此条件触发状态转换（如切换到 Flee 或选举新 leader）

### 场景 4：群体逃跑

狼群中的 wolf_1 FSM 转入 Flee 状态：
1. GroupManager 检测到同组成员进入 Flee
2. 向同组其他成员 BB 写入 `group_alert = true`
3. 其他成员的 FSM 读到 group_alert → 也进入 Flee
4. 效果：一只跑 → 全组跑

## 依赖分析

- **依赖**：
  - 需求 1 组件化架构（已完成）：SocialComponent 数据结构
  - 需求 2 感知深化（已完成）：PerceiveResult 是共享感知的数据单元
  - 需求 5 移动系统（已完成）：follower 跟随需要 MoveToward

- **被依赖**：
  - 需求 7 区域系统：群组整体休眠/唤醒
  - 需求 8 可观测性：群组状态查询

## 改动范围

| 包 | 变更类型 | 文件数 | 说明 |
|----|---------|--------|------|
| `internal/runtime/social/` | **新增** | 2-3 | GroupManager + 群组感知共享 |
| `internal/runtime/component/social.go` | **修改** | 1 | follow 模式 Tick 实现 |
| `internal/runtime/scheduler.go` | **修改** | 1 | 集成 GroupManager |
| `internal/core/blackboard/keys.go` | **修改** | 1 | 新增 BB Key |
| 测试文件 | **新增** | 3-4 | GroupManager 测试、集成测试 |

预估 10-12 个文件。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|--------|---------|------|
| 加事件源 | **正面** | 新事件通过群组感知共享自动传播给同组 NPC |
| 加 NPC 类型 | **正面** | 不同 NPC 配置不同 group_id/faction/role |
| NPC 间交互 | **直接服务** | 这就是扩展轴 3 的实现 |

## 验收标准

### GroupManager

- **R1**：定义 `GroupManager` 结构体，管理 group_id → []*Instance 的映射
- **R2**：NPC 创建时自动注册到 GroupManager（有 social 组件且 group_id 非空）
- **R3**：NPC 移除时自动从 GroupManager 注销
- **R4**：`GetGroup(groupID) []*Instance` 获取同组成员

### 群组感知共享

- **R5**：Scheduler Tick 中，每个 NPC 的 perceived 感知结果广播给同组其他成员
- **R6**：共享的感知结果合并到接收方的 perceived 列表中（去重：同 Event.ID 不重复）
- **R7**：无 social 组件或 group_id 为空的 NPC 不参与共享

### follower 跟随

- **R8**：MovementComponent 的 follow 模式实现：从 GroupManager 查找 leader 位置，用 MoveToward 跟随
- **R9**：follower 与 leader 保持最小距离 2m（避免重叠）
- **R10**：leader 不存在时 follower 的 move_state 写 "idle"

### leader 丢失检测

- **R11**：GroupManager 在 NPC 移除时检测是否为 leader，如是则向同组 follower 的 BB 写 `leader_lost = true`
- **R12**：新增 BB Key `leader_lost`(bool)

### 群体状态传播

- **R13**：GroupManager 在 Tick 中检查每组：如果有成员 FSM 状态为 "Flee" → 向同组其他成员 BB 写 `group_alert = true`
- **R14**：无成员 Flee 时 `group_alert = false`
- **R15**：新增 BB Key `group_alert`(bool)

### 向后兼容

- **R16**：无 social 组件的 NPC 不受影响
- **R17**：现有集成测试和 e2e 测试全部通过

### 性能

- **R18**：100 NPC 分 10 组，群组感知共享 + 状态传播总耗时 < 1ms/Tick

## 不做什么

- **不做阵营间关系配置**：阵营关系（友好/中立/敌对）需要配置表，在运营平台配置后续 spec 做。本 spec 只区分"同阵营=同 group"
- **不做动态加入/离开群组**：NPC 的 group_id 在创建时固定，运行时不变
- **不做 leader 选举**：leader 丢失后写 BB `leader_lost`，但不自动选新 leader。选举逻辑通过 FSM/BT 配置实现
- **不做跨区域群组**：同组 NPC 必须在同一区域。跨区域群组在需求 7 处理
