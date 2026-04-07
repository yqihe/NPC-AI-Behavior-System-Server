# zone-system 设计方案

## 方案描述

### 1. Zone 结构体

```go
// internal/runtime/zone/zone.go
type SpawnEntry struct {
    TemplateRef    string     `json:"template_ref"`
    Count          int        `json:"count"`
    SpawnPoints    []Position `json:"spawn_points"`
    WanderRadius   float64    `json:"wander_radius"`
    RespawnSeconds float64    `json:"respawn_seconds"`
}

type Position struct {
    X float64 `json:"x"`
    Z float64 `json:"z"`
}

type Zone struct {
    ID         string       `json:"region_id"`
    Name       string       `json:"name"`
    RegionType string       `json:"region_type"`
    SpawnTable []SpawnEntry `json:"spawn_table"`
    Active     bool         // true=活跃, false=休眠
    npcs       []string     // 该区域的 NPC ID 列表
}
```

### 2. ZoneManager

```go
// internal/runtime/zone/manager.go
type ZoneManager struct {
    mu    sync.RWMutex
    zones map[string]*Zone // region_id → Zone
}

func NewZoneManager() *ZoneManager
func (zm *ZoneManager) AddZone(z *Zone)
func (zm *ZoneManager) GetZone(id string) (*Zone, bool)
func (zm *ZoneManager) IsActive(zoneID string) bool          // 空 zoneID 返回 true（向后兼容）
func (zm *ZoneManager) Sleep(zoneID string)
func (zm *ZoneManager) Wake(zoneID string)
func (zm *ZoneManager) AllZones() []*Zone
```

`IsActive` 是 Scheduler 的核心判断：
```go
func (zm *ZoneManager) IsActive(zoneID string) bool {
    if zoneID == "" {
        return true // 无区域的 NPC 始终活跃（v2 兼容）
    }
    zm.mu.RLock()
    defer zm.mu.RUnlock()
    z, ok := zm.zones[zoneID]
    if !ok {
        return true // 未注册的区域默认活跃
    }
    return z.Active
}
```

### 3. NPC 批量生成

```go
func (z *Zone) Spawn(compReg *component.Registry, src config.Source, btReg *bt.Registry, npcReg *npc.Registry, gm *social.GroupManager) error {
    for _, entry := range z.SpawnTable {
        for i := 0; i < entry.Count; i++ {
            // 循环分配 spawn point
            sp := entry.SpawnPoints[i % len(entry.SpawnPoints)]
            pos := event.Vec3{X: sp.X, Z: sp.Z}

            // 加载模板
            raw, err := src.LoadNPCTemplate(entry.TemplateRef)
            // fallback 旧格式
            if err != nil { raw, err = src.LoadNPCTypeConfig(entry.TemplateRef) }
            tmpl, _ := npc.ParseNPCTemplate(raw)

            // 设置 zone_id
            // ... 注入到 position 组件

            id := fmt.Sprintf("%s_%s_%d", z.ID, entry.TemplateRef, i)
            inst, _ := npc.NewInstanceFromTemplate(id, pos, tmpl, compReg, src, btReg)

            npcReg.Add(inst)
            if gm != nil { gm.Register(inst) }
            z.npcs = append(z.npcs, id)
        }
    }
    return nil
}
```

### 4. Scheduler 按区域跳过休眠

在第一遍感知遍历中，跳过休眠区域的 NPC：

```go
s.Registry.ForEach(func(inst *npc.Instance) {
    // 检查区域是否活跃
    if s.ZoneManager != nil {
        zoneID := ""
        if pos, ok := npc.GetComponent[*component.PositionComponent](inst, "position"); ok {
            zoneID = pos.ZoneID
        }
        if !s.ZoneManager.IsActive(zoneID) {
            return // 跳过休眠区域的 NPC
        }
    }
    // ... 正常感知逻辑
})
```

同样在第二遍决策遍历中跳过。

### 5. 区域级广播

Gateway Conn 新增 `ZoneID string` 字段：

```go
type Conn struct {
    // ... 现有字段
    ZoneID string // 客户端所在区域，空=全局
}
```

新增 WS 消息类型：
```go
const TypeEnterZone = "enter_zone"   // {zone_id: "meadow"}
const TypeLeaveZone = "leave_zone"   // {}
```

`enter_zone` handler 设置 `conn.ZoneID`，`leave_zone` 清空。

broadcastLoop 改为按区域分组广播：

```go
func broadcastLoop(...) {
    // 构建 zone → []NPCState 映射
    zoneSnapshots := map[string][]protocol.NPCState{}
    reg.ForEach(func(inst *npc.Instance) {
        zoneID := getZoneID(inst)
        // ... 构建 NPCState
        zoneSnapshots[zoneID] = append(zoneSnapshots[zoneID], state)
    })

    // 每个连接只发其 ZoneID 对应的 NPC
    hub.BroadcastByZone(zoneSnapshots)
}
```

Hub 新增 `BroadcastByZone`：遍历连接，按 conn.ZoneID 筛选发送。conn.ZoneID 为空时发全部（v2 兼容）。

### 6. Config Source 扩展

```go
// Source 接口新增
LoadRegionConfig(regionID string) ([]byte, error)
LoadAllRegionConfigs() (map[string][]byte, error)
```

JSONSource：从 `configs/regions/<regionID>.json` 加载。
HTTPSource：从 `/api/v1/regions/export` 拉取（optional）。

### 7. 启动流程

```go
// main.go
zm := zone.NewZoneManager()
regionConfigs, _ := src.LoadAllRegionConfigs()
for id, data := range regionConfigs {
    var z zone.Zone
    json.Unmarshal(data, &z)
    z.Active = true // 启动时全部活跃
    zm.AddZone(&z)
    z.Spawn(compReg, src, btReg, reg, gm)
}
sched.ZoneManager = zm
```

---

## 方案对比

### 备选方案：每个区域独立 Scheduler（不选）

每个 Zone 有自己的 Scheduler + EventBus + NPC Registry，完全隔离。

**不选的理由**：
1. 跨区域群组感知共享无法实现（需求 6 的 GroupManager 跨区域）
2. 事件总线隔离后，global 事件需要额外的跨 Scheduler 转发机制
3. 实现复杂度高，需要多 goroutine 协调
4. 当前规模（千级 NPC）单 Scheduler + 区域跳过已足够

选定方案：单 Scheduler + ZoneManager.IsActive 跳过休眠区域，简单高效。

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| 禁止硬编码 NPC 参数 | **不违反** | 区域配置从 JSON 加载 |
| 禁止 core/ import runtime/ | **不违反** | zone 在 runtime/ 下 |
| 禁止 Gateway 承担非网络职责 | **不违反** | Gateway 只做广播过滤，区域逻辑在 ZoneManager |
| 禁止静默降级 | **不违反** | 无区域配置时保持全局行为（日志记录） |
| 禁止过度设计 | **不违反** | 不做独立 Scheduler，不做区域进程 |

---

## 扩展轴影响

| 扩展轴 | 影响 | 说明 |
|--------|------|------|
| 加事件源 | **正面** | 事件自动归属区域 |
| 加 NPC 类型 | **正面** | spawn_table 引用模板名 |
| NPC 间交互 | **中性** | 群组不受区域影响 |

---

## 依赖方向

```
cmd/server/main.go
  → internal/runtime/zone/        (ZoneManager)
  → internal/gateway/             (BroadcastByZone)

internal/runtime/zone/
  → internal/runtime/npc/         (Registry, Instance)
  → internal/runtime/component/   (PositionComponent)
  → internal/config/              (Source)

internal/runtime/ (Scheduler)
  → internal/runtime/zone/        (IsActive)
```

单向向下，无循环。

---

## 并发安全

ZoneManager.zones 由 Scheduler 读（IsActive）、Gateway 写（Sleep/Wake 如果由客户端触发）。RWMutex 保护。Zone.Active 布尔值读写通过 ZoneManager 的锁保护。

---

## 配置变更

新增 `configs/regions/meadow.json` 示例配置，格式与需求 0 的 region.json Schema 一致。

---

## 测试策略

### 单元测试

| 模块 | 覆盖 |
|------|------|
| `zone/` | AddZone/GetZone/IsActive/Sleep/Wake |
| `zone/` | Spawn 批量创建 NPC + zone_id 设置 |
| `zone/` | 休眠区域 IsActive=false |

### 集成测试

| 场景 | 验证 |
|------|------|
| 休眠跳过 | 休眠区域 NPC 不 Tick（threat_level 不变） |
| 唤醒恢复 | 唤醒后 NPC 恢复 Tick |
| 批量生成 | spawn_table 按 count 创建，zone_id 正确 |
