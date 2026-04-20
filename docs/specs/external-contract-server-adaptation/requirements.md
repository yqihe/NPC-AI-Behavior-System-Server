# external-contract-server-adaptation 需求分析

**范围**：服务端仓单仓工作。ADMIN 侧已冻结（`external-contract-admin-shape-alignment` 已合并 PR #39）。
**输入契约锚**：
- [docs/architecture/api-contract.md](../../architecture/api-contract.md) v1.1.2（ADMIN 契约源 `e04cf0d`，含 6 NPC 字段形态基线 §4）

## 动机

当前服务端存在两条 NPC spawn 路径：

| 路径 | Schema | 调用点 | 状态 |
|---|---|---|---|
| 组件化 | `{name, preset, components}` | zone.Spawn / gateway handler / 全部集成测试 / configs/npc_templates/*.json | 主路径 |
| ADMIN 扁平 | `{template_ref, fields, behavior}` | main.go spawnFromADMINTemplates 回退路径 | 仅 ADMIN HTTP 接入时激活 |

**双路径带来的实际问题**（非假设，已发生）：

1. **sync 即坏测试**：`go run ./cmd/sync -api http://localhost:9821` 用 ADMIN 形状覆盖本地 `configs/npc_templates/wolf_common.json`，立刻导致 13 个集成测试（decision/memory/movement/perception/social）在 `ParseNPCTemplate` 阶段炸 `unrecognized config format`
2. **v2 兼容层死代码**：`convertV2Format` 路径（v2 `{type_name, fsm_ref, bt_refs}` → 组件化）在 ADMIN 一统后永无来源，属"存在但无代码路径"的死兼容（违反 `red-lines.md` 禁止过度设计）
3. **契约漂移风险**：双路径意味着 Instance 创建有两套 BB 初始化逻辑——组件化走 identity/position/behavior 组件，ADMIN 走 `SetDynamic` 扁平写入。未来新字段行为在两条路径上不一致的概率逐次累积
4. **毕设可扩展性证明打折**：扩展轴 2"加 NPC 类型 = 加配置 + 加测试"在双路径下不成立——新增 NPC 类型必须考虑两套 schema 与两套创建函数的兼容

**不做会怎样**：ADMIN 侧接入每前进一步，服务端侧测试基线每被打破一次；最终陷入"跑完 sync 还要手动回滚 configs/"的泥潭，毕设答辩时无法出示端到端联调证据。

## 优先级

**高**。

依据：
- 直接阻塞毕设联调验收路径（扩展轴 3-4 步、ADMIN 端到端数据回路）
- 是上一个 spec `external-contract-admin-shape-alignment`（ADMIN 侧）的**必要配对**，不做等于白做（ADMIN 已 seed 好 6 NPC，服务端拉不下来跑通等于联调断链）
- 毕设答辩前剩余时间窗口有限，尽早收敛双路径

## 预期效果

### 场景 1：sync → 测试 → e2e 一条直路

开发者从干净仓库起步：
```bash
go run ./cmd/sync -api http://localhost:9821   # 拉 6 NPC + 事件/FSM/BT
go test ./...                                  # 全绿（不需要 revert configs/）
docker compose up --build                      # 6 NPC 全部 spawn + tick
```
**当前状态**：第 2 步会炸 13 个测试；第 3 步 ADMIN 路径 spawn 成功但组件化路径残留死代码。

### 场景 2：加新 NPC 类型 = 加 ADMIN 配置 + 加测试 + cmd/sync

运营在 ADMIN UI 创建新 NPC "bandit_raider"（warrior_base 模板，fields 齐全，behavior 引用已有 FSM/BT），服务端侧：
```bash
go run ./cmd/sync -api http://localhost:9821   # bandit_raider.json 自动出现在 configs/npc_templates/
go test ./...                                  # 全绿，新 NPC 被 zone spawn 测试读到
```
**不需要改**：admin_template.go / zone.go / handler.go / 任何 core 包。
**当前状态**：需要同时维护 `configs/npc_templates/` 下的组件化 JSON + ADMIN 端数据，schema 还不一致。

### 场景 3：集成测试使用 ADMIN 形状 fixture

集成测试从 `configs/npc_templates/wolf_common.json`（ADMIN 形状）加载 NPC，通过翻译层创建 Instance，memory/emotion/perception 等组件按翻译层规则初始化（ADMIN fields 未提供的组件取默认值或不启用）。测试断言仍基于 Instance 行为，不绑死组件初始化来源。

### 场景 4：ADMIN `guard_basic.fields={hp:100}` 端到端跑通

服务端拉取、解析、spawn、tick 全流程无 WARN/ERROR；`hp` 通过 `SetDynamic` 写入 BB（现行为保留），不被任何 BT 节点消费（R13 场景，ADMIN 侧孤儿字段既定策略）。

### 场景 5：v2 组件化 schema 彻底下线

`ParseNPCTemplate` / `convertV2Format` / `TemplateConfig`（JSON schema） / `NPCTypeConfig`（v2）相关代码从 `internal/runtime/npc/` 删除；`configs/npc_types/`（v2 兼容目录）删除；新增 NPC 文档只剩 ADMIN 路径说明。

## 依赖分析

**依赖**（已完成）：
- `external-contract-admin-shape-alignment`（ADMIN 仓 PR #39 合并）：8 字段 catalog + 4 模板 + 6 NPC 入库 + [api-contract.md](../../architecture/api-contract.md) 锚定
- 服务端 PR #17/#18/#19：ADMINTemplate 解析 + HTTPSource 端点对齐 + perception_range fallback
- 本地 PR #20：api-contract.md 镜像 + cmd/sync 端点修复

**被依赖**：
- 后续"压力/异常/回归深度联调"：只有单路径 + 稳定测试基线下才可做
- 毕设对照实验数据采集：实验三个模式（Hybrid/PureFSM/PureBT）加载 NPC 的路径必须单一、可复现

## 改动范围

| 包 / 目录 | 变更类型 | 文件数（估） | 说明 |
|---|---|---|---|
| `internal/runtime/npc/` | **重构** | 5–7 | 删 template.go / compat.go / v2 相关；admin_template.go 升级为翻译层 |
| `internal/runtime/zone/` | **修改** | 1–2 | zone.go spawn 路径切到 ADMIN 入口 |
| `internal/gateway/` | **修改** | 1 | handler.go spawn 消息处理切 ADMIN 入口 |
| `internal/config/` | **修改** | 3–4 | Source 接口去 LoadNPCTypeConfig；JSONSource/HTTPSource/MongoSource 同步 |
| `cmd/server/main.go` | **修改** | 1 | 合并 spawnFromADMINTemplates 为主路径，删组件化回退 |
| `configs/npc_templates/` | **重写** | 2（+ 5-6 新增）| butterfly_01 / wolf_common 转 ADMIN 形状；可选纳入 6 ADMIN NPC |
| `configs/npc_types/` | **删除** | 3 | civilian/guard/police 全删（v2 遗留） |
| 集成测试 `internal/runtime/*_test.go` | **修改** | 8–10 | 8 个 integration test + benchmark 换 fixture |
| e2e `test/e2e/` | **修改** | 1–2 | helpers_test 的 NPC spawn 消息用 ADMIN 形状 |
| `internal/runtime/npc/*_test.go` | **修改** | 2 | npc_test / compat_test（后者整体删除） |

**预估 30–40 个文件**。属大手术 spec；任务拆分必须按"不可破坏基线"顺序推进。

## 扩展轴检查

| 扩展轴 | 是否服务 | 说明 |
|---|---|---|
| 加事件源 | **中性** | 事件 schema 不变 |
| 加 NPC 类型 | **正面强** | 消除双 schema，"加配置不改代码"的承诺在 ADMIN 单路径下才真正成立 |
| NPC 间交互 | **正面弱** | 统一 Instance 创建路径，decision / social 组件初始化逻辑一致化 |

**主服务轴：加 NPC 类型**。此 spec 是扩展轴 2 的兜底保障。

## 验收标准

### 核心代码

- **R1**：`internal/runtime/npc/admin_template.go` 是 NPC 实例创建的**唯一**对外入口，暴露 `NewInstanceFromADMIN(id, pos, tmpl, src, btReg) (*Instance, error)`；任何调用方（zone / handler / main / 测试）不得直接构造 `Instance`
- **R2**：删除 `internal/runtime/npc/template.go` 中的 `TemplateConfig` struct 与 `NewInstanceFromTemplate` 函数
- **R3**：删除 `internal/runtime/npc/compat.go`（`ParseNPCTemplate` + `convertV2Format`）
- **R4**：删除 `NPCTypeConfig`（v2）在**生产 spawn 路径**（`zone/zone.go` / `gateway/handler.go` / `main.go`）的所有调用点。**保留** `NPCTypeConfig` struct、`ParseNPCTypeConfig` 函数、`Source.LoadNPCTypeConfig` 接口方法、3 个 Source 实现——供实验层（`internal/experiment/`）与测试层（integration / benchmark / source contract test）消费。NPCTypeConfig 是"本地 V2 fixture API"，不作为 production NPC 创建路径；R1 "NewInstanceFromADMIN 是唯一生产入口"语义不变。Phase 2 修正记录：原草案"全删 API"会击穿 `hybrid.go`（毕设 measurement framework 核心）+ 多处 e2e/integration 测试，属需求偏差
- **R5**：`component.Registry` **保留全量 13 个 factory**。5 个能力 component（memory / emotion / needs / personality / social）的实例化由 R17 opt-in 契约驱动——factory 不删除，但默认不调用

### 翻译层行为

- **R6**：`NewInstanceFromADMIN` 必须把 ADMIN `fields` + `behavior` 翻译到 `Instance`，且 `Instance` 的可观测行为（BB 值、FSM 状态、BT 注册、Perception 范围）与当前 [admin_template.go:53](../../../internal/runtime/npc/admin_template.go#L53) 一致（即不回归已通过 PR #17-#19 的行为）
- **R7**：ADMIN `fields` 中 ADMIN 未声明的任意字段通过 `SetDynamic` 写入 BB；已知 `hp`（guard_basic 孤儿）、`loot_table`、`is_boss` 等扩展字段保持 PR #17 以来的透明透传
- **R8**：`perception_range` fallback 链保留（PR #18 行为）

### 配置

- **R9**：`configs/npc_templates/butterfly_01.json` 与 `wolf_common.json` 重写为 ADMIN 形状 `{template_ref, fields, behavior}`
- **R10**：删除 `configs/npc_types/guard.json`（compat_test.go 同步删除后无 Go 消费者）+ `configs/bt_trees/guard/{alert,defend}.json`。**保留** `configs/npc_types/{civilian,police}.json`——civilian 是实验层（`hybrid.go` / `pure_fsm.go` / `fsm_dc.go`）+ 多处测试依赖，police 是 `test/e2e/extension_test.go` **扩展轴 2 演示核心**（police spawn → explosion → Engage 响应，对比 civilian Flee，证明"加 NPC 类型零代码"）。相应保留 `configs/fsm/{civilian,police}.json` + `configs/bt_trees/{civilian,police}/` 整目录。Phase 2 修正记录（两轮）：原草案整删目录；第一轮 grep 未覆盖 test/e2e/ 未纠正 civilian；第二轮补查发现 police 同类漏删。**Recurrent gap 教训**：声明"零消费者"前，grep 必须覆盖 `internal/` + `test/e2e/` + `internal/experiment/` + `docs/specs/` 全域
- **R11**：`go run ./cmd/sync -api http://localhost:9821` 执行后 `configs/` 全量改动可直接 `git add` 提交，不需人工编辑

### 测试

- **R12**：`go test ./...` 全绿（含原 13 个因 sync 而挂的集成测试）
- **R13**：集成测试与 e2e 测试的 fixture 改用 ADMIN 形状；fixture 辅助函数（若新增）不得超过一个（不为一次性 helper 引抽象）
- **R14**：至少一个集成测试锚 snapshot §4 的 `guard_basic.fields={hp:100}` 行为——验证孤儿字段透明透传不破坏 Instance 创建

### 联调端到端

- **R15**：`docker compose up --build` 启动，服务端拉 ADMIN live，6 NPC 全部 spawn 成功、无 WARN/ERROR、tick 连续 ≥ 30s；等价于重现 2026-04-18 smoke test 成功结果但在**删光双路径后**
- **R16**：关闭 ADMIN（`curl localhost:9821` 不可达）时，服务端降级到 JSONSource 从 `configs/` 加载，同样 6 NPC 全部 spawn 成功（证明 ADMIN 与 JSON 数据源等价可互换）

### 组件 opt-in 契约

**背景**：ADMIN 侧采用扁平 bool field 方案（A/B 决策后锁为 B，理由：ADMIN `/api/configs/npc_templates` 导出的是 NPC 实例 snapshot 不是模板；boolean 是 ADMIN 一等公民字段，零代码改动即可 seed；per-NPC 勾选在毕设规模 10-50 NPC 下 UX 可接受；避免拖延外部契约对齐 spec）。

- **R17（组件 opt-in 契约）**：服务端从 `config.fields` 中约定的 5 个 bool 字段读取组件 opt-in 标志：
  - `enable_memory` → MemoryComponent
  - `enable_emotion` → EmotionComponent
  - `enable_needs` → NeedsComponent
  - `enable_personality` → PersonalityComponent
  - `enable_social` → SocialComponent

  **absent ≡ false 语义锁定**：字段不存在等同于显式 `false`，消除"未声明 vs 显式关闭"歧义。翻译层按标志决定是否调用对应 factory 创建 component 并注册到 Instance。ADMIN seed 侧必须显式将 5 个 bool field 的 `properties.default_value` 设为 `false`，确保新建 NPC 携带 false 而非 null（否则导出 null 服务端解析歧义）

- **R18（级联依赖校验）**：服务端启动加载外部契约时，**遍历每个 NPC 实例**，遇 `enable_emotion=true ∧ enable_memory=false` **立即 fatal**，日志打印违规 NPC name 列表 + 指向 ADMIN UI 的修正路径；不跳过违规 NPC、不部分启动 Registry。校验在 Registry 填充阶段完成，先于任何 Tick 启动。级联矩阵文档化见 [docs/architecture/api-contract.md](../../architecture/api-contract.md) v1.1 "组件 opt-in 依赖矩阵"章节

- **R19（软/硬依赖契约）**：
  - **软依赖（允许）**：组件 X 读取组件 Y 写入的 BB key（如 `emotion.Tick()` 读 `KeyMemoryThreatValue`——由 `memory.Tick()` 写入）。Y 缺席时 X 必须降级到默认值，不得阻塞 tick 或 panic
  - **硬依赖（禁止）**：**组件代码**中 `GetComponent[Y]` 访问其他组件类型 Y。例：`emotion.Tick()` 内部 `GetComponent[*MemoryComponent](inst, "memory")` 属违规
  - **编排层例外**：scheduler / gateway / group_manager 等**编排器**对组件的直接访问是组件化架构的**合法协调机制**，不计入硬依赖。这是设计本身，不是技术债
  - **当前合规**：scheduler.go（7）+ group_manager.go（4）+ gateway/handler.go（1）共 12 处组件访问**全部位于编排层**，**无组件间硬依赖违规**。本 spec 维持此现状——不是因为妥协，是因为本就合规

- **R20（缺席降级契约）**：5 组件缺席时系统行为必须可观测、可预测，且当前实现**已经合规**（本条为文档化而非重构）：
  - `memory` 缺席 → 威胁不入记忆 → `KeyMemoryThreatValue` unset → emotion 读默认 0 → fear 不累积
  - `emotion` 缺席 → `KeyEmotionDominant/Val` unset → scheduler.buildDecisionInput.EmotionValue=0
  - `needs` 缺席 → `KeyNeedLowest/Val` unset → scheduler.calcNeedUrgency=0 → decision.NeedUrgency=0
  - `personality` 缺席 → decision.Weights 用 DefaultWeights（Threat/Needs/Emotion 均为 1）
  - `social` 缺席 → GroupManager 不可见（既非"游离"也非"幽灵"）→ shareGroupPerception/updateFollowerTargets/propagateGroupState 逐 NPC 跳过
  - 所有访问点必须走 `npc.GetComponent[T](inst, name)` 的 `(T, ok)` 返回值决策——禁止裸 nil 访问、禁止类型断言 panic

- **R21（npctest 封闭）**：`internal/runtime/npc/npctest/` 子包提供 `NewInstanceWithExtras(admin *npc.ADMINTemplate, extras map[string]json.RawMessage, ...) (*npc.Instance, error)` 允许测试绕过 R17 opt-in 直接注入 component（用于集成测试 fixture 迁移）。**封闭红线**：`internal/runtime/` 及 `internal/gateway/` 的生产代码**禁止** import `*test` 子包；此红线追加到 [docs/standards/red-lines.md](../../standards/red-lines.md) "禁止过度设计" 章节

### 文档与红线

- **R22（api-contract.md 变更协议）**：本仓 `docs/architecture/api-contract.md` **允许 v1.x 演进**（本 spec 即推 v1.1 含 opt-in 依赖矩阵），但必须遵循：① ADMIN 仓先 commit（权威源）；② 服务端仓后镜像同步（commit message 带 `契约源: <ADMIN commit hash>`）；③ **禁止**服务端仓单边修改镜像文件。违反即触发协作失序红线
- **R23**：不违反 [docs/architecture/red-lines.md](../../architecture/red-lines.md) 任一条（Phase 2 design 阶段逐条 review）
- **R24**：不违反 [docs/standards/red-lines.md](../../standards/red-lines.md) 任一条——尤其 "禁止过度设计"（死 factory 不保留 / 死配置不 seed）+ "禁止静默降级"（翻译层字段缺失/类型不匹配必须打 WARN 或 Debug 日志）

## 不做什么

- **不做 ADMIN 侧改动**：ADMIN 已冻结；任何需要 ADMIN 端配合的问题走独立 integration 流程
- **不做 BB key 运行时注册表**：该项已有独立 spec `bb-key-runtime-registry`，保持独立
- **不做 hp → max_hp 迁移**：guard_basic 孤儿字段由 ADMIN 41008 硬约束解封后一次性清除（已在 memory 记录延期）
- **不引入新 component 类型**：component registry 只做减法（删冗余）不做加法；若翻译层需要新结构，优先扁平字段 + BB
- **不做 cmd/sync 能力扩展**：端点修复完成即可（PR #20），增量同步/冲突合并等高阶功能不在本 spec
- **不改 FSM / BT 节点库**：这是数据侧的改造，不动核心引擎
- **不做性能基准**：本 spec 目标是架构收敛，不追量化指标；深度联调（含压力）归后续独立 spec
- **不做向后兼容 shim**：v2 `NPCTypeConfig` + 组件化 `TemplateConfig` 直接下线，不保留"双读"兼容层（与 path A 决策一致）
