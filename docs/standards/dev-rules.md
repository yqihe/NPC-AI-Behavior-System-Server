# 通用开发规则

适用于所有项目。项目专属规则见 `docs/development/dev-rules.md`。

## 文档同步

**代码改动和文档更新必须在同一步骤完成。**

改代码时检查：
- spec 文档（requirements/design/tasks）是否需要同步
- 架构文档（red-lines、decisions）是否需要补充
- 索引文档（INDEX.md）是否需要更新

不受影响则不动。

## Git 规范

- commit message 格式：`类型(范围): 描述`
  - 类型：`feat` / `fix` / `test` / `refactor` / `docs` / `chore`
  - 范围：模块路径，如 `core/fsm`、`runtime/decision`
- 每个需求创建 feature 分支，按 task 逐个 commit
- 全部 task 完成且验证通过后合并

## 代码提交纪律

- **每次 commit 后必须考虑是否需要 push 到远端**。本地 commit 不等于提交——只有 push 后代码才进入协作流程
- **阶段性工作完成后必须 push**：一轮代码修改+测试通过后，立即 push，不要囤积本地 commit
- **push 后考虑是否需要合并**：feature 分支工作完成后，创建 PR 合并到主分支

## 经验沉淀

开发中发现的新规则、新坑、新禁令，按类型添加到对应文档：

| 发现类型 | 添加到 |
|----------|--------|
| 所有项目通用禁令 | `docs/standards/red-lines.md` |
| 所有项目通用规则 | `docs/standards/dev-rules.md` |
| Go 语言陷阱 | `docs/standards/go-pitfalls.md` |
| 项目架构禁令 | `docs/architecture/red-lines.md` |
| 项目开发规则 | `docs/development/dev-rules.md` |

## 结构化日志

统一使用结构化日志，格式因语言而异：

```
组件.动作  key1=val1  key2=val2
```

**何时加日志**：如果这行代码出 bug 时，没有日志会导致排查困难，就加。不确定就加。

**何时不加**：纯计算、getter/setter、循环体内每次迭代（会刷屏）。
