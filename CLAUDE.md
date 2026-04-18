## CLAUDE.md

本文件为 Claude Code 提供项目上下文和开发指导。

## 项目概述

**项目名称**：基于状态机和行为树的游戏 AI 角色系统（v2）
**当前版本**：v2.0.0-dev
**项目状态**：开发中
**项目性质**：毕业设计

**毕设核心创新点**：**FSM + BT + 智能决策中心 三位一体架构**——FSM 管宏观状态转换，BT 管状态内行为编排，决策中心做威胁评估与优先级仲裁。三者缺一不可，这是整个系统的学术价值所在。需要对照实验数据（Hybrid vs PureFSM vs PureBT）证明该架构的优越性。

**业务目标**：为在线网游构建可扩展的 NPC AI 行为系统，支持大量 NPC 在开放世界中自主行动和交互
**核心价值**：**"加新 NPC 类型或新事件源 = 加配置 + 加测试，不改核心代码"**

## 技术栈

**客户端**：Unity C#（WebSocket 通信 + GM 面板 + AutoTestRunner）

**服务端**（Golang）：
- 通信协议：WebSocket，JSON 序列化
- 日志：Go 标准库 `log/slog`，结构化输出到 stdout
- 容器化：Docker Compose
- 配置格式：统一通过 ADMIN HTTP API 拉取（`NPC_ADMIN_API`），启动时全量加载到内存
- 测试框架：Go 标准 testing + e2e 测试（走 WS 协议，模拟无头客户端）

## 开发指令

```bash
# Docker Compose 启动全部服务（代码改动后必须加 --build）
docker compose up --build

# 后台启动
docker compose up --build -d

# 查看服务日志
docker compose logs -f server

# 停止全部服务
docker compose down

# 本地运行全部测试
go test ./...

# 本地运行 e2e 测试
go test ./test/e2e/... -v

# 本地编译（不含实验框架）
go build ./cmd/server/

# 本地编译（含实验框架）
go build -tags experiment ./cmd/server/

```

## 目录结构

```
cmd/
  server/                    # 程序入口（main.go）
internal/
  config/                    # 配置加载（ADMIN HTTP API → 内存）
  core/                      # 纯引擎，无业务逻辑
    blackboard/              #   强类型 Blackboard + keys.go
    fsm/                     #   FSM 引擎（配置驱动）
    bt/                      #   BT 引擎 + 节点库
    rule/                    #   条件规则匹配器
  runtime/                   # 运行时业务组装
    npc/                     #   NPC 实例管理、注册表、Tick 调度
    event/                   #   事件总线（TTL 衰减模型）
    decision/                #   决策中心（威胁评估+优先级仲裁）
    perception/              #   感知过滤（配置驱动）
  gateway/                   # WebSocket 连接、消息路由、状态广播
  experiment/                # 对照实验（build tag 隔离）
pkg/protocol/                # WS 消息协议（客户端可引用）
configs/                     # 服务端启动配置（server.json）
testdata/configs/            # 测试用配置夹具
test/e2e/                    # e2e 测试（WS 协议无头客户端）
docs/
  standards/                 # 通用标准（可跨项目复用）
  architecture/              # 项目架构文档
  development/               # 项目开发规范
```

## 命名约定

- 文件名：`snake_case.go`
- 包名：小写单词，不用下划线
- 接口：动词或形容词（`Tickable`、`Perceiver`），不加 `I` 前缀
- 配置文件：`snake_case.json`
- Blackboard Key 常量：`Key` 前缀 + PascalCase（`KeyThreatLevel`）

## 架构约束

- 类型安全严格模式，Blackboard 禁止裸 `map[string]any`，必须通过泛型 `BBKey[T]` 访问
- 禁止 switch-case 做 NPC 类型分发，使用注册表/工厂模式
- core/ 包禁止 import runtime/ 或 gateway/，依赖方向单向向下
- 配置源唯一来源：`NPC_ADMIN_API`（必填，启动时全量拉取）

## Git 工作流

- **主分支**：`main`（受保护，只接受 PR，禁止 force push）
- **功能分支**：`feature/task-id-description`
- **修复分支**：`hotfix/critical-bug-description`
- **合并策略**：仅 Squash Merge（PR 合并后 main 上保持单条干净提交）
- **分支清理**：PR 合并后远端分支自动删除
- 自动化测试必须通过，新功能必须包含测试
- **环境配置**：`.env` 不入库，首次开发需 `cp .env.example .env`

## 详细文档

详见 `docs/INDEX.md`，按需查阅。
