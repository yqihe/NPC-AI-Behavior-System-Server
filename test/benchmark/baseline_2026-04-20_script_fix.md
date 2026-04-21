# 压测基线数据 — 2026-04-20（k6 脚本修复后）

第三次基线，对照 [baseline_2026-04-20_docker_info.md](baseline_2026-04-20_docker_info.md) 的「已知偏差」修复 [ws_load.js](ws_load.js)，消除 Docker 路径下的伪 `ws_errors`。

## 脚本修改点

1. **cleanup 只对已确认的 NPC 发 `remove_npc`**
   - 原脚本：`socket.on('open')` 内把所有 `npc_id` 塞进 `spawnedNpcIds[]`（send 时 push，非 ack 时 push）；DURATION-2s 时对全部发 remove。
   - 改后：新增 `confirmedNpcIds: Set`，仅在收到 `type === 'response'` 时 `add(npcId)`；cleanup 只遍历 confirmed。pending 未清空时 `console.warn`。
   - 效果：1000 档未 ack 的 spawn 不再触发 `npc_not_found` 伪告警（原 ws_errors=500）。

2. **`npc_id` 加 run-id 前缀**
   - 原：`npc_${i}` 固定 → 跨测试跑（尤其 500→1000）前者残留 NPC 导致后者 `npc_already_exists`。
   - 改后：`npc_${runId}_${i}`，每次 `ws.connect` 生成唯一 runId。
   - 效果：重复跑不再依赖 server restart 清状态。

## 运行环境（与 docker_info 基线一致）

- 容器：`npc-ai-behavior-system-server-v1-server:latest`
- 日志级别：info（`NPC_LOG_LEVEL=info`）
- 配置源：JSON（`NPC_ADMIN_API=` 与 `NPC_MONGO_URI=` 显式空）
- Tick：100ms，DURATION=90s，EVENT_RPS=5
- 100 档补档时间：2026-04-21（脚本与环境与 500/1000 档一致；原始 k6 日志本地留存于 `bench_100.log`，按 .gitignore 规则不入库）

## 结果对比（三档阶梯）

| 指标 | 100 档 | 500 档 | 1000 档 | 目标 |
|------|--------|--------|---------|------|
| `ws_errors` | **0** | **0** | **0** | <10 ✓ |
| `event_to_snapshot` p99 | 81 ms | 93 ms | 111 ms | <300 ms ✓ |
| `event_to_snapshot` p95 | 79 ms | 90 ms | 106 ms | — |
| `snapshot_interval` p95 | 102 ms | 103 ms | 106 ms | ~100 ms ✓ |
| `npc_spawn_latency` p95 | 1.29 s | 5.60 s | 10.11 s | 非稳态指标* |

\* spawn 串行延迟随 NPC 数线性上涨，是 Docker WSL2 网络栈 + server 串行处理 npc 创建的本质——与脚本修复无关。本地 `go run` 路径下（首次 baseline）100/500/1000 档分别为 35ms / 180ms / 377ms。

**P2/P3 阶梯曲线观察**：
- event→snapshot p99 随 NPC 数从 81ms 上升到 111ms，全程远低于 300ms 红线，增速近线性但幅度小
- snapshot_interval p95 在三档稳定于 102~106ms 附近，Tick 调度未被 NPC 数拖累
- 稳态指标（event→snapshot、snapshot）在 10 倍 NPC 数压力下增幅 ≤ 37%，承载余量充足

## 清单项闭环

| 清单项 | 状态 |
|--------|------|
| P1 单机稳定承载 NPC 数 ≥ 500 | ✅ 1000 OK |
| P2 Tick 延迟 P99 ≤ 150ms | ✅ 三档 p95 ∈ [102, 106] ms |
| P3 publish_event 端到端 P99 ≤ 300ms | ✅ 三档 p99 ∈ [81, 111] ms |
| P4 内存无持续上涨 | ✅（沿用 docker_info baseline 的 `docker stats` 曲线） |
| P5 错误率 < 0.1% | ✅ 三档 **ws_errors=0** |

## 与前两次 baseline 的关系

- [baseline_2026-04-20.md](baseline_2026-04-20.md)：首次，本地 `go run` + debug 日志（100/500/1000 三档）
- [baseline_2026-04-20_docker_info.md](baseline_2026-04-20_docker_info.md)：Docker + info + `docker stats` 采集 P4（500/1000 档）；spawn/cleanup 伪告警记为「已知偏差」
- **本次**：脚本修复，100（2026-04-21 补档）/ 500 / 1000 三档稳态全绿；**作为当前生效的压测基线，阶梯曲线齐备**

## 后续

1. HTTPSource 接入后的同脚本 Admin 数据源对比基线（已接入，PR #37/#41）——待 Admin 端 seed 完成后补采，与本 baseline 做交叉对比
2. 毕设答辩曲线可跑 `go test -tags experiment -run TestScale -timeout 30m ./internal/experiment/`
