# 文档索引

## standards/ — 通用标准（可跨项目复用）
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [red-lines.md](standards/red-lines.md) | 通用禁止红线（安全、静默降级、过度设计、测试质量） | 编写或审查任何代码时 |
| [dev-rules.md](standards/dev-rules.md) | 通用开发规则（文档同步、Git 规范、日志、经验沉淀） | 所有开发活动 |
| [go-pitfalls.md](standards/go-pitfalls.md) | Go 语言陷阱与禁令（并发、数据结构、JSON/BSON、测试） | 写 Go 代码和 review 时 |
| [acceptance-checklist.md](standards/acceptance-checklist.md) | 服务端达标验收清单（正确性/可观测性/性能/健壮性/配置/毕设专属） | 阶段性交付或答辩前 |

## architecture/ — 项目架构与约束
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [overview.md](architecture/overview.md) | 架构总览：五层分层、各层组件职责、数据流 | 首次了解项目时 |
| [red-lines.md](architecture/red-lines.md) | 项目架构红线（硬编码、层次、弱类型、实验、联调） | 编写或审查项目代码时 |
| [decisions.md](architecture/decisions.md) | 7 项核心架构决策 + 配置覆盖范围 | 做设计选型时 |
| [extension-axes.md](architecture/extension-axes.md) | 三个扩展轴 + 渐进式验证路径 | 验证可扩展性时 |

## development/ — 项目开发规范
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [dev-rules.md](development/dev-rules.md) | 项目专属规则（日志格式、Docker、联调 sync、Agent） | 项目日常开发 |
| [deployment.md](deployment.md) | 环境规范（dev/prod 区分）、部署流程、环境变量 | 部署和环境切换时 |
| [protocol.md](protocol.md) | WebSocket 协议规范（消息类型、字段、交互流程） | 客户端对接时 |

## specs/ — 功能层 Spec（需求 → 设计 → 任务）
| 目录 | 状态 | 内容概括 |
|------|------|----------|
| [specs/core-engine/](specs/core-engine/) | 已完成 | Blackboard、FSM、BT、Rule 四大引擎 |
| [specs/runtime-layer/](specs/runtime-layer/) | 已完成 | 事件总线、感知过滤、决策中心、NPC 调度 |
| [specs/experiment-layer/](specs/experiment-layer/) | 已完成 | 5 模式对照实验框架 + 定性/定量数据采集 |
| [specs/gateway-layer/](specs/gateway-layer/) | 已完成 | WebSocket 连接、消息路由、状态广播 |
| [specs/extension-validation/](specs/extension-validation/) | 已完成 | 扩展轴验证（fire 事件 + police NPC） |
| [specs/mongo-source/](specs/mongo-source/) | 已完成 | MongoDB 配置源 + dev/prod 环境切换 |

## 项目规划
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [roadmap.md](roadmap.md) | 未来规划：短期、中期、长期、明确排除项 | 规划下一步工作时 |

## history/ — 历史参考
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [v1-postmortem.md](history/v1-postmortem.md) | v1 尸检：核心缺陷、可复用资产、关键教训 | 复用 v1 设计或避坑时 |
