# 文档索引

## architecture/ — 架构与约束
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [red-lines.md](architecture/red-lines.md) | 禁止事项红线（硬编码、层次破坏、弱类型、实验污染、过度设计） | 编写或审查代码时 |
| [decisions.md](architecture/decisions.md) | 7 项核心架构决策 + 配置覆盖范围 | 做设计选型时 |
| [extension-axes.md](architecture/extension-axes.md) | 三个扩展轴 + 渐进式验证路径 | 验证可扩展性时 |

## development/ — 开发规范
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [dev-rules.md](development/dev-rules.md) | 日志格式、文档同步、Git 规则、Agent 规则、经验沉淀 | 所有开发活动 |
| [go-pitfalls.md](development/go-pitfalls.md) | Go 常见陷阱（并发、数据结构、接口、错误处理、性能） | 写代码和 review 时 |

## history/ — 历史参考
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [v1-postmortem.md](history/v1-postmortem.md) | v1 尸检：核心缺陷、可复用资产、关键教训 | 复用 v1 设计或避坑时 |
