# V3 改造路线图

> 基于 2026-04-07 与 ADMIN 平台确认的接口契约，将服务端从 v2 适配到 v3 ADMIN API。

## 依赖关系

```
需求1 (BB Key 动态注册)
  └─→ 需求3 (NPC Template)
        ├─→ 需求4 (Perception 重构)
        └─→ 需求5 (Source 接口 + HTTPSource)
              └─→ 需求6 (Region 区域系统)

需求2 (BT 扁平格式)        ← 独立，可与需求1 并行
需求7 (EventType 扩展字段) ← 独立，可与需求1/2 并行
```

---

## 需求 1：Blackboard Key 动态注册

**背景**：v2 所有 BB Key 必须在 `blackboard/keys.go` 编译期通过 `NewKey[T]()` 注册。v3 中 NPC 的 fields（hp、attack、visual_range 等）由策划在 ADMIN 动态配置，不可能每加一个字段就改代码。

**目标**：支持 NPC 创建时动态注册 field Key，同时保留运行时 Key 的编译期注册和安全校验。

### 改动范围

| 文件 | 改动 |
|------|------|
| `internal/core/blackboard/blackboard.go` | 新增 `RegisterDynamic(name, typeName)`，跳过重复注册（幂等） |
| `internal/core/blackboard/blackboard.go` | 新增 `SetDynamic(bb, name, val)`，写入前自动注册 |
| `internal/core/blackboard/keys.go` | 不动，运行时 Key 保持编译期注册 |

### 详细设计

```go
// RegisterDynamic 动态注册 Key（NPC fields 用）。
// 与 NewKey 不同：同名重复注册不 panic，而是静默跳过（幂等）。
// 原因：多个同类 NPC 共享相同的 field key。
func RegisterDynamic(name string, typeName string) {
    registryMu.Lock()
    defer registryMu.Unlock()
    if _, ok := registry[name]; ok {
        return // 幂等：已注册则跳过
    }
    registry[name] = keyInfo{name: name, typeName: typeName}
}

// SetDynamic 写入动态 field 值，自动注册。
// 用于 NPC 创建时遍历 fields 写入 BB。
func SetDynamic(bb *Blackboard, name string, val any) {
    typeName := reflect.TypeOf(val).String()
    RegisterDynamic(name, typeName)
    bb.mu.Lock()
    defer bb.mu.Unlock()
    bb.data[name] = val
}
```

### 验收标准

- [ ] `RegisterDynamic` 同名重复调用不 panic
- [ ] `SetDynamic` 写入后，`GetRaw` 能读到值
- [ ] `SetDynamic` 写入后，`ValidateKeyName` 返回 nil（已注册）
- [ ] `SetRaw` 对未注册 Key 仍然 panic（安全网不变）
- [ ] 编译期 `NewKey[T]` 注册的 Key 不受影响
- [ ] 单元测试覆盖以上场景

---

## 需求 2：BT Builder 适配扁平格式

**背景**：v2 BT 叶子节点参数包在 `params` 子对象里；v3 ADMIN 编辑器出的格式是扁平的，参数直接在节点对象上。

**v2 格式**（废弃）：
```json
{"type": "check_bb_float", "params": {"key": "threat_level", "op": ">=", "value": 50}}
```

**v3 格式**（目标）：
```json
{"type": "check_bb_float", "key": "threat_level", "op": ">=", "value": 50}
```

### 改动范围

| 文件 | 改动 |
|------|------|
| `internal/core/bt/builder.go` | `TreeConfig` 去掉 `Params` 字段；factory 接收整个节点 raw JSON |
| `internal/core/bt/registry.go` | `NodeFactory` 签名不变（仍接收 `json.RawMessage`），但语义变为整个节点 JSON |
| `internal/core/bt/leaves.go` | 各 factory 不用改（已经从 JSON 中解析 key/op/value） |
| `testdata/configs/bt_trees/**/*.json` | 所有测试配置改为扁平格式 |

### 详细设计

`TreeConfig` 改为只保留结构字段，额外持有原始 JSON：

```go
type TreeConfig struct {
    Type     string       `json:"type"`
    Children []TreeConfig `json:"children,omitempty"`
    Child    *TreeConfig  `json:"child,omitempty"`
}
```

`Build` 函数改为将整个节点的 raw JSON 传给 factory（factory 自行解析需要的字段，忽略 type/children/child）：

```go
func BuildFromJSON(data []byte, reg *Registry) (Node, error) {
    var cfg TreeConfig
    if err := json.Unmarshal(data, &cfg); err != nil { ... }
    return buildNode(data, &cfg, reg)  // data = 整个节点的 raw JSON
}

func buildNode(raw json.RawMessage, cfg *TreeConfig, reg *Registry) (Node, error) {
    factory, _ := reg.Get(cfg.Type)
    node, _ := factory(raw)  // factory 收到完整节点 JSON，自行取 key/op/value 等
    // 填充 children / child ...
}
```

composite 节点（sequence/selector/parallel）的 factory 收到的 JSON 里虽然有 children，但它们不读 children（builder 负责递归填充），所以无影响。

### 验收标准

- [ ] v3 扁平格式 JSON 能正确构建所有节点类型
- [ ] composite 节点（sequence/selector/parallel）+ decorator（inverter）正常工作
- [ ] 所有 leaf 节点（check_bb_float/check_bb_string/set_bb_value/stub_action）正常解析
- [ ] `testdata/configs/bt_trees/` 全部改为扁平格式
- [ ] 现有 BT 相关测试全部通过

---

## 需求 3：NPC Template 结构改造

**背景**：v2 的 `NPCTypeConfig` 结构是 `{type_name, fsm_ref, bt_refs, perception}`。v3 ADMIN 出的格式变为 `{template_ref, fields, behavior: {fsm_ref, bt_refs}}`。fields 是扁平 key-value，包含了所有 NPC 属性。

### 改动范围

| 文件 | 改动 |
|------|------|
| `internal/runtime/npc/instance.go` | `NPCTypeConfig` 结构体重写；`NewInstance` 遍历 fields 写入 BB |
| `internal/runtime/npc/instance.go` | `ParseNPCTypeConfig` 适配新 JSON 结构 |
| `internal/runtime/npc/instance.go` | `Instance` 去掉 `Perception *PerceptionConfig` 字段 |
| `internal/gateway/handler.go` | SpawnNPC handler 适配（如果接口变化） |
| `testdata/configs/npc_types/*.json` | 重命名目录为 `npc_templates/`，改为 v3 格式 |

### 详细设计

```go
// NPCTypeConfig v3 NPC 模板配置
type NPCTypeConfig struct {
    TemplateRef string                 `json:"template_ref"` // ADMIN 内部用，可忽略
    Fields      map[string]any         `json:"fields"`       // 扁平 key-value
    Behavior    NPCBehaviorConfig      `json:"behavior"`
}

type NPCBehaviorConfig struct {
    FSMRef string            `json:"fsm_ref"`
    BTRefs map[string]string `json:"bt_refs"` // FSM 状态名 → BT 树名
}
```

`NewInstance` 关键改动：

```go
func NewInstance(id string, pos event.Vec3, typeCfg *NPCTypeConfig, src config.Source, btReg *bt.Registry) (*Instance, error) {
    bb := blackboard.New()

    // 1. 遍历 fields，动态注册并写入 BB
    for k, v := range typeCfg.Fields {
        blackboard.SetDynamic(bb, k, v)
    }

    // 2. 写入运行时 Key（编译期注册的）
    blackboard.Set(bb, blackboard.KeyNPCType, typeCfg.Fields["display_name"].(string)) // 或用 name
    blackboard.Set(bb, blackboard.KeyNPCPosX, pos.X)
    blackboard.Set(bb, blackboard.KeyNPCPosZ, pos.Z)
    blackboard.Set(bb, blackboard.KeyCurrentTime, int64(0))

    // 3. 加载 FSM（通过 behavior.fsm_ref）
    fsmCfg, err := src.LoadFSMConfig(typeCfg.Behavior.FSMRef)
    ...

    // 4. 加载 BT（通过 behavior.bt_refs）
    for state, treeName := range typeCfg.Behavior.BTRefs {
        ...
    }
}
```

### 验收标准

- [ ] v3 格式 NPC template JSON 能正确解析
- [ ] fields 全部写入 BB（hp、attack、visual_range、auditory_range 等）
- [ ] `behavior.fsm_ref` 正确关联 FSM 配置
- [ ] `behavior.bt_refs` 正确关联各状态的 BT
- [ ] SpawnNPC handler 正常工作
- [ ] `template_ref` 不影响运行逻辑

---

## 需求 4：Perception 感知重构

**背景**：v2 感知配置是 NPC 上的独立子对象 `PerceptionConfig{VisualRange, AuditoryRange}`。v3 把 visual_range 和 auditory_range 作为普通 field 存在 BB 里，不再单独建模。

### 改动范围

| 文件 | 改动 |
|------|------|
| `internal/runtime/perception/perception.go` | `CanPerceive` 签名改为从 BB 读取范围值 |
| `internal/runtime/perception/perception.go` | 删除 `PerceptionConfig` 结构体 |
| `internal/runtime/npc/instance.go` | `Instance` 去掉 `Perception` 字段 |
| `internal/runtime/scheduler.go` | 调用 `CanPerceive` 的地方适配新签名 |
| `internal/runtime/perception/perception_test.go` | 适配新签名 |

### 详细设计

```go
// CanPerceive 判断 NPC 是否能感知到事件
// visual_range / auditory_range 从 BB 读取（原来的 PerceptionConfig 字段）
func CanPerceive(npcPos event.Vec3, bb *blackboard.Blackboard, evt *event.Event, evtTypeCfg *event.EventTypeConfig) bool {
    switch evtTypeCfg.PerceptionMode {
    case "global":
        return true
    case "visual":
        visualRange, _ := bb.GetRaw("visual_range")
        r, _ := toFloat64(visualRange)
        dist := event.Distance(npcPos, evt.Position)
        return dist <= min(r, evtTypeCfg.Range)
    case "auditory":
        auditoryRange, _ := bb.GetRaw("auditory_range")
        r, _ := toFloat64(auditoryRange)
        dist := event.Distance(npcPos, evt.Position)
        return dist <= min(r, evtTypeCfg.Range)
    default:
        return false
    }
}
```

### 验收标准

- [ ] `PerceptionConfig` 结构体删除
- [ ] `CanPerceive` 从 BB 读取 visual_range / auditory_range
- [ ] BB 中无 visual_range 时，该感知模式返回 false（安全默认）
- [ ] Scheduler 调用处适配
- [ ] 感知测试全部通过

---

## 需求 5：Source 接口 + HTTPSource 适配 v3 API

**背景**：v3 API 端点有变化（`npc_types` → `npc_templates`），新增 `regions` 端点，Source 接口方法需要更新。

**适配优先级**（2026-04-11 与 ADMIN 对齐，按 ADMIN 侧开发进度排）：

1. `/api/configs/event_types` — ADMIN 本期开工，最先到位（配合需求 7 联调）
2. `/api/configs/npc_templates` — ADMIN 字段/模板基础已完成，管理页骨架在做
3. `/api/configs/fsm_configs` — ADMIN 尚未启动
4. `/api/configs/bt_trees` — ADMIN 尚未启动
5. `/api/configs/regions` — ADMIN 尚未启动

**策略**：分端点单独适配 + 单独联调，不等 5 个全到位才开工。字段对不上时停下来找 ADMIN 对契约，不自己凑数据结构。

### 改动范围

| 文件 | 改动 |
|------|------|
| `internal/config/source.go` | 接口方法重命名 + 新增 regions |
| `internal/config/http_source.go` | 端点 URL 更新，新增 regions map |
| `internal/config/test_source.go` | 适配新接口 |
| `internal/config/http_source_test.go` | 适配新接口 |

### 详细设计

```go
// Source v3 配置数据源接口
type Source interface {
    LoadFSMConfig(name string) (*fsm.FSMConfig, error)
    LoadBTTree(name string) ([]byte, error)
    LoadEventConfig(name string) ([]byte, error)
    LoadAllEventConfigs() (map[string][]byte, error)
    LoadNPCTemplate(name string) ([]byte, error)       // 重命名：LoadNPCTypeConfig → LoadNPCTemplate
    LoadAllNPCTemplates() (map[string][]byte, error)    // 新增：Region spawn 需要
    LoadAllRegionConfigs() (map[string][]byte, error)   // 新增：Region 系统
}
```

HTTPSource 端点更新：

```go
endpoints := []struct{...}{
    {"/api/configs/event_types", s.eventTypes},
    {"/api/configs/npc_templates", s.npcTemplates},  // 改
    {"/api/configs/fsm_configs", s.fsmConfigs},
    {"/api/configs/bt_trees", s.btTrees},
    {"/api/configs/regions", s.regions},              // 新增
}
```

### 验收标准

- [ ] 端点 `/api/configs/npc_templates` 替换旧的 `/api/configs/npc_types`
- [ ] 新增 `/api/configs/regions` 拉取
- [ ] `Source` 接口所有实现（HTTPSource、test_source）同步更新
- [ ] `LoadAllNPCTemplates` 返回全部模板（供 Region spawn 用）
- [ ] regions 允许为空（`{"items": []}`），不阻塞启动
- [ ] HTTPSource 测试通过

---

## 需求 6：Region 区域系统

**背景**：v3 新增区域概念，包含边界、天气、NPC 刷新表。服务端需要根据 spawn_table 自动生成 NPC，并做边界校验。

### 改动范围

| 文件 | 改动 |
|------|------|
| `internal/runtime/region/region.go` | 新增 package：Region 数据结构 + spawn 逻辑 |
| `internal/runtime/region/boundary.go` | 多边形边界检测（点是否在多边形内） |
| `cmd/server/main.go` | 启动时加载 regions，执行初始 spawn |
| `internal/runtime/npc/instance.go` | `NewInstance` 支持从 spawn_table 的 spawn_points 取初始位置 |
| `pkg/protocol/messages.go` | 如需推送区域信息给客户端 |

### 详细设计

```go
// RegionConfig 区域配置
type RegionConfig struct {
    DisplayName string          `json:"display_name"`
    RegionType  string          `json:"region_type"`
    Boundary    BoundaryConfig  `json:"boundary"`
    Weather     WeatherConfig   `json:"weather"`
    SpawnTable  []SpawnEntry    `json:"spawn_table"`
    Properties  json.RawMessage `json:"properties"` // 透传
}

type SpawnEntry struct {
    TemplateRef    string       `json:"template_ref"`  // → npc_templates 的 name
    Count          int          `json:"count"`
    SpawnPoints    []Vec2       `json:"spawn_points"`
    WanderRadius   float64      `json:"wander_radius"`
    RespawnSeconds int          `json:"respawn_seconds"`
}

type BoundaryConfig struct {
    Type   string `json:"type"` // "polygon"
    Points []Vec2 `json:"points"`
}
```

启动流程：

```
main.go:
  regions := loadAllRegions(src)
  for _, region := range regions {
      for _, entry := range region.SpawnTable {
          template := src.LoadNPCTemplate(entry.TemplateRef)
          for i := 0; i < entry.Count; i++ {
              pos := pickSpawnPoint(entry.SpawnPoints, i)
              inst := npc.NewInstance(genID(), pos, template, src, btReg)
              registry.Add(inst)
          }
      }
  }
```

### 验收标准

- [ ] 启动时根据 regions 的 spawn_table 自动生成 NPC
- [ ] spawn_points 正确分配初始位置
- [ ] wander_radius 写入 NPC 的 BB（已在 fields 里）
- [ ] 多边形边界检测函数正确（射线法）
- [ ] regions 为空时不报错，正常启动（零 NPC）
- [ ] boundary / weather / properties 结构加载正确（weather 暂不处理逻辑，只加载）
- [ ] 集成测试：region spawn → NPC 存在于 registry → 客户端能查询到

---

## 需求 7：EventType 扩展字段支持

**背景**：ADMIN v3 把事件类型字段分成两类：
- **系统字段**（硬编码，语义不变）：`name` / `display_name` / `default_severity` / `default_ttl` / `perception_mode` / `range`
- **扩展字段**（运营 Schema 管理页自定义）：`priority` / `category` / `cooldown` / `stackable` 等

导出 API 返回的 `config` 对象扁平包含两类字段。当前 `json.Unmarshal` 按标准行为会丢弃未知 key，扩展字段进不来。

### 与 ADMIN 的契约（2026-04-11 确认）

1. **默认值**：集中定义在 `internal/runtime/event/defaults.go`，ADMIN UI 可展示当前默认
2. **类型错误策略**：加载成功（不 reject），**消费时退化到默认值 + warn 日志**。系统字段仍严格 reject
3. **ADMIN 侧义务**：Schema 管理页保存时必须按类型校验，脏数据到服务端视为 ADMIN bug
4. **访问方式**：主推访问器（`GetIntExt`/`GetStringExt`/`GetFloatExt`/`GetBoolExt`），raw map 作兜底
5. **未知字段**：透明保留在 Extensions map 中，不拒绝、不警告

### 改动范围

| 文件 | 改动 |
|------|------|
| `internal/runtime/event/event.go` | `EventTypeConfig` 新增 `Extensions map[string]json.RawMessage`；自定义 `UnmarshalJSON` |
| `internal/runtime/event/defaults.go` | 新建：扩展字段默认值表 |
| `internal/runtime/event/extensions.go` | 新建：`GetIntExt` / `GetStringExt` / `GetFloatExt` / `GetBoolExt` 访问器 |
| `internal/runtime/event/event_test.go` | 新增扩展字段测试 case |
| `testdata/configs/events/*.json` | 至少一个样例带扩展字段，用于 e2e 覆盖 |

### 详细设计

```go
// EventTypeConfig v3：系统字段 + 扩展字段
type EventTypeConfig struct {
    Name            string  `json:"name"`
    DisplayName     string  `json:"display_name"`
    DefaultSeverity float64 `json:"default_severity"`
    DefaultTTL      float64 `json:"default_ttl"`
    PerceptionMode  string  `json:"perception_mode"`
    Range           float64 `json:"range"`

    Extensions map[string]json.RawMessage `json:"-"`
}

func (c *EventTypeConfig) UnmarshalJSON(data []byte) error {
    // 两阶段解析：先全量到 map，再分拣
    var raw map[string]json.RawMessage
    if err := json.Unmarshal(data, &raw); err != nil {
        return err
    }

    // 用 type alias 避免递归
    type alias EventTypeConfig
    var sys alias
    if err := json.Unmarshal(data, &sys); err != nil {
        return err
    }
    *c = EventTypeConfig(sys)

    // 剩下的进 Extensions
    systemKeys := map[string]bool{
        "name": true, "display_name": true,
        "default_severity": true, "default_ttl": true,
        "perception_mode": true, "range": true,
    }
    c.Extensions = make(map[string]json.RawMessage)
    for k, v := range raw {
        if !systemKeys[k] {
            c.Extensions[k] = v
        }
    }
    return nil
}
```

访问器：

```go
// internal/runtime/event/extensions.go
func (c *EventTypeConfig) GetIntExt(key string) int {
    defaultVal, _ := extensionDefaults[key].(int)
    raw, ok := c.Extensions[key]
    if !ok {
        return defaultVal
    }
    var v int
    if err := json.Unmarshal(raw, &v); err != nil {
        slog.Warn("event.ext_type_mismatch",
            "event_type", c.Name,
            "field", key,
            "expected", "int",
            "raw_value", string(raw),
        )
        return defaultVal
    }
    return v
}
// GetStringExt / GetFloatExt / GetBoolExt 同理
```

### 验收标准

- [ ] 纯系统字段配置加载成功，行为与今天一致
- [ ] 带扩展字段（`priority: 5`）的配置加载成功，`GetIntExt("priority")` 返回 5
- [ ] 异构配置（gunshot 有 priority、earthquake 没有）同时加载成功，后者返回默认值
- [ ] 类型错误（`priority: "high"` 但代码按 int 消费）加载成功，消费时返回默认值并记 warn 日志
- [ ] 系统字段缺失（缺 `perception_mode`）仍然严格 reject 并定位到是哪条 event_type
- [ ] "ADMIN 禁用/软删字段"对服务端透明：未知 key 进 Extensions，不拒绝
- [ ] e2e 测试：混合数据投入 → 启动 → 运行时决策全链路稳定

---

## 执行顺序

| 阶段 | 需求 | 可并行 | 预计影响文件数 |
|------|------|--------|---------------|
| Phase 1 | 需求 1（BB Key 动态注册） | 与需求 2、7 并行 | 2 |
| Phase 1 | 需求 2（BT 扁平格式） | 与需求 1、7 并行 | 3 + testdata |
| Phase 1 | 需求 7（EventType 扩展字段） | 与需求 1、2 并行 | 3 新文件 + testdata |
| Phase 2 | 需求 3（NPC Template） | 需求 1 完成后 | 4 + testdata |
| Phase 2 | 需求 4（Perception 重构） | 需求 3 完成后 | 4 |
| Phase 3 | 需求 5（Source 接口） | 需求 3、4 完成后 | 5 |
| Phase 4 | 需求 6（Region 系统） | 需求 5 完成后 | 5+ (新 package) |
