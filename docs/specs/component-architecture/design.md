# component-architecture 设计方案

## 方案描述

### 1. 组件接口（`internal/runtime/component/`）

```go
// Component 所有 NPC 组件的基接口
type Component interface {
    Name() string
}

// Tickable 需要每帧更新的组件
// behavior 组件不走此接口——AI 管线由 Scheduler 显式编排
type Tickable interface {
    Component
    Tick(bb *blackboard.Blackboard, dt float64)
}
```

**为什么 behavior 不走 Tickable**：AI 管线有严格执行顺序——感知过滤 → 决策评估 → FSM → BT。如果 behavior 作为通用 Tickable，Scheduler 无法保证它在 Decision 之后执行。因此 AI 管线保持显式编排，movement/needs/emotion/memory 通过 Tickable 泛化。

### 2. 组件注册表

```go
type Factory func(raw json.RawMessage) (Component, error)

type Registry struct {
    factories map[string]Factory
}

func NewRegistry() *Registry
func (r *Registry) Register(name string, factory Factory)
func (r *Registry) Create(name string, raw json.RawMessage) (Component, error)

func DefaultRegistry() *Registry  // 注册全部 10 个组件工厂
```

启动时调用 `DefaultRegistry()` 获取包含全部工厂的注册表。后续新增组件只需加工厂函数 + 调 Register。

### 3. 10 个组件 struct

每个组件定义在独立文件中，字段与需求 0 Schema 精确一致：

| 文件 | struct | 实现 Tickable | Tick 最小行为 |
|------|--------|--------------|-------------|
| `identity.go` | IdentityComponent | 否 | — |
| `position.go` | PositionComponent | 否 | — |
| `behavior.go` | BehaviorComponent | 否（Scheduler 显式驱动） | — |
| `perception.go` | PerceptionComponent | 否 | — |
| `movement.go` | MovementComponent | 是 | 写 BB `move_state = "idle"` |
| `personality.go` | PersonalityComponent | 否 | — |
| `needs.go` | NeedsComponent | 是 | 每项 current -= decay_rate * dt，写 BB need_lowest/need_lowest_val |
| `emotion.go` | EmotionComponent | 是 | 每项 value -= decay_rate * dt（clamp 0），写 BB emotion_dominant/emotion_dominant_val |
| `memory.go` | MemoryComponent | 是 | 本 spec 无条目可清理，只写 BB memory_count=0 |
| `social.go` | SocialComponent | 否 | — |

BehaviorComponent 特殊处理：

```go
type BehaviorComponent struct {
    FSMRef string            `json:"fsm_ref"`
    BTRefs map[string]string `json:"bt_refs"`
    // 运行时（工厂中不填，Instance 初始化时构建）
    FSM    *fsm.FSM           `json:"-"`
    BTrees map[string]bt.Node  `json:"-"`
}
```

BehaviorComponent 的工厂只解析 JSON 填 FSMRef/BTRefs，FSM 和 BTrees 在 `NewInstanceFromTemplate` 中从配置源加载并构建。

### 4. NPC Instance 重构

```go
type Instance struct {
    ID         string
    Name       string                     // 从 identity 提取，便于日志
    BB         *blackboard.Blackboard
    components map[string]component.Component
    tickables  []component.Tickable       // 创建时缓存，避免每 Tick 类型断言
}

func (inst *Instance) HasComponent(name string) bool
func (inst *Instance) RawComponent(name string) (component.Component, bool)
func (inst *Instance) TickComponents(dt float64)  // 遍历 tickables 调用 Tick
func (inst *Instance) Position() event.Vec3        // 从 position 组件读取

func GetComponent[T component.Component](inst *Instance, name string) (T, bool)
```

**Position() 辅助方法**：Scheduler 和 broadcastLoop 需要频繁读 NPC 坐标。从 position 组件读取并返回 `event.Vec3`，避免每处手写类型断言。

**tickables 缓存**：创建时遍历 components，对实现 Tickable 的组件收集到 slice。运行时只遍历 slice，无类型断言开销。

### 5. NPC 创建流程

```go
// TemplateConfig 组件化 NPC 模板配置
type TemplateConfig struct {
    Name       string                         `json:"name"`
    Preset     string                         `json:"preset"`
    Components map[string]json.RawMessage      `json:"components"`
}

func NewInstanceFromTemplate(
    id string,
    pos event.Vec3,
    tmpl *TemplateConfig,
    compReg *component.Registry,
    src config.Source,
    btReg *bt.Registry,
) (*Instance, error)
```

流程：
1. 校验 identity 和 position 必须存在
2. 遍历 `tmpl.Components`，调用 `compReg.Create(name, raw)` 创建每个组件
3. 如果有 behavior 组件 → 加载 FSM 配置 + 构建 BT 树，填入 BehaviorComponent
4. 从 position 组件覆盖坐标（spawn 时传入的 pos 优先）
5. 创建 BB，设置初始 Key
6. 收集 Tickable 组件到缓存
7. 返回 Instance

### 6. v2 旧格式兼容

```go
func ParseNPCTemplate(data []byte, compReg *component.Registry) (*TemplateConfig, error) {
    var probe struct {
        Components json.RawMessage `json:"components"`
        TypeName   string          `json:"type_name"`
    }
    json.Unmarshal(data, &probe)

    if len(probe.Components) > 0 {
        return parseNewFormat(data)
    }
    if probe.TypeName != "" {
        return convertV2Format(data)
    }
    return nil, errors.New("unrecognized NPC config format")
}
```

convertV2Format 将 v2 的 `NPCTypeConfig` 转为 `TemplateConfig`：
- `type_name` → `name`
- `preset` = `"reactive"`
- `fsm_ref` + `bt_refs` → behavior 组件
- `perception` → perception 组件
- 自动添加 identity（name=type_name, model_id=type_name）和 position（零值）

### 7. Scheduler 适配

```go
func (s *Scheduler) Tick(dt float64) {
    s.EventBus.Tick(dt)
    activeEvents := s.EventBus.Active()
    now := time.Now().UnixMilli()

    s.Registry.ForEach(func(inst *npc.Instance) {
        blackboard.Set(inst.BB, blackboard.KeyCurrentTime, now)

        // AI 管线：仅有对应组件的 NPC 执行
        var perceived []*event.Event
        if perc, ok := npc.GetComponent[*component.PerceptionComponent](inst, "perception"); ok {
            perceived = s.filterPerception(inst, perc, activeEvents)
        }

        if beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior"); ok {
            s.Decision.Evaluate(inst.BB, inst.Position(), perceived, s.EvtTypes, dt)
            beh.FSM.Tick(inst.BB)
            state := beh.FSM.Current()
            if tree, ok := beh.BTrees[state]; ok {
                tree.Tick(&bt.Context{BB: inst.BB})
            }
        }

        // 通用组件 Tick
        inst.TickComponents(dt)
    })
}

func (s *Scheduler) filterPerception(
    inst *npc.Instance,
    perc *component.PerceptionComponent,
    events []*event.Event,
) []*event.Event {
    cfg := &perception.PerceptionConfig{
        VisualRange:   perc.VisualRange,
        AuditoryRange: perc.AuditoryRange,
    }
    perceived := make([]*event.Event, 0, len(events))
    for _, evt := range events {
        typeCfg, ok := s.EvtTypes[evt.Type]
        if !ok {
            continue
        }
        if perception.CanPerceive(inst.Position(), cfg, evt, typeCfg) {
            perceived = append(perceived, evt)
        }
    }
    return perceived
}
```

**关键**：`perception.CanPerceive` 的签名不变，Scheduler 从 PerceptionComponent 提取字段构造 PerceptionConfig 传入。perception 包本身不改动。

### 8. Config Source 扩展

```go
type Source interface {
    // 已有（全部保留）
    LoadFSMConfig(npcType string) (*fsm.FSMConfig, error)
    LoadBTTree(treeName string) ([]byte, error)
    LoadEventConfig(eventType string) ([]byte, error)
    LoadAllEventConfigs() (map[string][]byte, error)
    LoadNPCTypeConfig(npcType string) ([]byte, error)

    // 新增
    LoadNPCTemplate(name string) ([]byte, error)
}
```

JSONSource：`LoadNPCTemplate("wolf")` → 读 `configs/npc_templates/wolf.json`
HTTPSource：启动时额外拉取 `/api/v1/npc-templates/export`

### 9. Gateway Handler 适配

spawn_npc handler：

```go
// 1. 先尝试新格式
raw, err := src.LoadNPCTemplate(req.TypeName)
if err != nil {
    // 2. 降级旧格式
    raw, err = src.LoadNPCTypeConfig(req.TypeName)
    if err != nil { /* 报错 */ }
}
// 3. 统一解析
tmpl, err := npc.ParseNPCTemplate(raw, compReg)
// 4. 创建实例
inst, err := npc.NewInstanceFromTemplate(req.NpcID, pos, tmpl, compReg, src, btReg)
```

query_npc / buildSnapshot：

```go
// 从组件安全读取，无组件返回零值
pos := inst.Position()
fsmState := ""
if beh, ok := npc.GetComponent[*component.BehaviorComponent](inst, "behavior"); ok {
    fsmState = beh.FSM.Current()
}
```

### 10. BB Key 新增

在 `keys.go` 追加：

```go
var KeyNeedLowest       = NewKey[string]("need_lowest")
var KeyNeedLowestVal    = NewKey[float64]("need_lowest_val")
var KeyEmotionDominant  = NewKey[string]("emotion_dominant")
var KeyEmotionDominantVal = NewKey[float64]("emotion_dominant_val")
var KeyMemoryCount      = NewKey[int64]("memory_count")
var KeyGroupID          = NewKey[string]("group_id")
var KeySocialRole       = NewKey[string]("social_role")
var KeyMoveState        = NewKey[string]("move_state")
```

---

## 方案对比

### 备选方案：扁平 struct + 可选指针字段

```go
type Instance struct {
    // ... 现有字段 ...
    Movement    *MovementConfig    // nil = 无移动
    Needs       *NeedsConfig       // nil = 无需求
    // ... 每加一个能力加一个字段
}
```

Scheduler 通过 nil 检查决定是否执行。

**不选的理由**：
1. 每加新能力必须改 Instance struct → 违反开闭原则
2. Scheduler 的 nil 检查线性增长，每个新能力加一个 if
3. 不支持自定义组件——只有硬编码在 struct 中的才存在
4. 不服务于第四扩展轴"加能力不改代码"
5. ADMIN 表单无法泛化——仍需 per-field 渲染逻辑

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码 NPC 类型参数 | **不违反** | 所有参数在组件 JSON 配置中 |
| 禁止 switch-case 做 NPC 类型分发 | **不违反** | 组件注册表 + 工厂模式 |
| 禁止 BT 反向驱动 FSM | **不违反** | FSM → BT 方向不变 |
| 禁止 core/ import runtime/ | **不违反** | component/ 在 runtime/ 下，依赖 core/ 向下 |
| 禁止 Blackboard 裸 map | **不违反** | 新 Key 通过 BBKey[T] 注册 |
| 禁止 Key 散落各文件 | **不违反** | 8 个新 Key 集中在 keys.go |
| 禁止 Gateway 承担非网络职责 | **不违反** | 组件 Tick 在 Scheduler 中 |
| 禁止静默降级 | **不违反** | 旧格式转换记日志，未知组件名报错 |
| 禁止配置错误延迟暴露 | **不违反** | 创建时校验 identity/position 必须存在，behavior 引用的 FSM/BT 不存在立即报错 |
| 禁止过度设计 | **不违反** | 不引入 ECS 框架，Component 接口只有 1 个方法 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | 中性 | 事件系统不变 |
| 加 NPC 类型 | **正面** | 组件自由组合，更灵活 |
| NPC 间交互 | **正面** | social 组件数据结构就位 |
| 加 NPC 能力（新） | **强正面** | 实现 Component + 注册工厂，不改现有代码 |

---

## 依赖方向

```
cmd/server/main.go
  → internal/gateway/
  → internal/runtime/              (Scheduler)
  → internal/runtime/component/    (Registry, 10 组件)
  → internal/runtime/npc/          (Instance, NPC Registry)
  → internal/config/

internal/runtime/component/
  → internal/core/blackboard/      (BBKey 引用)
  → internal/core/fsm/             (BehaviorComponent.FSM)
  → internal/core/bt/              (BehaviorComponent.BTrees)

internal/runtime/npc/
  → internal/runtime/component/
  → internal/core/blackboard/

internal/runtime/ (Scheduler)
  → internal/runtime/npc/
  → internal/runtime/component/
  → internal/runtime/event/        (不变)
  → internal/runtime/decision/     (不变)
  → internal/runtime/perception/   (不变)

internal/runtime/perception/       (不改动)
  → internal/runtime/event/
```

单向向下，无循环。component/ 依赖 core/（向下），被 npc/ 和 Scheduler 依赖（向上引用类型）。perception/ 包不感知 component/ 的存在——Scheduler 负责桥接。

---

## 并发安全

| 共享状态 | 读者 | 写者 | 保护方式 |
|---------|------|------|---------|
| Instance.components | Scheduler(Tick) | 创建时一次性写入 | 创建后只读，无并发写 |
| Instance.BB | Scheduler 内顺序执行 | 同左 | BB 自带 RWMutex |
| component.Registry | Scheduler 间接使用 | main 启动时注册 | 启动后只读 |
| npc.Registry | Scheduler(ForEach) | Gateway(Add/Remove) | 已有 RWMutex，不变 |
| EventBus | Scheduler + Gateway | 同左 | 已有 RWMutex，不变 |

**新增风险**：无。组件在创建时装配完毕，运行时不增删。Tickable 组件的 Tick 在同一 NPC 的 ForEach 回调内顺序执行，无并发。

---

## 配置变更

### 新增

| 路径 | 说明 |
|------|------|
| `configs/npc_templates/butterfly_01.json` | simple 级示例 |
| `configs/npc_templates/wolf_common.json` | reactive 级示例 |

### 保留（不修改）

- `configs/npc_types/*.json`（v2 兼容）
- `configs/fsm/*.json`
- `configs/bt_trees/**/*.json`
- `configs/events/*.json`

---

## 测试策略

### 单元测试

| 模块 | 覆盖 |
|------|------|
| `component/registry_test.go` | Register/Create/重复注册/未知组件 |
| `component/*_test.go` | 每个组件 JSON 解析、Name() 返回值、Tickable Tick 行为 |
| `npc/instance_test.go` | NewInstanceFromTemplate、HasComponent、GetComponent、TickComponents |
| `npc/compat_test.go` | v2 格式检测、转换、转换后字段正确性 |
| `config/json_source_test.go` | LoadNPCTemplate 新方法 |

### 集成测试

| 场景 | 验证 |
|------|------|
| simple NPC Tick | 不走 AI 管线，只执行 movement Tick |
| reactive NPC 完整流程 | perception → decision → FSM → BT + movement Tick |
| v2 兼容 | civilian/guard/police 旧配置加载后 6 个集成场景全部通过 |
| 混合 Tick | 同时 simple + reactive NPC，各自只执行对应组件 |

### e2e 测试

现有 e2e 测试（spawn_npc/remove_npc/publish_event/query_npc）不修改，确认通过。

### Benchmark

| 测试 | 目标 |
|------|------|
| BenchmarkTick_SimpleNPC | identity+position+movement，单 Tick < 1μs |
| BenchmarkTick_FullNPC | 全 10 组件，单 Tick < 100μs |
