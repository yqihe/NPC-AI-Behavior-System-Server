# experiment-layer 任务拆解

## 依赖关系

```
T1 BB Keys + BT 树改造（core 层追加）
T2 Scenario + Metrics + Runner 框架           → 依赖 T1
T3 规模配置生成器 GenerateScaleConfig          → 依赖 T2
T4 Hybrid 模式（固定配置 + 规模配置）          → 依赖 T2, T3
T5 FSM+DC 模式                               → 依赖 T2, T3
T6 BT+DC 模式                                → 依赖 T2, T3
T7 PureFSM 模式                              → 依赖 T2, T3
T8 PureBT 模式                               → 依赖 T2, T3
T9 定性验证测试（表 1）                       → 依赖 T4-T8
T10 定量验证测试（图 1/3/4/6 数据）            → 依赖 T4-T8
T11 吞吐量 + 内存 Benchmark（图 2/5 数据）     → 依赖 T4-T8
```

---

## 任务列表

### [x] T1: BB Keys 追加 + BT 树改造 (R2, R5, R6)

**产出**：新增 3 个 BB Key + civilian_flee.json 改为多步骤 Sequence

**文件**：
- `internal/core/blackboard/keys.go` — 追加 KeyCurrentAction、KeyAlertStartTick、KeyExitCleanupDone
- `configs/bt_trees/civilian_flee.json` — 改为 Sequence（assess_threat → plan_escape → run_away）

**做完是什么样**：
- 3 个新 Key 可用
- `go test ./internal/core/...` 全部通过（含 runtime 集成测试）
- civilian_flee BT 树包含 6 个节点的多步骤 Sequence

---

### [x] T2: Scenario + Metrics + Runner 框架 (R3, R12, R13)

**产出**：定性场景定义 + BBCheckpoint 机制 + 指标收集 + Runner

**文件**：
- `internal/experiment/scenario.go` — Scenario/ScenarioEvent/ExpectedState/BBCheckpoint + 3 个定性场景
- `internal/experiment/metrics.go` — TickRecord/ModeResult/BBCheckResult/ComparisonReport/PrintTable
- `internal/experiment/runner.go` — ExperimentNPC 接口/ModeEntry/Runner/RunMode/RunAll

**做完是什么样**：
- 3 个定性场景可构造（distance_trap/multi_step_behavior/state_lifecycle）
- Runner 在指定 Tick 检查 BB 快照（不是场景结束后）
- ComparisonReport.PrintTable 输出含 BB 检查点的多模式对比表
- `go build -tags experiment ./internal/experiment/...` 通过

---

### [x] T3: 规模配置生成器 (R7, R8, R9)

**产出**：程序化生成 N 行为规模的 FSM/BT/Hybrid 配置

**文件**：
- `internal/experiment/generator.go` — ScaleConfig 结构体 + GenerateScaleConfig(N) 函数

**做完是什么样**：
- `GenerateScaleConfig(10)` 生成：PureFSM 10 状态+~30 转换，PureBT 10 分支大树，Hybrid 5 状态+5 棵各 2 节点 BT
- `GenerateScaleConfig(200)` 生成：PureFSM 200 状态+~600 转换，PureBT 200 分支大树，Hybrid 5 状态+5 棵各 40 节点 BT
- 生成的 FSMConfig 可被 `fsm.NewFSM` 接受，BT JSON 可被 `bt.BuildFromJSON` 解析
- 配置复杂度统计（转换规则数、节点数）随配置一起输出

---

### [x] T4: Hybrid 模式 (R4)

**产出**：Hybrid NPC，支持固定配置（定性）+ 规模配置（定量）

**文件**：
- `internal/experiment/modes/hybrid.go` — HybridNPC + NewHybridNPC（固定配置）+ NewHybridFromScale（规模配置）

**做完是什么样**：
- 固定配置版：复用 runtime 组件 + OnEnter/OnExit 回调（写 alert_start_tick / exit_cleanup_done）
- 规模配置版：从 ScaleConfig.HybridFSM + HybridBTrees 创建
- Flee 状态 BT 执行后 current_action="run_away"

---

### [x] T5: FSM+DC 模式 (R5)

**产出**：FSM+DC NPC，支持固定配置 + 规模配置

**文件**：
- `internal/experiment/modes/fsm_dc.go` — FSMDCNPC + NewFSMDCNPC + NewFSMDCFromScale

**做完是什么样**：
- 有决策中心（距离衰减+感知过滤）+ FSM 生命周期回调
- 无 BT → current_action 始终为空
- 规模版从 ScaleConfig.FSMConfig 创建

---

### [x] T6: BT+DC 模式 (R6)

**产出**：BT+DC NPC，支持固定配置 + 规模配置

**文件**：
- `internal/experiment/modes/bt_dc.go` — BTDCNPC + NewBTDCNPC + NewBTDCFromScale

**做完是什么样**：
- 有决策中心 + BT 行为
- 无 FSM → 无 OnExit → alert_start_tick 残留，exit_cleanup_done 为空
- 规模版从 ScaleConfig.BTTreeJSON 创建

---

### [x] T7: PureFSM 模式 (R7)

**产出**：PureFSM NPC，支持固定配置 + 规模配置

**文件**：
- `internal/experiment/modes/pure_fsm.go` — PureFSMNPC + NewPureFSMNPC + NewPureFSMFromScale

**做完是什么样**：
- 取最高 severity（公平），无距离衰减
- 规模版从 ScaleConfig.FSMConfig 创建（N 状态 + ~3N 转换）

---

### [x] T8: PureBT 模式 (R8)

**产出**：PureBT NPC，支持固定配置 + 规模配置

**文件**：
- `internal/experiment/modes/pure_bt.go` — PureBTNPC + NewPureBTNPC + NewPureBTFromScale
- `configs/bt_trees/civilian_pure_bt.json` — PureBT 固定配置用大行为树

**做完是什么样**：
- 取最高 severity（公平），无距离衰减
- 规模版从 ScaleConfig.BTTreeJSON 创建（N 分支大 Selector）

---

### [x] T9: 定性验证测试——表 1 (R4, R5, R6, R14, R15)

**产出**：三层不可替代性对照测试 + build tag 验证

**文件**：
- `internal/experiment/experiment_test.go` — 5 模式 × 3 场景定性对照

**做完是什么样**：
- `TestExperiment_DistanceTrap`：有 DC 100% vs 无 DC 50%
- `TestExperiment_MultiStepBehavior`：Hybrid BB current_action=PASS，FSM+DC=FAIL
- `TestExperiment_StateLifecycle`：Hybrid/FSM+DC exit_cleanup_done=PASS，BT+DC=FAIL
- `go test -tags experiment ... -v` 通过
- `go test ./...`（无 tag）通过

---

### [x] T10: 定量验证测试——图 1/3/4/6 数据 (R7, R9, R10, R12)

**产出**：规模增长数据采集

**文件**：
- `internal/experiment/scale_test.go` — 4 个数据采集测试

**做完是什么样**：
- `TestScale_TickLatency`：输出 5 档 × 3 模式的单 Tick 耗时(ns)（图 1 数据）
- `TestScale_ConfigComplexity`：输出 5 档 × 3 模式的转换规则数/节点数（图 3 数据）
- `TestScale_MarginalCost`：输出 4 档 × 3 模式的边际扩展代价（图 4 数据）
- `TestScale_EventResponseTime`：输出 5 档 × 3 模式的单事件墙钟耗时(ns)（图 6 数据）
- 所有数据不允许出现全零

---

### [x] T11: 吞吐量 + 内存 Benchmark——图 2/5 数据 (R8, R11, R13)

**产出**：NPC 规模 × 行为复杂度的 Benchmark

**文件**：
- `internal/experiment/benchmark_test.go` — 3 模式 × 4 行为档 × 4 NPC 档

**做完是什么样**：
- `BenchmarkScale_Hybrid/PureFSM/PureBT` 各含 16 个子 benchmark（4 行为 × 4 NPC）
- `b.ReportAllocs()` 收集内存（图 5 数据）
- `go test -tags experiment -bench=BenchmarkScale -benchmem ...` 输出完整矩阵
- 图 7 雷达图数据从 100 行为档提取
