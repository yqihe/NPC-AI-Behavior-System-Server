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

每次代码改动完成后，检查以下文档是否受影响：

- `CLAUDE.md` — 目录结构、技术栈、开发指令是否变化
- `docs/INDEX.md` — 是否有新文档需要加入索引
- `docs/architecture/red-lines.md` — 是否发现新的禁令需要补充
- `docs/architecture/decisions.md` — 是否产生了新的架构决策
- `docs/development/go-pitfalls.md` — 是否踩到新的 Go 坑需要记录
- 当前 spec 的 `requirements.md` / `design.md` / `tasks.md` — 执行中发现的偏差是否需要更新

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
