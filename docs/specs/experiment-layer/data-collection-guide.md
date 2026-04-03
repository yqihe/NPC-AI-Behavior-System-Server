# 实验数据采集指南

毕设论文需要数据时，按此文档执行对应命令。

## 前置条件

```bash
cd E:\gProject\NPC-AI-Behavior-System\NPC-AI-Behavior-System-Server
go test ./... -count=1   # 先确认正常构建通过
```

---

## 表 1：三层不可替代性（定性验证）

**用途**：论文正文表格，证明 DC/BT/FSM 各自不可替代

```bash
# 距离陷阱——证明 DC 不可替代
go test -tags experiment ./internal/experiment/... -v -run TestExperiment_DistanceTrap

# 多步骤行为——证明 BT 不可替代
go test -tags experiment ./internal/experiment/... -v -run TestExperiment_MultiStepBehavior

# 状态生命周期——证明 FSM 不可替代
go test -tags experiment ./internal/experiment/... -v -run TestExperiment_StateLifecycle

# 基线对照
go test -tags experiment ./internal/experiment/... -v -run TestExperiment_Civilian3Events
```

**数据特征**：确定性，跑 1 次即可，不需要多轮。

---

## 图 1：行为数量 vs 单 Tick 耗时（折线图）

**用途**：展示 Hybrid 增长率最低，PureBT 增长最快（交叉点 ~200 行为）

```bash
# 建议跑 3 轮取平均
go test -tags experiment ./internal/experiment/... -v -count=3 -run TestScale_TickLatency -timeout 600s
```

**数据特征**：50 万迭代/点，3 轮验证一致性。如出现零值说明环境有问题。

---

## 图 2：NPC × 行为 吞吐量矩阵（折线图组）

**用途**：展示不同规模组合下三种模式的总 Tick 耗时，找到 Hybrid 胜出区域

```bash
go test -tags experiment -bench=BenchmarkScale -benchmem ./internal/experiment/... -run=^$ -count=1 -timeout 600s
```

**数据特征**：Go Benchmark 框架自动控制迭代，最可靠的数据源。输出含 ns/op + B/op + allocs/op。

---

## 图 3：行为数量 vs 配置复杂度（折线图）

**用途**：展示 PureFSM 转换规则 O(3N) vs Hybrid FSM 转换 O(1)

```bash
go test -tags experiment ./internal/experiment/... -v -run TestScale_ConfigComplexity
```

**数据特征**：确定性（程序化生成），跑 1 次即可。

---

## 图 4：边际扩展成本（分组柱形图）

**用途**：展示 Hybrid 加 1 个行为的增量 = 0

```bash
go test -tags experiment ./internal/experiment/... -v -run TestScale_MarginalCost
```

**数据特征**：确定性，跑 1 次即可。

---

## 图 5：行为数量 vs 内存/NPC（折线图）

**用途**：展示三种模式的内存效率随行为增长的变化

**数据来源**：从图 2 的 Benchmark 输出中提取 `B/op` 列，除以 NPC 数。

```bash
# 同图 2 命令，关注 B/op 列
go test -tags experiment -bench=BenchmarkScale -benchmem ./internal/experiment/... -run=^$ -count=1 -timeout 600s
```

---

## 图 6：行为数量 vs 单事件响应墙钟时间（折线图）

**用途**：展示事件到达后的实际处理速度

```bash
# 建议跑 3 轮，取稳定的 2 轮平均（如第 3 轮异常则排除）
go test -tags experiment ./internal/experiment/... -v -count=3 -run TestScale_EventResponseTime -timeout 600s
```

**数据特征**：50 万迭代/点。可能受系统后台任务干扰，需检查是否有整轮异常。

---

## 图 7：综合评分雷达图

**用途**：论文结论页"一图说明 Hybrid 综合最优"

**数据来源**：从图 1-6 中提取 100 行为档位的数据，手动归一化为 0-100 分。

| 维度 | 数据来源 | 归一化方式 |
|------|---------|-----------|
| Tick 耗时 | 图 1 的 100B 行 | 最快=100，最慢=0 |
| 吞吐量 | 图 2 的 100B×1000N 行 | 最快=100 |
| 内存效率 | 图 5 的 100B 行 | 最低=100 |
| 配置复杂度 | 图 3 的 100B 行 | 最少=100 |
| 行为正确性 | 表 1 三个场景的综合得分 | 全 PASS=100 |

---

## 注意事项

1. 跑性能数据前**关闭无关程序**（浏览器、IDE 编译等），减少干扰
2. **不要在 VPN/代理切换时**跑性能测试，网络变化可能触发系统中断
3. 如果某轮数据整体偏高（如所有模式耗时翻倍），标记为异常轮排除
4. 图 2 的 Benchmark 数据是最可靠的，论文优先引用
5. 所有命令在项目根目录执行
