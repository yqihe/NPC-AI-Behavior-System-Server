# 压测基准（k6 + WebSocket）

配合 [docs/standards/acceptance-checklist.md](../../docs/standards/acceptance-checklist.md) 第三节"性能与压测"使用，产出毕设答辩所需的性能基线数据。

## 前置

1. 安装 k6：<https://grafana.com/docs/k6/latest/set-up/install-k6/>
2. 启动服务端：`docker compose up --build -d`
3. 确认 `ws://localhost:9820/ws` 可连通

## 快速跑

```bash
# 默认 100 NPC, 60 秒, 每秒 1 个事件
k6 run test/benchmark/ws_load.js

# 调整规模（通过环境变量）
k6 run -e NPC_COUNT=500 -e DURATION=120s -e EVENT_RPS=5 test/benchmark/ws_load.js

# 毕设三档阶梯
k6 run -e NPC_COUNT=100  -e DURATION=60s test/benchmark/ws_load.js | tee bench_100.log
k6 run -e NPC_COUNT=500  -e DURATION=60s test/benchmark/ws_load.js | tee bench_500.log
k6 run -e NPC_COUNT=1000 -e DURATION=60s test/benchmark/ws_load.js | tee bench_1000.log
```

## 采集什么

脚本会自动输出：

- `npc_spawn_latency` — spawn 请求 RTT
- `event_to_snapshot_latency` — publish_event 到下一次 world_snapshot 含该事件影响的端到端延迟
- `snapshot_interval` — 连续两次 world_snapshot 的实际间隔（理想值为服务端 Tick 周期）
- `ws_errors` — 协议错误数
- k6 内置指标（vus, iterations, data_sent 等）

同时建议并行采集服务端侧：

```bash
# 另一个终端
docker stats --no-stream NPC-AI-Behavior-System-Server-v1-server-1
docker compose logs -f server | grep -E "fsm\.|bt\.|scheduler\."
```

## 对接验收清单

| 清单项 | 看哪个指标 |
|--------|-----------|
| P1 单机 NPC 数 | 调高 `NPC_COUNT` 直到错误率 > 0.1%，前一档即承载上限 |
| P2 Tick 延迟 P99 | `snapshot_interval` 的 p(99) |
| P3 事件端到端延迟 P99 | `event_to_snapshot_latency` 的 p(99) |
| P4 内存是否泄漏 | `docker stats` 的 RSS 随时间曲线 |
| P5 错误率 | `ws_errors` / 总请求数 |

## 对照实验场景

毕设对照实验（T4）不走 WS 压测，而是通过 `experiment` build tag 运行：

```bash
go build -tags experiment -o server-exp ./cmd/server/
EXPERIMENT_MODE=PureFSM ./server-exp
EXPERIMENT_MODE=PureBT  ./server-exp
EXPERIMENT_MODE=Hybrid  ./server-exp
```

然后对每种模式跑同一份 k6 脚本，对比三组指标。
