# runtime-layer 任务拆解

## 依赖关系

```
T1 Event(事件定义+总线) → T3 Perception(依赖Event类型)  → T5 Decision(依赖Event+Perception+BB)
T2 Config扩展(依赖Event+NPC类型)                        → T6 NPC实例(依赖Config+Core全部+Event)
                                                          T7 Scheduler(依赖全部runtime包)
                                                          T8 集成测试(依赖全部)
T4 BB Keys + stub_action(独立)                          ↗
```

---

## 任务列表

### [x] T1: 事件定义与事件总线 (R1, R2, R3)

**产出**：Event 结构体 + EventTypeConfig + Bus（发布/TTL 衰减/过期清理）

**文件**：
- `internal/runtime/event/event.go` — Vec3、Event、EventTypeConfig、Bus struct、Publish/Tick/Active
- `internal/runtime/event/event_test.go` — 测试

**做完是什么样**：
- `Bus.Publish(evt)` 将事件加入活跃列表
- `Bus.Tick(dt)` 递减所有事件 TTL，TTL <= 0 的事件被移除
- `Bus.Active()` 返回当前活跃事件的只读快照
- 并发 Publish 安全（RWMutex）
- EventTypeConfig 可从 JSON 反序列化

---

### [x] T2: Config 扩展 — 事件和 NPC 类型加载 (R3, R11, R17)

**产出**：Source 接口新增方法 + JSONSource 实现 + 示例配置文件

**文件**：
- `internal/config/source.go` — Source 接口新增 LoadEventConfig/LoadAllEventConfigs/LoadNPCTypeConfig
- `internal/config/json_source.go` — JSONSource 实现新方法
- `internal/config/json_source_test.go` — 新方法测试

**做完是什么样**：
- `LoadEventConfig("explosion")` 读取 `configs/events/explosion.json` 返回 `[]byte`
- `LoadAllEventConfigs()` 遍历 `configs/events/` 目录返回所有配置
- `LoadNPCTypeConfig("civilian")` 读取 `configs/npc_types/civilian.json` 返回 `[]byte`
- 文件不存在/JSON 格式错误返回明确 error

---

### [x] T3: 感知过滤 (R4, R5)

**产出**：CanPerceive 纯函数 + Distance 工具函数

**文件**：
- `internal/runtime/perception/perception.go` — PerceptionConfig、CanPerceive、Distance
- `internal/runtime/perception/perception_test.go` — 测试

**做完是什么样**：
- `CanPerceive(npcPos, cfg, evt, evtTypeCfg)` 根据 perception_mode 和距离判断可感知性
- `"global"` 模式：无论距离都返回 true
- `"visual"` 模式：XZ 距离 <= min(visual_range, event.range) 返回 true
- `"auditory"` 模式：XZ 距离 <= min(auditory_range, event.range) 返回 true
- 边界值正确（距离刚好等于 range → true，超出 → false）

---

### [x] T4: Blackboard Keys 扩展 + stub_action 节点 (R10, R12)

**产出**：Runtime 层需要的 BB Key + BT stub 节点

**文件**：
- `internal/core/blackboard/keys.go` — 追加 KeyNPCType、KeyNPCPosX、KeyNPCPosZ
- `internal/core/bt/leaves.go` — 追加 stub_action 节点工厂
- `internal/core/bt/registry.go` — DefaultRegistry 中注册 stub_action

**做完是什么样**：
- `blackboard.KeyNPCType`、`KeyNPCPosX`、`KeyNPCPosZ` 可用
- `stub_action` 节点从 params 读取 `result` 字段，返回对应 Status（"success"/"failure"/"running"）
- `go test ./internal/core/...` 仍然全部通过

---

### [x] T5: 决策中心 (R6, R7, R8, R9)

**产出**：威胁评估 + 优先级仲裁 + 威胁衰减

**文件**：
- `internal/runtime/decision/decision.go` — Center、Evaluate、威胁计算、仲裁、衰减
- `internal/runtime/decision/decision_test.go` — 测试

**做完是什么样**：
- `Center.Evaluate(bb, npcPos, events, evtTypes)` 计算所有事件的 threat 值
- 威胁公式：`threat = severity × max(0, 1 - distance/range)`
- 多事件取最高 threat 写入 BB（threat_level, threat_source, last_event_type, threat_expire_at）
- 无可感知事件时 threat_level 按 decayRate × dt 衰减到 0
- 高威胁事件到达时覆写 BB（事件抢占验证）

---

### [x] T6: NPC 实例与注册表 (R10, R11, R12)

**产出**：Instance + NewInstance 工厂 + Registry

**文件**：
- `internal/runtime/npc/instance.go` — Instance struct、NewInstance 工厂、Tick
- `internal/runtime/npc/registry.go` — Registry（Add/Remove/Get/ForEach）
- `internal/runtime/npc/npc_test.go` — 测试

**做完是什么样**：
- `NewInstance(id, pos, typeCfg, src, btReg)` 从配置创建完整 NPC 实例（BB + FSM + BT 全部初始化）
- `Instance.Tick()` 执行 FSM.Tick → 获取当前状态 BT → BT.Tick
- `Registry.Add/Remove/Get/ForEach` 管理实例生命周期
- 并发 Add/Remove 安全（RWMutex）

---

### [x] T7: Scheduler + 配置文件 (R13, R14, R15, R16)

**产出**：Tick 调度器 + 所有 JSON 配置文件

**文件**：
- `internal/runtime/scheduler.go` — Scheduler struct、Tick、Run
- `configs/events/explosion.json` — 爆炸事件配置
- `configs/events/gunshot.json` — 枪声事件配置

**注意**：超过 3 个文件，配置文件单独拆出。

**做完是什么样**：
- `Scheduler.Tick(dt)` 执行完整链路：事件衰减 → 遍历 NPC → 感知过滤 → 决策评估 → NPC Tick
- 3 个事件配置文件可被 Config 加载
- `BenchmarkTick_100NPCs` 延迟 < 10ms

---

### [x] T7b: 剩余配置文件 (R3, R11, R15, R16)

**产出**：NPC 类型配置 + BT 树配置（stub）

**文件**：
- `configs/events/shout.json` — 呼叫事件配置
- `configs/npc_types/civilian.json` — 平民 NPC 类型配置
- `configs/bt_trees/civilian_idle.json` — 平民空闲 BT（stub）

**做完是什么样**：
- `LoadNPCTypeConfig("civilian")` 返回完整配置
- civilian 配置引用的 FSM (`civilian`) 和 BT 树 (`civilian_idle` 等) 文件都存在
- JSON 格式合法，可反序列化

---

### [x] T7c: 剩余 BT 配置文件 (R11)

**产出**：剩余状态的 BT 树配置

**文件**：
- `configs/bt_trees/civilian_alarmed.json` — 平民警戒 BT（stub）
- `configs/bt_trees/civilian_flee.json` — 平民逃跑 BT（stub）

**做完是什么样**：
- civilian NPC 类型配置中 bt_refs 引用的所有 BT 树文件都存在
- JSON 格式合法，bt.BuildFromJSON 可成功构建

---

### [x] T8: 集成测试 + Benchmark (R14, R18, R19)

**产出**：跨模块集成测试 + 性能 benchmark

**文件**：
- `internal/runtime/integration_test.go` — 场景 1-5 集成测试
- `internal/runtime/benchmark_test.go` — 100 NPC Tick benchmark

**做完是什么样**：
- `go test ./...` 全部通过（runtime 层 + core 层）
- 场景 1：平民遇爆炸 → Idle→Alarmed→Flee 完整链路
- 场景 2：事件过期 → threat 衰减 → Flee→Idle
- 场景 3：多事件同时到达 → 响应最高威胁
- 场景 4：低威胁行为中收到高威胁 → 状态打断转换
- 场景 5：新增事件类型配置 → 自动响应
- 场景 6（Runtime×Core 联调）：完整链路验证——JSON 配置加载 → NPC 工厂创建（BB+FSM+BT）→ 事件发布 → 感知过滤 → 决策中心写 BB → FSM 转换 → BT 读 BB 执行 → 验证 core 层的 Rule.Evaluate 在 Runtime 调度下正确求值
- Benchmark：100 NPC 单 Tick < 10ms
