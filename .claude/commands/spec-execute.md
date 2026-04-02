# /spec-execute — 执行任务

执行 spec 中的一个任务，写代码。

## Usage
```
/spec-execute <task-id> <feature-name>
```

---

## 执行流程

1. 加载 `docs/specs/<feature-name>/` 下的 requirements.md、design.md、tasks.md
2. 定位目标任务，确认未完成
3. 读取任务涉及的所有文件（先读再改，不准盲改）
4. 执行实现
5. 检查文档是否需要同步更新（参考 `docs/development/dev-rules.md` 文档同步章节）
6. 在 tasks.md 中将任务标记为 `[x]`
7. 停下，输出完成摘要，建议跑 `/verify <feature-name>`

---

## 禁止

- **一次只做一个任务**，做完停下等审批，不准自动进入下一个
- **不准加没要求的功能**——任务说实现 A，就只实现 A
- **不准过度封装**——不准为一个调用点创建接口/抽象层
- **不准瞎重构**——不准顺手改不相关的代码，哪怕它"看起来可以更好"
- **不准盲改**——动任何文件前必须先读它，理解上下文
- **不准自己判定测试通过**——自己写的代码自己不当裁判，交给 `/verify`
- **不准假装测试通过**——不准在完成摘要里写"测试应该没问题"

## Agent 使用

- 可以开多个 Agent 并行提高效率
- Agent 必须专职：探索代码的不改代码，写代码的不做验证
- 不准读其他 Agent worktree 的中间文件，等返回结果
- 不准给 Agent 设不同模型

## 写代码时必须检查

参考 `docs/development/go-pitfalls.md`，重点关注：

- **并发安全**：是否有共享状态？多 goroutine 读写同一个 map/slice/struct？加锁了吗？
- **资源泄漏**：goroutine 有退出路径吗？打开的连接/文件有 defer Close 吗？
- **nil 安全**：map 初始化了吗？指针解引用前检查 nil 了吗？
- **接口契约**：类型断言用两值形式了吗？error 有处理吗？
- **边界条件**：空 slice、零值 struct、空字符串 key 会怎样？

## DEBUG 日志

统一格式（参考 `docs/development/dev-rules.md`）：
```go
log.Debug("组件.动作", "key1", val1, "key2", val2)
```

**判断标准**：这行代码出 bug 时，有这条日志能帮助排查吗？能就加。

重点加日志的位置：
- FSM 状态转换
- BT 节点执行结果
- 事件发布和分发
- Blackboard 写入
- 配置加载

## 完成摘要模板

```
## Task T[N] 完成

**实现内容**：[一句话]
**改动文件**：[文件列表]
**满足需求**：[R1, R3, ...]
**新增测试**：[有/无，覆盖什么]
**文档同步**：[更新了哪些文档 / 无需更新]
**Go 陷阱检查**：[检查了哪些项，有无发现]

→ 建议跑 `/verify <feature-name>` 验证
```

## 经验沉淀

执行过程中踩到的 Go 坑追加到 `docs/development/go-pitfalls.md`。发现的新禁令追加到 `docs/architecture/red-lines.md`。
