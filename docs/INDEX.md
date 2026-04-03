# 文档索引

## architecture/ — 架构与约束
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [red-lines.md](architecture/red-lines.md) | 禁止事项红线（硬编码、层次破坏、弱类型、实验污染、过度设计、容器化） | 编写或审查代码时 |
| [decisions.md](architecture/decisions.md) | 7 项核心架构决策 + 配置覆盖范围 | 做设计选型时 |
| [extension-axes.md](architecture/extension-axes.md) | 三个扩展轴 + 渐进式验证路径 | 验证可扩展性时 |

## development/ — 开发规范
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [dev-rules.md](development/dev-rules.md) | 日志格式、文档同步、Git 规则、Agent 规则、Docker 构建与运行、经验沉淀 | 所有开发活动 |
| [go-pitfalls.md](development/go-pitfalls.md) | Go 常见陷阱（并发、数据结构、接口、错误处理、性能） | 写代码和 review 时 |
| [deployment.md](deployment.md) | 环境规范（dev/prod 区分）、部署流程、环境变量、文件规范 | 部署和环境切换时 |
| [protocol.md](protocol.md) | WebSocket 协议规范（消息类型、字段、交互流程） | 客户端对接时 |

## specs/ — 功能层 Spec（需求 → 设计 → 任务）
| 目录 | 状态 | 内容概括 |
|------|------|----------|
| [specs/core-engine/](specs/core-engine/) | 已完成 | Blackboard、FSM、BT、Rule 四大引擎 |
| [specs/runtime-layer/](specs/runtime-layer/) | 已完成 | 事件总线、感知过滤、决策中心、NPC 调度 |
| [specs/experiment-layer/](specs/experiment-layer/) | 已完成 | 5 模式对照实验框架 + 定性/定量数据采集（含 [data-collection-guide.md](specs/experiment-layer/data-collection-guide.md)） |
| [specs/gateway-layer/](specs/gateway-layer/) | 已完成 | WebSocket 连接、消息路由、状态广播 |
| [specs/extension-validation/](specs/extension-validation/) | 已完成 | 扩展轴验证（fire 事件 + police NPC，零代码改动） |
| [specs/mongo-source/](specs/mongo-source/) | 已完成 | MongoDB 配置源 + dev/prod 环境切换 |

## history/ — 历史参考
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [v1-postmortem.md](history/v1-postmortem.md) | v1 尸检：核心缺陷、可复用资产、关键教训 | 复用 v1 设计或避坑时 |
