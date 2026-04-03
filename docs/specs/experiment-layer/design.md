# experiment-layer 设计方案

## 总体架构

```
internal/experiment/                //go:build experiment
├── scenario.go                    # 定性场景定义（距离陷阱/多步骤/生命周期）
├── generator.go                   # 规模配置生成器（程序化生成 N 行为的 FSM/BT 配置）
├── metrics.go                     # 指标收集 + ComparisonReport + 图表数据输出
├── runner.go                      # ExperimentNPC 接口 + Runner（驱动场景）
├── modes/
│   ├── hybrid.go                  # Hybrid: FSM + BT + DC
│   ├── fsm_dc.go                  # FSM+DC: FSM + DC, 无 BT
│   ├── bt_dc.go                   # BT+DC: BT + DC, 无 FSM
│   ├── pure_fsm.go               # PureFSM: FSM only
│   └── pure_bt.go                # PureBT: BT only
├── experiment_test.go             # 定性验证（表 1）
├── scale_test.go                  # 定量验证（图 1-7 数据）
└── benchmark_test.go              # 吞吐量 + 内存 Benchmark（图 2, 5）
```

---

## 1. 规模配置生成器（核心新增）

### 方案描述

手写 200 个行为的 JSON 不现实，也不科学。用 Go 代码程序化生成不同规模的配置。

```go
// ScaleConfig 某个规模档位的配置集合
type ScaleConfig struct {
    BehaviorCount int
    // PureFSM 用
    FSMConfig     *fsm.FSMConfig     // N 状态 + O(n²) 转换规则
    // PureBT 用
    BTTreeJSON    []byte             // 一棵大 Selector 树，N 个分支
    // Hybrid 用
    HybridFSM     *fsm.FSMConfig     // 固定 5 状态 + ~10 转换
    HybridBTrees  map[string][]byte  // 5 棵子树，每棵 N/5 个行为节点
}

// GenerateScaleConfig 为指定行为数生成三种模式的配置
func GenerateScaleConfig(behaviorCount int) *ScaleConfig
```

### 生成逻辑

**PureFSM（N 个状态）**：
- 状态名：S0, S1, S2, ..., S(N-1)
- 转换规则：每个状态到相邻状态 + 到高优先级状态（Flee 等价），总规则数 ≈ 2N~3N
- 每条转换用 `threat_level` 阈值条件，不同阈值触发不同转换
- 实际转换规则数随 N 增长，但不做完全 O(n²)（太极端），用 ≈ 3N 模拟真实场景

**PureBT（一棵大树，N 个行为节点）**：
```json
{
    "type": "selector",
    "children": [
        // N 个分支，每个 = sequence(check_bb_float + stub_action)
        {"type": "sequence", "children": [
            {"type": "check_bb_float", "params": {"key": "threat_level", "op": ">=", "value": 90}},
            {"type": "set_bb_value", "params": {"key": "fsm_state", "value": "S0"}},
            {"type": "stub_action", "params": {"name": "action_0", "result": "success"}}
        ]},
        // ... N 个这样的分支
    ]
}
```
- DFS 遍历，最坏情况需遍历全部 N 个分支

**Hybrid（5 FSM 状态 + 5 棵 BT 子树）**：
- FSM：固定 Idle/Alarmed/Flee/Patrol/Search 5 个状态，~10 条转换规则（与 N 无关）
- 每个状态一棵 BT 子树，每棵 N/5 个行为节点
- 单次 Tick：FSM 评估 ~10 条规则 + BT 遍历 N/5 个节点

### 备选方案：手写 JSON 配置文件（不选）

**不选的理由**：200 个行为 = 200 个 JSON 文件或一个超长 JSON。不可维护，不科学（手写容易出错），不可复现。程序化生成保证参数一致性和可复现性。

---

## 2. ExperimentNPC 接口 + Runner

```go
// ExperimentNPC 五种模式的统一接口
type ExperimentNPC interface {
    Tick(events []*event.Event, evtTypes map[string]*event.EventTypeConfig, dt float64) string
    State() string
    BB() *blackboard.Blackboard
}

// Runner 驱动模式跑场景
type Runner struct {
    Scenario *Scenario
    EvtTypes map[string]*event.EventTypeConfig
}

// ModeEntry 一个模式
type ModeEntry struct {
    Name string
    NPC  ExperimentNPC
}

func (r *Runner) RunMode(npc ExperimentNPC, name string) *ModeResult
func (r *Runner) RunAll(modes []ModeEntry) *ComparisonReport
```

Runner 内部维护活跃事件列表（模拟 Bus TTL 衰减），按 Tick 投递事件，在指定 Tick 检查 BB 状态。

---

## 3. 五种模式实现

### 3a. Hybrid（FSM + BT + DC）

复用 runtime 组件。注册 FSM OnEnter/OnExit 回调（证明 FSM 生命周期价值）。感知过滤 → 决策中心 → FSM Tick → BT Tick。

### 3b. FSM+DC（FSM + DC，无 BT）

有决策中心的距离衰减和感知过滤。有 FSM 生命周期回调。**无 BT** → current_action 始终为空。

### 3c. BT+DC（BT + DC，无 FSM）

有决策中心。有 BT 多步骤行为。**无 FSM** → 无 OnEnter/OnExit → 脏数据残留。

### 3d. PureFSM（FSM only）

取最高 severity（公平），**无距离衰减**（架构固有限制）。无 BT。

### 3e. PureBT（BT only）

取最高 severity（公平），**无距离衰减**（架构固有限制）。无 FSM。

### 规模测试版本

定性场景用固定配置（civilian 配置）；规模测试用 `GenerateScaleConfig(N)` 动态生成配置。每种模式需要一个 `NewXxxFromScaleConfig(cfg *ScaleConfig)` 工厂函数。

---

## 4. 定性场景设计（表 1）

| 场景 | 证明什么 | 关键检查 |
|------|---------|---------|
| distance_trap | DC 不可替代 | 有 DC: Alarmed(100%), 无 DC: Flee(50%) |
| multi_step_behavior | BT 不可替代 | 有 BT: current_action="run_away", 无 BT: 空 |
| state_lifecycle | FSM 不可替代 | 有 FSM: exit_cleanup_done 有值, 无 FSM: 空 |

需要的 BB Key 追加：
- `KeyCurrentAction` — BT 执行的子行为名
- `KeyAlertStartTick` — 进入 Alarmed 的时间戳
- `KeyExitCleanupDone` — OnExit 清理完成标记

需要的 BT 树改造：
- `civilian_flee.json` 改为多步骤 Sequence（assess_threat → plan_escape → run_away）

---

## 5. 定量数据采集设计（图 1-7）

### 图 1：行为数量 vs 单 Tick 耗时

```go
func TestScale_TickLatency(t *testing.T) {
    for _, n := range []int{10, 50, 100, 150, 200} {
        cfg := GenerateScaleConfig(n)
        // 创建 3 种模式的 NPC，各跑 1000 次 Tick 取平均
        // 输出：n, hybrid_ns, pureFSM_ns, pureBT_ns
    }
}
```

用 `time.Now()` 测量单 NPC 单 Tick 墙钟时间。跑 1000 次取平均消除抖动。

### 图 2：NPC × 行为 吞吐量矩阵

```go
func BenchmarkScale_Throughput(b *testing.B) {
    for _, n := range []int{10, 50, 100, 200} {
        for _, npcCount := range []int{100, 500, 1000, 5000} {
            b.Run(fmt.Sprintf("Hybrid/%dB_%dN", n, npcCount), ...)
            b.Run(fmt.Sprintf("PureFSM/%dB_%dN", n, npcCount), ...)
            b.Run(fmt.Sprintf("PureBT/%dB_%dN", n, npcCount), ...)
        }
    }
}
```

### 图 3：行为数量 vs 配置复杂度

```go
func TestScale_ConfigComplexity(t *testing.T) {
    for _, n := range []int{10, 50, 100, 150, 200} {
        cfg := GenerateScaleConfig(n)
        // 统计：FSM 转换规则数、BT 树节点数、Hybrid FSM 规则数 + BT 总节点数
    }
}
```

直接在生成时统计，不需要解析配置文件。

### 图 4：边际扩展成本

```go
func TestScale_MarginalCost(t *testing.T) {
    for _, n := range []int{10, 50, 100, 200} {
        cfgN := GenerateScaleConfig(n)
        cfgN1 := GenerateScaleConfig(n + 1)
        // 对比两者配置差异：新增了几条转换规则、几个 BT 节点
    }
}
```

### 图 5：行为数量 vs 内存/NPC

通过 Benchmark 的 `b.ReportAllocs()` 收集。

### 图 6：单事件响应墙钟时间

```go
func TestScale_EventResponseTime(t *testing.T) {
    for _, n := range []int{10, 50, 100, 150, 200} {
        cfg := GenerateScaleConfig(n)
        // 创建 NPC，注入 1 个事件，测量从注入到状态变化的墙钟耗时
    }
}
```

### 图 7：综合雷达图

在 100 行为 × 1000 NPC 下收集 5 个维度的数据，归一化后输出。手动画雷达图。

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止实验侵入核心 | **不违反** | build tag 隔离 |
| 禁止 core/runtime import experiment | **不违反** | 正向依赖 |
| 禁止实验作弊（公平性） | **不违反** | 最佳努力实现 |
| 禁止实验作弊（立论） | **不违反** | 三个定性场景覆盖三层 + 规模增长量化 |
| 禁止只用玩具规模 | **不违反** | 10~200 行为递增 |
| 禁止接受全零指标 | **不违反** | 响应延迟改为墙钟时间(ns) |
| 禁止把标配当创新 | **不违反** | 创新是三层协作模式，不是某单一组件 |

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面** | 图 4 量化扩展成本 |
| 加 NPC 类型 | **正面** | 同上 |

## 依赖方向

```
internal/experiment/ → internal/core/ + internal/runtime/ + internal/config/
（反向绝不存在）
```

## 并发安全

实验代码全部同步执行。Benchmark 中每个 sub-benchmark 独立。无并发问题。

## 配置变更

修改：
- `internal/core/blackboard/keys.go` — 追加 3 个 Key
- `configs/bt_trees/civilian_flee.json` — 改为多步骤 Sequence

新增：无新 JSON 文件（规模配置程序化生成）

## 测试策略

| 测试文件 | 覆盖 |
|---------|------|
| `experiment_test.go` | 表 1 定性验证（R4-R6）+ 基线对照 |
| `scale_test.go` | 图 1/3/4/6 数据采集（R7-R10, R12） |
| `benchmark_test.go` | 图 2/5 吞吐量+内存 Benchmark（R8, R11） |

图 7 雷达图数据从图 1-6 的 100 行为档位提取，手动归一化。
