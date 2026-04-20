# 验收清单

本项目通用的"服务端是否达标"检验清单。裁剪自业界生产就绪标准（Mercari PRR、Susan Fowler 8 维度），结合本项目毕设性质保留关键项、精简合规/SLA 类条款。

**用法**：每次阶段性交付或答辩前，对照本清单逐项打钩。不通过项需明确标注原因与后续计划。

---

## 一、正确性（Correctness）

| # | 项目 | 判定 | 工具/依据 |
|---|------|------|----------|
| C1 | 核心引擎单测覆盖率 ≥ 70% | `go test -cover ./internal/core/...` | `-coverprofile` |
| C2 | e2e 覆盖所有已注册 NPC 类型的完整状态转换 | 通读 [test/e2e/gateway_test.go](../../test/e2e/gateway_test.go) | Go testing |
| C3 | `go vet ./...` 与 `staticcheck ./...` 零告警 | CI 输出 | Tier A |
| C4 | 所有 handler 错误码覆盖 [docs/protocol.md](../protocol.md) 列出的场景 | 逐项人工核对 | — |

## 二、可观测性（Observability）

| # | 项目 | 判定 |
|---|------|------|
| O1 | 所有关键事件走 `log/slog` 结构化输出，格式 `组件.动作` + kv | `development/dev-rules.md` |
| O2 | Tick 延迟、NPC 数、事件总线积压等关键指标可采集（至少日志中可检索） | 压测日志 |
| O3 | Panic 被顶层 recover 兜底并落日志，不终止整个进程 | 单元测试 [safego_test.go](../../internal/runtime/safego_test.go) + [panic_recover_test.go](../../internal/gateway/panic_recover_test.go)；热路径（scheduler.tick / hub.run / conn.read_pump / conn.write_pump / broadcast.loop）均 `defer Recover` |
| O4 | 配置源切换（JSON/Mongo/Admin）有 INFO 级日志标注实际生效源 | 启动日志 |

## 三、性能与压测（Performance）

压测脚本位于 [test/benchmark/](../../test/benchmark/)，以 k6 + WebSocket 为基准。已落盘基线：[本地+debug](../../test/benchmark/baseline_2026-04-20.md)、[Docker+info+内存](../../test/benchmark/baseline_2026-04-20_docker_info.md)。

| # | 项目 | 目标值（毕设基线） | 采集方式 |
|---|------|------------------|---------|
| P1 | 单机稳定承载 NPC 数 | ≥ 500 | k6 spawn |
| P2 | Tick 延迟 P99 | ≤ 150ms（Tick=100ms 下） | 服务端日志 |
| P3 | publish_event → 相关 NPC 状态变更 的端到端时延 P99 | ≤ 300ms | k6 自定义指标 |
| P4 | 压测期间内存无持续上涨（无泄漏） | 观察 RSS | `docker stats` / pprof |
| P5 | 压测期间错误率 | < 0.1% | k6 checks |

> **毕设基线**仅为及格线。答辩材料应给出"N=100/500/1000 三档阶梯数据 + 对比曲线"。

## 四、健壮性（Robustness）

| # | 项目 | 判定 |
|---|------|------|
| R1 | 无效/残缺 JSON 消息返回 `invalid_json` / `invalid_data`，连接不崩 | e2e |
| R2 | 未知 `type` 返回 `unknown_message_type`，不关闭连接 | e2e |
| R3 | 配置字段缺失或引用错误**启动失败**（不延迟到运行时），见红线 | 人工造错 |
| R4 | 客户端异常断连不影响其他客户端的广播 | 多连接 e2e |
| R5 | `SIGTERM` 后优雅关闭：停止接收新连接、排空 in-flight HTTP、等待后台 goroutine 退出 | 单元测试 [shutdown_test.go](../../internal/gateway/shutdown_test.go)；`main.go` 监听 SIGINT+SIGTERM，`httpSrv.Shutdown(ctx, 5s)` + `WaitGroup` 等 scheduler/hub/broadcastLoop 排空。**注**：MongoDB 由 `NewMongoSource` 加载完即 `Disconnect`，runtime 期间无连接可关 |

## 五、配置与数据源（Configuration）

| # | 项目 | 判定 |
|---|------|------|
| CF1 | 配置源优先级 `NPC_ADMIN_API` > `NPC_MONGO_URI` > JSON 与 `CLAUDE.md` 一致 | `internal/config/` |
| CF2 | `cmd/sync` 可从 ADMIN 同步配置到本地 `configs/` 并通过 e2e | 人工跑一次 |
| CF3 | 引用关系（FSM→BT 名、事件→感知等）加载期校验，不通过启动失败 | 启动日志 |

## 六、毕设专属（Thesis Requirements）

本项目毕设创新点为 **FSM + BT + 决策中心 三位一体架构**，以下项目**不可裁剪**——这是学术价值所在。

| # | 项目 | 判定 |
|---|------|------|
| T1 | `go build -tags experiment ./cmd/server/` 成功 | 编译 |
| T2 | 5 种对照模式（Hybrid / PureFSM / PureBT / NoDecision / NoThreat）可切换运行 | [experiment/](../../internal/experiment/) |
| T3 | 对照实验产出定量数据（决策次数、状态变更频次、威胁响应延迟），CSV 或 JSON 落盘 | [experiment-results.md](../experiment-results.md) |
| T4 | 同一压测场景下 Hybrid 优于其他模式的数据可复现 | 两次独立运行结果一致性误差 < 10% |
| T5 | 答辩材料中包含至少一张 Hybrid vs 单一架构的对比曲线 | 图表 |

---

## 附：本清单未覆盖的项（毕设无需但生产需要）

- 多副本部署 + 负载均衡
- 灰度发布 / 蓝绿切换 / 回滚脚本
- 告警系统（PagerDuty / 钉钉机器人等）
- 数据库备份与灾难恢复演练
- 合规与审计（GDPR / 等保）
- SLI/SLO/SLA 正式定义与看板

若后续要投入生产，补充上述项并参照 [Mercari production-readiness-checklist](https://github.com/mercari/production-readiness-checklist) 逐条评审。
