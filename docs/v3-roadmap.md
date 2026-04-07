# v3 路线图：游戏 AI 角色系统深化

## 背景

v2 验证了 FSM + BT + 决策中心三位一体架构的正确性（扩展轴步骤 1-4 通过）。v3 的目标是将这个架构从"可行性验证"推进到"企业级 AI 角色系统"——NPC 能自主感知、决策、行动，行为真实可信且高度可配置。

**核心原则**：
- 只做游戏 AI 角色系统的**深度**，不做游戏服务器的广度
- 商店/任务/战斗/玩家等系统只预留接口，不实现
- 企业级工程标准：每个需求走 spec-create 流程（需求分析 → 设计方案 → 任务拆解）

## 参考来源

架构方向参考《洛克王国：世界》，并研究了 WoW、GW2、ESO、Pokemon Legends、Monster Hunter 等游戏的实现方案。详细研究记录见本次对话。

## 协作方

| 角色 | 职责 | 关系 |
|------|------|------|
| 游戏服务端 | AI 角色系统核心实现 | Schema 定义方，Go 代码 |
| ADMIN 运营平台 | Schema 驱动动态表单、配置管理 | Schema 消费方，Vue + MongoDB |
| Unity 客户端 | NPC 表现层、区域边界编辑（后续） | 暂不参与 |

**已与 ADMIN 对齐的约定**：
- Schema 格式：JSON Schema Draft 7
- Schema 归属：服务端定义 → ADMIN 存储渲染
- API 路径：`/api/v1/npc-templates`（替代旧 `/api/v1/npc-types`）、`/api/v1/regions`（新增）、`/api/v1/component-schemas`（只读）、`/api/v1/npc-presets`（只读）、`/api/v1/node-type-schemas`（只读）、`/api/v1/condition-type-schemas`（只读）
- NPC 模板导出格式：组件化结构 `{"name", "preset", "components": {"identity": {...}, "behavior": {...}, ...}}`
- 黑板 Key：从硬编码白名单改为动态（每组件声明所需 Key，NPC 的可用 Key = 已启用组件 Key 并集）
- Schema 版本兼容：新增字段补 default，删除字段前端忽略，不做迁移系统

## 9 个需求（按顺序执行）

### 需求 0：Schema 契约

**状态**：✅ 已完成（PR #7 merged）

**目标**：输出服务端与 ADMIN 的共享数据格式契约

**产出**：
- 10 个组件 JSON Schema（identity / position / behavior / perception / movement / personality / needs / emotion / memory / social）
- 4 个预设定义（simple / reactive / autonomous / social）
- 8 个 BT 节点类型 Schema（与 `bt.DefaultRegistry()` 精确一致）
- 2 个 FSM 条件类型 Schema（leaf + composite，与 `rule.Condition` 精确一致）
- 1 个区域数据结构 Schema

**改动**：25 个 JSON 文件，零 Go 代码

**spec 文档**：`docs/specs/component-schema-contract/`

---

### 需求 1：组件化 NPC 架构

**状态**：✅ 已完成（PR #8 merged）

**目标**：NPC Instance 从固定 struct 改为组件容器，运营加配置就能创造新 NPC

**关键设计**：
- `Component` 接口 + `Tickable` 接口（AI 管线显式编排，其他组件泛化 Tick）
- 组件注册表 + 工厂模式（加新组件不改现有代码）
- v2 旧配置格式自动检测转换（现有测试不需改配置文件）
- 10 个组件数据结构定义（深入逻辑留后续需求）
- Scheduler 按组件有无决定执行路径

**改动**：`internal/runtime/component/`（新增）、`internal/runtime/npc/`（重构）、`internal/runtime/scheduler.go`（适配）、`internal/config/`（扩展）、`configs/npc_templates/`（新增）

**依赖**：需求 0

---

### 需求 2：感知系统深化

**状态**：✅ 已完成（PR #9）

**目标**：NPC 有注意力上限，感知不是 0/1 二值判断

**关键点**：
- 注意力容量：最多同时关注 N 个事件（可配置），超出时保留最高优先级
- 感知衰减：距离越远感知越弱，连续衰减曲线
- 区域隔离：NPC 只感知本区域事件
- 遮挡机制预留接口，不实现

**改动**：`internal/runtime/perception/`

**依赖**：需求 1

---

### 需求 3：决策系统深化

**状态**：✅ 已完成（PR #10）

**目标**：多维度决策——威胁 + 需求 + 情绪三路输入，性格权重仲裁

**关键点**：
- 需求驱动：每个需求按 decay_rate 衰减，低于阈值时产生行为动机
- 情绪影响：情绪修改决策阈值（恐惧高 → 逃跑阈值降低）
- 多目标仲裁：威胁分 vs 需求分 vs 情绪修正的加权仲裁，权重由 personality 组件配置

**改动**：`internal/runtime/decision/`

**依赖**：需求 1、需求 2（感知输入）

---

### 需求 4：记忆系统

**状态**：未开始

**目标**：NPC 能记住过去发生的事，影响未来决策

**关键点**：
- 记忆条目：`{type, target_id, value, timestamp, ttl}`
- 记忆类型：威胁记忆、位置记忆、社交记忆
- 记忆衰减：TTL 到期自动遗忘，重复刺激强化
- 记忆影响决策：有威胁记忆时对该威胁源更敏感

**改动**：`internal/runtime/component/` memory 组件实现

**依赖**：需求 3（影响决策）

---

### 需求 5：移动系统

**状态**：✅ 已完成（PR #12）

**目标**：NPC 真正在世界中移动，不再是 stub_action

**关键点**：
- 游荡（wander）：spawn 点 + 半径内随机目标，匀速移动
- 巡逻（patrol）：按路点顺序循环
- 追逐（chase）：朝目标移动，有最大追逐距离
- 逃跑（flee）：沿威胁反方向移动
- 行为持续性：移动跨多 Tick 完成
- 边界约束：不超出区域和 wander_radius

**改动**：movement 组件实现 + 新 BT 节点（move_to / flee_from / patrol）

**依赖**：需求 1

---

### 需求 6：社交系统

**状态**：未开始

**目标**：NPC 能感知其他 NPC，产生群体行为

**关键点**：
- 群组：同 group_id 共享感知（一个发现威胁，全组知道）
- 阵营：同阵营友好，异阵营关系可配置
- 角色：leader / follower，follower 跟随 leader
- 群体行为：群体逃跑、群体聚集、leader 被消灭后行为变化

**改动**：social 组件实现 + 群组管理器 + NPC→NPC 事件传播

**依赖**：需求 2（感知）、需求 5（移动）

---

### 需求 7：区域系统

**状态**：未开始

**目标**：AI 系统自己的性能基础设施

**关键点**：
- 区域定义：ID、名称、边界（多边形）、刷怪表
- 休眠 / 唤醒：无玩家区域不 Tick NPC，玩家进入时从配置初始化
- 空间索引：区域内空间哈希，加速 AOI 查询
- 区域级广播：玩家只收所在区域的 NPC 状态快照
- 区域级事件隔离：事件默认限定在发生区域

**改动**：`internal/runtime/world/`（新增）

**依赖**：需求 1

---

### 需求 8：可观测性与调试

**状态**：未开始

**目标**：企业级 AI 系统必须能回答"这个 NPC 为什么做了这个决策"

**关键点**：
- 决策日志：每次仲裁记录输入（威胁分 / 需求分 / 情绪修正）和输出
- 行为追踪：FSM 状态变迁 + BT 执行路径的结构化日志
- 性能指标：每 Tick 耗时、每区域 NPC 数量（Prometheus 格式）
- NPC 调试查询：通过 API 查询单个 NPC 完整内部状态

**改动**：结构化日志 + 指标采集 + 调试 API

**依赖**：需求 1-7 全部完成后贯穿

---

## 依赖链

```
需求 0 Schema 契约
  └→ 需求 1 组件化架构
       ├→ 需求 2 感知深化
       │    └→ 需求 3 决策深化
       │         └→ 需求 4 记忆系统
       ├→ 需求 5 移动系统
       │
       ├→ 需求 6 社交系统 ←── 依赖需求 2 + 需求 5
       │
       └→ 需求 7 区域系统
            └→ 需求 8 可观测性
```

可并行的：需求 2 和需求 5 可以在需求 1 完成后同时启动。

## 开发流程

每个需求严格按 `/spec-create` 流程：
1. Phase 1：需求分析（`requirements.md`）→ 审批
2. Phase 2：设计方案（`design.md`）→ 审批
3. Phase 3：任务拆解（`tasks.md`）→ 审批
4. 创建 feature 分支，按任务逐个 `/spec-execute`
5. 每个任务完成后运行测试验证
6. 全部任务完成后 PR → squash merge → main

## v2 保留的核心资产

以下模块在 v3 中 **100% 复用，不重写**：

| 模块 | 路径 | 说明 |
|------|------|------|
| FSM 引擎 | `internal/core/fsm/` | 配置驱动状态机，不改 |
| BT 引擎 | `internal/core/bt/` | 节点注册表 + 从 JSON 构建，不改 |
| Blackboard | `internal/core/blackboard/` | 泛型强类型访问，只追加新 Key |
| 条件规则引擎 | `internal/core/rule/` | AND/OR 树 + 7 种操作符，不改 |
| 事件总线 | `internal/runtime/event/` | TTL 衰减模型，不改 |

## 不做的事情

| 排除项 | 理由 |
|--------|------|
| 商店 / 交易系统 | 不是 AI 角色系统的职责 |
| 任务 / 对话系统 | 不是 AI 角色系统的职责 |
| 战斗 / 伤害计算 | 不是 AI 角色系统的职责 |
| 玩家数据管理 | 不是 AI 角色系统的职责 |
| 频道 / 世界实例化 | 不是 AI 角色系统的职责 |
| 配置热更新 | 预留接口，不在 v3 范围 |
| 多节点分布式部署 | 预留接口，不在 v3 范围 |

以上系统如需与 AI 角色系统交互，通过预留接口（Go interface + mock 测试）对接。
