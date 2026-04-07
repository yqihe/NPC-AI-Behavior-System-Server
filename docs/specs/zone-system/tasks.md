# zone-system 任务拆解

## T1: Zone struct + ZoneManager 核心 (R1, R2)

**文件**：
- `internal/runtime/zone/zone.go`
- `internal/runtime/zone/manager.go`
- `internal/runtime/zone/manager_test.go`

**做完了是什么样**：
- Zone struct（ID/Name/RegionType/SpawnTable/Active/npcs）
- ZoneManager（AddZone/GetZone/IsActive/Sleep/Wake/AllZones）+ RWMutex
- IsActive：空 zoneID → true（v2 兼容），未注册 → true，休眠 → false
- 测试覆盖：添加/查询/休眠/唤醒/空 ID 兼容

---

## T2: Config Source 扩展 (R3, R13, R14)

**文件**：
- `internal/config/source.go`
- `internal/config/json_source.go`
- `internal/config/http_source.go`

**做完了是什么样**：
- Source 接口新增 `LoadRegionConfig(regionID) ([]byte, error)` + `LoadAllRegionConfigs() (map[string][]byte, error)`
- JSONSource 从 `configs/regions/<id>.json` 加载
- HTTPSource 从 `/api/v1/regions/export`（optional）
- MongoSource 同步适配

---

## T3: Zone.Spawn 批量创建 NPC (R4, R5)

**文件**：
- `internal/runtime/zone/zone.go`（追加 Spawn 方法）
- `internal/runtime/zone/zone_test.go`

**做完了是什么样**：
- Spawn 方法从 spawn_table 加载模板 → 按 count 创建 → spawn_points 循环分配 → zone_id 注入
- NPC ID 格式 `{zoneID}_{templateRef}_{index}`
- 注册到 npcReg 和 gm
- 测试：按 count 创建、zone_id 正确、spawn_points 循环

---

## T4: 示例区域配置 (R3)

**文件**：
- `configs/regions/meadow.json`

**做完了是什么样**：
- 格式与需求 0 region.json Schema 一致
- spawn_table 引用 butterfly_01 模板
- JSON 语法校验通过

---

## T5: Scheduler 跳过休眠区域 (R6, R7, R8, R9)

**文件**：
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- Scheduler 新增 `ZoneManager *zone.ZoneManager` 字段（已有，需 import zone 包）
- 第一遍和第二遍遍历中，ZoneManager != nil 时检查 IsActive，休眠则 skip
- ZoneManager 为 nil 时全部活跃（v2 兼容）

---

## T6: 区域级广播 — Conn.ZoneID + enter_zone/leave_zone (R10, R11, R12)

**文件**：
- `internal/gateway/conn.go`
- `pkg/protocol/message.go`
- `internal/gateway/handler.go`

**做完了是什么样**：
- Conn struct 新增 `ZoneID string` 字段
- protocol 新增 `TypeEnterZone`/`TypeLeaveZone` 常量 + `EnterZoneRequest{ZoneID}`
- enter_zone handler 设 conn.ZoneID，leave_zone 清空
- RegisterHandlers 注册两个新 handler

---

## T7: Hub.BroadcastByZone + broadcastLoop 适配 (R11)

**文件**：
- `internal/gateway/hub.go`
- `cmd/server/main.go`

**做完了是什么样**：
- Hub 新增 `BroadcastByZone(zoneSnapshots map[string][]byte)` 方法：遍历连接按 ZoneID 筛选
- conn.ZoneID 为空时发全部（v2 兼容）
- broadcastLoop 改为构建 zone→snapshot 映射，调用 BroadcastByZone

---

## T8: main.go 初始化 + 启动流程 (R15)

**文件**：
- `cmd/server/main.go`

**做完了是什么样**：
- 创建 ZoneManager，加载 configs/regions/ 所有配置
- 每个区域调用 Spawn 批量创建 NPC
- 传 ZoneManager 给 Scheduler
- 无区域配置时日志警告，保持全局行为

---

## T9: 集成测试 + 现有测试验证 (R15, R16, R17, R18)

**文件**：
- `internal/runtime/zone_integration_test.go`

**做完了是什么样**：
- 测试 1 休眠跳过：休眠区域 NPC 不 Tick（位置不变、threat_level 不变）
- 测试 2 唤醒恢复：唤醒后 NPC 恢复 Tick
- 测试 3 批量生成：从配置加载区域 → Spawn → 验证 NPC 数量和 zone_id
- 现有集成测试 + e2e 全部通过
- `go test ./...` 全绿

---

## 执行顺序

```
T1  Zone + ZoneManager
 └→ T2  Config Source
     └→ T3  Zone.Spawn
         └→ T4  示例配置
             └→ T5  Scheduler 休眠跳过
                 └→ T6  Conn.ZoneID + enter/leave
                     └→ T7  BroadcastByZone
                         └→ T8  main.go 初始化
                             └→ T9  集成测试
```

严格顺序。
