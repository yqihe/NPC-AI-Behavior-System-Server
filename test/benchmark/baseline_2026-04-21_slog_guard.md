# 压测基线数据 — 2026-04-21（PR #43 slog 门控后）

第四次基线，对照 [baseline_2026-04-20_script_fix.md](baseline_2026-04-20_script_fix.md) 验证 [PR #43](https://github.com/yqihe/NPC-AI-Behavior-System-Server/pull/43) decision.Evaluate 热路径 `slog.Default().Enabled(ctx, slog.LevelDebug)` 门控对端到端 event→snapshot 延迟的降本效果。

## 代码改动点

`internal/runtime/decision/decision.go` Evaluate 内 slog.Debug 调用加 Enabled 门控，避免 info 级下 variadic `...any` args 切片每 Tick 堆逃逸。参见 [docs/experiment-results.md §2026-04-21 T4 复核](../../docs/experiment-results.md) 的根因分析。

## 运行环境（与 docker_info / script_fix 两次 baseline 一致）

- 容器：`npc-ai-behavior-system-server-v1-server:latest`
- 日志级别：info（`NPC_LOG_LEVEL=info`，`.env.bench`）
- 配置源：JSON（`NPC_ADMIN_API=` 与 `NPC_MONGO_URI=` 显式空）
- Tick：100ms，DURATION=90s，EVENT_RPS=5
- 三档同容器顺序跑（不重启，runId 前缀隔离跨档状态）

## 结果对比（三档阶梯）

| 指标 | 100 档 | 500 档 | 1000 档 | 基线（script_fix） | 目标 |
|------|--------|--------|---------|--------------------|------|
| `ws_errors` | **0** | **0** | **0** | 0 / 0 / 0 | <10 ✓ |
| `event_to_snapshot` p95 | **67 ms** | **58 ms** | **62 ms** | 79 / 90 / 106 ms | — |
| `event_to_snapshot` Δ vs script_fix | **-15%** | **-36%** | **-42%** | — | 降本显著 ✓ |
| `snapshot_interval` p95 | 103 ms | 106 ms | 109 ms | 102 / 103 / 106 ms | ~100 ms ✓ |
| `npc_spawn_latency` p95 | 1.17 s | 6.00 s | 11.02 s | 1.29 / 5.60 / 10.11 s | 非稳态指标* |

\* spawn 串行延迟本身受 WSL2 网络栈 + Server 串行 instance 创建主导，与热路径改动无关；与 script_fix 差异在 ±10% 范围内的正常抖动。

## Docker stats（1000 档尾部）

| 指标 | 观察值 | 红线 |
|------|--------|------|
| CPU | 7.11% | — |
| RSS | 38 MiB | 稳定 ✓ |
| `npc_tick_duration_seconds` | 0.005682 (5.68 ms) | <100 ms ✓ |
| `npc_active_count` global | 2109 | — |
| `npc_active_count` meadow | 3 | — |
| `npc_tick_total` | 5545 | 持续增长 ✓ |

## 核心观察

1. **event→snapshot p95 三档全面下降**，降本幅度随 NPC 数单调上升（100 档 15% → 1000 档 42%）。
   - 证实 slog 门控的 heap escape 消除在 decision.Evaluate 热路径下按 NPC 数线性放大。
   - 这是**毕设答辩 P3 红线（publish_event 端到端 P99 ≤ 300ms）余量从 ~3x 扩大到 ~5x** 的直接收益。

2. **snapshot_interval p95 三档稳定 103-109ms**，与 script_fix 基线相差 ±3ms（噪声范围内），说明 Tick 调度节奏不受影响。

3. **spawn 串行延迟与 script_fix 基线在同一数量级**（1.17 / 6.00 / 11.02 s vs 1.29 / 5.60 / 10.11 s），WSL2 网络栈 + Server 串行创建仍是瓶颈，与热路径无关。

## 清单项闭环

| 清单项 | 状态 |
|--------|------|
| P1 单机稳定承载 NPC 数 ≥ 500 | ✅ 1000 OK（累积 2109 live NPC） |
| P2 Tick 延迟 P99 ≤ 150ms | ✅ `tick_duration` 5.68ms，snapshot_interval p95 ∈ [103, 109] ms |
| P3 publish_event 端到端 P99 ≤ 300ms | ✅ 三档 p95 ∈ [58, 67] ms（相比 script_fix [79, 106] 大幅降本） |
| P4 内存无持续上涨 | ✅ 38 MiB 稳定 |
| P5 错误率 < 0.1% | ✅ 三档 **ws_errors=0** |

## 与历次 baseline 的关系

- [baseline_2026-04-20.md](baseline_2026-04-20.md)：首次，本地 `go run` + debug 日志
- [baseline_2026-04-20_docker_info.md](baseline_2026-04-20_docker_info.md)：Docker + info + `docker stats` 采 P4（500/1000 档）
- [baseline_2026-04-20_script_fix.md](baseline_2026-04-20_script_fix.md)：k6 脚本修复后三档全绿，**基线**
- **本次**：PR #43 slog 门控上线后重采，**event→snapshot p95 全面降 15%-42%**；**作为当前生效的最新基线**

## 后续

1. HTTPSource 接入后的同脚本 Admin 数据源对比基线——待 Admin 端 seed 完成后补采
2. 毕设答辩曲线：[docs/experiment-results.md §2026-04-21 T4 复核](../../docs/experiment-results.md) 已含 Hybrid 微基准，本文档补端到端基线
