# runtime-layer 设计方案

## 数据流总览

```
                         ┌─────────────────────────────────────────────────┐
                         │                  Scheduler.Tick()               │
                         │                                                 │
                         │  1. EventBus.Tick()         — TTL 衰减，清理过期 │
                         │  2. for each NPC:                               │
                         │     a. Perception.Filter()  — 过滤可感知事件     │
                         │     b. Decision.Evaluate()  — 威胁评估+仲裁      │
                         │     c. FSM.Tick()            — 状态转换          │
                         │     d. BT.Tick()             — 行为执行          │
                         └─────────────────────────────────────────────────┘
```

---

## 1. Event（事件总线）

### 方案描述

```go
// Event 一个运行时事件实例
type Event struct {
    ID       string   // 唯一 ID（UUID 或递增）
    Type     string   // 事件类型名，对应 EventTypeConfig
    Position Vec3     // 事件发生位置
    Severity float64  // 实际 severity（可覆盖默认值）
    TTL      float64  // 剩余生存时间（秒），每 Tick 递减 deltaTime
    SourceID string   // 事件来源（哪个实体触发的）
}

type Vec3 struct {
    X, Y, Z float64
}

// EventTypeConfig 事件类型的配置定义
type EventTypeConfig struct {
    Name           string  `json:"name"`            // "explosion", "gunshot", "shout"
    DefaultSeverity float64 `json:"default_severity"` // 默认严重度
    DefaultTTL     float64 `json:"default_ttl"`      // 默认 TTL（秒）
    PerceptionMode string  `json:"perception_mode"`  // "visual" | "auditory" | "global"
    Range          float64 `json:"range"`            // 事件传播范围（米）
}

// Bus 事件总线
type Bus struct {
    mu       sync.RWMutex
    active   []*Event           // 当前活跃的事件列表
    types    map[string]*EventTypeConfig  // 已注册的事件类型
}

// Publish 发布一个事件
func (b *Bus) Publish(evt *Event)

// Tick 衰减所有事件的 TTL，移除过期事件
func (b *Bus) Tick(dt float64)

// Active 返回当前所有活跃事件的快照（只读切片）
func (b *Bus) Active() []*Event
```

**TTL 衰减模型**（复用 v1 经验，经 68 断言验证）：
- 每次 Tick 调用 `evt.TTL -= dt`
- TTL <= 0 的事件从 active 列表移除
- 外部通过 `Active()` 获取当前活跃事件快照

### 备选方案：channel-based 事件总线（不选）

用 Go channel 做发布/订阅，每个 NPC 一个订阅 channel。

**不选的理由**：channel 是并发原语，而 Tick 调度是同步循环——所有 NPC 在同一 goroutine 中顺序 Tick。用 channel 会增加不必要的同步开销和复杂度。直接用 slice + mutex 更简单高效。

### 并发安全

Bus 被 Scheduler goroutine 读写，可能被 Gateway goroutine 调用 Publish（外部事件输入）。`sync.RWMutex` 保护 active 列表：Publish 加写锁，Tick 加写锁，Active 加读锁。

---

## 2. Perception（感知过滤）

### 方案描述

```go
// PerceptionConfig NPC 类型的感知参数
type PerceptionConfig struct {
    VisualRange   float64 `json:"visual_range"`   // 视觉感知距离（米）
    AuditoryRange float64 `json:"auditory_range"` // 听觉感知距离（米）
}

// CanPerceive 判断 NPC 是否能感知到事件
// 不是一个 struct，是一个纯函数——无状态，易测试
func CanPerceive(npcPos Vec3, perceptionCfg *PerceptionConfig, evt *Event, evtTypeCfg *EventTypeConfig) bool
```

逻辑：
1. `perception_mode == "global"` → 所有 NPC 都能感知
2. `perception_mode == "visual"` → 距离 <= min(visual_range, event.range)
3. `perception_mode == "auditory"` → 距离 <= min(auditory_range, event.range)
4. 距离 = 欧氏距离（XZ 平面，忽略 Y）

### 备选方案：Perception 作为 struct + 接口（不选）

```go
type Perceiver interface {
    CanPerceive(evt *Event) bool
}
```

**不选的理由**：感知过滤的输入全部来自配置（NPC 感知参数 + 事件类型感知方式），无内部状态。纯函数比 struct+接口更简单，且调用方已经持有所有必要参数。为一个纯函数创建接口是过度设计。

### 距离计算

```go
func Distance(a, b Vec3) float64 {
    dx := a.X - b.X
    dz := a.Z - b.Z
    return math.Sqrt(dx*dx + dz*dz) // XZ 平面距离，忽略 Y
}
```

不做 AOI 空间索引，直接遍历。100 NPC × 10 事件 = 1000 次距离计算，纯算术，性能不是问题。

---

## 3. Decision（决策中心）

### 方案描述

```go
// Center 决策中心
type Center struct{}

// Evaluate 对单个 NPC 评估所有可感知事件，仲裁并写入 BB
// events: 已经过感知过滤的事件列表
func (c *Center) Evaluate(bb *blackboard.Blackboard, npcPos Vec3, events []*event.Event, evtTypes map[string]*event.EventTypeConfig)
```

**威胁评估公式**：

```
threat = severity × distanceFactor

distanceFactor = max(0, 1 - distance / eventRange)
```

- 距离为 0 时 factor=1，威胁等于 severity
- 距离等于 eventRange 时 factor=0，威胁为 0
- 超出 range 的事件不应到达这里（已被感知过滤拦截）

**仲裁逻辑（每 Tick 执行）**：

1. 对所有可感知事件计算 threat 值
2. 取 threat 最高的事件
3. 如果最高 threat > 当前 BB 的 threat_level → 覆写 BB：
   - `KeyThreatLevel = maxThreat`
   - `KeyThreatSource = event.ID`
   - `KeyLastEventType = event.Type`
   - `KeyThreatExpireAt = 当前时间 + 事件剩余 TTL`
4. 如果没有任何可感知事件 → 威胁衰减：
   - `KeyThreatLevel = max(0, current - decayRate × dt)`
   - 衰减到 0 时清空 threat_source 和 last_event_type

### 备选方案：事件队列 + 优先级堆（不选）

维护一个按 severity 排序的优先级队列，新事件入队，取队头。

**不选的理由**：活跃事件数量很少（通常 < 20），每 Tick 全量重新计算比维护堆更简单，且能正确处理距离衰减（同一事件对不同 NPC 的 threat 值不同，堆无法处理这个）。

### 设计要点

- **无状态**：Center 不缓存任何 NPC 的历史数据，所有状态在 BB 中。这意味着 NPC 的创建/销毁不需要通知 Center。
- **每 Tick 重新仲裁**：不缓存上一 Tick 的结果，保证新事件到达时立即生效（R9 事件抢占）。
- **衰减由 Center 负责，不由 FSM**：FSM 只读 BB 判断转换条件，不负责写 threat_level。职责分离清晰。

---

## 4. NPC（实例 + 注册表）

### 方案描述

```go
// NPCTypeConfig NPC 类型配置
type NPCTypeConfig struct {
    TypeName   string                      `json:"type_name"`    // "civilian", "police"
    FSMRef     string                      `json:"fsm_ref"`      // FSM 配置名，如 "civilian"
    BTRefs     map[string]string           `json:"bt_refs"`      // 状态名 → BT 树名，如 {"Idle": "civilian_idle", "Flee": "civilian_flee"}
    Perception perception.PerceptionConfig `json:"perception"`   // 感知参数
}

// Instance 一个 NPC 运行时实例
type Instance struct {
    ID         string
    TypeName   string
    Position   Vec3
    BB         *blackboard.Blackboard
    FSM        *fsm.FSM
    BTrees     map[string]bt.Node          // 状态名 → 已构建的 BT 根节点
    Perception *perception.PerceptionConfig
}

// Tick 执行单个 NPC 的一次 Tick（FSM + BT）
func (inst *Instance) Tick()

// Registry NPC 注册表
type Registry struct {
    mu    sync.RWMutex
    npcs  map[string]*Instance  // ID → Instance
}

func (r *Registry) Add(inst *Instance)
func (r *Registry) Remove(id string)
func (r *Registry) Get(id string) (*Instance, bool)
func (r *Registry) ForEach(fn func(*Instance))
```

**NPC 创建流程（工厂函数）**：

```go
// NewInstance 从配置创建 NPC 实例
func NewInstance(id string, pos Vec3, typeCfg *NPCTypeConfig, src config.Source, btReg *bt.Registry) (*Instance, error)
```

1. 创建 Blackboard
2. 通过 `src.LoadFSMConfig(typeCfg.FSMRef)` 加载 FSM 配置，创建 FSM
3. 遍历 `typeCfg.BTRefs`，通过 `src.LoadBTTree(treeName)` + `bt.BuildFromJSON` 构建每个状态的 BT
4. 组装 Instance

**不使用 switch-case**：不同 NPC 类型的差异全部在配置中（FSM 配置、BT 配置、感知参数），Go 代码完全通用。

### Instance.Tick 内部逻辑

```go
func (inst *Instance) Tick() {
    // 1. FSM Tick（评估转换条件）
    inst.FSM.Tick(inst.BB)

    // 2. 获取当前状态对应的 BT
    currentState := inst.FSM.Current()
    tree, ok := inst.BTrees[currentState]
    if !ok {
        return // 该状态没有行为树，跳过
    }

    // 3. BT Tick
    ctx := &bt.Context{BB: inst.BB}
    tree.Tick(ctx)
}
```

### 备选方案：NPC 类型接口多态（不选）

```go
type NPCType interface {
    CreateFSM(bb *Blackboard) *FSM
    CreateBT(state string) Node
}
```

每种 NPC 类型实现该接口。

**不选的理由**：这就是 v1 的做法。加新 NPC 类型需要写新的 Go 实现文件。配置驱动的核心就是消灭 per-type Go 代码。

---

## 5. Scheduler（Tick 调度器）

### 方案描述

```go
// Scheduler 驱动整个 Runtime 的 Tick 循环
type Scheduler struct {
    eventBus   *event.Bus
    registry   *npc.Registry
    decision   *decision.Center
    evtTypes   map[string]*event.EventTypeConfig
    tickRate   float64  // Tick 间隔（秒），如 0.1 = 100ms
}

// Tick 执行一次完整的 Tick 循环
func (s *Scheduler) Tick(dt float64) {
    // 1. 事件 TTL 衰减
    s.eventBus.Tick(dt)

    // 2. 获取当前活跃事件快照
    activeEvents := s.eventBus.Active()

    // 3. 遍历所有 NPC
    s.registry.ForEach(func(inst *npc.Instance) {
        // 3a. 感知过滤
        perceived := make([]*event.Event, 0)
        for _, evt := range activeEvents {
            typeCfg := s.evtTypes[evt.Type]
            if perception.CanPerceive(inst.Position, inst.Perception, evt, typeCfg) {
                perceived = append(perceived, evt)
            }
        }

        // 3b. 决策中心评估
        s.decision.Evaluate(inst.BB, inst.Position, perceived, s.evtTypes)

        // 3c. NPC Tick（FSM + BT）
        inst.Tick()
    })
}

// Run 启动 Tick 循环（阻塞，直到 ctx 取消）
func (s *Scheduler) Run(ctx context.Context)
```

### 备选方案：每个 NPC 一个 goroutine（不选）

**不选的理由**：1000 个 NPC = 1000 个 goroutine，goroutine 调度开销+BB 并发保护开销远大于顺序遍历。v1 的单 goroutine 遍历模式已验证可行。并发优化（如分片）在 100 NPC 的规模下完全不需要。

---

## 6. Config 扩展

### 方案描述

扩展 `config.Source` 接口：

```go
type Source interface {
    LoadFSMConfig(npcType string) (*fsm.FSMConfig, error)
    LoadBTTree(treeName string) ([]byte, error)
    // 新增
    LoadEventConfig(eventType string) (*event.EventTypeConfig, error)
    LoadAllEventConfigs() (map[string]*event.EventTypeConfig, error)
    LoadNPCTypeConfig(npcType string) (*npc.NPCTypeConfig, error)
}
```

JSONSource 实现：
- `LoadEventConfig("explosion")` → 读 `configs/events/explosion.json`
- `LoadAllEventConfigs()` → 遍历 `configs/events/` 目录下所有 JSON
- `LoadNPCTypeConfig("civilian")` → 读 `configs/npc_types/civilian.json`

### 配置文件 Schema

**事件类型配置** `configs/events/explosion.json`：
```json
{
    "name": "explosion",
    "default_severity": 80,
    "default_ttl": 15.0,
    "perception_mode": "auditory",
    "range": 500.0
}
```

**NPC 类型配置** `configs/npc_types/civilian.json`：
```json
{
    "type_name": "civilian",
    "fsm_ref": "civilian",
    "bt_refs": {
        "Idle": "civilian_idle",
        "Alarmed": "civilian_alarmed",
        "Flee": "civilian_flee"
    },
    "perception": {
        "visual_range": 200.0,
        "auditory_range": 500.0
    }
}
```

**BT 树配置** `configs/bt_trees/civilian_idle.json`（stub）：
```json
{
    "type": "sequence",
    "children": [
        {
            "type": "check_bb_string",
            "params": {"key": "fsm_state", "op": "==", "value": "Idle"}
        },
        {
            "type": "stub_action",
            "params": {"name": "idle_wander", "result": "success"}
        }
    ]
}
```

### 新增 Blackboard Keys

在 `keys.go` 中追加 Runtime 层需要的 Key：

```go
// --- NPC 实例 ---
var KeyNPCType    = NewKey[string]("npc_type")     // NPC 类型名
var KeyNPCPosX    = NewKey[float64]("npc_pos_x")   // NPC 位置 X
var KeyNPCPosZ    = NewKey[float64]("npc_pos_z")   // NPC 位置 Z
```

已有 Key（core-engine 定义）直接复用：`KeyThreatLevel`, `KeyThreatSource`, `KeyThreatExpireAt`, `KeyLastEventType`, `KeyCurrentTime`, `KeyFSMState`。

### 新增 BT 节点

注册一个 `stub_action` 节点工厂到默认 Registry：

```go
// stub_action：占位行为节点，根据 params.result 返回固定状态
// 用于本 spec 中未实现的业务 Action（move_to、play_animation 等）
r.Register("stub_action", stubActionFactory)
```

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码 FSM 状态/转换规则 | **不违反** | FSM 配置从 JSON 加载（core-engine 已实现） |
| 禁止硬编码事件→感知映射 | **不违反** | 事件的 perception_mode 在事件类型 JSON 配置中声明，NPC 感知参数在 NPC 类型 JSON 中声明 |
| 禁止硬编码 NPC 类型参数 | **不违反** | NPC 类型参数全部在 JSON 配置中 |
| 禁止 switch-case 做 NPC 类型分发 | **不违反** | 用配置驱动 + 通用 Instance，无 per-type Go 代码 |
| 禁止 BT 反向驱动 FSM | **不违反** | BT 不写 FSM 状态，FSM 转换由规则匹配器求值 BB 驱动 |
| 禁止 core/ import runtime/ | **不违反** | runtime/ 依赖 core/，反向不存在 |
| 禁止 Blackboard 裸 map | **不违反** | 通过 BBKey[T] 访问 |
| 禁止 Key 散落各文件 | **不违反** | 新 Key 追加到 keys.go |
| 禁止 Gateway 承担非网络职责 | **不违反** | Tick 调度、事件发布、感知过滤全在 Runtime 层 |
| 禁止实验侵入核心 | **不违反** | Runtime 层无实验相关代码 |
| 禁止过度设计 | **不违反** | 感知用纯函数不做接口、不做 AOI、不做三级调度 |

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面** | 新增 `configs/events/xxx.json` 即可，Bus.Publish 是通用的，CanPerceive 按配置过滤，Decision.Evaluate 按公式计算，无需改 Go 代码 |
| 加 NPC 类型 | **正面** | 新增 `configs/npc_types/xxx.json` + FSM 配置 + BT 配置，NewInstance 是通用工厂 |
| NPC 间交互 | **中性** | Decision.Evaluate 只处理 Event→NPC，不处理 NPC→NPC。但架构不阻碍后续扩展：NPC 交互可以转化为事件发布 |

## 依赖方向

```
internal/runtime/npc/
    → internal/core/blackboard/
    → internal/core/fsm/
    → internal/core/bt/
    → internal/runtime/perception/  (类型引用)

internal/runtime/decision/
    → internal/core/blackboard/
    → internal/runtime/event/       (Event 类型引用)

internal/runtime/perception/
    → internal/runtime/event/       (Event/EventTypeConfig 类型引用)

internal/runtime/event/
    → (无 internal 依赖)

internal/config/
    → internal/core/fsm/           (FSMConfig)
    → internal/runtime/event/      (EventTypeConfig)
    → internal/runtime/npc/        (NPCTypeConfig)

Scheduler (在 npc/ 或独立包)
    → internal/runtime/event/
    → internal/runtime/npc/
    → internal/runtime/decision/
    → internal/runtime/perception/
```

单向向下，无循环依赖。

**注意**：config/ 依赖 runtime/ 类型定义。如果要避免这个方向（config 不应知道 runtime），可以让配置结构体定义在各自包中，config/ 只返回 `[]byte` 或 `json.RawMessage`。

**决策：EventTypeConfig 定义在 event/ 包中，NPCTypeConfig 定义在 npc/ 包中。config.Source 的新方法返回这些类型。** 这意味着 config/ 依赖 runtime/event 和 runtime/npc 的类型。这是 Loader 层依赖业务层的类型定义，方向合理（config 知道要加载什么）。

**替代方案**：LoadEventConfig 返回 `json.RawMessage`，调用方自己 Unmarshal。但这失去了 Source 接口的类型安全性。

**最终决策**：为避免 config/ 和 runtime/ 的循环依赖风险，EventTypeConfig 和 NPCTypeConfig 定义在 config/ 包中作为纯数据结构，runtime/ 包引用 config/ 中的类型。这样依赖方向是 runtime/ → config/ → core/，完全单向。

```
修正后依赖方向：

runtime/npc/        → config/  → core/fsm/
                    → core/blackboard/
                    → core/bt/

runtime/decision/   → core/blackboard/
                    → runtime/event/

runtime/perception/ → runtime/event/
                    → config/

runtime/event/      → config/ (EventTypeConfig)
                    或 → (无依赖，EventTypeConfig 独立定义在 event/ 中)

config/             → core/fsm/ (已有)
```

**最终最终决策**：类型定义归属于使用它的包——EventTypeConfig 在 event/ 包中定义，NPCTypeConfig 在 npc/ 包中定义。config.Source 接口的新方法返回 `[]byte`（原始 JSON），由调用方 Unmarshal 为各自包的类型。这样 config/ 只依赖 core/fsm（已有），不新增对 runtime/ 的依赖。

```
最终依赖方向：

config/             → core/fsm/              (已有，LoadFSMConfig)

runtime/event/      → (无 internal 依赖)
runtime/perception/ → runtime/event/
runtime/decision/   → runtime/event/ + core/blackboard/
runtime/npc/        → core/blackboard/ + core/fsm/ + core/bt/ + runtime/event/ + config/

Scheduler           → runtime/*
```

## 并发安全

| 共享状态 | 读者 | 写者 | 保护方式 |
|---------|------|------|---------|
| EventBus.active | Scheduler(Tick) | Scheduler(Tick) + Gateway(Publish) | sync.RWMutex |
| Registry.npcs | Scheduler(ForEach) | Gateway(Add/Remove) | sync.RWMutex |
| Blackboard | Decision(写) + FSM(读) + BT(读写) | 同一 NPC 在 Scheduler 内顺序执行 | BB 自带 RWMutex；同一 NPC 的 Tick 内无并发 |

**关键点**：单个 NPC 的 Decision → FSM → BT 是顺序执行的（同一 Tick 内），所以 BB 的并发竞争只发生在跨 NPC 场景（NPC 间交互），本 spec 不涉及。

## 配置变更

新增配置文件：
- `configs/events/explosion.json` — 爆炸事件
- `configs/events/gunshot.json` — 枪声事件
- `configs/events/shout.json` — 呼叫事件
- `configs/npc_types/civilian.json` — 平民 NPC 类型
- `configs/bt_trees/civilian_idle.json` — 平民空闲行为（stub）
- `configs/bt_trees/civilian_alarmed.json` — 平民警戒行为（stub）
- `configs/bt_trees/civilian_flee.json` — 平民逃跑行为（stub）

修改配置文件：
- 无（已有的 `configs/fsm/civilian.json` 不需要修改）

## 测试策略

| 模块 | 单元测试覆盖 |
|------|-------------|
| event/ | Publish/Tick/TTL 衰减/过期移除/Active 快照/并发 Publish |
| perception/ | 各 perception_mode 下的 CanPerceive 正确性/距离边界/超出范围 |
| decision/ | 威胁计算公式/多事件仲裁取最高/无事件衰减/事件抢占覆写 |
| npc/ | NewInstance 从配置创建/Registry CRUD/Instance.Tick 链路 |
| config/ | LoadEventConfig/LoadNPCTypeConfig/文件不存在/JSON 格式错误 |
| Scheduler | 完整 Tick 链路/多 NPC 遍历/事件过期后状态恢复 |

**集成测试**（`internal/runtime/integration_test.go`）：
- 场景 1：平民遇爆炸逃跑（完整链路）
- 场景 2：事件过期后恢复
- 场景 3：多事件优先级仲裁
- 场景 4：高威胁打断低威胁行为
- 场景 5：加新事件类型配置后自动响应

**Benchmark**：
- `BenchmarkTick_100NPCs` — 100 NPC 单 Tick 延迟 < 10ms（R14）
