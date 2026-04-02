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
- 数据库：MongoDB（配置存储+生产环境）+ Redis（热状态缓存）
- 通信协议：WebSocket，JSON 序列化
- 配置格式：开发阶段 JSON 文件（git 可追踪），生产环境 MongoDB（支持热更新），Loader 层抽象数据源
- 测试框架：Go 标准 testing + e2e 测试（走 WS 协议，模拟无头客户端）
- e2e 测试走 WebSocket 协议与服务端交互，和 Unity 客户端走同一入口，未来 Unity 接入后可直接替换

## 开发指令

```bash
# 运行服务端
go run cmd/server/main.go

# 运行全部测试
go test ./...

# 运行 e2e 测试
go test ./test/e2e/... -v

# 仅编译核心（不含实验框架）
go build ./cmd/server/

# 编译含实验框架
go build -tags experiment ./cmd/server/
```

## 架构和约束

### 目录结构

```
cmd/server/              # 程序入口
internal/
  config/                # 配置加载（JSON/MongoDB 双数据源抽象）
  core/                  # 纯引擎，无业务逻辑
    blackboard/          #   强类型 Blackboard + keys.go
    fsm/                 #   FSM 引擎（配置驱动）
    bt/                  #   BT 引擎 + 节点库
    rule/                #   条件规则匹配器
  runtime/               # 运行时业务组装
    npc/                 #   NPC 实例管理、注册表、Tick 调度
    event/               #   事件总线（TTL 衰减模型）
    decision/            #   决策中心（威胁评估+优先级仲裁）
    perception/          #   感知过滤（配置驱动）
    world/               #   世界状态、空间索引、AOI
  gateway/               # WebSocket 连接、消息路由、状态广播
  admin/                 # 管理接口
  experiment/            # 对照实验（build tag 隔离）
pkg/protocol/            # WS 消息协议（客户端可引用）
configs/                 # JSON 配置文件
test/e2e/                # e2e 测试（WS 协议无头客户端）
```

### 命名约定

- 文件名：`snake_case.go`
- 包名：小写单词，不用下划线
- 接口：动词或形容词（`Tickable`、`Perceiver`），不加 `I` 前缀
- 配置文件：`snake_case.json`
- Blackboard Key 常量：`Key` 前缀 + PascalCase（`KeyThreatLevel`）

### 代码风格

- 类型安全严格模式，Blackboard 禁止裸 `map[string]any`，必须通过泛型 `BBKey[T]` 访问
- 禁止 switch-case 做 NPC 类型分发，使用注册表/工厂模式
- core/ 包禁止 import runtime/ 或 gateway/，依赖方向单向向下

## 环境配置

### 开发环境
- **数据库**：MongoDB localhost:27017
- **缓存**：Redis localhost:6379
- **配置源**：`configs/` 目录 JSON 文件
- **日志级别**：DEBUG

### 生产环境
- **数据库**：MongoDB（配置连接串）
- **缓存**：Redis（配置连接串）
- **监控**：
- **日志级别**：INFO

## Git 工作流

- **主分支**：`main`（只接受 PR）
- **开发分支**：`develop`（日常开发）
- **功能分支**：`feature/task-id-description`
- **修复分支**：`hotfix/critical-bug-description`
- 自动化测试必须通过，代码覆盖率不低于 85%，新功能必须包含测试

## 详细文档

详见 `docs/INDEX.md`，按需查阅。
