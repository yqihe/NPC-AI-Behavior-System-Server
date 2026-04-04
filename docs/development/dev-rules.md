# 项目开发规则

本项目（NPC AI 行为系统）专属规则。通用规则见 `docs/standards/dev-rules.md`。

## 日志格式

使用 `log/slog` 结构化日志：

```go
slog.Debug("fsm.transition", "npc_id", npc.ID, "from", old, "to", new)
slog.Warn("scheduler.event_type_not_found", "event_type", evt.Type)
```

格式：`组件.动作`，后跟 key-value 对。

## Docker 构建与运行

### 服务编排

通过 `docker-compose.yml` 管理，Go 服务端端口 9820。

```bash
docker compose up --build        # 启动（代码改动后必须加 --build）
docker compose up --build -d     # 后台启动
docker compose logs -f server    # 查看日志
docker compose down              # 停止
```

### Dockerfile 规范

- 多阶段构建：builder 阶段 `go build`，runtime 阶段仅含二进制
- 先复制 `go.mod`/`go.sum` 再复制源码，利用层缓存
- 环境变量通过 Compose 或 `.env` 注入（`.env` 不入库，首次开发需 `cp .env.example .env`），不硬编码

### 本地开发不受限

单元测试、`go run` 等本地操作不受 Docker 约束。Docker 解决多服务联合启动的环境一致性。

## ADMIN 联调

- 配置变更必须通过 ADMIN REST API 写入 MongoDB，不能只改 `configs/` 目录
- 服务端验证前必须先执行 `go run ./cmd/sync -api http://<ADMIN地址>` 同步配置
- 联调流程详见 `.claude/commands/integration.md`

## Agent 使用规则

- 探索代码的 Agent 只探索，不改代码
- 写代码的 Agent 只写代码，不做验证
- 验证代码的 Agent 只跑测试，不改业务代码
- 不准读其他 Agent worktree 的中间文件

## 已沉淀教训

| 来源 | 教训 | 沉淀到 |
|------|------|--------|
| experiment-layer 首版 | 确认偏误——削弱对照组、暗含实验组能力、场景无区分力 | `architecture/red-lines.md` 实验作弊 |
| experiment-layer 修正 | 改了代码未同步 spec 文档 | `standards/dev-rules.md` 文档同步 |
| gateway-layer 审查 | 路径穿越、Broadcast 死锁、nil slice JSON null | `standards/` 安全+Go 陷阱 |
| mongo-source 类型丢失 | `json.Unmarshal` 到 `any` 把 int 变 float64 | `standards/go-pitfalls.md` JSON/BSON |
| Admin 联调 | 静默降级、handler return、omitempty 零值、bson tag | `standards/` 各对应章节 |
| Admin 联调 | 只改 configs/ 不写 MongoDB，服务端 sync 拉不到 | `architecture/red-lines.md` 联调配置脱节 |
| 代码审计 | 测试硬编码计数、忽略 Unmarshal error、空断言、死代码 | `standards/red-lines.md` 测试质量 |
| 代码审计 | BT/事件类型/感知模式未找到时 silent return | `standards/red-lines.md` 静默降级 |
