# 通用开发规则

所有 Skill 执行过程中必须遵守的规则。

## DEBUG 日志格式

统一使用结构化日志，格式：

```go
log.Debug("组件.动作", "key1", val1, "key2", val2)
```

示例：
```go
log.Debug("fsm.transition", "npc_id", npc.ID, "from", old, "to", new)
log.Debug("decision.dispatch", "event_type", e.Type, "npc_count", len(targets))
log.Debug("bt.tick", "npc_id", npc.ID, "node", node.Name(), "result", result)
```

**何时加日志**：如果这行代码出 bug 时，没有日志会导致排查困难，就加。不确定就加。

**何时不加**：纯计算、getter/setter、循环体内每次迭代（会刷屏）。

## 文档同步

**强制规则：代码改动和文档更新必须在同一步骤完成，不允许"先改代码回头再补文档"。**

来源：experiment-layer 修正了 PureFSM/PureBT 代码和新增了距离场景，但未同步 spec 文档，直到用户指出。根因是把文档同步当成独立的事后步骤而非代码改动的一部分。

### 改代码时必须同步的文档

- 当前 spec 的 `requirements.md` / `design.md` / `tasks.md` — **实现偏离了 spec 设计时，必须在同一次改动中更新 spec。spec 描述的必须是代码的真实状态，不是历史计划**

### 改完代码后检查的文档

- `CLAUDE.md` — 目录结构、技术栈、开发指令是否变化
- `docs/INDEX.md` — 是否有新文档需要加入索引
- `docs/architecture/red-lines.md` — 是否发现新的禁令需要补充
- `docs/architecture/decisions.md` — 是否产生了新的架构决策
- `docs/development/go-pitfalls.md` — 是否踩到新的 Go 坑需要记录

受影响则更新，不受影响则不动。

## Git 规则

- 每个需求（spec）创建一个 feature 分支：`feature/<spec-name>`
- 分支内按 task 逐个 commit
- commit message 格式：`类型(范围): 描述`
  - 类型：`feat` / `fix` / `test` / `refactor` / `docs` / `chore`
  - 范围：模块路径，如 `core/fsm`、`runtime/decision`
  - 示例：`feat(core/fsm): 配置驱动状态转换`
- 全部 task 完成且验证通过后，合并到 develop

## Agent 使用规则

- 可以开多个 Agent 提高效率，但必须专职分工
- 探索代码的 Agent 只探索，不改代码
- 写代码的 Agent 只写代码，不做验证
- 验证代码的 Agent 只跑测试，不改业务代码
- 不准读其他 Agent worktree 的中间文件，等 Agent 返回结果
- 不准给 Agent 设不同模型

## 经验沉淀

在开发过程中发现的新规则、新坑、新禁令，按类型添加到对应文档：

| 发现类型 | 添加到 |
|----------|--------|
| Go 语言层面的坑 | `docs/development/go-pitfalls.md` |
| 架构层面的禁令 | `docs/architecture/red-lines.md` |
| 新的架构决策 | `docs/architecture/decisions.md` |
| Skill 流程缺陷 | 对应的 Skill 文件 |
| 项目特有的约定 | `CLAUDE.md` 或 `docs/development/dev-rules.md` |

## 已沉淀教训

| 来源 | 教训 | 沉淀到 |
|------|------|--------|
| experiment-layer 首版 | 确认偏误——带着"证明 Hybrid 最好"的目标设计实验，导致：对照组被削弱（PureFSM 取第一个事件）、对照组暗含实验组能力（PureBT Go 代码内联仲裁）、场景无区分力（三者都 100%）、数据未攻击就采信 | `red-lines.md` → 禁止实验作弊 |
| experiment-layer 修正 | 修正了代码（PureFSM 排序、新增距离场景）但未同步 spec 文档（requirements/design/tasks），直到用户指出。根因：把文档同步当成独立的事后步骤 | `dev-rules.md` → 文档同步强制规则 |
| experiment-layer 立论 | 把"决策中心有价值"当创新点设计实验，但决策中心是工业标配。真正创新是三层协作的架构模式。实验数据中 FSM+DC ≈ Hybrid、BT+DC ≈ Hybrid，说明实验完全没有证明 BT 和 FSM 各自的不可替代性 | `red-lines.md` → 禁止实验作弊（立论部分） |
| experiment-layer 规模 | 只用 5 状态 3 事件的玩具规模测试，无法体现纯 FSM 状态爆炸和纯 BT 树膨胀的痛点。架构优势在规模增长后才显现，必须测试不同规模下的交叉点 | `red-lines.md` → 禁止只用玩具规模验证架构 |
| experiment-layer 指标 | 响应延迟(Tick 数)所有模式都是 0.0，说明指标设计有问题——同一 Tick 内完成全部处理，Tick 数无区分力。应改为墙钟时间(ns)或重新设计量化方式 | `red-lines.md` → 禁止接受全零指标数据 |
