# component-architecture 任务拆解

## T1: Component/Tickable 接口 + 组件注册表 (R1, R2) [x]

**文件**：
- `internal/runtime/component/component.go`
- `internal/runtime/component/registry.go`
- `internal/runtime/component/registry_test.go`

**做完了是什么样**：
- `Component` 接口定义 `Name() string`
- `Tickable` 接口嵌入 `Component`，增加 `Tick(bb *blackboard.Blackboard, dt float64)`
- `Registry` 支持 `Register(name, factory)` / `Create(name, raw) (Component, error)`
- 测试覆盖：注册+创建、未知组件报错、重复注册 panic

---

## T2: identity + position + personality 组件 (R4) [x]

**文件**：
- `internal/runtime/component/identity.go`
- `internal/runtime/component/position.go`
- `internal/runtime/component/personality.go`

**做完了是什么样**：
- 三个 struct 字段与 Schema 精确一致
- 实现 `Component` 接口（Name 返回 `"identity"` / `"position"` / `"personality"`）
- 不实现 Tickable
- 各自导出工厂函数供注册表使用
- PositionComponent 提供 `ToVec3() event.Vec3` 辅助方法

---

## T3: perception + social 组件 (R4) [x]

**文件**：
- `internal/runtime/component/perception.go`
- `internal/runtime/component/social.go`

**做完了是什么样**：
- 两个 struct 字段与 Schema 精确一致（perception 含 AttentionCapacity，social 含 GroupID/Faction/Role/FollowTarget）
- 实现 `Component` 接口，不实现 Tickable
- 各自导出工厂函数

---

## T4: behavior 组件 (R4, R5) [x]

**文件**：
- `internal/runtime/component/behavior.go`
- `internal/runtime/component/behavior_test.go`

**做完了是什么样**：
- BehaviorComponent 包含 `FSMRef string` + `BTRefs map[string]string`（JSON 字段）和 `FSM *fsm.FSM` + `BTrees map[string]bt.Node`（运行时字段，`json:"-"`）
- 实现 `Component` 接口，不实现 Tickable
- 工厂函数只解析 JSON 填 FSMRef/BTRefs，FSM/BTrees 为 nil
- 导出 `BuildRuntime(src, btReg) error` 方法：加载 FSM 配置 + 构建 BT 树，填入运行时字段
- 测试：工厂 JSON 解析、BuildRuntime 从测试配置加载

---

## T5: movement 组件 — Tickable (R4, R6, R7) [x]

**文件**：
- `internal/runtime/component/movement.go`
- `internal/runtime/component/movement_test.go`

**做完了是什么样**：
- MovementComponent 字段与 Schema 一致（MoveType/MoveSpeed/WanderRadius/PatrolWaypoints）
- 实现 Tickable，Tick 写 BB `move_state = "idle"`（最小实现，真实移动在需求 5）
- 测试：工厂解析、Tick 写 BB 验证

---

## T6: needs 组件 — Tickable (R4, R6, R7) [x]

**文件**：
- `internal/runtime/component/needs.go`
- `internal/runtime/component/needs_test.go`

**做完了是什么样**：
- NeedsComponent 含 NeedTypes 数组（每项 Name/Current/Max/DecayRate）
- 实现 Tickable，Tick 中每项 Current -= DecayRate * dt（clamp 0），找最低需求写 BB `need_lowest` + `need_lowest_val`
- 测试：衰减计算、最低需求选取、BB 写入

---

## T7: emotion 组件 — Tickable (R4, R6, R7) [x]

**文件**：
- `internal/runtime/component/emotion.go`
- `internal/runtime/component/emotion_test.go`

**做完了是什么样**：
- EmotionComponent 含 EmotionStates 数组（每项 Name/Value/AccumulateRate/DecayRate）
- 实现 Tickable，Tick 中每项 Value -= DecayRate * dt（clamp 0），找最高情绪写 BB `emotion_dominant` + `emotion_dominant_val`
- 测试：衰减计算、主导情绪选取、BB 写入

---

## T8: memory 组件 — Tickable (R4, R6, R7) [x]

**文件**：
- `internal/runtime/component/memory.go`
- `internal/runtime/component/memory_test.go`

**做完了是什么样**：
- MemoryComponent 含 Capacity/MemoryTypes/DecayTime
- 实现 Tickable，Tick 写 BB `memory_count = 0`（本 spec 无条目，深入实现在需求 4）
- 测试：工厂解析、Tick 写 BB

---

## T9: DefaultRegistry 注册全部 10 个组件 (R2) [x]

**文件**：
- `internal/runtime/component/defaults.go`
- `internal/runtime/component/defaults_test.go`

**做完了是什么样**：
- `DefaultRegistry()` 返回注册了全部 10 个组件工厂的 Registry
- 测试：用每个组件名 + 合法 JSON 调用 Create，确认返回正确类型

---

## T10: BB Key 新增 (R19) [x]

**文件**：
- `internal/core/blackboard/keys.go`

**做完了是什么样**：
- 新增 8 个 Key：`need_lowest`(string)、`need_lowest_val`(float64)、`emotion_dominant`(string)、`emotion_dominant_val`(float64)、`memory_count`(int64)、`group_id`(string)、`social_role`(string)、`move_state`(string)
- 现有 Schema 测试 `TestBlackboardKeysConsistency` 仍通过（Key 名与 Schema 一致）

---

## T11: Instance 重构为组件容器 (R3, R8, R9) [x]

**文件**：
- `internal/runtime/npc/instance.go`
- `internal/runtime/npc/template.go`

**做完了是什么样**：
- Instance struct 改为：`ID` + `Name` + `BB` + `components map[string]Component` + `tickables []Tickable`
- 导出 `HasComponent(name)` / `RawComponent(name)` / `TickComponents(dt)` / `Position() Vec3`
- 导出泛型函数 `GetComponent[T](inst, name) (T, bool)`
- `template.go` 定义 `TemplateConfig` struct + `NewInstanceFromTemplate` 工厂
- 创建时校验 identity/position 必须存在
- 保留旧的 `NPCTypeConfig` / `ParseNPCTypeConfig` / `NewInstance` 不删（兼容期）

---

## T12: v2 旧格式兼容 (R10, R22) [x]

**文件**：
- `internal/runtime/npc/compat.go`
- `internal/runtime/npc/compat_test.go`

**做完了是什么样**：
- `ParseNPCTemplate(data, compReg) (*TemplateConfig, error)` 自动检测格式（`components` → 新格式，`type_name` → 旧格式）
- `convertV2Format(data) (*TemplateConfig, error)` 将旧格式转组件化
- 测试：civilian/guard/police 三个旧配置加载 → 转换 → 验证 behavior/perception 组件正确

---

## T13: Config Source 扩展 (R14, R15, R16) [x]

**文件**：
- `internal/config/source.go`
- `internal/config/json_source.go`
- `internal/config/http_source.go`

**做完了是什么样**：
- Source 接口新增 `LoadNPCTemplate(name string) ([]byte, error)`
- JSONSource 从 `configs/npc_templates/<name>.json` 加载
- HTTPSource 启动时额外拉取 `/api/v1/npc-templates/export`
- 旧方法 `LoadNPCTypeConfig` 保留不改

---

## T14: Scheduler 适配 (R11, R12, R13) [x]

**文件**：
- `internal/runtime/scheduler.go`

**做完了是什么样**：
- Tick 中按组件有无分支：有 perception → filterPerception，有 behavior → Decision + FSM + BT
- 新增 `filterPerception` 方法：从 PerceptionComponent 提取字段构造 PerceptionConfig
- 每个 NPC 末尾调用 `inst.TickComponents(dt)`
- perception 包本身不改

---

## T15: Gateway handler 适配 (R17, R18) [x]

**文件**：
- `internal/gateway/handler.go`

**做完了是什么样**：
- `RegisterHandlers` 签名新增 `compReg *component.Registry` 参数
- spawn_npc：先 LoadNPCTemplate 再降级 LoadNPCTypeConfig → ParseNPCTemplate → NewInstanceFromTemplate
- query_npc：从组件安全读取（无 behavior → fsm_state 为空字符串）

---

## T16: main.go + broadcastLoop 适配 (R18) [x]

**文件**：
- `cmd/server/main.go`

**做完了是什么样**：
- 创建 `component.DefaultRegistry()`，传给 RegisterHandlers
- `buildSnapshot` 从 `inst.Position()` 读坐标，从 behavior 组件读 FSM 状态

---

## T17: 示例配置文件 (R8) [x]

**文件**：
- `configs/npc_templates/butterfly_01.json`
- `configs/npc_templates/wolf_common.json`

**做完了是什么样**：
- butterfly_01：simple 级，只有 identity + position + movement（wander 模式）
- wolf_common：reactive 级，有 identity + position + behavior + perception + movement + personality

---

## T18: 集成测试 + v2 兼容验证 (R23, R24) [x]

**文件**：
- `internal/runtime/integration_test.go`
- `internal/runtime/component_integration_test.go`

**做完了是什么样**：
- 现有 6 个集成测试场景用旧配置路径全部通过（R24）
- 新增测试：simple NPC 不走 AI 管线验证、reactive NPC 完整管线验证、混合 Tick 验证
- integration_test.go 的 NPC 创建路径适配（可能需要小幅修改以兼容新 Instance 结构）

---

## T19: Benchmark (R25) [x]

**文件**：
- `internal/runtime/benchmark_test.go`

**做完了是什么样**：
- `BenchmarkTick_SimpleNPC`：identity+position+movement，单 Tick < 1μs
- `BenchmarkTick_FullNPC`：全 10 组件，单 Tick < 100μs
- 现有 benchmark 适配新 Instance 创建方式

---

## T20: e2e 测试验证 (R24) [x]

**文件**：
- `test/e2e/gateway_test.go`
- `test/e2e/extension_test.go`

**做完了是什么样**：
- `go test ./test/e2e/... -v` 全部通过
- 如果 e2e 测试依赖 `inst.FSM` 等旧字段，做最小修改适配（通过 WS 协议交互的测试应该不需要改）

---

## 执行顺序

```
T1  组件接口+注册表
 ├→ T2  identity/position/personality
 ├→ T3  perception/social
 ├→ T4  behavior
 ├→ T5  movement (Tickable)
 ├→ T6  needs (Tickable)
 ├→ T7  emotion (Tickable)
 └→ T8  memory (Tickable)
      └→ T9  DefaultRegistry
          └→ T10 BB Key 新增
              └→ T11 Instance 重构
                  └→ T12 v2 兼容
                      └→ T13 Config Source
                          └→ T14 Scheduler 适配
                              └→ T15 Gateway handler
                                  └→ T16 main.go
                                      └→ T17 示例配置
                                          └→ T18 集成测试
                                              └→ T19 Benchmark
                                                  └→ T20 e2e 验证
```

T2-T8 可并行（独立组件，无互相依赖）。T9 之后严格顺序。
