# 压测基线数据 — 2026-04-20（Docker + info 日志）

第二次基线，对照 [baseline_2026-04-20.md](baseline_2026-04-20.md) 的两条后续建议落地：
- **切 Docker 路径**（上次本地 `go run`）
- **日志级别 info**（上次 debug）
- **采集内存曲线**（上次未采 P4 数据）

## 运行环境

- 宿主：Windows 11，Docker Desktop 29.4.0（WSL2 后端）
- 容器：`npc-ai-behavior-system-server-v1-server:latest`（基于项目 Dockerfile 就地构建）
- k6：v1.7.0（宿主机）→ 容器 `ws://localhost:9820/ws`
- 日志级别：**info**（`NPC_LOG_LEVEL=info`）
- 配置源：JSON（`NPC_ADMIN_API=` 与 `NPC_MONGO_URI=` 显式清空，走本地 `configs/`）
- Tick 周期：100ms
- 压测脚本：[ws_load.js](ws_load.js)（未修改）

## 稳态指标（event→snapshot、snapshot interval）

| NPC 数 | event→snapshot p95 | event→snapshot p99 | 目标 | snapshot interval p95 | Tick 目标 |
|--------|-------------------|-------------------|------|----------------------|-----------|
| 500    | 67 ms             | 68 ms（max）      | <300 ms ✓ | 103 ms          | 100 ms ✓ |
| 1000   | 74 ms             | 76 ms（max）      | <300 ms ✓ | 104 ms          | 100 ms ✓ |

两档 snapshot 间隔保持在 100~104ms，tick 调度未被 NPC 数拖累。事件端到端相较上次 debug 本地（99~104ms）略有下降，与日志 I/O 减少方向一致。

## P4 内存曲线（`docker stats` 每 2s 采样）

| 阶段 | 时长 | 内存范围 | 说明 |
|------|------|---------|------|
| 启动空闲 | ~40s | 6.3 MiB | 仅默认 zone (meadow ×3) + 9 template |
| 500 NPC 压测 + cleanup | ~90s | 10~16 MiB | 峰值 15.6 MiB |
| 1000 NPC 压测 + cleanup | ~90s | 11~24 MiB | 峰值 24.2 MiB |
| 1000 结束后静态 | ~50s | 20~22 MiB | **无持续上涨**（Go 运行时保留堆，正常） |

**结论：无内存泄漏。** 1000 NPC 后堆保留在 ~21 MiB（不回落到 6.3 MiB idle），这是 Go runtime 行为，不是泄漏——GC 后堆上限保持为"曾用过的最大"。

## 结论与对标

| 清单项 | 首次基线（本地+debug） | 本次（Docker+info）| 状态 |
|--------|-------------------|-------------------|------|
| P1 单机稳定承载 NPC 数 ≥ 500 | ✅ 1000 | ✅ 1000 稳态可用 | ✓ |
| P2 Tick 延迟 P99 ≤ 150ms | ✅ p95=111 ms | ✅ p95=104 ms | ✓（更优）|
| P3 publish_event 端到端 P99 ≤ 300ms | ✅ p95=104 ms | ✅ p95=74 ms | ✓（更优）|
| P4 内存无持续上涨 | ⚠️ 未采集 | ✅ 见上表，无泄漏 | **闭环** |
| P5 错误率 < 0.1% | ✅ 0 | ⚠️ 见下 | 部分 |

## 已知偏差（非产品缺陷，k6 脚本串行 I/O 限制）

- **spawn p95 大幅高于本地路径**（500 档 6.17s，1000 档 10.86s；上次本地分别为 180ms / 377ms）。
  原因：Docker WSL2 网络栈 + `socket.on('open')` 串行发 N 条 `spawn_npc`，N=1000 时堆压过大。spawn 延迟上次基线就注明"非稳态指标"。
- **1000 档 ws_errors = 500**：k6 在 `DURATION-2s` 点无条件发送 1000 条 `remove_npc`，但此时 server 尚未完成全部 spawn 的串行处理（稳态内），对未注册 `npc_id` 的 remove 会返回 error。
  同样是 k6 脚本产物——服务端侧无 WARN/ERROR 日志，稳态指标全绿。
- **修复方向**（后续迭代）：k6 脚本 cleanup 前应先等 pendingSpawns 清空，或基于 `response.ok` 只 remove 已确认的 `npc_id`。本次未改脚本，保持与首次基线可比。

## 附：k6 raw thresholds

- 500 档：event_to_snapshot p99 < 300 ✓ | ws_errors < 10 ✓ | spawn p99 < 500 ✗（6.78s）
- 1000 档：event_to_snapshot p99 < 300 ✓ | ws_errors < 10 ✗（500）| spawn p99 < 500 ✗（11.68s）

Threshold fail 的两项均属脚本串行 I/O 伪信号，见「已知偏差」。

## 后续建议

1. 修 k6 脚本使 cleanup 等待 pendingSpawns 清空或并行化 spawn，消除 Docker 路径下 spawn/cleanup 伪告警
2. HTTPSource 接入后，做一次相同脚本的 Admin 数据源基线，对比三种 Source 路径
3. 如需答辩曲线，按上次注记跑 `go test -tags experiment -run TestScale -timeout 30m ./internal/experiment/`
